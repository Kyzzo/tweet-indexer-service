package service

import (
	"context"
	"sync"
	"time"

	"github.com/jeetdotso/tweet-indexer-service/cache"
	"github.com/jeetdotso/tweet-indexer-service/config"
	"github.com/jeetdotso/tweet-indexer-service/connection/proto/grpcclient"
	"github.com/jeetdotso/tweet-indexer-service/connection/twitteruser"
	"github.com/jeetdotso/tweet-indexer-service/connection/twitteruser/saas"
	"github.com/jeetdotso/tweet-indexer-service/connection/twitteruser/user"
	"github.com/jeetdotso/tweet-indexer-service/cycle"
	"github.com/jeetdotso/tweet-indexer-service/dummyaccount"
	"github.com/jeetdotso/tweet-indexer-service/indexer/mapper"
	"github.com/jeetdotso/tweet-indexer-service/indexer/model"
	"github.com/jeetdotso/tweet-indexer-service/indexer/repository"
	"github.com/jeetdotso/tweet-indexer-service/list"
	"github.com/jeetdotso/tweet-indexer-service/logg"
	"github.com/jeetdotso/tweet-indexer-service/persistence/mongodb"
	twitterTypes "github.com/jeetdotso/tweet-indexer-service/scrape/internals/api/types"
	"github.com/jeetdotso/tweet-indexer-service/scrape/rollback"
	track "github.com/jeetdotso/tweet-indexer-service/scrape/tracking"
	tracking "github.com/jeetdotso/tweet-indexer-service/scrape/tracking/types"
	"github.com/jeetdotso/tweet-indexer-service/stream"
	"github.com/jeetdotso/tweet-indexer-service/stream/kafka"
)

var (
	handleScrapeTokenGroup   sync.Mutex
	hanldeRollbackTokenGroup sync.Mutex

	// TOPIC_CON = fmt.Sprintf("%s.%s", stream.StreamTweetPrefixTopic, TOPIC_NAME)
)

type TokenTweetIndexer struct {
	tweetRepo      repository.TweetEventRepository
	cycleSvc       cycle.Service
	checkpointRepo repository.UserCheckPointRepository
	producer       stream.Producer
	dummSvc        dummyaccount.Service
	grokSvc        GrokIndexer
	listSvc        list.ListRepository
	saasSvc        saas.SassUserService
	accSvc         user.TwitteruserService
}

func NewTokenTweetIndexer(cfg config.Config, conn *mongodb.MongoConnection, grpcconn *grpcclient.GrpcClientMap) TokenTweetIndexer {
	tweetRepo := repository.NewTweetIndexRepo(conn)
	checkpointRepo := repository.NewUserCheckPointRepository(conn)
	dummSvc := dummyaccount.NewSeleniumService(conn, &cfg)
	cycleSvc := cycle.NewCycleService(conn, &cfg)
	producer := kafka.NewProducer(cfg.Stream, TOPIC_CON)
	grokSvc := NewGrokIndexer(cfg, conn, dummSvc)
	saasCli := saas.NewSassUserService(grpcconn, cfg.Services)
	listSvc := list.NewListRepo(conn)
	accSvc := user.NewTwitterUserService(grpcconn, cfg.Services)

	return TokenTweetIndexer{
		tweetRepo:      tweetRepo,
		checkpointRepo: checkpointRepo,
		producer:       producer,
		cycleSvc:       cycleSvc,
		dummSvc:        dummSvc,
		saasSvc:        saasCli,
		listSvc:        listSvc,
		accSvc:         accSvc,
		grokSvc:        grokSvc,
	}
}

var trackedUsers = make(map[string]twitteruser.User) // Keeps track of monitored users
func (ti *TokenTweetIndexer) Start(ctx context.Context) {

	logg.Log.Info().Caller().Ctx(ctx).Msg("Starting tweet indexer service...")

	for {
		start := time.Now()
		// users, err := ti.saasSvc.FindAllTracked(ctx)
		// if err != nil {
		// 	logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error fetching users.")
		// 	continue
		// }

		users, err := ti.listSvc.FindAll(ctx)
		if err != nil {
			logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error fetching users.")
			continue
		}

		// otherUsers2, err := ti.accSvc.FindAllRawUsers(ctx)
		// if err != nil {
		// 	logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error fetching users.")
		// 	continue
		// }
		// users = append(users, otherUsers2[:10]...)

		// Track new users and remove old ones
		usersToTrack := make(map[string]twitteruser.User)
		newUsernames := []string{}
		var newUsersArr []twitteruser.User
		var deletedUsers int

		for _, usr := range users {
			u := *usr.TOTwitterUser()
			usersToTrack[u.ID] = u
			if _, exists := trackedUsers[u.ID]; !exists {
				// New user detected, sync their tweets
				cache.UpdateUserID(u.Handle, u.ID)
				newUsernames = append(newUsernames, u.Handle)
				newUsersArr = append(newUsersArr, u)
			}
		}

		// Remove users no longer in the system
		for id, u := range trackedUsers {
			if _, exists := usersToTrack[id]; !exists {
				logg.Log.Info().Caller().Ctx(ctx).Str("id", id).Msg("Removing user from tracking.")
				cache.RemoveUserID(u.ID)
				deletedUsers++
			}
		}

		// Update trackedUsers to reflect the latest state
		trackedUsers = usersToTrack
		// err = ti.cycleSvc.CreateNewCycle(ctx, len(trackedUsers), deletedUsers, len(newUsernames), cycle.TOKEN_GROUP_INDEXER)
		// if err != nil {
		// 	logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error creating new cycle.")
		// }
		// Sync only newly added users
		if len(newUsernames) > 0 {

			err = ti.Sync(ctx, newUsersArr)
			if err != nil {
				logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error syncing tweets.")
			}
			logg.Log.Info().Caller().Ctx(ctx).Msgf("Synced %d new users", len(newUsernames))

		}
		var trackableUser []string
		for _, u := range trackedUsers {
			trackableUser = append(trackableUser, u.Handle)
		}
		// break
		logg.Log.Info().Caller().Ctx(ctx).Msg("Monitoring users for new tweets...")
		break
		track.MonitorTweetsMultipleAccounts(ctx, trackableUser, min(MAX_THREADS, len(trackableUser)), ti.dummSvc, ti.HandleTweetEvent)

		if err = ti.cycleSvc.EndCycle(ctx); err != nil {
			logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Failed to end cycle.")
		}
		logg.Log.Info().Caller().Ctx(ctx).Msgf("---------------------\nCycle completed in %s\n----------------", time.Since(start))
	}
}

// TODO: wait until utilized then return so scrape can continue
// ✅ **Sync tweets for a list of users**
func (ti TokenTweetIndexer) Sync(ctx context.Context, users []twitteruser.User) error {
	userToSync := make(map[string]time.Time)
	for _, user := range users {
		userCheckpoint := ti.syncLatestToCheckPoint(ctx, user)
		userToSync[user.Handle] = userCheckpoint
	}
	rollback.TweetsMultithreaded(ctx, userToSync, 40, 5, 5*time.Second, ti.dummSvc, ti.HandleTweetEventRollback)
	for _, user := range users {
		_ = ti.syncLatestToCheckPoint(ctx, user)
	}
	//sync tweets for users
	return nil
}

func (ti TokenTweetIndexer) syncLatestToCheckPoint(ctx context.Context, user twitteruser.User) time.Time {
	twt, err := ti.tweetRepo.FindMostRecentByUserId(ctx, user.ID)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error fetching user checkpoint")
	}
	if twt == nil || twt.ID == "" {
		return time.Now().AddDate(0, 0, -20)
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
func (ti TokenTweetIndexer) HandleTweetEvent(ctx context.Context, tweet *twitterTypes.TweetInfo, username string) {
	// fmt.Println("HandleTweetEvent called ", tweet.ID)
	if tweet == nil {
		logg.Log.Error().Caller().Ctx(ctx).Msg("HandleTweetEvent received a nil tweet")
		return
	}

	tweetEvent := mapper.MapTweetToEvent(tweet, username)
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

	handleScrapeTokenGroup.Lock()
	defer handleScrapeTokenGroup.Unlock()
	if err = ti.cycleSvc.AddScrapedTweets(ctx); err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Failed to add scraped tweets.")
	}
}

func (ti TokenTweetIndexer) HandleTweetEventRollback(ctx context.Context, tweet *twitterTypes.TweetInfo, username string) {
	if tweet == nil {
		logg.Log.Error().Msg("HandleTweetEvent received a nil tweet")
		return
	}

	tweetEvent := mapper.MapTweetToEvent(tweet, username)
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

	hanldeRollbackTokenGroup.Lock()
	defer hanldeRollbackTokenGroup.Unlock()
	err = ti.cycleSvc.AddRollback(ctx)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Failed to add rollback.")
	}

}

func (ti TokenTweetIndexer) streamTweet(ctx context.Context, tweetEvent model.TweetEvent) {
	_, err := ti.tweetRepo.SaveOrUpdate(ctx, tweetEvent)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Failed to save tweet event.")
		return
	}

	// go ti.grokSvc.Start(ctx, &tweetEvent, nil)
	// tweetProto := tweetEvent.ToProto()

	// // Marshal to binary format
	// b, err := proto.Marshal(tweetProto)
	// if err != nil {
	// 	logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Failed to marshal proto message.")
	// 	return
	// }
	// // Produce message to Kafka
	// if err = ti.producer.Produce([]byte(LIVE_KEY), b); err != nil {
	// 	logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Failed to produce stream message.")
	// 	return
	// }

	// logg.Log.Info().Caller().Ctx(ctx).Str("tweetId", tweetEvent.ID).Msg("Tweet event sent!")

}
func (ti TokenTweetIndexer) streamRollbackTweet(ctx context.Context, tweetEvent model.TweetEvent) {
	_, err := ti.tweetRepo.SaveOrUpdate(ctx, tweetEvent)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Failed to save tweet event.")
		return
	}

	// go ti.grokSvc.Start(ctx, &tweetEvent, nil)
	// tweetProto := tweetEvent.ToProto()

	// // Marshal to binary format
	// b, err := proto.Marshal(tweetProto)
	// if err != nil {
	// 	logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Failed to marshal proto message.")
	// 	return
	// }
	// // Produce message to Kafka
	// if err = ti.producer.Produce([]byte(ROLLBACK_KEY), b); err != nil {
	// 	logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Failed to produce stream message.")
	// 	return
	// }

	// logg.Log.Info().Caller().Ctx(ctx).Str("tweetId", tweetEvent.ID).Msg("Tweet event sent!")

}
