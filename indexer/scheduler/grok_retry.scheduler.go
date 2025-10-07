package scheduler

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bsm/redislock"
	"github.com/jeetdotso/tweet-indexer-service/config"
	"github.com/jeetdotso/tweet-indexer-service/cycle"
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
	scr       cycle.Service
	locker    *redislock.Client
}

const (
	SCHEDULER_THREADS = 20
	LOCK_TTL          = 10 * time.Minute
	HEARTBEAT_EVERY   = 3 * time.Minute
	SOFT_STOP_AFTER   = 8 * time.Minute // < LOCK_TTL
	MAX_PER_RUN       = 50              // cap items per run
)

func NewGrokRetryScheduler(cfg *config.Config, conn *mongodb.MongoConnection, dummySvc dummyaccount.Service, saasIdx service.SaasIndexer) scheduler.Scheduler {
	locker := redislock.New(redis.NewClient(redis2.GetRedisOptions(cfg.Memory)))
	var (
		svc       = service.NewGrokIndexer(*cfg, conn, dummySvc, saasIdx)
		tweetRepo = repository.NewTweetIndexRepo(conn)
		scr       = cycle.NewCycleService(conn, cfg)
	)
	return &GrokRetyImpl{
		svc:       svc,
		locker:    locker,
		tweetRepo: tweetRepo,
		scr:       scr,
	}
}

func (s *GrokRetyImpl) Execute() {
	// parent ctx is used for repo ops / control signals, NOT for killing tasks
	ctrlCtx := context.Background()

	lock, err := s.locker.Obtain(ctrlCtx, GrokRetryKey, LOCK_TTL, &redislock.Options{
		RetryStrategy: redislock.LimitRetry(redislock.LinearBackoff(100*time.Millisecond), 20),
	})
	if err == redislock.ErrNotObtained {
		logg.Log.Warn().Msg("Grok retry: lock busy")
		return
	} else if err != nil {
		logg.Log.Error().Err(err).Msg("Grok retry: lock error")
		return
	}
	logg.Log.Info().Msg("Grok retry: obtained lock")

	// ---- heartbeat (soft-stop on failure) ----
	var stopNew atomic.Bool // when true, stop enqueueing new work
	hbDone := make(chan struct{})
	go func() {
		t := time.NewTicker(HEARTBEAT_EVERY)
		defer t.Stop()
		for {
			select {
			case <-t.C:
				if err := lock.Refresh(ctrlCtx, LOCK_TTL, nil); err != nil {
					logg.Log.Error().Err(err).Msg("lock refresh failed; soft-stopping enqueue")
					stopNew.Store(true) // don't cancel tasks; just stop adding more
					return
				}
			case <-hbDone:
				return
			}
		}
	}()
	defer func() {
		close(hbDone)
		if err := lock.Release(context.Background()); err != nil {
			logg.Log.Warn().Err(err).Msg("release failed (maybe expired)")
		}
	}()

	softStopDeadline := time.Now().Add(SOFT_STOP_AFTER)

	// fetch a bounded batch (add nextAttemptAt <= now in your repo)
	twts, err := s.tweetRepo.FindAllNoneGrok(ctrlCtx) // implement limit + filters
	if err != nil {
		logg.Log.Error().Err(err).Msg("find candidates failed")
		return
	}
	if len(twts) == 0 {
		logg.Log.Info().Msg("no tweets to retry")
		return
	}
	logg.Log.Info().Int("count", len(twts)).Msg("processing retry batch")
	//add grok errors
	if err := s.scr.AddGrokError(ctrlCtx, len(twts)); err != nil {
		logg.Log.Error().Caller().Ctx(ctrlCtx).Err(err).Msg("Error adding grok error to scrape cycle")
	}
	twts = twts[:min(len(twts), MAX_PER_RUN)]

	jobs := make(chan model.TweetEvent, SCHEDULER_THREADS*2)
	var wg sync.WaitGroup
	for i := 0; i < SCHEDULER_THREADS; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for twt := range jobs {
				// independent per-task timeout; NOT tied to heartbeat/lock
				taskCtx, cancelTask := context.WithTimeout(context.Background(), 2*time.Minute)
				done := make(chan struct{})
				s.svc.Start(taskCtx, &twt, done)
				select {
				case <-done:
				case <-taskCtx.Done():
					// mark transient failure / schedule backoff here if needed
				}
				cancelTask()
			}
		}()
	}
	started := 0
OUTER:
	for _, t := range twts {
		// obey soft stop or lost-lock signal
		if stopNew.Load() || time.Now().After(softStopDeadline) {
			logg.Log.Warn().Msg("soft stop reached; not enqueuing more jobs this run")
			break OUTER
		}

		// (Optional) per-item SETNX guard to avoid dupe processing across pods/runs
		// ok, _ := s.redis.SetNX(ctrlCtx, fmt.Sprintf("grok:processing:%s", t.ID), "1", 30*time.Minute).Result()
		// if !ok { continue }

		select {
		case jobs <- t: // blocking send = natural backpressure
			started++
		default:
			// if you want a bounded offer instead of blocking:
			// but blocking is usually better to avoid bursty floods
			jobs <- t
		}
	}
	close(jobs)
	wg.Wait()
	logg.Log.Info().Int("started", started).Msg("grok retry run finished")
}

func (c *GrokRetyImpl) Expression() string {
	return "@every 1m"
}
