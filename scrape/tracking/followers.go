package tracking

import (
	"context"
	"sync"
	"time"

	cache "github.com/jeetdotso/tweet-indexer-service/cache"
	"github.com/jeetdotso/tweet-indexer-service/logg"
	twitterAPI "github.com/jeetdotso/tweet-indexer-service/scrape/internals/api"
)

func ScrapeFollowers(ctx context.Context, username string, maxRetries int, retryDelay time.Duration) error {
	userID, _ := cache.GetUserID(username)
	client, _ := twitterAPI.CreateTwitterClient()
	ScrapePaginatedData(
		ctx,
		username,
		userID,
		client.Followers,
		"followers",
		maxRetries,
		retryDelay,
	)
	return nil
}

func ScrapeFollowersWithMultithreaded(ctx context.Context, usernames []string, threadsNum int, maxRetries int, retryDelay time.Duration) {
	var wg sync.WaitGroup
	sem := make(chan struct{}, threadsNum)

	for _, username := range usernames {
		wg.Add(1)
		sem <- struct{}{}

		go func(username string) {
			defer wg.Done()
			defer func() { <-sem }()
			if err := ScrapeFollowers(ctx, username, maxRetries, retryDelay); err != nil {
				logg.Log.Error().Caller().Ctx(ctx).Err(err).Msgf("Error scraping %s", username)
			}
		}(username)
	}
	wg.Wait()
}
