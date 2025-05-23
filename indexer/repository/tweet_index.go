package repository

import (
	"context"
	"time"

	"github.com/jeetdotso/tweet-indexer-service/indexer/model"
	"github.com/jeetdotso/tweet-indexer-service/logg"
	"github.com/jeetdotso/tweet-indexer-service/persistence"
	"github.com/jeetdotso/tweet-indexer-service/persistence/mongodb"
	twitterTypes "github.com/jeetdotso/tweet-indexer-service/scrape/internals/api/types"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// TODO: change back to tweet.indexes
const tweetIndexCollection = "tweet.iii"

type TweetEventRepository interface {
	SaveOrUpdate(ctx context.Context, blockIndex model.TweetEvent) (interface{}, error)
	FindAllByUserId(ctx context.Context, userId string, idx int64) ([]model.TweetEvent, error)
	FindAllByID(ctx context.Context, id string) (*model.TweetEvent, error)
	FindByTweetId(ctx context.Context, tweetId string) (*model.TweetEvent, error)
	FindAll(ctx context.Context, idx int64) ([]model.TweetEvent, error)
	FindAllRaw(ctx context.Context) ([]model.TweetEvent, error)
	FindMostRecentByUserId(ctx context.Context, userId string) (*model.TweetEvent, error)
	FindAllNoneGrok(ctx context.Context) ([]model.TweetEvent, error)
	FindAllNoneTokenGrok(ctx context.Context) ([]model.TweetEvent, error)
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
	filter := bson.M{"_id": tweetIndex.ID}
	update := bson.M{"$set": tweetIndex}
	opts := options.Update().SetUpsert(true)

	collection := repo.conn.Datastore.Collection(tweetIndexCollection)
	res, err := collection.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		logg.Log.Error().Err(err).Msg("Error updating or inserting tweet index.")
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
func (repo *mongoEventRepository) FindAllRaw(ctx context.Context) ([]model.TweetEvent, error) {
	findOpt := options.Find()
	findOpt.SetSort(bson.D{{"created_at", 1}})
	filter := bson.D{{}}
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

func (repo *mongoEventRepository) FindAllNoneGrok(ctx context.Context) ([]model.TweetEvent, error) {
	pipeline := mongo.Pipeline{
		// stage 1
		bson.D{{
			Key: "$match",
			Value: bson.D{{
				"tweet_type", bson.D{{
					"$nin", bson.A{
						twitterTypes.Reply,
						twitterTypes.Retweet,
						twitterTypes.QuoteRetweet,
					},
				}},
			}},
		}},
		// stage 2
		{{Key: "$lookup", Value: bson.D{
			{"from", grokIndexCollection},
			{"localField", "tweet_id"},
			{"foreignField", "_id"},
			{"as", "grokMatch"},
		}}},
		// stage 3
		{{Key: "$match", Value: bson.D{{"grokMatch.0", bson.D{{"$exists", false}}}}}},
		// optional
		{{"$sort", bson.D{{"captured_at", 1}}}},
		{{Key: "$project", Value: bson.D{{"grokMatch", 0}}}},
	}

	cur, err := repo.conn.Datastore.Collection(tweetIndexCollection).Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	var misses []model.TweetEvent
	if err := cur.All(ctx, &misses); err != nil {
		return nil, err
	}
	return misses, nil
}
func (repo *mongoEventRepository) FindAllNoneTokenGrok(ctx context.Context) ([]model.TweetEvent, error) {
	pipeline := mongo.Pipeline{
		// stage 1
		bson.D{{
			Key: "$match",
			Value: bson.D{{
				"tweet_type", bson.D{{
					"$nin", bson.A{
						twitterTypes.Reply,
					},
				}},
			}},
		}},
		// stage 2
		{{Key: "$lookup", Value: bson.D{
			{"from", grokTokenCollection},
			{"localField", "_id"},
			{"foreignField", "_id"},
			{"as", "grokMatch"},
		}}},
		// stage 3
		{{Key: "$match", Value: bson.D{{"grokMatch.0", bson.D{{"$exists", false}}}}}},
		// optional
		{{"$sort", bson.D{{"captured_at", 1}}}},
		{{Key: "$project", Value: bson.D{{"grokMatch", 0}}}},
	}

	cur, err := repo.conn.Datastore.Collection(tweetIndexCollection).Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	var misses []model.TweetEvent
	if err := cur.All(ctx, &misses); err != nil {
		return nil, err
	}
	return misses, nil
}

func (repo *mongoEventRepository) FindByTweetId(ctx context.Context, tweetId string) (*model.TweetEvent, error) {
	filter := bson.M{"tweet_id": tweetId}
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
