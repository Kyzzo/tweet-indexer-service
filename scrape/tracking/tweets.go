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
	types "github.com/jeetdotso/tweet-indexer-service/scrape/tracking/types"
)

type AccountMonitor struct {
	TweetsCache map[string]*types.Tweet
	twitterAPI  *twitterAPI.TwitterClient
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
	selAccount, err := svc.LogInToNext(ctx)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error retrieving Selenium account.")
		return nil, nil, err
	}

	// Create Twitter API client using the cookies from the Selenium account
	twitterAPIClient, _ := twitterAPI.CreateTwitterClientWithCookies(selAccount.GetCookiesHttp())
	acc := &AccountMonitor{
		TweetsCache: cache.TweetsCache,
		twitterAPI:  twitterAPIClient,
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

func (am *AccountMonitor) MonitorTweets(ctx context.Context, username string, handleTweet func(ctx context.Context, twt *twitterTypes.TweetInfo)) {
	UserID, _ := cache.GetUserID(username)
	r, _ := am.twitterAPI.AccountTweets(UserID, "")
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
		legacy := entries[0].Get("content.itemContent.tweet_results.result.legacy")
		if !legacy.Exists() {
			legacy = entries[0].Get("content.items.0.item.itemContent.tweet_results.result.legacy")
		}
		tweet := NewTweetFromLegacy(username, &legacy)
		am.ProcessTweet(ctx, username, tweet, handleTweet)
		break
	}
}

func (am *AccountMonitor) ProcessTweet(ctx context.Context, username string, tweet *types.Tweet, handleTweet func(ctx context.Context, twt *twitterTypes.TweetInfo)) {
	if cache.GetTweet(username) == nil || cache.GetTweet(username).ConversationIDStr != tweet.ConversationIDStr {
		cache.UpdateTweet(username, tweet)
		r, _ := am.twitterAPI.GetTweet(tweet.ConversationIDStr)
		tweetParsed := tweetParsers.ParseTweet(r, tweet.ConversationIDStr)
		handleTweet(ctx, tweetParsed)
	} else {
		logg.Log.Debug().Msgf("no new tweets found for user: [%s]", username)
	}
}

func MonitorTweetsMultipleAccounts(ctx context.Context, usernames []string, maxWorkers int, svc dummyaccount.Service, handleTweet func(ctx context.Context, twt *twitterTypes.TweetInfo)) {
	var wg sync.WaitGroup
	sem := make(chan struct{}, maxWorkers)

	for _, username := range usernames {
		wg.Add(1)
		sem <- struct{}{}

		go func(ctx context.Context, username string) {
			defer wg.Done()
			defer func() { <-sem }()

			// Fetch a Selenium account
			monitor, selAccount, err := NewAccountMonitor(ctx, svc)
			if err != nil {
				logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Failed to get account monitor.")
				return
			}
			defer func() {
				_ = svc.LogOut(ctx, selAccount.UserName)
			}()

			// Start monitoring tweets
			monitor.MonitorTweets(ctx, username, handleTweet)
		}(ctx, username)

		time.Sleep(50 * time.Millisecond)
	}
	// Wait for all workers to finish this round before looping again
	wg.Wait()

}
