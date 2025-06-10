package tracking

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/tidwall/gjson"

	cache "github.com/jeetdotso/tweet-indexer-service/cache"
	dummyaccount "github.com/jeetdotso/tweet-indexer-service/dummyaccount"
	"github.com/jeetdotso/tweet-indexer-service/logg"
	twitterAPI "github.com/jeetdotso/tweet-indexer-service/scrape/internals/api"
	twitterTypes "github.com/jeetdotso/tweet-indexer-service/scrape/internals/api/types"
	tweetParsers "github.com/jeetdotso/tweet-indexer-service/scrape/internals/parsers/tweet"
	userParsers "github.com/jeetdotso/tweet-indexer-service/scrape/internals/parsers/user"
	"github.com/jeetdotso/tweet-indexer-service/scrape/proxy"
	types "github.com/jeetdotso/tweet-indexer-service/scrape/tracking/types"
)

type AccountMonitor struct {
	TweetsCache map[string]*types.Tweet
	twitterAPI  *twitterAPI.TwitterClient
	svc         dummyaccount.Service
	selAccount  *dummyaccount.SelAccount
	maxRetries  int
	retryDelay  time.Duration
}

func NewTweetFromLegacy(username string, legacy *gjson.Result) *types.Tweet {
	conversationID := legacy.Get("conversation_id_str").String()
	return &types.Tweet{
		URL:               fmt.Sprintf("https://x.com/%s/status/%s", username, conversationID),
		ConversationIDStr: conversationID,
		CreatedAt:         legacy.Get("created_at").String(),
		FullText:          legacy.Get("full_text").String(),
		UserIDStr:         legacy.Get("user_id_str").String(),
	}
}

func NewAccountMonitor(ctx context.Context, svc dummyaccount.Service) (*AccountMonitor, *dummyaccount.SelAccount, error) {
	// Fetch the next available Selenium account
	selAccount, err := svc.LogInToNextGroup(ctx, dummyaccount.WORKING_GROUP)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error retrieving Selenium account.")
		return nil, nil, err
	}
	px := proxy.NextProxy()

	// Create Twitter API client using the cookies from the Selenium account
	twitterAPIClient, _ := twitterAPI.CreateTwitterClientWithCookies(selAccount.GetCookiesHttp(), px)
	acc := &AccountMonitor{
		TweetsCache: cache.TweetsCache,
		twitterAPI:  twitterAPIClient,
		svc:         svc,
		selAccount:  selAccount,
		maxRetries:  8,
		retryDelay:  2 * time.Second,
	}
	return acc, selAccount, nil
}
func (am *AccountMonitor) ExtractTweetFromResponse(response *gjson.Result) *[]gjson.Result {
	instructions := gjson.Parse(response.Get("data.user.result.timeline_v2.timeline.instructions").String()).Array()
	if len(instructions) <= 2 {
		return nil
	}
	return &instructions
}

func (am *AccountMonitor) GetUser(ctx context.Context, username string) (*twitterTypes.User, error) {
	r, err := am.twitterAPI.FullUser(username)
	if err != nil {
		return nil, err
	}
	return userParsers.ParseUserFromGJSON(r)
}

func (am *AccountMonitor) MonitorTweets(ctx context.Context, username string, handleTweet func(ctx context.Context, twt *twitterTypes.TweetInfo)) {
	UserID, _ := cache.GetUserID(username)
	r, _ := am.twitterAPI.AccountTweets(UserID, "")
	if r == nil {
		logg.Log.Error().
			Caller().
			Ctx(ctx).
			Str("username", username).
			Msg("failed to get account tweets")
		return
	}

	instructions := am.ExtractTweetFromResponse(r)
	if instructions == nil {
		return
	}
	for _, instruction := range *instructions {
		if instruction.Get("type").String() != "TimelineAddEntries" {
			continue
		}
		entries := instruction.Get("entries").Array()
		if len(entries) == 0 {
			continue
		}
		for _, entry := range entries {
			twt := tweetParsers.ParseTweetFromEntry(&entry, UserID, username)
			if twt == nil {

				continue
			}
			// hand it off to your handler
			handleTweet(ctx, twt)
		}

		break
	}
}

func (am *AccountMonitor) fetchTweetWithRotation(
	ctx context.Context,
	statusID, username string,
) (*gjson.Result, error) {
	var lastErr error
	prev := am.selAccount

	for attempt := 1; attempt <= am.maxRetries; attempt++ {
		if res, err := am.twitterAPI.GetTweet(statusID, username); err == nil {
			return res, nil
		} else {
			lastErr = err
			logg.Log.
				Warn().
				Caller().
				Ctx(ctx).
				Str("tweetID", statusID).
				Str("account", prev.UserName).
				Int("attempt", attempt).
				Err(err).
				Msg("GetTweet failed, rotating account")
		}

		// log out old
		if err := am.svc.LogOut(ctx, prev.UserName); err != nil {
			logg.Log.Warn().
				Caller().
				Ctx(ctx).
				Str("account", prev.UserName).
				Err(err).
				Msg("failed to logout old account")
		}

		// grab next
		next, err := am.svc.LogInToNext(ctx)
		if err != nil {
			logg.Log.Error().Caller().Ctx(ctx).Err(err).
				Msg("couldn't login next account, aborting rotation")
			break
		}
		px := proxy.NextProxy()
		client, err := twitterAPI.CreateTwitterClientWithCookies(next.GetCookiesHttp(), px)
		if err != nil {
			logg.Log.Error().Caller().Ctx(ctx).Err(err).
				Msg("couldn't create client for rotated account")
			break
		}
		am.selAccount = next
		am.twitterAPI = client
		prev = next

		time.Sleep(am.retryDelay)
	}

	return nil, fmt.Errorf(
		"after %d attempts, unable to fetch tweet %s: last error: %w",
		am.maxRetries, statusID, lastErr,
	)
}

func (am *AccountMonitor) ProcessTweet(
	ctx context.Context,
	username, trackedUserID string,
	tweet *types.Tweet,
	handleTweet func(ctx context.Context, twt *twitterTypes.TweetInfo),
) {
	last := cache.GetTweet(username)
	if last == nil || last.ConversationIDStr != tweet.ConversationIDStr {
		cache.UpdateTweet(username, tweet)

		// <- here is the only line we swap out:
		r, err := am.fetchTweetWithRotation(ctx, tweet.ConversationIDStr, username)
		if err != nil {
			logg.Log.Error().
				Caller().
				Ctx(ctx).
				Str("username", username).
				Str("tweetID", tweet.ConversationIDStr).
				Err(err).
				Msg("giving up on this tweet")
			return
		}

		parsed := tweetParsers.ParseTweet(r, tweet.ConversationIDStr, trackedUserID)
		handleTweet(ctx, parsed)
	} else {
		logg.Log.Debug().Msgf("no new tweets for %s", username)
	}
}
func MonitorTweetsMultipleAccounts(ctx context.Context, usernames []string, maxWorkers int, svc dummyaccount.Service, handleTweet func(ctx context.Context, twt *twitterTypes.TweetInfo)) {
	var wg sync.WaitGroup
	sem := make(chan struct{}, maxWorkers)

	for _, username := range usernames {
		wg.Add(1)
		sem <- struct{}{}

		go func(username string) {
			// 1) Get AccountMonitor and selAccount up front.
			monitor, _, err := NewAccountMonitor(ctx, svc)
			if err != nil {
				logg.Log.Error().
					Caller().
					Ctx(ctx).
					Err(err).
					Msg("Failed to get account monitor.")
				wg.Done()
				<-sem
				return
			}

			// 2) selAccount, defer its logout.
			defer func() {
				if err := svc.LogOut(ctx, monitor.selAccount.UserName); err != nil {
					logg.Log.Error().
						Caller().
						Ctx(ctx).
						Err(err).
						Msg("Failed to log out selenium account.")
				}
			}()

			// 3) Still defer wg.Done and semaphore release.
			defer wg.Done()
			defer func() { <-sem }()

			// 4) Start the monitor.
			monitor.MonitorTweets(ctx, username, handleTweet)

		}(username)

		time.Sleep(1 * time.Second)
	}
	wg.Wait()

}

func RetweetById(ctx context.Context, tweetID string, svc dummyaccount.Service) error {
	retries := 0
	maxRetries := 3
	retryDelay := 5 * time.Second
	for {

		if retries >= maxRetries {
			logg.Log.Error().Caller().Ctx(ctx).Msgf("Max retries reached for retweeting tweet with ID: %s", tweetID)
			return fmt.Errorf("max retries reached for retweeting tweet with ID: %s", tweetID)
		}
		selAccount, err := svc.LogInToNextGroup(ctx, dummyaccount.TOKEN_GROUP)
		if err != nil {
			logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error retrieving Selenium account.")
			return err
		}
		px := proxy.NextProxy()
		// Create Twitter API client using the cookies from the Selenium account
		twitterAPIClient, _ := twitterAPI.CreateTwitterClientWithCookies(selAccount.GetCookiesHttp(), px)

		success, err := twitterAPIClient.Retweet(tweetID)
		if err != nil {
			logg.Log.Error().Caller().Ctx(ctx).Err(err).Msgf("Error retweeting tweet with ID: %s", tweetID)

		}
		_ = svc.LogOut(ctx, selAccount.UserName)
		if success {
			break
		}
		retries++
		logg.Log.Info().Caller().Ctx(ctx).Msgf("Retrying retweeting tweet with ID: %s", tweetID)
		time.Sleep(retryDelay)
	}
	return nil
}
