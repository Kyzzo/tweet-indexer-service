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
)

type TweetsRollBack struct {
	twitterAPI *twitterAPI.TwitterClient
	maxRetries int
	retryDelay time.Duration
}

func NewTweetsRollBack(ctx context.Context, maxRetries int, retryDelay time.Duration, svc dummyaccount.Service) (*TweetsRollBack, *dummyaccount.SelAccount, error) {
	selAccount, err := svc.LogInToNext(ctx)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error retrieving Selenium account.")
		return nil, nil, err
	}
	twitterAPIClient, err := twitterAPI.CreateTwitterClientWithCookies(selAccount.GetCookiesHttp())
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error creating Twitter client.")
		return nil, nil, err
	}
	tr := &TweetsRollBack{
		twitterAPI: twitterAPIClient,
		maxRetries: maxRetries,
		retryDelay: retryDelay,
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

func (trb *TweetsRollBack) Tweets(ctx context.Context, username string, targetDate time.Time) (*[]twitterTypes.TweetInfo, error) {
	if username == "" {
		return nil, fmt.Errorf("username and targetDate are required")
	}

	UserID, err := cache.GetUserID(username)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Str("username", username).Msg("Error fetching UserID")
		return nil, fmt.Errorf("failed to get UserID: %w", err)
	}

	tweets, err := trb.collectTweets(ctx, username, UserID, targetDate)
	if err != nil {
		return tweets, fmt.Errorf("error collecting tweets: %w", err)
	}

	return tweets, nil
}

func TweetsMultithreaded(ctx context.Context, users map[string]time.Time, threadsNum int, maxRetries int, retryDelay time.Duration, svc dummyaccount.Service, handleTweet func(ctx context.Context, twt *twitterTypes.TweetInfo)) {

	var wg sync.WaitGroup
	sem := make(chan struct{}, threadsNum)

	for username, date := range users {
		wg.Add(1)
		sem <- struct{}{}
		go func(username string, date time.Time) {
			defer wg.Done()
			defer func() { <-sem }()
			trb, selAcc, err := NewTweetsRollBack(ctx, maxRetries, retryDelay, svc) //avoid rate limiting by creating new client for each user
			if err != nil {
				logg.Log.Error().Caller().Ctx(ctx).Err(err).Str("username", username).Msg("Error creating TweetsRollBack")
				return
			}
			defer func() {
				_ = svc.LogOut(ctx, selAcc.UserName)
			}()
			tweets, err := trb.Tweets(ctx, username, date)
			if err != nil {
				logg.Log.Error().Caller().Ctx(ctx).Err(err).Str("username", username).Msg("Error rollback tweets for user")
				return
			}
			for _, tweet := range *tweets {
				handleTweet(ctx, &tweet)
			}
		}(username, date)
	}
	wg.Wait()
}

func (trb *TweetsRollBack) collectTweets(ctx context.Context, username, UserID string, limitDate time.Time) (*[]twitterTypes.TweetInfo, error) {
	var tweets []twitterTypes.TweetInfo
	cursor := ""
	dateReached := false

	for {
		r, err := trb.fetchTweetsWithRetry(ctx, UserID, cursor)
		if err != nil {
			logg.Log.Error().Caller().Ctx(ctx).Err(err).Str("username", username).Msg("Error fetching tweets")
			return &tweets, err
		}

		instructions := r.Get("data.user.result.timeline_v2.timeline.instructions").Array()
		if len(instructions) == 0 {
			logg.Log.Info().Caller().Ctx(ctx).Str("username", username).Msg("No instructions found")
			break
		}

		entries := instructions[len(instructions)-1].Get("entries").Array()
		if len(entries) < 2 {
			break
		}

		if dateReached = trb.processTweetEntries(ctx, username, &tweets, entries, limitDate); dateReached {
			break
		}

		newCursor := entries[len(entries)-2].Get("content.value").String()
		if !trb.isValidCursor(newCursor) || len(entries) == 2 {
			logg.Log.Info().Caller().Ctx(ctx).Str("username", username).Str("cursor", newCursor).Bool("dateReached", dateReached).Msg("Reached end condition")
			break
		}
		cursor = newCursor
	}

	return &tweets, nil
}

func (trb *TweetsRollBack) processTweetEntries(ctx context.Context, username string, tweets *[]twitterTypes.TweetInfo, entries []gjson.Result, limitDate time.Time) bool {
	for _, entry := range entries {
		statusID := trb.extractStatusID(entry)
		if statusID == "" {
			continue
		}

		statusrR, err := trb.twitterAPI.GetTweet(statusID)
		if err != nil {
			logg.Log.Error().Caller().Ctx(ctx).Err(err).Str("username", username).Str("statusID", statusID).Msg("Error fetching tweet")
			continue
		}

		statusParsed := tweetParsers.ParseTweet(statusrR, statusID)
		tweetTime, err := time.Parse("Mon Jan 02 15:04:05 -0700 2006", statusParsed.CreatedAt)
		if err != nil {
			logg.Log.Error().Caller().Ctx(ctx).Err(err).Str("username", username).Str("statusID", statusID).Msg("Error parsing tweet date")
			continue
		}

		if limitDate.After(tweetTime) {
			return true
		}
		*tweets = append(*tweets, *statusParsed)
	}
	return false
}

func (trb *TweetsRollBack) extractStatusID(entry gjson.Result) string {
	statusID := entry.Get("content.itemContent.tweet_results.result.rest_id").String()
	if statusID == "" {
		statusID = entry.Get("content.items.0.item.itemContent.tweet_results.result.rest_id").String()
	}
	return statusID
}

func (trb *TweetsRollBack) isValidCursor(cursor string) bool {
	return cursor != "" && cursor[0:2] != "0|"
}
