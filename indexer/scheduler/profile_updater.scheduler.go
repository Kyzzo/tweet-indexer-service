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
	twitterTypes "github.com/jeetdotso/tweet-indexer-service/scrape/internals/api/types"
	"github.com/jeetdotso/tweet-indexer-service/shared"
	"github.com/jeetdotso/tweet-indexer-service/twitterprofile"
	"github.com/redis/go-redis/v9"
)

const ProfileUpdater = "profile-updater-lock-sync-daily-job"

type ProfileUpdaterImpl struct {
	svc         service.Profile
	locker      *redislock.Client
	twitterRepo twitterprofile.Repository
}

func NewProfileUpdater(cfg *config.Config, conn *mongodb.MongoConnection, dummySvc dummyaccount.Service) scheduler.Scheduler {
	locker := redislock.New(redis.NewClient(redis2.GetRedisOptions(cfg.Memory)))
	var (
		svc         = service.NewProfile(*cfg, conn, dummySvc)
		twitterRepo = twitterprofile.NewRepository(conn)
	)
	return &ProfileUpdaterImpl{
		svc:         svc,
		locker:      locker,
		twitterRepo: twitterRepo,
	}
}

func (s *ProfileUpdaterImpl) Execute() {
	ctx := context.Background()
	lock, err := s.locker.Obtain(ctx, ProfileUpdater, 5*time.Minute, &redislock.Options{
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

	// Get all fetched profiles from the database
	s.RunFetchedProfileUpdater(ctx)
	logg.Log.Info().Caller().Ctx(ctx).Msg("Fetched profile updater job completed successfully.")

	// get all user profiles from db
	s.RunUserProfileUpdater(ctx)
	logg.Log.Info().Caller().Ctx(ctx).Msg("Profile updater job completed successfully.")
}

func (c *ProfileUpdaterImpl) Expression() string {
	return "@daily" // equivalent to "0 0 * * *"
}

func (c *ProfileUpdaterImpl) RunUserProfileUpdater(ctx context.Context) {
	profiles, err := c.twitterRepo.
		FindAllProfilesNotUpdatedSince(ctx, time.Now().Add(-23*time.Hour))
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).
			Msg("Failed to list user profiles")
		return
	}
	// if no profiles found, return early
	if len(profiles) == 0 {
		logg.Log.Info().Caller().Ctx(ctx).Msg("No user profiles to update")
		return
	}

	// extract IDs
	ids := make([]string, len(profiles))
	for i, p := range profiles {
		ids[i] = p.ID
	}

	c.processIDBatches(ctx, ids, service.MAX_THREADS, c.twitterRepo.UpsertUserProfile)
}

func (c *ProfileUpdaterImpl) RunFetchedProfileUpdater(ctx context.Context) {
	profiles, err := c.twitterRepo.
		FindAllFetchedProfilesTopHitSince(
			ctx,
			time.Now().Add(-23*time.Hour),
			shared.MAX_ACCOUNTS_RUN,
			shared.RUNNING_TAGS,
		)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).
			Msg("Failed to list fetched profiles")
		return
	}
	// if no profiles found, return early
	if len(profiles) == 0 {
		logg.Log.Info().Caller().Ctx(ctx).Msg("No fetched profiles to update")
		return
	}

	// extract IDs
	ids := make([]string, len(profiles))
	for i, p := range profiles {
		ids[i] = p.ID
	}

	// for fetched profiles we first get a twitterTypes.User, then turn
	// it into your domain.FollowingUser via ToFollowingUser()
	c.processIDBatches(ctx, ids, service.MAX_THREADS, func(ctx context.Context, u *twitterTypes.User) error {
		return c.twitterRepo.UpsertFetchedProfile(ctx, u.ToFollowingUser())
	})
}

// processIDBatches will:
//   - split ids into batches of up to batchSize
//   - for each batch, retry up to 3× calling GetMultiProfilesByIds
//   - on success, call upsertFn for each returned User
func (c *ProfileUpdaterImpl) processIDBatches(
	ctx context.Context,
	ids []string,
	batchSize int,
	upsertFn func(context.Context, *twitterTypes.User) error,
) {
	for start := 0; start < len(ids); start += batchSize {
		end := start + batchSize
		if end > len(ids) {
			end = len(ids)
		}
		batch := ids[start:end]

		var (
			users []*twitterTypes.User
			err   error
		)
		// retry loop
		for attempt := 1; attempt <= 3; attempt++ {
			users, err = c.svc.GetMultiProfilesByIds(ctx, batch)
			if err == nil {
				break
			}
			logg.Log.Warn().
				Caller().
				Ctx(ctx).
				Err(err).
				Msgf("Attempt %d/3 failed fetching IDs %v", attempt, batch)
			time.Sleep(time.Duration(attempt) * 500 * time.Millisecond)
		}
		if err != nil {
			logg.Log.Error().
				Caller().
				Ctx(ctx).
				Err(err).
				Msgf("All retries failed for IDs %v", batch)
			continue
		}

		// upsert each user
		for _, u := range users {
			if u.ID == "" {
				logg.Log.Warn().Caller().Ctx(ctx).Msgf("User fetched with empty ID: %v", u)
				continue // skip users with empty IDs
			}
			if upErr := upsertFn(ctx, u); upErr != nil {
				logg.Log.Error().
					Caller().
					Ctx(ctx).
					Err(upErr).
					Msgf("Upsert failed for user %s", u.ID)
			}
		}
	}
}
