package main

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jeetdotso/tweet-indexer-service/config"
	"github.com/jeetdotso/tweet-indexer-service/connection/proto/grpcclient"
	"github.com/jeetdotso/tweet-indexer-service/indexer/controller"
	"github.com/jeetdotso/tweet-indexer-service/indexer/service"
	"github.com/jeetdotso/tweet-indexer-service/persistence/mongodb"
	"github.com/jeetdotso/tweet-indexer-service/stream/kafka"

	"github.com/rs/zerolog/log"
)

func main() {
	//Load app config
	var (
		cfg      = config.Load("./config/env/")
		conn     = mongodb.NewConnection(&cfg.Persistence)
		grpcconn = grpcclient.NewClientMap(&cfg.Grpc.Clients)
	)

	//Register and start scrapes
	topicNames := registerScrapes(context.Background(), *cfg, &conn, grpcconn)

	//create indexer kafka topic
	kafka.CreateTopics(cfg, topicNames)

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

	r.POST("/tweet/index/replay/:id", twtCtrl.ReplayEventsByID)
	r.POST("/tweet/index/replay", twtCtrl.ReplayEvents)

	r.POST("/tweet/index/monitors/replay/:userId", twtCtrl.ReplayEventsByUser)

	err := r.Run(":" + cfg.Server.Port)
	if err != nil {
		log.Panic().Err(err).Msg("Error starting server")
	}
}

func registerScrapes(ctx context.Context, cfg config.Config, conn *mongodb.MongoConnection, grpcconn *grpcclient.GrpcClientMap) []string {
	// deps
	var (
		tweetIndexer = service.NewTweetIndexer(cfg, conn, grpcconn)
	)

	// start tweet indexer
	go tweetIndexer.Start(ctx)

	var topicNames []string
	// add topic names
	topicNames = append(topicNames, tweetIndexer.TopicName())

	return topicNames
}
