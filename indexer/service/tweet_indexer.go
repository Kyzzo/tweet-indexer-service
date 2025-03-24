package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/jeetdotso/tweet-indexer-service/cache"
	"github.com/jeetdotso/tweet-indexer-service/config"
	"github.com/jeetdotso/tweet-indexer-service/connection/proto/grpcclient"
	"github.com/jeetdotso/tweet-indexer-service/connection/twitteruser/user"
	"github.com/jeetdotso/tweet-indexer-service/cycle"
	"github.com/jeetdotso/tweet-indexer-service/dummyaccount"
	"github.com/jeetdotso/tweet-indexer-service/indexer/mapper"
	"github.com/jeetdotso/tweet-indexer-service/indexer/model"
	"github.com/jeetdotso/tweet-indexer-service/indexer/repository"
	"github.com/jeetdotso/tweet-indexer-service/logg"
	"github.com/jeetdotso/tweet-indexer-service/persistence/mongodb"
	twitterTypes "github.com/jeetdotso/tweet-indexer-service/scrape/internals/api/types"
	"github.com/jeetdotso/tweet-indexer-service/scrape/rollback"
	track "github.com/jeetdotso/tweet-indexer-service/scrape/tracking"
	tracking "github.com/jeetdotso/tweet-indexer-service/scrape/tracking/types"
	"github.com/jeetdotso/tweet-indexer-service/stream"
	"github.com/jeetdotso/tweet-indexer-service/stream/kafka"
	"github.com/jeetdotso/tweet-indexer-service/util"
	"google.golang.org/protobuf/proto"
)

const (
	MAX_THREADS = 100
)

var (
	handleScrape   sync.Mutex
	hanldeRollback sync.Mutex
)

type TweetIndexer struct {
	tweetRepo      repository.TweetEventRepository
	cycleSvc       cycle.Service
	checkpointRepo repository.UserCheckPointRepository
	producer       stream.Producer
	accSvc         user.TwitteruserService
	dummSvc        dummyaccount.Service
}

func NewTweetIndexer(cfg config.Config, conn *mongodb.MongoConnection, grpcconn *grpcclient.GrpcClientMap) TweetIndexer {
	tweetRepo := repository.NewTweetIndexRepo(conn)
	checkpointRepo := repository.NewUserCheckPointRepository(conn)
	accSvc := user.NewTwitterUserService(grpcconn, cfg.Services)
	dummSvc := dummyaccount.NewSeleniumService(conn, &cfg)
	cycleSvc := cycle.NewCycleService(conn, &cfg)
	producer := kafka.NewProducer(cfg.Stream, fmt.Sprintf("%s.%s", stream.StreamTweetPrefixTopic, "monitor"))

	return TweetIndexer{
		tweetRepo:      tweetRepo,
		checkpointRepo: checkpointRepo,
		producer:       producer,
		cycleSvc:       cycleSvc,
		accSvc:         accSvc,
		dummSvc:        dummSvc,
	}
}

func (ti *TweetIndexer) Start(ctx context.Context) {
	logg.Log.Info().Caller().Ctx(ctx).Msg("Starting tweet indexer service...")

	trackedUsers := make(map[string]user.User) // Keeps track of monitored users

	for {

		users, err := ti.accSvc.FindAllRawUsers(ctx)
		if err != nil {
			logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error fetching users.")
			time.Sleep(30 * time.Second) // Retry after delay
			continue
		}

		// Track new users and remove old ones
		newUsers := make(map[string]user.User)
		newUsernames := []string{}
		var newUsersArr []user.User
		var deletedUsers int

		for _, u := range users {
			newUsers[u.ID] = u
			if _, exists := trackedUsers[u.ID]; !exists {
				// New user detected, sync their tweets
				cache.UpdateUserID(u.Handle, u.ID)
				newUsernames = append(newUsernames, u.Handle)
				newUsersArr = append(newUsersArr, u)
			}
		}

		// Remove users no longer in the system
		for id, u := range trackedUsers {
			if _, exists := newUsers[id]; !exists {
				logg.Log.Info().Caller().Ctx(ctx).Str("id", id).Msg("Removing user from tracking.")
				cache.RemoveUserID(u.Handle) // You need to implement this function
				deletedUsers++
				delete(trackedUsers, id)
			}
		}

		var wg sync.WaitGroup
		// Sync only newly added users
		if len(newUsernames) > 0 {
			wg.Add(1)
			ctl := util.CreateBackgroundContextFromExisting(ctx)
			go func(ctx context.Context, newUsersArr []user.User) {
				err = ti.Sync(ctx, newUsersArr)
				if err != nil {
					logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error syncing tweets.")
				}
			}(ctl, newUsersArr)
		}

		// Update trackedUsers to reflect the latest state
		trackedUsers = newUsers
		err = ti.cycleSvc.CreateNewCycle(ctx, len(trackedUsers), deletedUsers, len(newUsernames))
		if err != nil {
			logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error creating new cycle.")
		}
		logg.Log.Info().Caller().Ctx(ctx).Msg("Monitoring users for new tweets...")
		track.MonitorTweetsMultipleAccounts(ctx, newUsernames, MAX_THREADS, ti.dummSvc, ti.HandleTweetEvent)

		wg.Wait()
		if err = ti.cycleSvc.EndCycle(ctx); err != nil {
			logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Failed to end cycle.")
		}
	}
}

func (ti TweetIndexer) Stop(ctx context.Context) {
	logg.Log.Info().Caller().Ctx(ctx).Msg("Stopping tweet indexer service...")
	// stop monitoring
	// stop sync
	// stop all go routines
}

// TODO: wait until utilized then return so scrape can continue
// ✅ **Sync tweets for a list of users**
func (ti TweetIndexer) Sync(ctx context.Context, users []user.User) error {
	userToSync := make(map[string]time.Time)
	for _, user := range users {
		userCheckpoint := ti.syncLatestToCheckPoint(ctx, user)
		userToSync[user.Handle] = userCheckpoint
	}
	//sync tweets for users
	rollback.TweetsMultithreaded(ctx, userToSync, 10, 3, 5*time.Second, ti.dummSvc, ti.HandleTweetEventRollback)
	for _, user := range users {
		_ = ti.syncLatestToCheckPoint(ctx, user)
	}
	return nil
}

func addUsers(ctx context.Context, users []user.User) []string {
	//add to cache
	userHandles := make([]string, len(users))
	for _, user := range users {
		//add to cache
		cache.UpdateUserID(user.Handle, user.ID)
		userHandles = append(userHandles, user.Handle)
	}
	return userHandles
}

func (ti TweetIndexer) syncLatestToCheckPoint(ctx context.Context, user user.User) time.Time {
	twt, err := ti.tweetRepo.FindMostRecentByUserId(ctx, user.ID)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error fetching user checkpoint")
	}
	if twt == nil {
		return time.Now().AddDate(0, 0, -30)
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
	cache.UpdateTweet(user.Handle, &tracking.Tweet{
		URL:               parseUrl(user.Handle, twt.ID),
		ConversationIDStr: twt.ConversationID,
		FullText:          twt.Text,
		UserIDStr:         twt.UserID,
		CreatedAt:         twt.CreatedAt.String(),
	})
	return twt.CreatedAt
}

// ✅ **Handler for new tweets (passed as a callback)**
func (ti TweetIndexer) HandleTweetEvent(ctx context.Context, tweet *twitterTypes.TweetInfo) {
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
		logg.Log.Warn().Caller().Ctx(ctx).Str("tweetID", tweetEvent.ID).Msg("Tweet event already exists.")
		return
	}

	ti.streamTweet(ctx, tweetEvent)

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

	handleScrape.Lock()
	defer handleScrape.Unlock()
	if err = ti.cycleSvc.AddScrapedTweets(ctx); err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Failed to add scraped tweets.")
	}
}

func (ti TweetIndexer) HandleTweetEventRollback(ctx context.Context, tweet *twitterTypes.TweetInfo) {
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
		logg.Log.Warn().Caller().Ctx(ctx).Str("tweetID", tweetEvent.ID).Msg("Tweet event already exists.")
		return
	}
	ti.streamTweet(ctx, tweetEvent)

	hanldeRollback.Lock()
	defer hanldeRollback.Unlock()
	err = ti.cycleSvc.AddRollback(ctx)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Failed to add rollback.")
	}

}

// TODO: add varibale to show it is form rollback and not live (for streaming to SASS PRODUCT)
func (ti TweetIndexer) streamTweet(ctx context.Context, tweetEvent model.TweetEvent) {
	_, err := ti.tweetRepo.SaveOrUpdate(ctx, tweetEvent)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Failed to save tweet event.")
		return
	}
	tweetProto := tweetEvent.ToProto()

	// Marshal to binary format
	b, err := proto.Marshal(tweetProto)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Failed to marshal proto message.")
		return
	}
	// Produce message to Kafka
	if err = ti.producer.Produce(b); err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Failed to produce stream message.")
		return
	}

	logg.Log.Info().Caller().Ctx(ctx).Str("tweetId", tweetEvent.ID).Msg("Tweet event sent!")

}

func parseUrl(username, tweetId string) string {
	return fmt.Sprintf("https://x.com/%s/status/%s", username, tweetId)
}
