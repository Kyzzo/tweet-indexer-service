package service

import (
	"context"
	"time"

	"github.com/jeetdotso/tweet-indexer-service/config"
	"github.com/jeetdotso/tweet-indexer-service/dummyaccount"
	"github.com/jeetdotso/tweet-indexer-service/indexer/model"
	"github.com/jeetdotso/tweet-indexer-service/indexer/repository"
	"github.com/jeetdotso/tweet-indexer-service/logg"
	"github.com/jeetdotso/tweet-indexer-service/persistence/mongodb"
	twitterTypes "github.com/jeetdotso/tweet-indexer-service/scrape/internals/api/types"
	"github.com/jeetdotso/tweet-indexer-service/scrape/tracking"
	"github.com/jeetdotso/tweet-indexer-service/twitterprofile"
)

// receive who's following by followed id
// following list by id
// following list by id true | false if tracked
// get followed by list via id w/ excluding ids
type FollowingIndexer struct {
	twtPrf   twitterprofile.Repository
	dummSvc  dummyaccount.Service
	follRepo repository.FollowingRepository
}

const (
	SINGLE_PAGE_WORKER_COUNT  = 10
	SINGLE_PAGE_HANDLER_COUNT = 20
	SINGLE_PAGE_MAX_RETRIES   = 3
	SINGLE_PAGE_RETRY_DELAY   = 3 * time.Second

	FOLLOWING_WORKER_COUNT  = 20
	FOLLOWING_HANDLER_COUNT = 40
	FOLLOWING_MAX_RETRIES   = 3
	FOLLOWING_RETRY_DELAY   = 3 * time.Second

	SINGLE_USER_WORKER_COUNT  = 1
	SINGLE_USER_HANDLER_COUNT = 5
	SINGLE_USER_MAX_RETRIES   = 4
	SINGLE_USER_RETRY_DELAY   = 2 * time.Second
)

func NewFollowingIndexer(cfg config.Config, conn *mongodb.MongoConnection, dummySvc dummyaccount.Service) FollowingIndexer {
	follRepo := repository.NewFollowingRepository(conn)
	twtPrf := twitterprofile.NewRepository(conn)

	return FollowingIndexer{
		follRepo: follRepo,
		dummSvc:  dummySvc,
		twtPrf:   twtPrf,
	}
}

func (f *FollowingIndexer) ScrapeFollowingList(ctx context.Context) error {
	startTime := time.Now()
	userIds, err := f.follRepo.FindAllUniqueUserIDs(ctx)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Failed to find all unique user IDs.")
		return err
	}
	if len(userIds) == 0 {
		logg.Log.Info().Caller().Ctx(ctx).Msg("No unique user IDs found, skipping.")
		return nil
	}

	tracking.MonitorFollowingsPipeline(ctx, userIds, min(FOLLOWING_WORKER_COUNT, len(userIds)), min(FOLLOWING_HANDLER_COUNT, len(userIds)), FOLLOWING_MAX_RETRIES, FOLLOWING_RETRY_DELAY, f.HandleFollowing, f.dummSvc)
	_, err = f.follRepo.DeleteAllBeforeTime(ctx, startTime) // Clean up old followings
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Failed to delete old followings.")
		return err
	}
	return nil
}

func (f *FollowingIndexer) ScrapeFollowingListById(ctx context.Context, userId string) {
	if userId == "" {
		logg.Log.Warn().Caller().Ctx(ctx).Msg("User ID is empty, skipping.")
		return
	}
	//check if userId already exists in the following repo
	exists, err := f.follRepo.ExistsByUserId(ctx, userId)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Failed to check if user ID exists in following repo.")
		return
	}
	if exists {
		logg.Log.Info().Caller().Ctx(ctx).Msgf("User ID %s already exists in following repo, skipping.", userId)
		return
	}
	userIds := map[string]struct{}{userId: {}}
	tracking.MonitorFollowingsPipeline(ctx, userIds, SINGLE_USER_WORKER_COUNT, SINGLE_PAGE_HANDLER_COUNT, SINGLE_USER_MAX_RETRIES, SINGLE_USER_RETRY_DELAY, f.HandleFollowing, f.dummSvc)
}

func (f *FollowingIndexer) ScrapeFollowingListBySinglePage(ctx context.Context) error {
	userIds, err := f.follRepo.FindAllUniqueUserIDs(ctx)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Failed to find all unique user IDs.")
		return err
	}
	if len(userIds) == 0 {
		logg.Log.Info().Caller().Ctx(ctx).Msg("No unique user IDs found, skipping.")
		return nil
	}
	tracking.MonitorSingleFollowingPagePipeline(ctx, userIds, min(SINGLE_PAGE_WORKER_COUNT, len(userIds)), min(SINGLE_PAGE_HANDLER_COUNT, len(userIds)), SINGLE_PAGE_MAX_RETRIES, SINGLE_USER_RETRY_DELAY, f.HandleFollowing, f.dummSvc)
	return nil
}

func (f *FollowingIndexer) HandleFollowing(ctx context.Context, followedByID string, user twitterTypes.FollowingUser) {
	if user.ID == "" {
		logg.Log.Warn().Caller().Ctx(ctx).Msg("User ID is empty, skipping.")
		return
	}
	//add to following repo
	follow := model.NewFollowing(followedByID, user.ID)
	if err := f.follRepo.UpdateOrInsert(ctx, follow); err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Failed to upsert following.")
		return
	}
	//add to twt prf
	if err := f.twtPrf.UpsertTwitterProfile(ctx, user); err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Failed to upsert twitter profile.")
		return
	}
}
