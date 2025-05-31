package service

import (
	"context"

	"github.com/jeetdotso/tweet-indexer-service/config"
	"github.com/jeetdotso/tweet-indexer-service/dummyaccount"
	"github.com/jeetdotso/tweet-indexer-service/logg"
	"github.com/jeetdotso/tweet-indexer-service/persistence/mongodb"
	twitterTypes "github.com/jeetdotso/tweet-indexer-service/scrape/internals/api/types"
	"github.com/jeetdotso/tweet-indexer-service/scrape/tracking"
)

// get a profile by username
type Profile struct {
	dummSvc dummyaccount.Service
}

func NewProfile(cfg config.Config, conn *mongodb.MongoConnection, dummySvc dummyaccount.Service) Profile {
	return Profile{
		dummSvc: dummySvc,
	}
}

func (p *Profile) GetProfileByUsername(ctx context.Context, username string) (*twitterTypes.User, error) {
	twtApi, selAcc, err := tracking.NewAccountMonitor(ctx, p.dummSvc)
	if err != nil {
		logg.Log.Error().Caller().Msgf("Error creating account monitor: %v", err)
		return nil, err
	}
	defer func() {
		if err = p.dummSvc.LogOut(ctx, selAcc.UserName); err != nil {
			logg.Log.Error().Caller().Msgf("Error logging out dummy account: %v", err)
		}
	}()
	return twtApi.GetUser(ctx, username)
}
