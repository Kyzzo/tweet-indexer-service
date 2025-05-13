package user

import (
	"context"

	"github.com/jeetdotso/tweet-indexer-service/config"
	"github.com/jeetdotso/tweet-indexer-service/connection/proto/grpcclient"
	"github.com/jeetdotso/tweet-indexer-service/connection/proto/grpctwitteruser"
	"github.com/jeetdotso/tweet-indexer-service/connection/twitteruser"
)

type TwitteruserService interface {
	FindUserByTwitterHandle(ctx context.Context, twitterHandle string) (*twitteruser.User, error)
	FindAllRawUsers(ctx context.Context) ([]twitteruser.User, error)
	CheckHealth(ctx context.Context) error
	FindUserById(ctx context.Context, id string) (*twitteruser.User, error)
}
type TwitteruserServiceImpl struct {
	cfg     config.Services
	acctCli grpctwitteruser.UserServiceClient
}

func NewTwitterUserService(grpcconn *grpcclient.GrpcClientMap, cfg config.Services) TwitteruserService {
	return &TwitteruserServiceImpl{
		cfg:     cfg,
		acctCli: grpctwitteruser.NewUserServiceClient(grpcconn.AccountableCli.Session),
	}
}

func (svc *TwitteruserServiceImpl) FindUserByTwitterHandle(ctx context.Context, twitterHandle string) (*twitteruser.User, error) {
	//try from repo first
	req := &grpctwitteruser.FindUserRequest{TwitterHandle: twitterHandle}
	user, err := svc.acctCli.FindUserByTwitterHandle(ctx, req)
	if err != nil {
		return nil, err
	}
	var usr = user.GetUser()

	var userBack = &twitteruser.User{
		ID:           usr.Id,
		Handle:       usr.TwitterHandle,
		UserId:       usr.Id,
		UserName:     usr.Name,
		ProfileImg:   usr.ProfileImg,
		ProfileImgS3: usr.ProfileImgS3,
		Followers:    int(usr.FollowerCount),
		Following:    int(usr.FollowingCount),
		Verified:     usr.Verified,
		CreatedAt:    usr.CreatedAt.AsTime(),
		UpdatedAt:    usr.UpdatedAt.AsTime(),
	}

	return userBack, nil

}

func (svc *TwitteruserServiceImpl) FindAllRawUsers(ctx context.Context) ([]twitteruser.User, error) {
	//try from repo first
	users, err := svc.acctCli.FindAllRawUsers(ctx, &grpctwitteruser.EmptyRequest{})
	if err != nil {
		return nil, err
	}
	var usersBack []twitteruser.User
	for _, usr := range users.Users {
		var user = &twitteruser.User{
			ID:           usr.Id,
			Handle:       usr.TwitterHandle,
			UserId:       usr.Id,
			UserName:     usr.Name,
			ProfileImg:   usr.ProfileImg,
			ProfileImgS3: usr.ProfileImgS3,
			Followers:    int(usr.FollowerCount),
			Following:    int(usr.FollowingCount),
			Verified:     usr.Verified,
			CreatedAt:    usr.CreatedAt.AsTime(),
			UpdatedAt:    usr.UpdatedAt.AsTime(),
		}
		usersBack = append(usersBack, *user)
	}

	return usersBack, nil
}

func (svc *TwitteruserServiceImpl) CheckHealth(ctx context.Context) error {
	_, err := svc.acctCli.CheckHealth(ctx, &grpctwitteruser.EmptyRequest{})
	return err

}

func (svc *TwitteruserServiceImpl) FindUserById(ctx context.Context, id string) (*twitteruser.User, error) {
	//try from repo first
	req := &grpctwitteruser.FindUserByIdRequest{Id: id}
	user, err := svc.acctCli.FindUserById(ctx, req)
	if err != nil {
		return nil, err
	}
	var usr = user.GetUser()

	var userBack = &twitteruser.User{
		ID:           usr.Id,
		Handle:       usr.TwitterHandle,
		UserId:       usr.Id,
		UserName:     usr.Name,
		ProfileImg:   usr.ProfileImg,
		ProfileImgS3: usr.ProfileImgS3,
		Followers:    int(usr.FollowerCount),
		Following:    int(usr.FollowingCount),
		Verified:     usr.Verified,
		CreatedAt:    usr.CreatedAt.AsTime(),
		UpdatedAt:    usr.UpdatedAt.AsTime(),
	}

	return userBack, nil

}
