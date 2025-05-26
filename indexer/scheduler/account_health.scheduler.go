package scheduler

import (
	"context"
	"time"

	"github.com/bsm/redislock"
	"github.com/jeetdotso/tweet-indexer-service/config"
	"github.com/jeetdotso/tweet-indexer-service/cycle"
	"github.com/jeetdotso/tweet-indexer-service/dummyaccount"
	"github.com/jeetdotso/tweet-indexer-service/logg"
	redis2 "github.com/jeetdotso/tweet-indexer-service/memory/redis"
	"github.com/jeetdotso/tweet-indexer-service/persistence/mongodb"
	"github.com/jeetdotso/tweet-indexer-service/scheduler"
	"github.com/jeetdotso/tweet-indexer-service/scrape/accounts_health"
	"github.com/redis/go-redis/v9"
)

type HealthCheck struct {
	dummySvc    dummyaccount.Service
	locker      *redislock.Client
	healthCycle cycle.Service
}

const (
	ACCOUNT_SCHEDULER_THREADS = 10
	HealthRetryKey            = "health-retry-lock-sync-daily-job"
)

func NewAccountHealthScheduler(cfg *config.Config, conn *mongodb.MongoConnection, dummySvc dummyaccount.Service) scheduler.Scheduler {
	locker := redislock.New(redis.NewClient(redis2.GetRedisOptions(cfg.Memory)))
	healthCycle := cycle.NewCycleService(conn, cfg)
	return &HealthCheck{
		dummySvc:    dummySvc,
		locker:      locker,
		healthCycle: healthCycle,
	}
}

func (s *HealthCheck) Execute() {

	ctx := context.Background()
	lock, err := s.locker.Obtain(ctx, HealthRetryKey, 15*time.Minute, &redislock.Options{
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
	logg.Log.Info().Caller().Ctx(ctx).Msg("Obtained lock Daily account health sync up")

	defer lock.Release(ctx)
	hc, err := s.healthCycle.FindLatest(ctx)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error finding latest health cycle.")
		return
	}
	if hc != nil && hc.StartTime.Add(23*time.Hour).After(time.Now()) {
		logg.Log.Info().Caller().Ctx(ctx).Msg("Health cycle already exists for today, skipping.")
		return
	}
	sels, err := s.dummySvc.FindAllByGroup(ctx, dummyaccount.WORKING_GROUP)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error finding all dummy accounts for health check.")
		return
	}
	newCycle := cycle.NewHealthCycle(len(sels))
	accounts_health.Check(ctx, ACCOUNT_SCHEDULER_THREADS, s.dummySvc, dummyaccount.WORKING_GROUP)
	sels, err = s.dummySvc.FindAllByGroup(ctx, dummyaccount.WORKING_GROUP)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error finding all dummy accounts for health check.")
		return
	}
	newCycle.EndCycle(len(sels))
	err = s.healthCycle.UpdateOrInsertHealth(ctx, newCycle)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error updating or inserting health cycle.")
		return
	}
}

func (c *HealthCheck) Expression() string {
	return "@every 24h"
}
