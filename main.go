package main

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jeetdotso/tweet-indexer-service/config"
	"github.com/jeetdotso/tweet-indexer-service/dummyaccount"
	"github.com/jeetdotso/tweet-indexer-service/indexer/controller"
	"github.com/jeetdotso/tweet-indexer-service/indexer/service"
	"github.com/jeetdotso/tweet-indexer-service/persistence/mongodb"

	// accHealth "github.com/jeetdotso/tweet-indexer-service/scrape/accounts_health"
	"github.com/rs/zerolog/log"
)

func main() {
	//Load app config
	var (
		cfg  = config.Load("./config/env/")
		conn = mongodb.NewConnection(&cfg.Persistence)
		// grpcconn = grpcclient.NewClientMap(&cfg.Grpc.Clients)
	)

	//Register and start monitors
	// for _, m := range cfg.Monitors {
	// 	if m.Enabled {
	// 		blockIndexer := service.NewBlockIndexer(cfg, m)
	// 		go blockIndexer.Start()
	// 	}
	// }
	repo := dummyaccount.NewRepository(&conn)
	ctx := context.Background()
	hlp := dummyaccount.NewSeleniumHelper()
	svc := dummyaccount.NewSeleniumService(&conn, cfg)

	// repo.UpdateAllToGroup(ctx, 0)
	// accHealth.Check(ctx, 100, svc)
	// return

	auth, err := svc.FindAllByGroup(ctx, 0)
	if err != nil {
		log.Error().Err(err).Msg("Error getting accounts")
		return
	}
	maxThreads := 1

	var wg sync.WaitGroup
	sem := make(chan struct{}, maxThreads) // Limit concurrent WebDrivers

	for _, a := range auth {
		if a.TwoFAPre == "" {
			continue
		}
		wg.Add(1)
		sem <- struct{}{} // Block if too many WebDrivers are running
		////////////////////////////////////For 2fa accounts////////////////////////////////////
		if a.TwoFAPre != "" {
			go func(account *dummyaccount.SelAccount) {
				defer wg.Done()
				defer func() { <-sem }() // Release semaphore slot

				// Create a WebDriver for this worker
				wd, err := hlp.LoadWebDriverNoCookies()
				if err != nil {
					log.Printf("Error loading WebDriver: %v\n", err)
					return
				}
				defer wd.Quit() // Ensure WebDriver closes after this account

				session2fa, err := hlp.Get2FAToken(wd, account.TwoFAPre)
				if err != nil {
					log.Printf("Error getting 2fa token: %v\n", err)
					return
				}
				time.Sleep(10 * time.Second) // Wait for 2fa code to be entered

				// Log in and fetch cookies
				cookies, err := hlp.LogInAndGetCookies2fa(ctx, account.UserName, account.Password, session2fa, wd)
				if err != nil {
					log.Printf("Error logging in for user: %s\n", account.UserName)
					return
				}

				// Save cookies to DB
				account.Cookies = cookies
				account.Group = 0
				account.TwoFACode = session2fa

				err = repo.UpdateOrInsert(ctx, account)
				if err != nil {
					log.Printf("Error updating cookies for user: %s\n", account.UserName)
				}
			}(a)
			break
		} else {

			////////////////////////////////////For non 2fa accounts////////////////////////////////////
			go func(account *dummyaccount.SelAccount) {
				defer wg.Done()
				defer func() { <-sem }() // Release semaphore slot

				// Create a WebDriver for this worker
				wd, err := hlp.LoadWebDriverNoCookies()
				if err != nil {
					log.Printf("Error loading WebDriver: %v\n", err)
					return
				}
				defer wd.Quit() // Ensure WebDriver closes after this account
				// Log in and fetch cookies
				cookies, err := hlp.LogInAndGetCookies(ctx, account.UserName, account.Password, wd)
				if err != nil {
					log.Printf("Error logging in for user: %s\n", account.UserName)
					return
				}

				// Save cookies to DB
				account.Cookies = cookies
				account.Group = 0

				err = repo.UpdateOrInsert(ctx, account)
				if err != nil {
					log.Printf("Error updating cookies for user: %s\n", account.UserName)
				}
			}(a)
		}
	}
	//create indexer kafka topics
	// kafka.CreateTopics(cfg)

	//Register http endpoints
	// registerHttp(cfg)
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
