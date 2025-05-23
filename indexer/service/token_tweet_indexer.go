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
	// producer       stream.Producer
	dummSvc dummyaccount.Service
	grokSvc GrokIndexer
	listSvc list.ListRepository
	saasSvc saas.SassUserService
	accSvc  user.TwitteruserService
}

func NewTokenTweetIndexer(cfg config.Config, conn *mongodb.MongoConnection, grpcconn *grpcclient.GrpcClientMap, sassIndx SaasIndexer) TokenTweetIndexer {
	tweetRepo := repository.NewTweetIndexRepo(conn)
	checkpointRepo := repository.NewUserCheckPointRepository(conn)
	dummSvc := dummyaccount.NewSeleniumService(conn, &cfg)
	cycleSvc := cycle.NewCycleService(conn, &cfg)
	// producer := kafka.NewProducer(cfg.Stream, TOPIC_CON)
	grokSvc := NewGrokIndexer(cfg, conn, dummSvc, sassIndx)
	saasCli := saas.NewSassUserService(grpcconn, cfg.Services)
	listSvc := list.NewListRepo(conn)
	accSvc := user.NewTwitterUserService(grpcconn, cfg.Services)

	return TokenTweetIndexer{
		tweetRepo:      tweetRepo,
		checkpointRepo: checkpointRepo,
		// producer:       producer,
		cycleSvc: cycleSvc,
		dummSvc:  dummSvc,
		saasSvc:  saasCli,
		listSvc:  listSvc,
		accSvc:   accSvc,
		grokSvc:  grokSvc,
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

		// users, err := ti.listSvc.FindAll(ctx)
		// if err != nil {
		// 	logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error fetching users.")
		// 	continue
		// }
		users, err := ti.accSvc.FindAllRawUsers(ctx)
		if err != nil {
			logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error fetching users.")
			continue
		}
		for _, user := range users {
			cache.UpdateUserID(user.Handle, user.ID)
		}
		// otherUsers2, err := ti.accSvc.FindAllRawUsers(ctx)
		// if err != nil {
		// 	logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error fetching users.")
		// 	continue
		// }
		// users = append(users, otherUsers2[:10]...)

		err = ti.Sync(ctx, users)
		if err != nil {
			logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error syncing tweets.")
		}

		// }
		// break
		logg.Log.Info().Caller().Ctx(ctx).Msg("Monitoring users for new tweets...")

		// track.MonitorTweetsMultipleAccounts(ctx, trackableUser, min(MAX_THREADS, len(trackableUser)), ti.dummSvc, ti.HandleTweetEvent)

		// if err = ti.cycleSvc.EndCycle(ctx); err != nil {
		// 	logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Failed to end cycle.")
		// }
		logg.Log.Info().Caller().Ctx(ctx).Msgf("---------------------\nCycle completed in %s\n----------------", time.Since(start))

	}
}

// TODO: wait until utilized then return so scrape can continue
// ✅ **Sync tweets for a list of users**
func (ti TokenTweetIndexer) Sync(ctx context.Context, users []twitteruser.User) error {
	startTime := time.Now()
	userToSync, err := ti.parallelCheckpoints(ctx, users, 10)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error fetching user checkpoints")
		return err
	}

	logg.Log.Info().Caller().Ctx(ctx).Msgf("-------------------------Sync completed in : %s", time.Since(startTime))
	rollback.TweetsPipeline(ctx, userToSync, 80, 20, 2, 2*time.Second, ti.dummSvc, ti.HandleTweetEvent, ti.syncLatestToCheckPoint)
	// sync tweets for users
	return nil
}

func (ti TokenTweetIndexer) parallelCheckpoints(
	ctx context.Context,
	users []twitteruser.User,
	maxConcurrency int,
) (map[string]time.Time, error) {

	type job struct{ user twitteruser.User }
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

func (ti TokenTweetIndexer) syncLatestToCheckPoint(ctx context.Context, user twitteruser.User) time.Time {
	twt, err := ti.tweetRepo.FindMostRecentByUserId(ctx, user.ID)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error fetching user checkpoint")
	}
	if twt == nil || twt.ID == "" {
		// fmt.Println("No tweets found for user, skipping checkpoint update: ", user.Handle)
		return time.Now().Add(-time.Hour)
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
	// cache.UpdateTweet(user.Handle, &tracking.Tweet{
	// 	URL:               parseUrl(user.Handle, twt.ID),
	// 	ConversationIDStr: twt.ConversationID,
	// 	FullText:          twt.Text,
	// 	UserIDStr:         twt.UserID,
	// 	CreatedAt:         twt.CreatedAt.String(),
	// })
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

	go ti.grokSvc.Start(ctx, &tweetEvent, nil)
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
