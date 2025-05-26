package main

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jeetdotso/tweet-indexer-service/config"
	"github.com/jeetdotso/tweet-indexer-service/connection/proto/grpcclient"
	"github.com/jeetdotso/tweet-indexer-service/dummyaccount"
	"github.com/jeetdotso/tweet-indexer-service/indexer/controller"
	sched "github.com/jeetdotso/tweet-indexer-service/indexer/scheduler"
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

	// ctx := context.Background()

	// dummSvc := dummyaccount.NewSeleniumService(&conn, cfg)
	// dummSvc.ResetAll(ctx)
	// //Register Scrapes
	// _, saasIdx := registerScrapes(ctx, *cfg, &conn, grpcconn)

	// //Register Schedulers
	// go registerSchedulers(cfg, &conn, dummSvc, saasIdx)

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
		c.JSON(http.StatusOK, gin.H{"response": "Tweet Indexer Health OK"})
	})

	r.POST("/public/indexer/replay/:id", twtCtrl.ReplayEventsByID)
	r.POST("/public/indexer/replay", twtCtrl.ReplayEvents)
	r.GET("/public/indexer/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"response": "Tweet indexer Health OK"})
	})
	r.POST("/public/indexer/grok/health", func(ctx *gin.Context) {
		//call the health check function for grok
		url := cfg.Services.GrokAPIServices.Host + "/health"
		resp, err := http.Get(url)
		if err != nil {
			log.Error().Err(err).Msg("Failed to get health check from Grok API")
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get health check from Grok API", "details": err.Error()})
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			log.Error().Msgf("Grok API health check failed with status code: %d", resp.StatusCode)
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Grok API health check failed"})
			return
		}

		ctx.JSON(http.StatusOK, gin.H{"response": "Grok API Health OK"})

	})

	r.POST("/public/indexer/monitors/replay/:userId", twtCtrl.ReplayEventsByUser)

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
		grokRetry   = sched.NewGrokRetryScheduler(cfg, conn, dummSvc, saasIdx)
		healthCheck = sched.NewAccountHealthScheduler(cfg, conn, dummSvc)
	)

	return []scheduler.Scheduler{
		grokRetry,
		healthCheck,
	}
}
