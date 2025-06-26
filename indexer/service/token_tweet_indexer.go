package service

import (
	"context"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jeetdotso/tweet-indexer-service/cache"
	"github.com/jeetdotso/tweet-indexer-service/config"
	"github.com/jeetdotso/tweet-indexer-service/connection/proto/grpcclient"
	"github.com/jeetdotso/tweet-indexer-service/dummyaccount"
	"github.com/jeetdotso/tweet-indexer-service/indexer/mapper"
	"github.com/jeetdotso/tweet-indexer-service/indexer/model"
	"github.com/jeetdotso/tweet-indexer-service/indexer/repository"
	"github.com/jeetdotso/tweet-indexer-service/logg"
	"github.com/jeetdotso/tweet-indexer-service/persistence/mongodb"
	twitterTypes "github.com/jeetdotso/tweet-indexer-service/scrape/internals/api/types"
	"github.com/jeetdotso/tweet-indexer-service/scrape/rollback"
	"github.com/jeetdotso/tweet-indexer-service/shared"
	"github.com/jeetdotso/tweet-indexer-service/twitterprofile"
)

const (
	MAX_THREADS  = 20
	TOPIC_NAME   = "monitor"
	LIVE_KEY     = "live"     // used to indicate live scraping in the topic
	ROLLBACK_KEY = "rollback" // used to indicate rollback scraping in the topic
)

type TokenTweetIndexer struct {
	tweetRepo      repository.TweetEventRepository
	ftPrf          twitterprofile.Repository
	checkpointRepo repository.UserCheckPointRepository
	dummSvc        dummyaccount.Service
	rawTweetRepo   repository.RawTweetRepository
	grokSvc        GrokIndexer
}

func NewTokenTweetIndexer(cfg config.Config, conn *mongodb.MongoConnection, grpcconn *grpcclient.GrpcClientMap, sassIndx SaasIndexer) TokenTweetIndexer {
	tweetRepo := repository.NewTweetIndexRepo(conn)
	checkpointRepo := repository.NewUserCheckPointRepository(conn)
	ftPrf := twitterprofile.NewRepository(conn)
	dummSvc := dummyaccount.NewSeleniumService(conn, &cfg)
	grokSvc := NewGrokIndexer(cfg, conn, dummSvc, sassIndx)
	rawTweetRepo := repository.NewRawTweetRepo(conn)

	return TokenTweetIndexer{
		tweetRepo:      tweetRepo,
		checkpointRepo: checkpointRepo,
		rawTweetRepo:   rawTweetRepo,
		dummSvc:        dummSvc,
		ftPrf:          ftPrf,
		grokSvc:        grokSvc,
	}
}

var trackedUsers = make(map[string]twitterprofile.TwitterProfile) // Keeps track of monitored users
func (ti *TokenTweetIndexer) Start(ctx context.Context, tag twitterprofile.TagType) {
	logg.Log.Info().Caller().Ctx(ctx).Msg("Starting tweet indexer service...")

	for {
		startTime := time.Now()

		podName := os.Getenv("POD_NAME") // e.g. "tweet-indexer-service-headless-1"
		parts := strings.Split(podName, "-")
		ordinal, err := strconv.Atoi(parts[len(parts)-1])
		if err != nil {
			log.Printf("invalid pod name %q: %v", podName, err)
		}

		// how many replicas total?
		reps, err := strconv.Atoi(os.Getenv("REPLICAS")) // injected by Helm
		if err != nil {
			log.Printf("invalid REPLICAS: %v", err)
		}
		users, err := ti.ftPrf.GetTopHitRaw(ctx, shared.MAX_ACCOUNTS_RUN, []string{tag.String()})
		if err != nil {
			logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error fetching users.")
			continue
		}

		chunkSize := (len(users) + reps - 1) / reps
		start := ordinal * chunkSize
		end := start + chunkSize
		if end > len(users) {
			end = len(users)
		}

		mySlice := users[start:end]
		usersToSync := make([]twitterprofile.FetchedProfile, 0, len(mySlice))
		for _, user := range mySlice {
			cache.UpdateUserID(user.Handle, user.ID)
			usersToSync = append(usersToSync, *user)
		}
		logg.Log.Info().Caller().Ctx(ctx).Msgf("Syncing %d users in pod %s (ordinal %d/%d)", len(usersToSync), podName, ordinal, reps)
		err = ti.Sync(ctx, tag, usersToSync)
		if err != nil {
			logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error syncing tweets.")
			continue
		}

		logg.Log.Info().Caller().Ctx(ctx).Msgf("---------------------\nCycle completed in %s\n----------------", time.Since(startTime))
		if time.Since(startTime) < 60*time.Second {
			logg.Log.Info().Caller().Ctx(ctx).Msgf("Sleeping for %s", 10*time.Second)
			time.Sleep(10 * time.Second)
		}

	}
}

// ✅ **Sync tweets for a list of users**
func (ti TokenTweetIndexer) Sync(ctx context.Context, tag twitterprofile.TagType, usersToSync []twitterprofile.FetchedProfile) error {
	startTime := time.Now()

	// 1) Extract just the IDs of the users we want to update checkpoints for
	userIDs := make([]string, 0, len(usersToSync))
	for _, u := range usersToSync {
		userIDs = append(userIDs, u.ID)
	}

	// 2) Fetch each user’s latest tweet in one aggregation call
	idToHandle := make(map[string]string, len(usersToSync))
	for _, u := range usersToSync {
		idToHandle[u.ID] = u.Handle
	}

	// 2) Run the aggregation to get each user’s latest tweet (by user ID):
	latests, err := ti.tweetRepo.FindLatestTweetsForUsers(ctx, userIDs)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error running aggregation for latest tweets")
		return err
	}

	// 3) Build userToTs keyed by handle, and collect the toUpsert slice (still keyed by ID):
	var toUpsert []model.UserCheckPoint
	userToTs := make(map[string]time.Time, len(userIDs))

	for _, lt := range latests {
		// look up this user’s handle from idToHandle:
		handle := idToHandle[lt.UserID]
		userToTs[handle] = lt.CreatedAt

		// still upsert by ID in the checkpoint collection:
		toUpsert = append(toUpsert, model.UserCheckPoint{
			ID:                 lt.UserID,
			LastTweet:          lt.TweetID,
			LastTweetCreatedAt: lt.CreatedAt,
			UpdatedAt:          time.Now(),
		})
	}

	// 4) Supply defaults for users with zero tweets (by handle):
	defaultTime := time.Now().Add(-5 * time.Minute)
	for _, u := range usersToSync {
		if _, exists := userToTs[u.Handle]; !exists {
			userToTs[u.Handle] = defaultTime
			// If you also want a checkpoint row for them (by ID), uncomment:
			// toUpsert = append(toUpsert, model.UserCheckPoint{
			//     ID:                 u.ID,
			//     LastTweet:          "",
			//     LastTweetCreatedAt: defaultTime,
			//     UpdatedAt:          time.Now(),
			// })
		}
	}

	if err := ti.checkpointRepo.BulkUpsert(ctx, toUpsert); err != nil {
		return err
	}

	logg.Log.Info().Caller().Ctx(ctx).Msgf("-------------------------Sync completed in : %s", time.Since(startTime))
	rollback.TweetsPipeline(ctx, tag, userToTs, 60, 20, 2, 2*time.Second, ti.dummSvc, ti.HandleTweetEvent, ti.syncLatestToCheckPoint)
	// sync tweets for users
	return nil
}

func (ti TokenTweetIndexer) parallelCheckpoints(
	ctx context.Context,
	users []twitterprofile.TwitterProfile,
	maxConcurrency int,
) (map[string]time.Time, error) {

	type job struct{ user twitterprofile.TwitterProfile }
	type result struct {
		user string
		ts   time.Time
	}

	jobs := make(chan job, len(users))
	results := make(chan result, len(users))
	var wg sync.WaitGroup

	// 1) spawn M workers
	for i := 0; i < maxConcurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				// guard against a panic in your sync function
				var ts time.Time
				func() {
					defer func() {
						if r := recover(); r != nil {
							logg.Log.Error().
								Caller().
								Msgf("panic syncLatestToCheckPoint(%s): %v", j.user.Handle, r)
						}
					}()
					ts = ti.syncLatestToCheckPoint(ctx, j.user)
				}()
				results <- result{j.user.Handle, ts}
			}
		}()
	}

	// 2) once all workers are done, close results
	go func() {
		wg.Wait()
		close(results)
	}()

	// 3) feed the jobs
	go func() {
		for _, u := range users {
			jobs <- job{u}
		}
		close(jobs)
	}()

	// 4) collect
	out := make(map[string]time.Time, len(users))
	for r := range results {
		out[r.user] = r.ts
	}

	return out, nil
}

func (ti TokenTweetIndexer) syncLatestToCheckPoint(ctx context.Context, user twitterprofile.TwitterProfile) time.Time {
	twt, err := ti.tweetRepo.FindMostRecentByUserId(ctx, user.ID)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error fetching user checkpoint")
	}
	if twt == nil || twt.ID == "" {
		return time.Now().Add(-time.Minute * 5)
	}
	checkpoint := model.UserCheckPoint{
		ID:                 user.ID,
		LastTweet:          twt.ID,
		LastTweetCreatedAt: twt.CreatedAt,
		UpdatedAt:          time.Now(),
	}
	_, err = ti.checkpointRepo.UpdateOrSave(ctx, checkpoint)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error updating user checkpoint")
		return twt.CreatedAt
	}
	return twt.CreatedAt
}

// ✅ **Handler for new tweets (passed as a callback)**
func (ti TokenTweetIndexer) HandleTweetEvent(ctx context.Context, tagType twitterprofile.TagType, tweet *twitterTypes.TweetInfo) {
	if tweet == nil {
		logg.Log.Error().Caller().Ctx(ctx).Msg("HandleTweetEvent received a nil tweet")
		return
	}
	if tweet.ID == "" {
		logg.Log.Error().Caller().Ctx(ctx).Msg("HandleTweetEvent received a tweet with empty ID")
		return
	}
	tweetEvent := mapper.MapTweetToEvent(tweet)
	if tweetEvent.CreatedAt.Before(time.Now().Add(-time.Hour)) {
		logg.Log.Warn().Caller().Ctx(ctx).Str("tweetID", tweetEvent.ID).Msg("Tweet event is too old, skipping.")
	}
	//Check if tweet already exists
	exist, err := ti.tweetRepo.FindAllByID(ctx, tweetEvent.ID)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Failed to fetch tweet event by ID.")
		return
	}
	if exist != nil {
		// logg.Log.Warn().Caller().Ctx(ctx).Str("tweetID", tweetEvent.ID).Msg("Tweet event already exists.")
		return
	}
	switch tagType {
	case twitterprofile.TAG_TRADER:
		tweetEvent.Category = model.CategoryTypeTRADER
	case twitterprofile.TAG_MEME:
		tweetEvent.Category = model.CategoryTypeMEME
	default:
		tweetEvent.Category = model.CategoryTypeTRADER
	}
	go ti.streamTweet(ctx, *tweetEvent, tweet.FullInterface)

}

func (ti TokenTweetIndexer) HandleTweetEventRollback(ctx context.Context, tweet *twitterTypes.TweetInfo) {
	if tweet == nil {
		logg.Log.Error().Msg("HandleTweetEvent received a nil tweet")
		return
	}

	tweetEvent := mapper.MapTweetToEvent(tweet)
	//Check if tweet already exists
	exist, err := ti.tweetRepo.FindAllByID(ctx, tweetEvent.ID)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Failed to fetch tweet event by ID.")
		return
	}
	if exist != nil {
		// logg.Log.Warn().Caller().Ctx(ctx).Str("tweetID", tweetEvent.ID).Msg("Tweet event already exists.")
		return
	}
	ti.streamRollbackTweet(ctx, *tweetEvent)

}

func (ti TokenTweetIndexer) streamTweet(ctx context.Context, tweetEvent model.TweetEvent, rawEntry string) {
	_, err := ti.tweetRepo.SaveOrUpdate(ctx, tweetEvent)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Failed to save tweet event.")
		return
	}
	_, err = ti.rawTweetRepo.SaveOrUpdate(ctx, model.RawTweet{
		ID:       tweetEvent.ID,
		TweetID:  tweetEvent.TweetID,
		UserID:   tweetEvent.UserID,
		RawEntry: rawEntry,
	})
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Failed to save raw tweet event.")
	}

	go ti.grokSvc.Start(ctx, &tweetEvent, nil)

}
func (ti TokenTweetIndexer) streamRollbackTweet(ctx context.Context, tweetEvent model.TweetEvent) {
	_, err := ti.tweetRepo.SaveOrUpdate(ctx, tweetEvent)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Failed to save tweet event.")
		return
	}

}
