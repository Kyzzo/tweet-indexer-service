package main

import (
	"context"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jeetdotso/tweet-indexer-service/config"
	"github.com/jeetdotso/tweet-indexer-service/connection/proto/grpcclient"
	"github.com/jeetdotso/tweet-indexer-service/dummyaccount"
	"github.com/jeetdotso/tweet-indexer-service/indexer/controller"
	"github.com/jeetdotso/tweet-indexer-service/indexer/controller/indexergrpchandler"
	"github.com/jeetdotso/tweet-indexer-service/indexer/controller/reviewgrpchandler"
	sched "github.com/jeetdotso/tweet-indexer-service/indexer/scheduler"
	"github.com/jeetdotso/tweet-indexer-service/indexer/service"
	"github.com/jeetdotso/tweet-indexer-service/logg"
	"github.com/jeetdotso/tweet-indexer-service/persistence/mongodb"
	"github.com/jeetdotso/tweet-indexer-service/proto/grpcreviewentry"
	"github.com/jeetdotso/tweet-indexer-service/proto/grpcsaasentry"
	"github.com/jeetdotso/tweet-indexer-service/scheduler"
	"github.com/jeetdotso/tweet-indexer-service/shared"
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

	// Root context that we'll cancel on SIGTERM/SIGINT
	rootCtx, rootCancel := context.WithCancel(context.Background())
	defer rootCancel()

	// Signal handler
	sigCh := make(chan os.Signal, 2)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	dummSvc := dummyaccount.NewSeleniumService(&conn, cfg)
	var tagsToRun []twitterprofile.TagType = shared.RUNNING_TAGS_TYPE
	dummSvc.ResetAll(rootCtx)
	//Register Scrapes
	_, tweetIndexer, saasIdx := registerScrapes(rootCtx, *cfg, &conn, grpcconn, tagsToRun)

	//Register GRPC server
	// gRPC
	gb, err := makeGRPCServer(cfg, &conn, grpcconn, dummSvc)
	if err != nil {
		logg.Log.Panic().Err(err).Msg("failed to init gRPC server")
	}
	go func() {
		logg.Log.Info().Msgf("gRPC listening on :%s", cfg.Grpc.Server.Port)
		if err := gb.srv.Serve(gb.lis); err != nil && err != grpc.ErrServerStopped {
			logg.Log.Panic().Err(err).Msg("gRPC server error")
		}
	}()
	//Register Schedulers
	cron_sched := makeCron(&conn, cfg, dummSvc, saasIdx, tagsToRun)
	cron_sched.Start()

	// HTTP
	httpSrv := makeHTTPServer(cfg, &conn, dummSvc)
	go func() {
		logg.Log.Info().Msgf("HTTP listening on :%s", cfg.Server.Port)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Panic().Err(err).Msg("HTTP server error")
		}
	}()

	// ======= Wait for shutdown signal =======
	sig := <-sigCh
	logg.Log.Warn().Str("signal", sig.String()).Msg("shutdown requested")

	// 1) Cancel root context → long loops should exit
	rootCancel()

	tweetIndexer.StopGrokPool() // stop tweet indexer

	cron_sched.Stop()
	// 3) Gracefully stop gRPC
	doneCh := make(chan struct{})
	go func() {
		gb.srv.GracefulStop()
		close(doneCh)
	}()

	select {
	case <-doneCh:
	case <-time.After(10 * time.Second):
		logg.Log.Warn().Msg("gRPC GracefulStop timed out; forcing Stop")
		gb.srv.Stop()
	}

	// 4) Gracefully stop HTTP
	shCtx, shCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shCancel()
	if err := httpSrv.Shutdown(shCtx); err != nil {
		logg.Log.Error().Err(err).Msg("HTTP shutdown error")
	}

	logg.Log.Info().Msg("shutdown complete")

}

// ---- HTTP wiring (uses http.Server for graceful shutdown) ----
func makeHTTPServer(cfg *config.Config, conn *mongodb.MongoConnection, dummSvc dummyaccount.Service) *http.Server {
	api := service.NewTweetAPI(*cfg, conn)
	twtCtrl := controller.NewTweetIndexApi(api)
	adminCtrl := controller.NewAdminController(cfg, conn, dummSvc)
	adminPnlCtrl := controller.NewAdminPanleController(cfg, conn, dummSvc)

	r := gin.New()
	r.Use(gin.Recovery())

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
	r.POST("/public/indexer/admin/single/list", adminCtrl.AddSingleToList)
	// admin panel
	r.GET("/public/indexer/admin-panel/scrape/cycle/:type", adminPnlCtrl.FindByCycleTypeAndLimit)
	r.GET("/public/indexer/admin-panel/scrape/cycle/latest", adminPnlCtrl.GetLatestScrapeCycle)
	r.GET("/public/indexer/admin-panel/health/cycle/latest", adminPnlCtrl.GetHealthCycle)
	r.GET("/public/indexer/admin-panel/health/cycle", adminPnlCtrl.GetAllHealthCycles)
	r.GET("/public/indexer/admin-panel/following/cycle/latest/:type", adminPnlCtrl.GetFollowingCycle)
	r.GET("/public/indexer/admin-panel/following/cycle", adminPnlCtrl.GetAllFollowingCycles)
	r.GET("/public/indexer/admin-panel/grok-chart", adminPnlCtrl.GetGrokDataCycles)

	srv := &http.Server{
		Addr:    ":" + cfg.Server.Port,
		Handler: r,
	}
	return srv
}

func registerScrapes(ctx context.Context, cfg config.Config, conn *mongodb.MongoConnection, grpcconn *grpcclient.GrpcClientMap, tagsToRun []twitterprofile.TagType) ([]string, *service.TokenTweetIndexer, service.SaasIndexer) {
	// deps
	var (
		saasIndexer       = service.NewSaasIndexer(cfg, conn)
		tokenTweetIndexer = service.NewTokenTweetIndexer(cfg, conn, grpcconn, saasIndexer)
	)

	var topicNames []string
	// add topic names
	topicNames = append(topicNames, saasIndexer.TopicNames()...)

	// start tweet indexer
	for _, tag := range tagsToRun {
		go tokenTweetIndexer.Start(ctx, tag)
	}

	return topicNames, tokenTweetIndexer, saasIndexer
}

func makeCron(conn *mongodb.MongoConnection, cfg *config.Config, dummSvc dummyaccount.Service, saasIdx service.SaasIndexer, tags []twitterprofile.TagType) *cron.Cron {
	c := cron.New()
	for _, job := range Schedulers(conn, cfg, dummSvc, saasIdx, tags) {
		_ = c.AddFunc(job.Expression(), job.Execute)
	}
	return c
}

func Schedulers(conn *mongodb.MongoConnection, cfg *config.Config, dummSvc dummyaccount.Service, saasIdx service.SaasIndexer, tags []twitterprofile.TagType) []scheduler.Scheduler {
	var (
		grokRetry   = sched.NewGrokRetryScheduler(cfg, conn, dummSvc, saasIdx)
		healthCheck = sched.NewAccountHealthScheduler(cfg, conn, dummSvc)
		scrapeCycle = sched.NewScrapeCycleScheduler(cfg, conn, tags)
		followCycle = sched.NewFollowingCycleScheduler(cfg, conn, dummSvc, tags)
		prfAnalysis = sched.NewProfileAnalysis(cfg, conn, dummSvc)
		prfUpdater  = sched.NewProfileUpdater(cfg, conn, dummSvc)
	)

	return []scheduler.Scheduler{
		grokRetry,
		healthCheck,
		followCycle,
		scrapeCycle,
		prfAnalysis,
		prfUpdater,
	}
}

// ---- gRPC wiring (we keep server+listener so we can stop gracefully) ----
type grpcBundle struct {
	srv *grpc.Server
	lis net.Listener
}

func makeGRPCServer(cfg *config.Config, conn *mongodb.MongoConnection, grpcconn *grpcclient.GrpcClientMap, dummSvc dummyaccount.Service) (*grpcBundle, error) {
	lis, err := net.Listen("tcp", ":"+cfg.Grpc.Server.Port)
	if err != nil {
		return nil, err
	}
	srv := grpc.NewServer()

	saasEntryCtrl := indexergrpchandler.NewSaasGRPCHandler(cfg, conn, dummSvc)
	grpcsaasentry.RegisterSaasEntryServiceServer(srv, saasEntryCtrl)

	reviewEntryCtrl := reviewgrpchandler.NewReviewGRPCHandler(cfg, conn)
	grpcreviewentry.RegisterReviewEntryServiceServer(srv, reviewEntryCtrl)

	reflection.Register(srv)
	return &grpcBundle{srv: srv, lis: lis}, nil
}
