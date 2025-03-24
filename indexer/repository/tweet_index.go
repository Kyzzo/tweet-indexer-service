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

const tweetIndexCollection = "tweet.indexes"

type TweetEventRepository interface {
	SaveOrUpdate(ctx context.Context, blockIndex model.TweetEvent) (interface{}, error)
	FindAllByUserId(ctx context.Context, userId string, idx int64) ([]model.TweetEvent, error)
	FindAllByID(ctx context.Context, id string) (*model.TweetEvent, error)
	FindAll(ctx context.Context, idx int64) ([]model.TweetEvent, error)
	FindMostRecentByUserId(ctx context.Context, userId string) (*model.TweetEvent, error)
}
type mongoEventRepository struct {
	conn    *mongodb.MongoConnection
	absrepo *persistence.AbstractMongoRepository[*model.TweetEvent]
}

// NewRatingRepository creates a new instance of RatingRepository.
func NewTweetIndexRepo(conn *mongodb.MongoConnection) TweetEventRepository {
	absrepo := persistence.NewAbstractRepository[*model.TweetEvent](conn, tweetIndexCollection)
	return &mongoEventRepository{
		conn:    conn,
		absrepo: absrepo,
	}
}

func (repo *mongoEventRepository) FindAllByID(ctx context.Context, id string) (*model.TweetEvent, error) {
	filter := bson.M{"_id": id}
	var index model.TweetEvent
	res := repo.conn.Datastore.Collection(tweetIndexCollection).FindOne(ctx, filter)
	if err := res.Decode(&index); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		logg.Log.Error().Err(err).Msg("Error decoding index result.")
		return nil, err
	}
	return &index, nil
}

func (repo *mongoEventRepository) SaveOrUpdate(ctx context.Context, tweetIndex model.TweetEvent) (interface{}, error) {

	b, err := bson.Marshal(tweetIndex)
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

	filter := bson.M{"_id": tweetIndex.ID}
	opts := options.UpdateOptions{}
	opts.SetUpsert(true)

	collection := repo.conn.Datastore.Collection(tweetIndexCollection)

	res, err := collection.UpdateOne(ctx, filter, bson.D{{Key: "$set", Value: update}}, &opts)
	if err != nil {
		logg.Log.Error().Err(err).Msg("Error updating/inserting block index.")
		return nil, err
	}

	return res.UpsertedID, nil
}

func (repo *mongoEventRepository) FindAllByUserId(ctx context.Context, userId string, idx int64) ([]model.TweetEvent, error) {
	filter := bson.M{"user_id": userId}
	findOpt := options.Find()
	createdAfter := time.Unix(int64(idx), 0)
	filter = bson.M{"created_at": bson.M{"$gt": createdAfter}}
	findOpt.SetSort(bson.D{{"created_at", 1}})

	var tweetIdxs []model.TweetEvent

	cursor, err := repo.conn.Datastore.Collection(tweetIndexCollection).Find(ctx, filter, findOpt)
	if err != nil {
		logg.Log.Error().Err(err).Msg("Error finding all block indexes by stv id.")
		return nil, err
	}

	for cursor.Next(ctx) {
		var tweetIdx model.TweetEvent
		err = cursor.Decode(&tweetIdx)
		if err != nil {
			logg.Log.Error().Err(err).Msg("Error decoding block index.")
			return nil, err
		}
		tweetIdxs = append(tweetIdxs, tweetIdx)
	}

	return tweetIdxs, nil
}

func (repo *mongoEventRepository) FindAll(ctx context.Context, idx int64) ([]model.TweetEvent, error) {
	findOpt := options.Find()
	createdAfter := time.Unix(int64(idx), 0)
	filter := bson.D{{"created_at", bson.D{{"$gt", createdAfter}}}}
	findOpt.SetSort(bson.D{{"created_at", 1}})

	var blockIdxs []model.TweetEvent

	cursor, err := repo.conn.Datastore.Collection(tweetIndexCollection).Find(ctx, filter, findOpt)
	if err != nil {
		logg.Log.Error().Err(err).Msg("Error finding all block indexes.")
		return nil, err
	}

	for cursor.Next(ctx) {
		var blockIdx model.TweetEvent
		err = cursor.Decode(&blockIdx)
		if err != nil {
			logg.Log.Error().Err(err).Msg("Error decoding block index.")
			return nil, err
		}
		blockIdxs = append(blockIdxs, blockIdx)
	}

	return blockIdxs, nil
}

func (repo *mongoEventRepository) FindMostRecentByUserId(ctx context.Context, userId string) (*model.TweetEvent, error) {
	filter := bson.M{"user_id": userId}
	findOpt := options.Find()
	findOpt.SetSort(bson.D{{"created_at", -1}})
	findOpt.SetLimit(1)

	var tweetIdx model.TweetEvent

	cursor, err := repo.conn.Datastore.Collection(tweetIndexCollection).Find(ctx, filter, findOpt)
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
