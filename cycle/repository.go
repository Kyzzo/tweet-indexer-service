package cycle

import (
	"context"

	"github.com/jeetdotso/tweet-indexer-service/logg"
	"github.com/jeetdotso/tweet-indexer-service/persistence"
	"github.com/jeetdotso/tweet-indexer-service/persistence/mongodb"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const Collection = "cycle"

type Repository interface {
	UpdateOrInsert(ctx context.Context, pl *ScrapeCycle) error
	FindById(ctx context.Context, id primitive.ObjectID) (*ScrapeCycle, error)
	FindLatest(ctx context.Context) (*ScrapeCycle, error)
}
type MongoRepository struct {
	conn    *mongodb.MongoConnection
	absrepo *persistence.AbstractMongoRepository[*ScrapeCycle]
}

func NewRepository(conn *mongodb.MongoConnection) Repository {
	absrepo := persistence.NewAbstractRepository[*ScrapeCycle](conn, Collection)
	return &MongoRepository{
		conn:    conn,
		absrepo: absrepo,
	}
}

func (repo *MongoRepository) UpdateOrInsert(ctx context.Context, pl *ScrapeCycle) error {
	err := repo.absrepo.InsertOrUpdate(pl)
	if err != nil {
		return err
	}
	return nil
}

func (repo *MongoRepository) FindById(ctx context.Context, id primitive.ObjectID) (*ScrapeCycle, error) {
	filter := bson.M{"_id": id}
	var pool ScrapeCycle
	res := repo.conn.Datastore.Collection(repo.absrepo.Collection).FindOne(ctx, filter)
	if err := res.Decode(&pool); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error decoding scraper account.")
		return nil, err
	}
	return &pool, nil
}

func (repo *MongoRepository) FindLatest(ctx context.Context) (*ScrapeCycle, error) {
	opts := options.FindOne().SetSort(bson.D{{"start_time", -1}})
	var pool ScrapeCycle
	res := repo.conn.Datastore.Collection(repo.absrepo.Collection).FindOne(ctx, bson.D{}, opts)
	if err := res.Decode(&pool); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error decoding scraper account.")
		return nil, err
	}
	return &pool, nil
}
