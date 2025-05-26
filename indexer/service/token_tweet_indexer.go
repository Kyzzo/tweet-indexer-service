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
	twtPrf         twitterprofile.Service
	checkpointRepo repository.UserCheckPointRepository
	twtProfile     twitterprofile.Service
	dummSvc        dummyaccount.Service
	grokSvc        GrokIndexer
}

func NewTokenTweetIndexer(cfg config.Config, conn *mongodb.MongoConnection, grpcconn *grpcclient.GrpcClientMap, sassIndx SaasIndexer) TokenTweetIndexer {
	tweetRepo := repository.NewTweetIndexRepo(conn)
	checkpointRepo := repository.NewUserCheckPointRepository(conn)
	twtPrf := twitterprofile.NewUserService(conn, &cfg)
	dummSvc := dummyaccount.NewSeleniumService(conn, &cfg)
	twtProfile := twitterprofile.NewUserService(conn, &cfg)
	grokSvc := NewGrokIndexer(cfg, conn, dummSvc, sassIndx)

	return TokenTweetIndexer{
		tweetRepo:      tweetRepo,
		checkpointRepo: checkpointRepo,
		twtProfile:     twtProfile,
		dummSvc:        dummSvc,
		twtPrf:         twtPrf,
		grokSvc:        grokSvc,
	}
}

var trackedUsers = make(map[string]twitterTypes.TrackedUser) // Keeps track of monitored users
func (ti *TokenTweetIndexer) Start(ctx context.Context) {
	logg.Log.Info().Caller().Ctx(ctx).Msg("Starting tweet indexer service...")

	for {
		startTime := time.Now()

		podName := os.Getenv("POD_NAME") // e.g. "grok-py-1"
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
		users, err := ti.twtPrf.GetAllTrackedUsers(ctx)
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
		usersToSync := make([]twitterTypes.TrackedUser, 0, len(mySlice))
		for _, user := range mySlice {
			cache.UpdateUserID(user.Handle, user.ID)
			usersToSync = append(usersToSync, *user.TOTwitterUser())
		}
		logg.Log.Info().Caller().Ctx(ctx).Msgf("Syncing %d users in pod %s (ordinal %d/%d)", len(usersToSync), podName, ordinal, reps)
		err = ti.Sync(ctx, usersToSync)
		if err != nil {
			logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error syncing tweets.")
			continue
		}

		logg.Log.Info().Caller().Ctx(ctx).Msgf("---------------------\nCycle completed in %s\n----------------", time.Since(startTime))

	}
}

// ✅ **Sync tweets for a list of users**
func (ti TokenTweetIndexer) Sync(ctx context.Context, users []twitterTypes.TrackedUser) error {
	startTime := time.Now()
	userToSync, err := ti.parallelCheckpoints(ctx, users, 10)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error fetching user checkpoints")
		return err
	}

	logg.Log.Info().Caller().Ctx(ctx).Msgf("-------------------------Sync completed in : %s", time.Since(startTime))
	rollback.TweetsPipeline(ctx, userToSync, 60, 20, 2, 2*time.Second, ti.dummSvc, ti.HandleTweetEvent, ti.syncLatestToCheckPoint)
	// sync tweets for users
	return nil
}

func (ti TokenTweetIndexer) parallelCheckpoints(
	ctx context.Context,
	users []twitterTypes.TrackedUser,
	maxConcurrency int,
) (map[string]time.Time, error) {

	type job struct{ user twitterTypes.TrackedUser }
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

func (ti TokenTweetIndexer) syncLatestToCheckPoint(ctx context.Context, user twitterTypes.TrackedUser) time.Time {
	twt, err := ti.tweetRepo.FindMostRecentByUserId(ctx, user.ID)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error fetching user checkpoint")
	}
	if twt == nil || twt.ID == "" {
		// fmt.Println("No tweets found for user, skipping checkpoint update: ", user.Handle)
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
func (ti TokenTweetIndexer) HandleTweetEvent(ctx context.Context, tweet *twitterTypes.TweetInfo) {
	// fmt.Println("HandleTweetEvent called ", tweet.ID)
	if tweet == nil {
		logg.Log.Error().Caller().Ctx(ctx).Msg("HandleTweetEvent received a nil tweet")
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
	if exist.CreatedAt.Before(time.Now().Add(-time.Minute * 15)) {
		logg.Log.Warn().Caller().Ctx(ctx).Str("tweetID", tweetEvent.ID).Msg("Tweet event is too old, skipping.")
	}
	go ti.streamTweet(ctx, tweetEvent)

	// **Update User Checkpoint**
	checkpoint := model.UserCheckPoint{
		ID:                 tweet.UserID,
		LastTweet:          tweet.ID,
		LastTweetCreatedAt: tweetEvent.CreatedAt,
		UpdatedAt:          time.Now(),
	}

	_, err = ti.checkpointRepo.UpdateOrSave(ctx, checkpoint)
	if err != nil {
		logg.Log.Error().Caller().Err(err).Ctx(ctx).Msg("Error updating user checkpoint")
	}

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
	ti.streamRollbackTweet(ctx, tweetEvent)

}

func (ti TokenTweetIndexer) streamTweet(ctx context.Context, tweetEvent model.TweetEvent) {
	_, err := ti.tweetRepo.SaveOrUpdate(ctx, tweetEvent)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Failed to save tweet event.")
		return
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
