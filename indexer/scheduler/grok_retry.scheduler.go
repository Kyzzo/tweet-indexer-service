package scheduler

import (
	"context"
	"sync"
	"time"

	"github.com/bsm/redislock"
	"github.com/jeetdotso/tweet-indexer-service/config"
	"github.com/jeetdotso/tweet-indexer-service/dummyaccount"
	"github.com/jeetdotso/tweet-indexer-service/indexer/model"
	"github.com/jeetdotso/tweet-indexer-service/indexer/repository"
	"github.com/jeetdotso/tweet-indexer-service/indexer/service"
	"github.com/jeetdotso/tweet-indexer-service/logg"
	redis2 "github.com/jeetdotso/tweet-indexer-service/memory/redis"
	"github.com/jeetdotso/tweet-indexer-service/persistence/mongodb"
	"github.com/jeetdotso/tweet-indexer-service/scheduler"
	"github.com/redis/go-redis/v9"
)

const GrokRetryKey = "grok-retry-lock-sync-minute-job"

type GrokRetyImpl struct {
	svc       service.GrokIndexer
	tweetRepo repository.TweetEventRepository
	locker    *redislock.Client
}

const (
	SCHEDULER_THREADS = 30
)

func NewGrokRetryScheduler(cfg *config.Config, conn *mongodb.MongoConnection, dummySvc dummyaccount.Service, saasIdx service.SaasIndexer) scheduler.Scheduler {
	locker := redislock.New(redis.NewClient(redis2.GetRedisOptions(cfg.Memory)))
	var (
		svc       = service.NewGrokIndexer(*cfg, conn, dummySvc, saasIdx)
		tweetRepo = repository.NewTweetIndexRepo(conn)
	)
	return &GrokRetyImpl{
		svc:       svc,
		locker:    locker,
		tweetRepo: tweetRepo,
	}
}

func (s *GrokRetyImpl) Execute() {
	ctx := context.Background()
	lock, err := s.locker.Obtain(ctx, GrokRetryKey, 30*time.Minute, &redislock.Options{
		// give it up to half a second to get the lock
		RetryStrategy: redislock.LimitRetry(redislock.LinearBackoff(50*time.Millisecond), 10),
	})
	if err == redislock.ErrNotObtained {
		logg.Log.Warn().Caller().Ctx(ctx).Err(err).Msg("Job lock busy...")
		return
	} else if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error acquiring lock lb.")
		return
	}
	logg.Log.Info().Caller().Ctx(ctx).Msg("Obtained lock Daily investments sync up")

	defer lock.Release(ctx)
	twts, err := s.tweetRepo.FindAllNoneGrok(ctx)
	if err != nil {
		logg.Log.Error().Err(err).Msg("Error finding all none grok tweets")
		return
	}
	if len(twts) == 0 {
		return
	}
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

			time.Sleep(1 * time.Second)
		}(t)
	}

	wg.Wait()
}

func (c *GrokRetyImpl) Expression() string {
	return "@every 1m"
}
