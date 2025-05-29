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

const grokIndexCollection = "grok.indexes"

type GrokEventRepository interface {
	SaveOrUpdate(ctx context.Context, blockIndex model.GrokEvent) (interface{}, error)
	FindAllByUserId(ctx context.Context, userId string, idx int64) ([]model.GrokEvent, error)
	FindAllByID(ctx context.Context, id string) (*model.GrokEvent, error)
	FindAll(ctx context.Context) ([]model.GrokEvent, error)
	FindMostRecentByUserId(ctx context.Context, userId string) (*model.GrokEvent, error)
	GrokTrueSince(ctx context.Context, since time.Time) (int64, error)
}
type mongoGrokEventRepository struct {
	conn    *mongodb.MongoConnection
	absrepo *persistence.AbstractMongoRepository[*model.GrokEvent]
}

// NewRatingRepository creates a new instance of RatingRepository.
func NewGrokIndexRepo(conn *mongodb.MongoConnection) GrokEventRepository {
	absrepo := persistence.NewAbstractRepository[*model.GrokEvent](conn, grokIndexCollection)
	return &mongoGrokEventRepository{
		conn:    conn,
		absrepo: absrepo,
	}
}

func (repo *mongoGrokEventRepository) FindAllByID(ctx context.Context, id string) (*model.GrokEvent, error) {
	filter := bson.M{"_id": id}
	var index model.GrokEvent
	res := repo.conn.Datastore.Collection(grokIndexCollection).FindOne(ctx, filter)
	if err := res.Decode(&index); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		logg.Log.Error().Err(err).Msg("Error decoding index result.")
		return nil, err
	}
	return &index, nil
}

func (repo *mongoGrokEventRepository) SaveOrUpdate(ctx context.Context, grokIndex model.GrokEvent) (interface{}, error) {

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

	collection := repo.conn.Datastore.Collection(grokIndexCollection)

	res, err := collection.UpdateOne(ctx, filter, bson.D{{Key: "$set", Value: update}}, &opts)
	if err != nil {
		logg.Log.Error().Err(err).Msg("Error updating/inserting block index.")
		return nil, err
	}

	return res.UpsertedID, nil
}

func (repo *mongoGrokEventRepository) FindAllByUserId(ctx context.Context, userId string, idx int64) ([]model.GrokEvent, error) {
	filter := bson.M{"user_id": userId}
	findOpt := options.Find()
	createdAfter := time.Unix(int64(idx), 0)
	filter = bson.M{"created_at": bson.M{"$gt": createdAfter}}
	findOpt.SetSort(bson.D{{"created_at", 1}})

	var tweetIdxs []model.GrokEvent

	cursor, err := repo.conn.Datastore.Collection(grokIndexCollection).Find(ctx, filter, findOpt)
	if err != nil {
		logg.Log.Error().Err(err).Msg("Error finding all block indexes by stv id.")
		return nil, err
	}

	for cursor.Next(ctx) {
		var tweetIdx model.GrokEvent
		err = cursor.Decode(&tweetIdx)
		if err != nil {
			logg.Log.Error().Err(err).Msg("Error decoding block index.")
			return nil, err
		}
		tweetIdxs = append(tweetIdxs, tweetIdx)
	}

	return tweetIdxs, nil
}

func (repo *mongoGrokEventRepository) FindAll(ctx context.Context) ([]model.GrokEvent, error) {
	findOpt := options.Find()
	filter := bson.D{}

	var blockIdxs []model.GrokEvent

	cursor, err := repo.conn.Datastore.Collection(grokIndexCollection).Find(ctx, filter, findOpt)
	if err != nil {
		logg.Log.Error().Err(err).Msg("Error finding all block indexes.")
		return nil, err
	}

	for cursor.Next(ctx) {
		var blockIdx model.GrokEvent
		err = cursor.Decode(&blockIdx)
		if err != nil {
			logg.Log.Error().Err(err).Msg("Error decoding block index.")
			return nil, err
		}
		blockIdxs = append(blockIdxs, blockIdx)
	}

	return blockIdxs, nil
}

func (repo *mongoGrokEventRepository) FindMostRecentByUserId(ctx context.Context, userId string) (*model.GrokEvent, error) {
	filter := bson.M{"user_id": userId}
	findOpt := options.Find()
	findOpt.SetSort(bson.D{{"created_at", -1}})
	findOpt.SetLimit(1)

	var tweetIdx model.GrokEvent

	cursor, err := repo.conn.Datastore.Collection(grokIndexCollection).Find(ctx, filter, findOpt)
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

func (repo *mongoGrokEventRepository) GrokTrueSince(ctx context.Context, since time.Time) (int64, error) {
	filter := bson.M{"is_shill": true, "created_at": bson.M{"$gte": since}}
	count, err := repo.conn.Datastore.Collection(grokIndexCollection).CountDocuments(ctx, filter)
	if err != nil {
		logg.Log.Error().Err(err).Msg("Error counting grok true since.")
		return 0, err
	}
	return count, nil
}
