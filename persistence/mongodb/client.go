package mongodb

import (
	"context"

	"github.com/jeetdotso/tweet-indexer-service/config"
	"github.com/jeetdotso/tweet-indexer-service/logg"
	"go.opentelemetry.io/contrib/instrumentation/go.mongodb.org/mongo-driver/mongo/otelmongo"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	DefaultUrl = "mongodb://localhost:27017"
)

type MongoConnection struct {
	Session   *mongo.Client
	Datastore *mongo.Database
	cfg       *config.Persistence
}

func NewConnection(cfg *config.Persistence) MongoConnection {
	cli := MongoConnection{cfg: cfg}
	cli.connect()
	return cli
}

func (con *MongoConnection) connect() {
	opts := options.Client()
	opts.Monitor = otelmongo.NewMonitor()
	opts.ApplyURI(getUrlConnection(con.cfg))
	client, err := mongo.Connect(context.Background(), opts)
	if err != nil {
		logg.Log.Panic().Err(err).Msg("Error connecting to mongodb.")
	}
	if defaultDB := getDefaultDatabase(con.cfg); len(defaultDB) > 0 {
		con.Datastore = client.Database(defaultDB)
	}
	con.Session = client
}

func getDefaultDatabase(cfg *config.Persistence) string {
	if len(cfg.MongoDB.Database) > 0 {
		return cfg.MongoDB.Database
	} else {
		return ""
	}
}

func getUrlConnection(cfg *config.Persistence) string {
	if len(cfg.MongoDB.URL) > 0 {
		return cfg.MongoDB.URL
	} else {
		return DefaultUrl
	}
}
