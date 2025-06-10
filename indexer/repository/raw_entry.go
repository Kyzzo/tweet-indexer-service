package repository

import (
	"context"

	"github.com/jeetdotso/tweet-indexer-service/indexer/model"
	"github.com/jeetdotso/tweet-indexer-service/logg"
	"github.com/jeetdotso/tweet-indexer-service/persistence"
	"github.com/jeetdotso/tweet-indexer-service/persistence/mongodb"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const rawtweetCollection = "tweet.raw"

type RawTweetRepository interface {
	SaveOrUpdate(ctx context.Context, blockIndex model.RawTweet) (interface{}, error)
	FindAllByUserId(ctx context.Context, userId string, idx int64) ([]model.RawTweet, error)
	FindAllByID(ctx context.Context, id string) (*model.RawTweet, error)
}
type mongoRawTweetRepository struct {
	conn    *mongodb.MongoConnection
	absrepo *persistence.AbstractMongoRepository[*model.RawTweet]
}

// NewRatingRepository creates a new instance of RatingRepository.
func NewRawTweetRepo(conn *mongodb.MongoConnection) RawTweetRepository {
	absrepo := persistence.NewAbstractRepository[*model.RawTweet](conn, rawtweetCollection)
	return &mongoRawTweetRepository{
		conn:    conn,
		absrepo: absrepo,
	}
}

func (repo *mongoRawTweetRepository) FindAllByID(ctx context.Context, id string) (*model.RawTweet, error) {
	filter := bson.M{"_id": id}
	var index model.RawTweet
	res := repo.conn.Datastore.Collection(rawtweetCollection).FindOne(ctx, filter)
	if err := res.Decode(&index); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		logg.Log.Error().Err(err).Msg("Error decoding index result.")
		return nil, err
	}
	return &index, nil
}
func (repo *mongoRawTweetRepository) SaveOrUpdate(ctx context.Context, tweetIndex model.RawTweet) (interface{}, error) {
	filter := bson.M{"_id": tweetIndex.ID}
	update := bson.M{"$set": tweetIndex}
	opts := options.Update().SetUpsert(true)

	collection := repo.conn.Datastore.Collection(rawtweetCollection)
	res, err := collection.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		logg.Log.Error().Err(err).Msg("Error updating or inserting tweet index.")
		return nil, err
	}

	return res.UpsertedID, nil
}

func (repo *mongoRawTweetRepository) FindAllByUserId(ctx context.Context, userId string, idx int64) ([]model.RawTweet, error) {
	filter := bson.M{"user_id": userId}
	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}}).SetLimit(idx)

	cursor, err := repo.conn.Datastore.Collection(rawtweetCollection).Find(ctx, filter, opts)
	if err != nil {
		logg.Log.Error().Err(err).Msg("Error finding raw tweets by user ID.")
		return nil, err
	}
	defer cursor.Close(ctx)

	var tweets []model.RawTweet
	if err = cursor.All(ctx, &tweets); err != nil {
		logg.Log.Error().Err(err).Msg("Error decoding raw tweets.")
		return nil, err
	}

	return tweets, nil
}
