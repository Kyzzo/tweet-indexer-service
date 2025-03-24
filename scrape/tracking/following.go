package tracking

import (
	"context"
	"sync"
	"time"

	cache "github.com/jeetdotso/tweet-indexer-service/cache"
	"github.com/jeetdotso/tweet-indexer-service/logg"
	twitterAPI "github.com/jeetdotso/tweet-indexer-service/scrape/internals/api"
)

func ScrapeFollowing(ctx context.Context, username string, maxRetries int, retryDelay time.Duration) error {
	client, _ := twitterAPI.CreateTwitterClient()
	userID, _ := cache.GetUserID(username)
	ScrapePaginatedData(
		ctx,
		username,
		userID,
		client.Following,
		"following",
		maxRetries,
		retryDelay,
	)
	return nil
}

func ScrapeFollowingWithMultithreaded(ctx context.Context, usernames []string, threadsNum int, maxRetries int, retryDelay time.Duration) {
	var wg sync.WaitGroup
	sem := make(chan struct{}, threadsNum)

	for _, username := range usernames {
		wg.Add(1)
		sem <- struct{}{}

		go func(username string) {
			defer wg.Done()
			defer func() { <-sem }()
			if err := ScrapeFollowing(ctx, username, maxRetries, retryDelay); err != nil {
				logg.Log.Error().Caller().Ctx(ctx).Err(err).Msgf("Error scraping %s", username)
			}
		}(username)
	}
	wg.Wait()
}
