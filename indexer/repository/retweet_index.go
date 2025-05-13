package repository

import (
	"context"
	"time"

	"github.com/jeetdotso/tweet-indexer-service/indexer/model"
	"github.com/jeetdotso/tweet-indexer-service/logg"
	"github.com/jeetdotso/tweet-indexer-service/persistence"
	"github.com/jeetdotso/tweet-indexer-service/persistence/mongodb"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const retweetIndexCollection = "retweet.indexes"

type RetweetEventRepository interface {
	SaveOrUpdate(ctx context.Context, blockIndex model.RetweetEvent) (interface{}, error)
	FindAllByUserId(ctx context.Context, userId string, idx int64) ([]model.RetweetEvent, error)
	FindAllByID(ctx context.Context, id string) (*model.RetweetEvent, error)
	FindAll(ctx context.Context, idx int64) ([]model.RetweetEvent, error)
	FindMostRecentByUserId(ctx context.Context, userId string) (*model.RetweetEvent, error)
}
type mongoRetweetEventRepository struct {
	conn    *mongodb.MongoConnection
	absrepo *persistence.AbstractMongoRepository[*model.RetweetEvent]
}

// NewRatingRepository creates a new instance of RatingRepository.
func NewRetweetIndexRepo(conn *mongodb.MongoConnection) RetweetEventRepository {
	absrepo := persistence.NewAbstractRepository[*model.RetweetEvent](conn, retweetIndexCollection)
	return &mongoRetweetEventRepository{
		conn:    conn,
		absrepo: absrepo,
	}
}

func (repo *mongoRetweetEventRepository) FindAllByID(ctx context.Context, id string) (*model.RetweetEvent, error) {
	filter := bson.M{"_id": id}
	var index model.RetweetEvent
	res := repo.conn.Datastore.Collection(retweetIndexCollection).FindOne(ctx, filter)
	if err := res.Decode(&index); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		logg.Log.Error().Err(err).Msg("Error decoding index result.")
		return nil, err
	}
	return &index, nil
}

func (repo *mongoRetweetEventRepository) SaveOrUpdate(ctx context.Context, grokIndex model.RetweetEvent) (interface{}, error) {
	collection := repo.conn.Datastore.Collection(retweetIndexCollection)

	res, err := collection.InsertOne(ctx, grokIndex)
	if err != nil {
		logg.Log.Error().Err(err).Msg("Error inserting block index.")
		return nil, err
	}

	return res.InsertedID, nil
}

func (repo *mongoRetweetEventRepository) FindAllByUserId(ctx context.Context, userId string, idx int64) ([]model.RetweetEvent, error) {
	filter := bson.M{"user_id": userId}
	findOpt := options.Find()
	createdAfter := time.Unix(int64(idx), 0)
	filter = bson.M{"created_at": bson.M{"$gt": createdAfter}}
	findOpt.SetSort(bson.D{{"created_at", 1}})

	var tweetIdxs []model.RetweetEvent

	cursor, err := repo.conn.Datastore.Collection(retweetIndexCollection).Find(ctx, filter, findOpt)
	if err != nil {
		logg.Log.Error().Err(err).Msg("Error finding all block indexes by stv id.")
		return nil, err
	}

	for cursor.Next(ctx) {
		var tweetIdx model.RetweetEvent
		err = cursor.Decode(&tweetIdx)
		if err != nil {
			logg.Log.Error().Err(err).Msg("Error decoding block index.")
			return nil, err
		}
		tweetIdxs = append(tweetIdxs, tweetIdx)
	}

	return tweetIdxs, nil
}

func (repo *mongoRetweetEventRepository) FindAll(ctx context.Context, idx int64) ([]model.RetweetEvent, error) {
	findOpt := options.Find()
	createdAfter := time.Unix(int64(idx), 0)
	filter := bson.D{{"created_at", bson.D{{"$gt", createdAfter}}}}
	findOpt.SetSort(bson.D{{"created_at", 1}})

	var blockIdxs []model.RetweetEvent

	cursor, err := repo.conn.Datastore.Collection(retweetIndexCollection).Find(ctx, filter, findOpt)
	if err != nil {
		logg.Log.Error().Err(err).Msg("Error finding all block indexes.")
		return nil, err
	}

	for cursor.Next(ctx) {
		var blockIdx model.RetweetEvent
		err = cursor.Decode(&blockIdx)
		if err != nil {
			logg.Log.Error().Err(err).Msg("Error decoding block index.")
			return nil, err
		}
		blockIdxs = append(blockIdxs, blockIdx)
	}

	return blockIdxs, nil
}

func (repo *mongoRetweetEventRepository) FindMostRecentByUserId(ctx context.Context, userId string) (*model.RetweetEvent, error) {
	filter := bson.M{"user_id": userId}
	findOpt := options.Find()
	findOpt.SetSort(bson.D{{"created_at", -1}})
	findOpt.SetLimit(1)

	var tweetIdx model.RetweetEvent

	cursor, err := repo.conn.Datastore.Collection(retweetIndexCollection).Find(ctx, filter, findOpt)
	if err != nil {
		logg.Log.Error().Err(err).Msg("Error finding most recent block index by user id.")
		return nil, err
	}

	for cursor.Next(ctx) {
		err = cursor.Decode(&tweetIdx)
		if err != nil {
			logg.Log.Error().Err(err).Msg("Error decoding block index.")
			return nil, err
		}
	}

	return &tweetIdx, nil
}
