package grpcclient

import (
	"github.com/jeetdotso/tweet-indexer-service/config"
)

type GrpcClientMap struct {
	AccountableCli AccountableClient
}

func NewClientMap(clis *config.GrpcClients) *GrpcClientMap {

	accountablecli := NewAccountableClient(clis.AccountableServices.Host)
	climap := &GrpcClientMap{
		AccountableCli: accountablecli,
	}
	return climap
}
