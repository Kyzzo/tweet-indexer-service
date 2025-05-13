package saas

import (
	"context"

	"github.com/jeetdotso/tweet-indexer-service/config"
	"github.com/jeetdotso/tweet-indexer-service/connection/proto/grpcclient"
	"github.com/jeetdotso/tweet-indexer-service/connection/proto/grpccustomer"
	"github.com/jeetdotso/tweet-indexer-service/connection/twitteruser"
)

type SassUserService interface {
	FindAllTracked(ctx context.Context) ([]twitteruser.User, error)
	CheckHealth(ctx context.Context) error
}
type SaasService struct {
	cfg     config.Services
	saasCli grpccustomer.CustomerServiceClient
}

func NewSassUserService(grpcconn *grpcclient.GrpcClientMap, cfg config.Services) SassUserService {
	return &SaasService{
		cfg:     cfg,
		saasCli: grpccustomer.NewCustomerServiceClient(grpcconn.SasCli.Session),
	}
}

func (svc *SaasService) FindAllTracked(ctx context.Context) ([]twitteruser.User, error) {
	//try from repo first
	users, err := svc.saasCli.GetTrackedKols(ctx, &grpccustomer.EmptyRequest{})
	if err != nil {
		return nil, err
	}
	var usersBack []twitteruser.User
	for _, usr := range users.Accounts {
		var user = twitteruser.User{
			ID:           usr.Id,
			Handle:       usr.Handle,
			UserId:       usr.Id,
			UserName:     usr.UserName,
			ProfileImg:   usr.ProfileImg,
			ProfileImgS3: usr.ProfileImgS3,
			Followers:    int(usr.Followers),
			Following:    int(usr.Following),
			Verified:     usr.Verified,
			CreatedAt:    usr.CreatedAt.AsTime(),
			UpdatedAt:    usr.UpdatedAt.AsTime(),
		}
		usersBack = append(usersBack, user)
	}

	return usersBack, nil
}

func (svc *SaasService) CheckHealth(ctx context.Context) error {
	_, err := svc.saasCli.CheckHealth(ctx, &grpccustomer.EmptyRequest{})
	return err

}
