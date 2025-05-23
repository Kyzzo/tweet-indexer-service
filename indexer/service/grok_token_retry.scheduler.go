package service

import (
	"context"
	"fmt"
	"sync"

	"github.com/jeetdotso/tweet-indexer-service/config"
	"github.com/jeetdotso/tweet-indexer-service/dummyaccount"
	"github.com/jeetdotso/tweet-indexer-service/indexer/model"
	"github.com/jeetdotso/tweet-indexer-service/indexer/repository"
	"github.com/jeetdotso/tweet-indexer-service/logg"
	"github.com/jeetdotso/tweet-indexer-service/persistence/mongodb"
	"github.com/jeetdotso/tweet-indexer-service/scheduler"
)

const GrokTokenRetryKey = "grok--token-retry-lock-sync-minute-job"

type GrokTokenRetryImpl struct {
	svc       GrokIndexer
	tweetRepo repository.TweetEventRepository
}

const (
	TOKEN_SCHEDULER_THREADS = 20
)

func NewGrokTokenRetryScheduler(cfg *config.Config, conn *mongodb.MongoConnection, dummySvc dummyaccount.Service, saasIdx SaasIndexer) scheduler.Scheduler {
	var (
		svc       = NewGrokIndexer(*cfg, conn, dummySvc, saasIdx)
		tweetRepo = repository.NewTweetIndexRepo(conn)
	)
	return &GrokTokenRetryImpl{
		svc:       svc,
		tweetRepo: tweetRepo,
	}
}

func (s *GrokTokenRetryImpl) Execute() {
	ctx := context.Background()
	twts, err := s.tweetRepo.FindAllNoneTokenGrok(ctx)
	if err != nil {
		logg.Log.Error().Err(err).Msg("Error finding all none grok tweets")
		return
	}
	if len(twts) == 0 {
		return
	}
	fmt.Println("Length of tweets for grok: ", len(twts))
	sem := make(chan struct{}, min(TOKEN_SCHEDULER_THREADS, len(twts)))
	var wg sync.WaitGroup

	for _, t := range twts {
		wg.Add(1)
		sem <- struct{}{}
		go func(twt model.TweetEvent) {
			defer wg.Done()
			defer func() { <-sem }()

			done := make(chan struct{})
			s.svc.Start(ctx, &twt, done) // Pass the done channel
			<-done                       // Wait for completion

		}(t)
	}

	wg.Wait()
}

func (c *GrokTokenRetryImpl) Expression() string {
	return "@every 1m"
}
