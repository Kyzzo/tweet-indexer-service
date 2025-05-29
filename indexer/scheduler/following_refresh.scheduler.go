package scheduler

import (
	"context"
	"time"

	"github.com/bsm/redislock"
	"github.com/jeetdotso/tweet-indexer-service/config"
	"github.com/jeetdotso/tweet-indexer-service/cycle"
	"github.com/jeetdotso/tweet-indexer-service/dummyaccount"
	"github.com/jeetdotso/tweet-indexer-service/indexer/repository"
	"github.com/jeetdotso/tweet-indexer-service/indexer/service"
	"github.com/jeetdotso/tweet-indexer-service/logg"
	redis2 "github.com/jeetdotso/tweet-indexer-service/memory/redis"
	"github.com/jeetdotso/tweet-indexer-service/persistence/mongodb"
	"github.com/jeetdotso/tweet-indexer-service/scheduler"
	"github.com/jeetdotso/tweet-indexer-service/twitterprofile"
	"github.com/redis/go-redis/v9"
)

type FollowingCycle struct {
	locker      *redislock.Client
	followCycle cycle.Service
	dummSvc     dummyaccount.Service
	follSvc     service.FollowingIndexer
	twtPrf      twitterprofile.Repository
	follRepo    repository.FollowingRepository
}

const (
	FollowingCycleKey = "following-cycle-lock-sync-daily-job"
)

func NewFollowingCycleScheduler(cfg *config.Config, conn *mongodb.MongoConnection, dummSvc dummyaccount.Service) scheduler.Scheduler {
	locker := redislock.New(redis.NewClient(redis2.GetRedisOptions(cfg.Memory)))
	followCycle := cycle.NewCycleService(conn, cfg)
	followSvc := service.NewFollowingIndexer(*cfg, conn, dummSvc)
	twtPrf := twitterprofile.NewRepository(conn)
	follRepo := repository.NewFollowingRepository(conn)
	return &FollowingCycle{
		locker:      locker,
		followCycle: followCycle,
		dummSvc:     dummSvc,
		follSvc:     followSvc,
		twtPrf:      twtPrf,
		follRepo:    follRepo,
	}
}

func (s *FollowingCycle) Execute() {
	ctx := context.Background()

	// Acquire distributed lock
	lock, err := s.locker.Obtain(ctx, FollowingCycleKey, 1*time.Hour, &redislock.Options{
		RetryStrategy: redislock.LimitRetry(redislock.LinearBackoff(50*time.Millisecond), 10),
	})
	if err == redislock.ErrNotObtained {
		logg.Log.Warn().Caller().Ctx(ctx).Msg("Job lock busy - another pod is handling the cycle")
		return
	} else if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error acquiring lock")
		return
	}
	defer func() {
		if err := lock.Release(ctx); err != nil {
			logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error releasing lock")
		}
	}()

	// Check monthly cycle first
	monthCycle, err := s.followCycle.FindFollowingLatest(ctx, cycle.MonthlyUpdate)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error finding latest monthly cycle")
		return
	}

	// Determine which cycle to run
	switch {
	// Case 1: Monthly cycle is due or doesn't exist
	case monthCycle == nil || time.Now().After(monthCycle.StartTime.AddDate(0, 1, 0).Add(-4*time.Hour)):
		if monthCycle == nil {
			// First time running - create new monthly cycle
			monthCycle, err = s.followCycle.CreateNewFollowingCycle(ctx, cycle.MonthlyUpdate)
			if err != nil {
				logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error creating new monthly cycle")
				return
			}
		} else if monthCycle.EndTime.IsZero() {
			// Previous monthly cycle didn't complete - retry
			logg.Log.Warn().Caller().Ctx(ctx).Msg("Previous monthly cycle didn't complete - retrying")
		}
		s.monthly(ctx, monthCycle)

	// Case 2: Daily cycle check
	default:
		dailyCycle, err := s.followCycle.FindFollowingLatest(ctx, cycle.DailyUpdate)
		if err != nil {
			logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error finding latest daily cycle")
			return
		}

		// Run daily if: no existing cycle OR last daily was >20 hours ago
		if dailyCycle == nil || time.Now().After(dailyCycle.StartTime.Add(20*time.Hour)) {
			if dailyCycle == nil {
				dailyCycle, err = s.followCycle.CreateNewFollowingCycle(ctx, cycle.DailyUpdate)
				if err != nil {
					logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error creating new daily cycle")
					return
				}
			}
			s.daily(ctx, dailyCycle)
		} else {
			logg.Log.Info().Caller().Ctx(ctx).
				Str("last_daily", dailyCycle.StartTime.String()).
				Msg("Daily cycle ran recently - skipping")
		}
	}
}

func (s *FollowingCycle) monthly(ctx context.Context, mc *cycle.FollowingCycle) {
	// Execute the monthly scrape
	if err := s.follSvc.ScrapeFollowingList(ctx); err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Monthly scrape failed")
		mc.Status = cycle.Failed
		mc.EndTime = time.Now()
		_ = s.followCycle.UpdateOrInsertFollowing(ctx, mc)
		return
	}

	// Update metrics
	if err := s.updateCycle(ctx, mc); err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Failed to update monthly cycle metrics")
		mc.Status = cycle.Failed
		mc.EndTime = time.Now()
		_ = s.followCycle.UpdateOrInsertFollowing(ctx, mc)
		return
	}

	dailyCycle, err := s.followCycle.FindFollowingLatest(ctx, cycle.DailyUpdate)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error finding latest daily cycle")
		return
	}

	// Run daily if: no existing cycle OR last daily was >20 hours ago
	if dailyCycle == nil || time.Now().After(dailyCycle.StartTime.Add(20*time.Hour)) {
		if dailyCycle == nil {
			dailyCycle, err = s.followCycle.CreateNewFollowingCycle(ctx, cycle.DailyUpdate)
			if err != nil {
				logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error creating new daily cycle")
				return
			}
		}
		if err = s.updateCycle(ctx, dailyCycle); err != nil {
			logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Failed to update daily cycle metrics after monthly")
			dailyCycle.Status = cycle.Failed
			dailyCycle.EndTime = time.Now()
			_ = s.followCycle.UpdateOrInsertFollowing(ctx, dailyCycle)
			return
		}
	}
}

func (s *FollowingCycle) daily(ctx context.Context, dc *cycle.FollowingCycle) {
	// Execute the daily scrape
	if err := s.follSvc.ScrapeFollowingListBySinglePage(ctx); err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Daily scrape failed")
		dc.Status = cycle.Failed
		dc.EndTime = time.Now()
		_ = s.followCycle.UpdateOrInsertFollowing(ctx, dc)
		return
	}
	// Update metrics
	err := s.updateCycle(ctx, dc)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Failed to update daily cycle metrics")
		dc.Status = cycle.Failed
		dc.EndTime = time.Now()
		_ = s.followCycle.UpdateOrInsertFollowing(ctx, dc)
		return
	}
}

// Helper methods
func (s *FollowingCycle) getInfluencerCount(ctx context.Context) (int, error) {
	profiles, err := s.twtPrf.FindAllUsers(ctx)
	if err != nil {
		return 0, err
	}
	return len(profiles), nil
}

func (s *FollowingCycle) getUniqueUserAccounts(ctx context.Context) (int, error) {
	users, err := s.follRepo.FindAllUniqueUserIDs(ctx)
	if err != nil {
		return 0, err
	}
	return len(users), nil
}

func (s *FollowingCycle) updateCycle(ctx context.Context, cyc *cycle.FollowingCycle) error {
	var err error
	if cyc.TotalInfluencerAccounts, err = s.getInfluencerCount(ctx); err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Failed to get total followings")
	}
	if cyc.TotalUserAccounts, err = s.getUniqueUserAccounts(ctx); err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Failed to get unique users")
	}

	// Finalize cycle
	cyc.EndTime = time.Now()
	cyc.Status = cycle.Completed
	if err := s.followCycle.UpdateOrInsertFollowing(ctx, cyc); err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msgf("Failed to finalize %s cycle", cyc.CycleType)
		return err
	}

	// Create next daily cycle (for tracking purposes)
	if _, err := s.followCycle.CreateNewFollowingCycle(ctx, cyc.CycleType); err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msgf("Failed to create next %s cycle", cyc.CycleType)
		return err
	}
	return nil

}

func (s *FollowingCycle) Expression() string {
	return "@daily" // equivalent to "0 0 * * *"
}
