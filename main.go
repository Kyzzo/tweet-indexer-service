package main

import (
	"context"
	"net"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jeetdotso/tweet-indexer-service/config"
	"github.com/jeetdotso/tweet-indexer-service/connection/proto/grpcclient"
	"github.com/jeetdotso/tweet-indexer-service/dummyaccount"
	"github.com/jeetdotso/tweet-indexer-service/indexer/controller"
	"github.com/jeetdotso/tweet-indexer-service/indexer/controller/indexergrpchandler"
	sched "github.com/jeetdotso/tweet-indexer-service/indexer/scheduler"
	"github.com/jeetdotso/tweet-indexer-service/indexer/service"
	"github.com/jeetdotso/tweet-indexer-service/logg"
	"github.com/jeetdotso/tweet-indexer-service/persistence/mongodb"
	"github.com/jeetdotso/tweet-indexer-service/proto/grpcsaasentry"
	"github.com/jeetdotso/tweet-indexer-service/scheduler"
	"github.com/jeetdotso/tweet-indexer-service/twitterprofile"
	"github.com/robfig/cron"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	//Load app config
	var (
		cfg      = config.Load("./config/env/")
		conn     = mongodb.NewConnection(&cfg.Persistence)
		grpcconn = grpcclient.NewClientMap(&cfg.Grpc.Clients)
	)

	ctx := context.Background()

	dummSvc := dummyaccount.NewSeleniumService(&conn, cfg)
	var tagsToRun []twitterprofile.TagType = []twitterprofile.TagType{twitterprofile.TAG_TRADER}
	dummSvc.ResetAll(ctx)
	// // // //Register Scrapes
	_, saasIdx := registerScrapes(ctx, *cfg, &conn, grpcconn, tagsToRun)

	// //Register GRPC server
	go registerGrpcServer(cfg, &conn, grpcconn, dummSvc)
	// //Register Schedulers
	go registerSchedulers(cfg, &conn, dummSvc, saasIdx, tagsToRun)

	// //Register http endpoints
	registerHttp(cfg, &conn, dummSvc)
}

func registerHttp(cfg *config.Config, conn *mongodb.MongoConnection, dummSvc dummyaccount.Service) {
	//deps
	var (
		api = service.NewTweetAPI(*cfg, conn)
	)
	twtCtrl := controller.NewTweetIndexApi(api)
	adminCtrl := controller.NewAdminController(cfg, conn, dummSvc)
	adminPnlCtrl := controller.NewAdminPanleController(cfg, conn, dummSvc)

	r := gin.Default()
	r.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"response": "Tweet Indexer Health OK"})
	})

	r.POST("/public/indexer/replay/:id", twtCtrl.ReplayEventsByID)
	r.POST("/public/indexer/replay", twtCtrl.ReplayEvents)
	r.GET("/public/indexer/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"response": "Tweet indexer Health OK"})
	})
	r.POST("/public/indexer/monitors/replay/:userId", twtCtrl.ReplayEventsByUser)
	r.POST("/public/indexer/admin/:username", adminCtrl.AddList)
	r.PUT("/public/indexer/admin/status/:id", adminCtrl.ChangeStatusById)
	r.PUT("/public/indexer/admin/tag/:id", adminCtrl.ChangeTagByID)
	r.GET("/public/indexer/admin/unique/tags", adminCtrl.FindTagOptions)
	r.PUT("/public/indexer/admin/status/multi", adminCtrl.ChangeStatusMultiple)
	r.GET("/public/indexer/admin/list", adminCtrl.GetList)
	r.POST("/public/indexer/admin/multi/manual", adminCtrl.AddMultiManual)
	r.GET("/public/indexer/admin/top", adminCtrl.GetTop1500)
	//admin panel
	r.GET("/public/indexer/admin-panel/scrape/cycle/:type", adminPnlCtrl.FindByCycleTypeAndLimit)
	r.GET("/public/indexer/admin-panel/scrape/cycle/latest", adminPnlCtrl.GetLatestScrapeCycle)
	r.GET("/public/indexer/admin-panel/health/cycle/latest", adminPnlCtrl.GetHealthCycle)
	r.GET("/public/indexer/admin-panel/health/cycle", adminPnlCtrl.GetAllHealthCycles)
	r.GET("/public/indexer/admin-panel/following/cycle/latest/:type", adminPnlCtrl.GetFollowingCycle)
	r.GET("/public/indexer/admin-panel/following/cycle", adminPnlCtrl.GetAllFollowingCycles)
	err := r.Run(":" + cfg.Server.Port)
	if err != nil {
		log.Panic().Err(err).Msg("Error starting server")
	}
}

func registerScrapes(ctx context.Context, cfg config.Config, conn *mongodb.MongoConnection, grpcconn *grpcclient.GrpcClientMap, tagsToRun []twitterprofile.TagType) ([]string, service.SaasIndexer) {
	// deps
	var (
		saasIndexer = service.NewSaasIndexer(cfg, conn)
		// tokenTweetIndexer = service.NewTokenTweetIndexer(cfg, conn, grpcconn, saasIndexer)
	)

	var topicNames []string
	// add topic names
	topicNames = append(topicNames, saasIndexer.TopicNames()...)

	// start tweet indexer
	// for _, tag := range tagsToRun {
	// 	go tokenTweetIndexer.Start(ctx, tag)
	// }

	return topicNames, saasIndexer
}

func registerSchedulers(cfg *config.Config, conn *mongodb.MongoConnection, dummSvc dummyaccount.Service, saasIdx service.SaasIndexer, tags []twitterprofile.TagType) {
	c := cron.New()
	jobs := Schedulers(conn, cfg, dummSvc, saasIdx, tags)
	for _, job := range jobs {
		_ = c.AddFunc(job.Expression(), job.Execute)
	}
	c.Start()
}

func Schedulers(conn *mongodb.MongoConnection, cfg *config.Config, dummSvc dummyaccount.Service, saasIdx service.SaasIndexer, tags []twitterprofile.TagType) []scheduler.Scheduler {
	var (
		grokRetry   = sched.NewGrokRetryScheduler(cfg, conn, dummSvc, saasIdx)
		healthCheck = sched.NewAccountHealthScheduler(cfg, conn, dummSvc)
		scrapeCycle = sched.NewScrapeCycleScheduler(cfg, conn, tags)
		followCycle = sched.NewFollowingCycleScheduler(cfg, conn, dummSvc, tags)
		prfAnalysis = sched.NewProfileAnalysis(cfg, conn, dummSvc)
	)

	return []scheduler.Scheduler{
		grokRetry,
		healthCheck,
		followCycle,
		scrapeCycle,
		prfAnalysis,
	}
}

func registerGrpcServer(cfg *config.Config, conn *mongodb.MongoConnection, grpcconn *grpcclient.GrpcClientMap, dummSvc dummyaccount.Service) {
	lis, err := net.Listen("tcp", ":"+cfg.Grpc.Server.Port)
	if err != nil {
		logg.Log.Panic().Err(err).Msgf("failed to listen on port %s", cfg.Grpc.Server.Port)
	}

	grpcsrv := grpc.NewServer()

	// Register Saas Entry GRPC
	saasEntryCtrl := indexergrpchandler.NewSaasGRPCHandler(cfg, conn, dummSvc)
	grpcsaasentry.RegisterSaasEntryServiceServer(grpcsrv, saasEntryCtrl)

	reflection.Register(grpcsrv)

	if err = grpcsrv.Serve(lis); err != nil {
		logg.Log.Panic().Err(err).Msgf("failed to serve gRPC on port %s", cfg.Grpc.Server.Port)
		return
	}
	logg.Log.Info().Msgf("grpc server started on port %s", cfg.Grpc.Server.Port)
}
