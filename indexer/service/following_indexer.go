package service

import (
	"context"
	"fmt"
	"time"

	"github.com/jeetdotso/tweet-indexer-service/config"
	"github.com/jeetdotso/tweet-indexer-service/connection/proto/grpcclient"
	"github.com/jeetdotso/tweet-indexer-service/dummyaccount"
	"github.com/jeetdotso/tweet-indexer-service/logg"
	"github.com/jeetdotso/tweet-indexer-service/persistence/mongodb"
	twitterTypes "github.com/jeetdotso/tweet-indexer-service/scrape/internals/api/types"
	"github.com/jeetdotso/tweet-indexer-service/scrape/tracking"
)

// scrape following list via id
// update list (hopefully by first page only)
// update list by id
// receive who's following by followed id
// following list by id
// following list by id true | false if tracked
// get followed by list via id w/ excluding ids
type FollowingINdexer struct {
	// producer stream.Producer
	dummSvc dummyaccount.Service
}

func NewFollowingIndexer(cfg config.Config, conn *mongodb.MongoConnection, grpcconn *grpcclient.GrpcClientMap) FollowingINdexer {
	dummSvc := dummyaccount.NewSeleniumService(conn, &cfg)
	// producer := kafka.NewProducer(cfg.Stream, TOPIC_CON)

	return FollowingINdexer{
		// producer: producer,
		dummSvc: dummSvc,
	}
}

func (f *FollowingINdexer) ScrapeFollowingListById(ctx context.Context, id string) {

	fm, sell, err := tracking.NewFollowingMonitor(ctx, f.dummSvc)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Failed to create following monitor.")
		return
	}
	defer func() {
		if err = f.dummSvc.LogOut(ctx, sell.UserName); err != nil {
			logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Failed to log out.")
		}
	}()
	err = fm.MonitorSingleAccount(ctx, id, 3, time.Second*5, f.HandleFollowing)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Failed to monitor single account.")
		return
	}
}

func (f *FollowingINdexer) HandleFollowing(ctx context.Context, followedByID string, user twitterTypes.FollowingUser) {
	fmt.Printf("\nID: %s, is following: %s, handle: %s, is verified: %t\n", followedByID, user.UserName, user.Handle, user.Verified)
}
