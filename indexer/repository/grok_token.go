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

const grokTokenCollection = "grok.token"

type GrokTokenEventRepository interface {
	SaveOrUpdate(ctx context.Context, blockIndex model.GrokTokenEvent) (interface{}, error)
	FindAllByUserId(ctx context.Context, userId string, idx int64) ([]model.GrokTokenEvent, error)
	FindAllByID(ctx context.Context, id string) (*model.GrokTokenEvent, error)
	FindAll(ctx context.Context) ([]model.GrokTokenEvent, error)
	FindMostRecentByUserId(ctx context.Context, userId string) (*model.GrokTokenEvent, error)
}
type mongoGrokTokenEventRepository struct {
	conn    *mongodb.MongoConnection
	absrepo *persistence.AbstractMongoRepository[*model.GrokTokenEvent]
}

// NewRatingRepository creates a new instance of RatingRepository.
func NewGrokTokenRepo(conn *mongodb.MongoConnection) GrokTokenEventRepository {
	absrepo := persistence.NewAbstractRepository[*model.GrokTokenEvent](conn, grokTokenCollection)
	return &mongoGrokTokenEventRepository{
		conn:    conn,
		absrepo: absrepo,
	}
}

func (repo *mongoGrokTokenEventRepository) FindAllByID(ctx context.Context, id string) (*model.GrokTokenEvent, error) {
	filter := bson.M{"_id": id}
	var index model.GrokTokenEvent
	res := repo.conn.Datastore.Collection(grokTokenCollection).FindOne(ctx, filter)
	if err := res.Decode(&index); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		logg.Log.Error().Err(err).Msg("Error decoding index result.")
		return nil, err
	}
	return &index, nil
}

func (repo *mongoGrokTokenEventRepository) SaveOrUpdate(ctx context.Context, grokIndex model.GrokTokenEvent) (interface{}, error) {

	b, err := bson.Marshal(grokIndex)
	if err != nil {
		logg.Log.Error().Err(err).Msg("Error marshaling tweet index.")
		return nil, err
	}

	var update bson.M
	err = bson.Unmarshal(b, &update)
	if err != nil {
		logg.Log.Error().Err(err).Msg("Error unmarshalling tweet index to bson.M.")
		return nil, err
	}

	filter := bson.M{"_id": grokIndex.ID}
	opts := options.UpdateOptions{}
	opts.SetUpsert(true)

	collection := repo.conn.Datastore.Collection(grokTokenCollection)

	res, err := collection.UpdateOne(ctx, filter, bson.D{{Key: "$set", Value: update}}, &opts)
	if err != nil {
		logg.Log.Error().Err(err).Msg("Error updating/inserting block index.")
		return nil, err
	}

	return res.UpsertedID, nil
}

func (repo *mongoGrokTokenEventRepository) FindAllByUserId(ctx context.Context, userId string, idx int64) ([]model.GrokTokenEvent, error) {
	filter := bson.M{"user_id": userId}
	findOpt := options.Find()
	createdAfter := time.Unix(int64(idx), 0)
	filter = bson.M{"created_at": bson.M{"$gt": createdAfter}}
	findOpt.SetSort(bson.D{{"created_at", 1}})

	var tweetIdxs []model.GrokTokenEvent

	cursor, err := repo.conn.Datastore.Collection(grokTokenCollection).Find(ctx, filter, findOpt)
	if err != nil {
		logg.Log.Error().Err(err).Msg("Error finding all block indexes by stv id.")
		return nil, err
	}

	for cursor.Next(ctx) {
		var tweetIdx model.GrokTokenEvent
		err = cursor.Decode(&tweetIdx)
		if err != nil {
			logg.Log.Error().Err(err).Msg("Error decoding block index.")
			return nil, err
		}
		tweetIdxs = append(tweetIdxs, tweetIdx)
	}

	return tweetIdxs, nil
}

func (repo *mongoGrokTokenEventRepository) FindAll(ctx context.Context) ([]model.GrokTokenEvent, error) {
	findOpt := options.Find()
	filter := bson.D{}

	var blockIdxs []model.GrokTokenEvent

	cursor, err := repo.conn.Datastore.Collection(grokTokenCollection).Find(ctx, filter, findOpt)
	if err != nil {
		logg.Log.Error().Err(err).Msg("Error finding all block indexes.")
		return nil, err
	}

	for cursor.Next(ctx) {
		var blockIdx model.GrokTokenEvent
		err = cursor.Decode(&blockIdx)
		if err != nil {
			logg.Log.Error().Err(err).Msg("Error decoding block index.")
			return nil, err
		}
		blockIdxs = append(blockIdxs, blockIdx)
	}

	return blockIdxs, nil
}

func (repo *mongoGrokTokenEventRepository) FindMostRecentByUserId(ctx context.Context, userId string) (*model.GrokTokenEvent, error) {
	filter := bson.M{"user_id": userId}
	findOpt := options.Find()
	findOpt.SetSort(bson.D{{"created_at", -1}})
	findOpt.SetLimit(1)

	var tweetIdx model.GrokTokenEvent

	cursor, err := repo.conn.Datastore.Collection(grokTokenCollection).Find(ctx, filter, findOpt)
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
