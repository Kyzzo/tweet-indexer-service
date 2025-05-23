package main

import (
	"context"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jeetdotso/tweet-indexer-service/config"
	"github.com/jeetdotso/tweet-indexer-service/connection/proto/grpcclient"
	"github.com/jeetdotso/tweet-indexer-service/dummyaccount"
	"github.com/jeetdotso/tweet-indexer-service/indexer/controller"
	"github.com/jeetdotso/tweet-indexer-service/indexer/service"
	"github.com/jeetdotso/tweet-indexer-service/persistence/mongodb"
	"github.com/jeetdotso/tweet-indexer-service/scheduler"
	"github.com/robfig/cron"
	"github.com/rs/zerolog/log"
)

func main() {
	//Load app config
	var (
		cfg  = config.Load("./config/env/")
		conn = mongodb.NewConnection(&cfg.Persistence)
		// grpcconn = grpcclient.NewClientMap(&cfg.Grpc.Clients)
	)

	//create []int 0-30
	var saasIdx []int
	for i := 0; i < 30; i++ {
		saasIdx = append(saasIdx, i)
	}
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

	// compute your “chunk”
	chunkSize := (len(saasIdx) + reps - 1) / reps
	start := ordinal * chunkSize
	end := start + chunkSize
	if end > len(saasIdx) {
		end = len(saasIdx)
	}

	mySlice := saasIdx[start:end]
	log.Printf("pod=%s ordinal=%d/%d picking indices %v\n",
		podName, ordinal, reps, mySlice,
	)

	ctx := context.Background()

	dummSvc := dummyaccount.NewSeleniumService(&conn, cfg)
	dummSvc.ResetAll(ctx)
	//Register Scrapes
	// topicNames, saasIdx := registerScrapes(ctx, *cfg, &conn, grpcconn)

	// //create indexer kafka topic
	// kafka.CreateTopics(cfg, topicNames)

	//Register Schedulers
	// go registerSchedulers(cfg, &conn)
	// grokRetry := service.NewGrokRetryScheduler(cfg, &conn, dummSvc, saasIdx)
	// for {
	// 	// fmt.Println("Running scheduler")
	// 	grokRetry.Execute()
	// 	time.Sleep(time.Second * 10)
	// }
	//Register http endpoints
	registerHttp(cfg, &conn)
}

func registerHttp(cfg *config.Config, conn *mongodb.MongoConnection) {
	//deps
	var (
		api = service.NewTweetAPI(*cfg, conn)
	)
	twtCtrl := controller.NewTweetIndexApi(api)

	r := gin.Default()
	r.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"response": "Tweet Indexer Health Check"})
	})

	r.POST("public/indexer/replay/:id", twtCtrl.ReplayEventsByID)
	r.POST("public/indexer/replay", twtCtrl.ReplayEvents)
	r.GET("public/indexer/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"response": "Tweet indexer Health Check"})
	})

	r.POST("public/indexer/monitors/replay/:userId", twtCtrl.ReplayEventsByUser)

	err := r.Run(":" + cfg.Server.Port)
	if err != nil {
		log.Panic().Err(err).Msg("Error starting server")
	}
}

func registerScrapes(ctx context.Context, cfg config.Config, conn *mongodb.MongoConnection, grpcconn *grpcclient.GrpcClientMap) ([]string, service.SaasIndexer) {
	// deps
	var (
		saasIndexer       = service.NewSaasIndexer(cfg, conn)
		tokenTweetIndexer = service.NewTokenTweetIndexer(cfg, conn, grpcconn, saasIndexer)
	)

	var topicNames []string
	// add topic names
	topicNames = append(topicNames, saasIndexer.TopicName())

	// start tweet indexer
	// go saasIndexer.Start(ctx)
	go tokenTweetIndexer.Start(ctx)

	return topicNames, saasIndexer
}

func registerSchedulers(cfg *config.Config, conn *mongodb.MongoConnection, dummSvc dummyaccount.Service, saasIdx service.SaasIndexer) {
	c := cron.New()
	jobs := Schedulers(conn, cfg, dummSvc, saasIdx)
	for _, job := range jobs {
		_ = c.AddFunc(job.Expression(), job.Execute)
	}
	c.Start()
}

func Schedulers(conn *mongodb.MongoConnection, cfg *config.Config, dummSvc dummyaccount.Service, saasIdx service.SaasIndexer) []scheduler.Scheduler {
	var (
		grokRetry = service.NewGrokRetryScheduler(cfg, conn, dummSvc, saasIdx)
		// healthCheck = accounts_health.NewAccountHealthScheduler(dummyaccount.NewSeleniumService(conn, cfg))
	)

	return []scheduler.Scheduler{
		grokRetry,
		// healthCheck,
	}
}
