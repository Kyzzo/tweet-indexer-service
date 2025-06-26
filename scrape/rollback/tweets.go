package rollback

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/tidwall/gjson"

	cache "github.com/jeetdotso/tweet-indexer-service/cache"
	"github.com/jeetdotso/tweet-indexer-service/dummyaccount"
	"github.com/jeetdotso/tweet-indexer-service/logg"
	twitterAPI "github.com/jeetdotso/tweet-indexer-service/scrape/internals/api"
	twitterTypes "github.com/jeetdotso/tweet-indexer-service/scrape/internals/api/types"
	tweetParsers "github.com/jeetdotso/tweet-indexer-service/scrape/internals/parsers/tweet"
	"github.com/jeetdotso/tweet-indexer-service/scrape/proxy"
	"github.com/jeetdotso/tweet-indexer-service/twitterprofile"
)

type TweetsRollBack struct {
	twitterAPI *twitterAPI.TwitterClient
	svc        dummyaccount.Service
	selAccount *dummyaccount.SelAccount
	maxRetries int
	retryDelay time.Duration
}

func NewTweetsRollBack(ctx context.Context, maxRetries int, retryDelay time.Duration, svc dummyaccount.Service) (*TweetsRollBack, *dummyaccount.SelAccount, error) {
	selAccount, err := svc.LogInToNextGroup(ctx, dummyaccount.WORKING_GROUP)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error retrieving Selenium account.")
		return nil, nil, err
	}
	px := proxy.NextProxy()
	twitterAPIClient, err := twitterAPI.CreateTwitterClientWithCookies(selAccount.GetCookiesHttp(), px)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error creating Twitter client.")
		return nil, nil, err
	}
	tr := &TweetsRollBack{
		twitterAPI: twitterAPIClient,
		maxRetries: maxRetries,
		retryDelay: retryDelay,
		svc:        svc,
		selAccount: selAccount,
	}

	return tr, selAccount, nil
}

func (trb *TweetsRollBack) fetchTweetsWithRetry(ctx context.Context, userID string, cursor string) (*gjson.Result, error) {
	for attempt := 1; attempt <= trb.maxRetries; attempt++ {
		r, err := trb.twitterAPI.AccountTweets(userID, cursor)
		if err == nil {
			return r, nil
		}
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Str("userID", userID).Str("cursor", cursor).Msg("Error fetching tweets")
		if attempt < trb.maxRetries {
			time.Sleep(trb.retryDelay)
			continue
		}
		return nil, fmt.Errorf("max retries reached: %v", err)
	}
	return nil, fmt.Errorf("failed to fetch tweets after %d attempts", trb.maxRetries)
}

func (trb *TweetsRollBack) Tweets(ctx context.Context, username string, targetDate time.Time) ([]*twitterTypes.TweetInfo, error) {
	if username == "" {
		return nil, fmt.Errorf("username and targetDate are required")
	}

	UserID, err := cache.GetUserID(username)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Str("username", username).Msg("Error fetching UserID")
		return nil, fmt.Errorf("failed to get UserID: %w", err)
	}
	if UserID == "" {
		logg.Log.Error().Caller().Ctx(ctx).Str("username", username).Msg("UserID is empty")
		return nil, fmt.Errorf("UserID for username %s is empty", username)
	}

	tweets, err := trb.collectTweets(ctx, username, UserID, targetDate)
	if err != nil {
		return tweets, fmt.Errorf("error collecting tweets: %w", err)
	}

	return tweets, nil
}

func TweetsPipeline(
	ctx context.Context,
	tag twitterprofile.TagType,
	users map[string]time.Time,
	fetcherCount int, // max concurrent fetchers
	handlerCount int, // max concurrent handleTweet calls
	maxRetries int,
	retryDelay time.Duration,
	svc dummyaccount.Service,
	handleTweet func(ctx context.Context, tag twitterprofile.TagType, twt *twitterTypes.TweetInfo),
	syncLatestToCheckPoint func(ctx context.Context, user twitterprofile.TwitterProfile) time.Time,
) {
	type userJob struct {
		username string
		since    time.Time
	}
	type tweetJob struct {
		username string
		tweet    *twitterTypes.TweetInfo
	}

	userJobs := make(chan userJob)
	tweetJobs := make(chan tweetJob)

	var wgFetchers sync.WaitGroup
	var wgHandlers sync.WaitGroup

	// —— 1) Fetch-worker pool ——
	wgFetchers.Add(fetcherCount)
	for i := 0; i < fetcherCount; i++ {
		go func() {
			// defer wgFetchers.Done()
			for job := range userJobs {
				trb, selAcc, err := NewTweetsRollBack(ctx, maxRetries, retryDelay, svc)
				if err != nil {
					logg.Log.Error().Err(err).Str("username", job.username).Msg("rollback init failed")
					continue
				}

				// make a little cleanup func
				cleanup := func() {
					if err := svc.LogOut(ctx, selAcc.UserName); err != nil {
						logg.Log.Error().
							Err(err).
							Str("username", selAcc.UserName).
							Msg("logout failed")
					}
				}

				tweets, err := trb.Tweets(ctx, job.username, job.since)
				if err != nil {
					logg.Log.Error().Err(err).Str("username", job.username).Msg("fetch tweets failed")
					cleanup()
					continue
				}
				if len(tweets) == 0 {
					cleanup()
					continue
				}

				for i := len(tweets) - 1; i >= 0; i-- {
					tweetJobs <- tweetJob{username: job.username, tweet: tweets[i]}
				}

				cleanup()
			}
			wgFetchers.Done()
		}()
	}

	// —— 2) Handler-worker pool ——
	wgHandlers.Add(handlerCount)
	for i := 0; i < handlerCount; i++ {
		go func() {
			defer wgHandlers.Done()
			for tj := range tweetJobs {
				if tj.tweet == nil {
					logg.Log.Error().
						Caller().
						Ctx(ctx).
						Str("username", tj.username).
						Msg("Received nil tweet in handler")
					continue
				}
				handleTweet(ctx, tag, tj.tweet)
			}
		}()
	}

	// —— 3) Producer: enqueue user jobs ——
	go func() {
		for u, since := range users {
			userJobs <- userJob{username: u, since: since}
		}
		close(userJobs)
	}()

	// —— 4) Wire up shutdown: when all fetchers done, close tweetJobs ——
	go func() {
		wgFetchers.Wait()
		close(tweetJobs)
	}()

	// —— 5) Wait for all handlers to finish ——
	wgHandlers.Wait()
}

func (trb *TweetsRollBack) fetchTweetWithRotation(
	ctx context.Context,
	statusID, username string,
) (*gjson.Result, error) {
	var lastErr error

	// We will re‐use this account until it fails, then swap it out:
	for attempt := 0; attempt <= trb.maxRetries; attempt++ {
		if res, err := trb.twitterAPI.GetTweet(statusID, username); err == nil {
			return res, nil
		} else {

			lastErr = err
		}
		logg.Log.
			Warn().
			Caller().
			Ctx(ctx).
			Err(lastErr).
			Str("statusID", statusID).
			Str("account", trb.selAccount.UserName).
			Int("attempt", attempt).
			Msg("GetTweet failed — rotating account")

		err := trb.SwitchAccounts(ctx)
		if err != nil {
			logg.Log.
				Error().
				Caller().
				Ctx(ctx).
				Err(err).
				Str("username", username).
				Str("statusID", statusID).
				Msg("Error switching accounts")
			return nil, err
		}

		// 5) (optional) back off before retrying
		time.Sleep(trb.retryDelay)
	}

	return nil, fmt.Errorf(
		"after %d attempts, failed to fetch tweet %s: last error: %w",
		trb.maxRetries+1, statusID, lastErr,
	)
}

func (trb *TweetsRollBack) processTweetEntries(
	ctx context.Context,
	username, trackedUser string,
	tweets *[]*twitterTypes.TweetInfo,
	entries []gjson.Result,
	limitDate time.Time,
) bool {
	var times []time.Time
	for _, entry := range entries {
		statusID := trb.extractStatusID(entry)
		if statusID == "" {
			continue
		}
		et := entry.Get("content.entryType").String()
		switch et {
		case "TimelineTimelineItem":
			if ti := tweetParsers.ParseTweetFromEntry(&entry, trackedUser, username); ti != nil {
				var ts time.Time
				ts, err := time.Parse(time.RubyDate, ti.CreatedAt)
				if err != nil {
					logg.Log.Error().
						Caller().
						Ctx(ctx).
						Err(err).
						Str("statusID", statusID).
						Str("createdAt", ti.CreatedAt).
						Msg("Error parsing tweet date")
					ts = time.Time{}
					*tweets = append(*tweets, ti)
					continue
				}
				times = append(times, ts)
				if limitDate.After(ts) {
					if ti.TweetType == twitterTypes.Tweet || ti.TweetType == twitterTypes.Quote {
						return true
					}
					continue
				}

				*tweets = append(*tweets, ti)
			} else {
				logg.Log.Error().
					Caller().
					Ctx(ctx).
					Str("statusID", statusID).
					Msg("Parsed tweet is nil")
				continue
			}
		case "TimelineTimelineModule":
			for _, mod := range entry.Get("content.items").Array() {
				item := mod.Get("item")
				if ti := tweetParsers.ParseTweetFromEntry(&item, trackedUser, username); ti != nil {

					if ti == nil {
						logg.Log.Error().
							Caller().
							Ctx(ctx).
							Str("statusID", statusID).
							Msg("Parsed tweet is nil")
						continue
					}
					var ts time.Time
					ts, err := time.Parse(time.RubyDate, ti.CreatedAt)
					if err != nil {
						logg.Log.Error().
							Caller().
							Ctx(ctx).
							Err(err).
							Str("username", username).
							Str("createdAt", ti.CreatedAt).
							Str("tweet_id", ti.ID).
							Str("statusID", statusID).
							Msg("Error parsing tweet date")
						ts = time.Time{}
						*tweets = append(*tweets, ti)
						continue
					}

					times = append(times, ts)
					if limitDate.After(ts) {
						if ti.TweetType == twitterTypes.Tweet || ti.TweetType == twitterTypes.Quote {
							return true
						}
						continue
					}
					*tweets = append(*tweets, ti)
				} else {
					logg.Log.Error().
						Caller().
						Ctx(ctx).
						Str("statusID", statusID).
						Msg("Parsed tweet is nil")
					continue
				}
			}
		default:
			fmt.Println("Unknown entry: ", entry)
			fmt.Println("Unknown entry type: ", et)

		}

	}
	for _, t := range times {
		if limitDate.After(t) {
			return true
		}
	}
	return false
}

func (trb *TweetsRollBack) collectTweets(ctx context.Context, username, UserID string, limitDate time.Time) ([]*twitterTypes.TweetInfo, error) {
	var tweets []*twitterTypes.TweetInfo
	cursor := ""
	dateReached := false

	for {
		r, err := trb.fetchTweetsWithRetry(ctx, UserID, cursor)
		if err != nil {
			logg.Log.Error().Caller().Ctx(ctx).Err(err).Str("username", username).Msg("Error fetching tweets")
			return tweets, err
		}

		instructions := r.Get("data.user.result.timeline_v2.timeline.instructions").Array()
		if len(instructions) == 0 {
			break
		}

		entries := instructions[len(instructions)-1].Get("entries").Array()
		if len(entries) < 2 {
			break
		}

		if dateReached = trb.processTweetEntries(ctx, username, UserID, &tweets, entries, limitDate); dateReached {
			// logg.Log.Info().Caller().Ctx(ctx).Str("username", username).Bool("is_dateReached", dateReached).Msg("Reached end condition")
			break
		}

		newCursor := entries[len(entries)-1].Get("content.value").String()
		if !trb.isValidCursor(newCursor) || len(entries) == 2 {
			logg.Log.Info().Caller().Ctx(ctx).Str("username", username).Str("cursor", newCursor).Bool("is_dateReached", dateReached).Msg("Reached end condition")
			break
		}
		cursor = newCursor
	}

	return tweets, nil
}

func (trb *TweetsRollBack) extractStatusID(entry gjson.Result) string {
	// 1) the normal path
	if id := entry.Get("content.itemContent.tweet_results.result.rest_id"); id.Exists() {
		return id.String()
	}
	// 2) module-wrapper: first child
	if id := entry.Get("content.items.0.item.itemContent.tweet_results.result.rest_id"); id.Exists() {
		return id.String()
	}
	// 3) passed the `item` itself
	if id := entry.Get("itemContent.tweet_results.result.rest_id"); id.Exists() {
		return id.String()
	}
	// 4) TweetWithVisibilityResults path
	if id := entry.Get("content.itemContent.tweet_results.result.tweet.rest_id"); id.Exists() {
		return id.String()
	}
	// and likewise in the module-wrapper context:
	if id := entry.Get("content.items.0.item.itemContent.tweet_results.result.tweet.rest_id"); id.Exists() {
		return id.String()
	}
	return ""
}

func (trb *TweetsRollBack) isValidCursor(cursor string) bool {
	return cursor != "" && cursor[0:2] != "0|"
}

func (trb *TweetsRollBack) SwitchAccounts(ctx context.Context) error {
	svc := trb.svc
	if err := trb.svc.LogOut(ctx, trb.selAccount.UserName); err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Str("username", trb.selAccount.UserName).Msg("Error logging out Selenium account")
		return err
	}
	selAcc, err := trb.svc.LogInToNext(ctx)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error logging in to next Selenium account")
		return err
	}
	px := proxy.NextProxy()

	twitterAPIClient, err := twitterAPI.CreateTwitterClientWithCookies(selAcc.GetCookiesHttp(), px)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error creating Twitter client.")
		selErr := trb.svc.LogOut(ctx, selAcc.UserName)

		if selErr != nil {
			logg.Log.Error().Caller().Ctx(ctx).Err(selErr).Msg("Error logging out Selenium account")
		}
		return err
	}
	trb.svc = svc
	trb.selAccount = selAcc
	trb.twitterAPI = twitterAPIClient
	return nil
}
