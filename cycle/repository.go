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

const Collection = "cycle.scrape"

const HealthCollection = "cycle.health"

type Repository interface {
	UpdateOrInsert(ctx context.Context, pl *ScrapeCycle) error
	FindById(ctx context.Context, id primitive.ObjectID) (*ScrapeCycle, error)
	FindLatest(ctx context.Context) (*ScrapeCycle, error)

	//HEALTH CYCLE
	FindHealthById(ctx context.Context, id primitive.ObjectID) (*HealthCycle, error)
	FindLatestHealth(ctx context.Context) (*HealthCycle, error)
	FindAllHealth(ctx context.Context) ([]*HealthCycle, error)
	UpdateOrInsertHealth(ctx context.Context, pl *HealthCycle) error
}
type MongoRepository struct {
	conn          *mongodb.MongoConnection
	absrepo       *persistence.AbstractMongoRepository[*ScrapeCycle]
	absHealthRepo *persistence.AbstractMongoRepository[*HealthCycle]
}

func NewRepository(conn *mongodb.MongoConnection) Repository {
	absrepo := persistence.NewAbstractRepository[*ScrapeCycle](conn, Collection)
	absHealthRepo := persistence.NewAbstractRepository[*HealthCycle](conn, HealthCollection)
	return &MongoRepository{
		conn:          conn,
		absrepo:       absrepo,
		absHealthRepo: absHealthRepo,
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

// /////////////////////////////Health Cycle////////////////////////////////////
func (repo *MongoRepository) FindHealthById(ctx context.Context, id primitive.ObjectID) (*HealthCycle, error) {
	filter := bson.M{"_id": id}
	var health HealthCycle
	res := repo.conn.Datastore.Collection(repo.absHealthRepo.Collection).FindOne(ctx, filter)
	if err := res.Decode(&health); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error decoding health cycle.")
		return nil, err
	}
	return &health, nil
}

func (repo *MongoRepository) FindLatestHealth(ctx context.Context) (*HealthCycle, error) {
	opts := options.FindOne().SetSort(bson.D{{"start_time", -1}})
	var health HealthCycle
	res := repo.conn.Datastore.Collection(repo.absHealthRepo.Collection).FindOne(ctx, bson.D{}, opts)
	if err := res.Decode(&health); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error decoding health cycle.")
		return nil, err
	}
	return &health, nil
}

func (repo *MongoRepository) FindAllHealth(ctx context.Context) ([]*HealthCycle, error) {
	filter := bson.D{}
	opts := options.Find().SetSort(bson.D{{"start_time", -1}})
	cursor, err := repo.conn.Datastore.Collection(repo.absHealthRepo.Collection).Find(ctx, filter, opts)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error finding all health cycles.")
		return nil, err
	}
	defer cursor.Close(ctx)

	var healthCycles []*HealthCycle
	for cursor.Next(ctx) {
		var health HealthCycle
		if err := cursor.Decode(&health); err != nil {
			logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error decoding health cycle.")
			return nil, err
		}
		healthCycles = append(healthCycles, &health)
	}

	return healthCycles, nil
}

func (repo *MongoRepository) UpdateOrInsertHealth(ctx context.Context, pl *HealthCycle) error {
	err := repo.absHealthRepo.InsertOrUpdate(pl)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error updating or inserting health cycle.")
		return err
	}
	return nil
}
