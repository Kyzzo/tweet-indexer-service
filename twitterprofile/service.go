package twitterprofile

import (
	"context"

	"github.com/jeetdotso/tweet-indexer-service/config"
	"github.com/jeetdotso/tweet-indexer-service/logg"
	"github.com/jeetdotso/tweet-indexer-service/persistence/mongodb"
	twitterTypes "github.com/jeetdotso/tweet-indexer-service/scrape/internals/api/types"
)

type Service interface {
	UpdateOrInsertUser(ctx context.Context, usr *twitterTypes.FollowingUser) error
	FindUserByName(ctx context.Context, usrName string) (*twitterTypes.FollowingUser, error)
	GetAllTrackedUsers(ctx context.Context) ([]*twitterTypes.FollowingUser, error)
}
type ServiceDefaultImpl struct {
	repo Repository
}

func NewUserService(conn *mongodb.MongoConnection, cfg *config.Config) Service {
	repos := NewRepository(conn)
	return &ServiceDefaultImpl{
		repo: repos,
	}
}

func (svc *ServiceDefaultImpl) UpdateOrInsertUser(ctx context.Context, usr *twitterTypes.FollowingUser) error {

	err := svc.repo.UpdateOrInsert(ctx, usr)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Str("handle", usr.Handle).Msg("Error updating user followings.")
		return err
	}

	return nil
}

func (svc *ServiceDefaultImpl) FindUserByName(ctx context.Context, usrName string) (*twitterTypes.FollowingUser, error) {
	usr, err := svc.repo.FindUserByName(ctx, usrName)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Str("handle", usrName).Msg("Error finding user by name.")
		return nil, err
	}
	return usr, nil
}

func (svc *ServiceDefaultImpl) GetAllTrackedUsers(ctx context.Context) ([]*twitterTypes.FollowingUser, error) {
	users, err := svc.repo.FindByIsTracking(ctx, true)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error getting all tracked users.")
		return nil, err
	}
	return users, nil
}
