package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/jeetdotso/tweet-indexer-service/config"
	"github.com/jeetdotso/tweet-indexer-service/dummyaccount"
	"github.com/jeetdotso/tweet-indexer-service/indexer/model"
	"github.com/jeetdotso/tweet-indexer-service/indexer/repository"
	"github.com/jeetdotso/tweet-indexer-service/logg"
	"github.com/jeetdotso/tweet-indexer-service/persistence/mongodb"
	"github.com/jeetdotso/tweet-indexer-service/scheduler"
)

const GrokRetryKey = "grok-retry-lock-sync-minute-job"

type TotalStatsImpl struct {
	svc       GrokIndexer
	tweetRepo repository.TweetEventRepository
}

const (
	SCHEDULER_THREADS = 5
)

func NewGrokRetryScheduler(cfg *config.Config, conn *mongodb.MongoConnection, dummySvc dummyaccount.Service) scheduler.Scheduler {
	var (
		svc       = NewGrokIndexer(*cfg, conn, dummySvc)
		tweetRepo = repository.NewTweetIndexRepo(conn)
	)
	return &TotalStatsImpl{
		svc:       svc,
		tweetRepo: tweetRepo,
	}
}

func (s *TotalStatsImpl) Execute() {
	ctx := context.Background()
	twts, err := s.tweetRepo.FindAllNoneGrok(ctx)
	if err != nil {
		logg.Log.Error().Err(err).Msg("Error finding all none grok tweets")
		return
	}
	if len(twts) == 0 {
		return
	}
	fmt.Println("Length of tweets for grok: ", len(twts))
	sem := make(chan struct{}, min(SCHEDULER_THREADS, len(twts)))
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

			time.Sleep(2 * time.Second)
		}(t)
	}

	wg.Wait()
}

func (c *TotalStatsImpl) Expression() string {
	return "@every 1m"
}
