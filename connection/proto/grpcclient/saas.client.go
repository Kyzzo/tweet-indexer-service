package grpcclient

import (
	"github.com/jeetdotso/tweet-indexer-service/logg"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type SaasClient struct {
	Session *grpc.ClientConn
	host    string
}

func NewSaasClient(host string) SaasClient {
	cli := SaasClient{host: host}
	cli.connect()
	return cli

}

func (con *SaasClient) connect() {
	cli, err := grpc.Dial(con.host, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logg.Log.Error().Caller().Err(err).Msg("failed to do grpc connection")
		panic(err)
	}
	con.Session = cli
}
