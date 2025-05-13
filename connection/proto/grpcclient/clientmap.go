package grpcclient

import (
	"github.com/jeetdotso/tweet-indexer-service/config"
)

type GrpcClientMap struct {
	AccountableCli AccountableClient
	SasCli         SaasClient
}

func NewClientMap(clis *config.GrpcClients) *GrpcClientMap {

	accountablecli := NewAccountableClient(clis.AccountableServices.Host)
	saasCli := NewSaasClient(clis.SaasServices.Host)
	climap := &GrpcClientMap{
		AccountableCli: accountablecli,
		SasCli:         saasCli,
	}
	return climap
}
