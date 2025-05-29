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

const (
	Collection = "cycle.scrape"

	HealthCollection    = "cycle.health"
	FollowingCollection = "cycle.following"
)

type Repository interface {
	UpdateOrInsert(ctx context.Context, pl *ScrapeCycle) error
	FindById(ctx context.Context, id primitive.ObjectID) (*ScrapeCycle, error)
	FindLatest(ctx context.Context) (*ScrapeCycle, error)

	//HEALTH CYCLE
	FindHealthById(ctx context.Context, id primitive.ObjectID) (*HealthCycle, error)
	FindLatestHealth(ctx context.Context) (*HealthCycle, error)
	FindAllHealth(ctx context.Context) ([]*HealthCycle, error)
	UpdateOrInsertHealth(ctx context.Context, pl *HealthCycle) error
	// Following Cycle
	FindFollowingLatest(ctx context.Context, ct CycleType) (*FollowingCycle, error)
	CreateNewFollowingCycle(ctx context.Context, ct CycleType) error
	FindFollowingById(ctx context.Context, id primitive.ObjectID) (*FollowingCycle, error)
	UpdateOrInsertFollowing(ctx context.Context, pl *FollowingCycle) error
	FindAllFollowing(ctx context.Context) ([]*FollowingCycle, error)
	FindAllFollowingByType(ctx context.Context, ct CycleType) ([]*FollowingCycle, error)
}
type MongoRepository struct {
	conn             *mongodb.MongoConnection
	absrepo          *persistence.AbstractMongoRepository[*ScrapeCycle]
	absHealthRepo    *persistence.AbstractMongoRepository[*HealthCycle]
	absFollowingRepo *persistence.AbstractMongoRepository[*FollowingCycle]
}

func NewRepository(conn *mongodb.MongoConnection) Repository {
	absrepo := persistence.NewAbstractRepository[*ScrapeCycle](conn, Collection)
	absHealthRepo := persistence.NewAbstractRepository[*HealthCycle](conn, HealthCollection)
	absFollowingRepo := persistence.NewAbstractRepository[*FollowingCycle](conn, FollowingCollection)
	return &MongoRepository{
		conn:             conn,
		absrepo:          absrepo,
		absHealthRepo:    absHealthRepo,
		absFollowingRepo: absFollowingRepo,
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

// /////////////////////////// Following Cycle ////////////////////////////////////
func (repo *MongoRepository) FindFollowingLatest(ctx context.Context, ct CycleType) (*FollowingCycle, error) {
	opts := options.FindOne().SetSort(bson.D{{"start_time", -1}})
	filter := bson.M{"cycle_type": ct}
	var cycle FollowingCycle
	res := repo.conn.Datastore.Collection(repo.absFollowingRepo.Collection).FindOne(ctx, filter, opts)
	if err := res.Decode(&cycle); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error decoding following cycle.")
		return nil, err
	}
	return &cycle, nil
}

func (repo *MongoRepository) CreateNewFollowingCycle(ctx context.Context, ct CycleType) error {
	cycle := NewFollowingCycle(ct)
	err := repo.absFollowingRepo.InsertOrUpdate(cycle)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error creating new following cycle.")
		return err
	}
	return nil
}

func (repo *MongoRepository) FindFollowingById(ctx context.Context, id primitive.ObjectID) (*FollowingCycle, error) {
	filter := bson.M{"_id": id}
	var cycle FollowingCycle
	res := repo.conn.Datastore.Collection(repo.absFollowingRepo.Collection).FindOne(ctx, filter)
	if err := res.Decode(&cycle); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error decoding following cycle.")
		return nil, err
	}
	return &cycle, nil
}

func (repo *MongoRepository) UpdateOrInsertFollowing(ctx context.Context, pl *FollowingCycle) error {
	err := repo.absFollowingRepo.InsertOrUpdate(pl)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error updating or inserting following cycle.")
		return err
	}
	return nil
}

func (repo *MongoRepository) FindAllFollowing(ctx context.Context) ([]*FollowingCycle, error) {
	filter := bson.D{}
	opts := options.Find().SetSort(bson.D{{"start_time", -1}})
	cursor, err := repo.conn.Datastore.Collection(repo.absFollowingRepo.Collection).Find(ctx, filter, opts)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error finding all following cycles.")
		return nil, err
	}
	defer cursor.Close(ctx)

	var followingCycles []*FollowingCycle
	for cursor.Next(ctx) {
		var cycle FollowingCycle
		if err := cursor.Decode(&cycle); err != nil {
			logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error decoding following cycle.")
			return nil, err
		}
		followingCycles = append(followingCycles, &cycle)
	}

	return followingCycles, nil
}

func (repo *MongoRepository) FindAllFollowingByType(ctx context.Context, ct CycleType) ([]*FollowingCycle, error) {
	filter := bson.M{"cycle_type": ct}
	opts := options.Find().SetSort(bson.D{{"start_time", -1}})
	cursor, err := repo.conn.Datastore.Collection(repo.absFollowingRepo.Collection).Find(ctx, filter, opts)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error finding all following cycles by type.")
		return nil, err
	}
	defer cursor.Close(ctx)

	var followingCycles []*FollowingCycle
	for cursor.Next(ctx) {
		var cycle FollowingCycle
		if err := cursor.Decode(&cycle); err != nil {
			logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error decoding following cycle by type.")
			return nil, err
		}
		followingCycles = append(followingCycles, &cycle)
	}

	return followingCycles, nil
}
