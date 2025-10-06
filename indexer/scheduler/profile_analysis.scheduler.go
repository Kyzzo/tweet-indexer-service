package scheduler

import (
	"context"
	"time"

	"github.com/bsm/redislock"
	"github.com/jeetdotso/tweet-indexer-service/config"
	"github.com/jeetdotso/tweet-indexer-service/dummyaccount"
	"github.com/jeetdotso/tweet-indexer-service/indexer/service"
	"github.com/jeetdotso/tweet-indexer-service/logg"
	redis2 "github.com/jeetdotso/tweet-indexer-service/memory/redis"
	"github.com/jeetdotso/tweet-indexer-service/persistence/mongodb"
	"github.com/jeetdotso/tweet-indexer-service/scheduler"
	"github.com/redis/go-redis/v9"
)

const ProfileAnalysis = "profile-retry-lock-sync-minute-job"

type ProfileAnalysisImpl struct {
	svc    service.FollowingIndexer
	locker *redislock.Client
}

func NewProfileAnalysis(cfg *config.Config, conn *mongodb.MongoConnection, dummySvc dummyaccount.Service) scheduler.Scheduler {
	locker := redislock.New(redis.NewClient(redis2.GetRedisOptions(cfg.Memory)))
	var (
		svc = service.NewFollowingIndexer(*cfg, conn, dummySvc)
	)
	return &ProfileAnalysisImpl{
		svc:    svc,
		locker: locker,
	}
}

func (s *ProfileAnalysisImpl) Execute() {
	ctx := context.Background()
	lock, err := s.locker.Obtain(ctx, ProfileAnalysis, 5*time.Minute, &redislock.Options{
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
	logg.Log.Info().Caller().Ctx(ctx).Msg("Obtained lock minute profile analysis sync up")

	defer lock.Release(ctx)
	if err := s.svc.GetEmptyTagsAndRunGrok(ctx); err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Failed to get empty tags and run grok.")
		return
	}
}

func (c *ProfileAnalysisImpl) Expression() string {
	return "@every 30m"
}
