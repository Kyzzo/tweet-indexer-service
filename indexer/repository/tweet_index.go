package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jeetdotso/tweet-indexer-service/indexer/model"
	"github.com/jeetdotso/tweet-indexer-service/logg"
	"github.com/jeetdotso/tweet-indexer-service/persistence"
	"github.com/jeetdotso/tweet-indexer-service/persistence/mongodb"
	twitterTypes "github.com/jeetdotso/tweet-indexer-service/scrape/internals/api/types"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const tweetIndexCollection = "tweet.indexes"

var (
	NONE_GROK = []string{
		string(twitterTypes.Reply),
		string(twitterTypes.Retweet),
		string(twitterTypes.QuoteRetweet)}

	noneGrokBson bson.A // This will hold the BSON array representation of NONE_GROK
)

func init() {
	noneGrokBson = bson.A{}
	for _, item := range NONE_GROK {
		noneGrokBson = append(noneGrokBson, item)
	}
}

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
	FindAvgScrapeTime(ctx context.Context, since time.Time) (float64, error)
	FindAvgGrokTime(ctx context.Context, since time.Time) (float64, error)
	FindUniqueUsersSince(ctx context.Context, since time.Time) (int64, error)
	CountGrokVsNonGrok(ctx context.Context, since time.Time) (grokCount, nonGrokCount int64, err error)
	FindLatestTweetsForUsers(ctx context.Context, userIds []string) ([]latestTweet, error)
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
	threshold := primitive.NewDateTimeFromTime(time.Now().Add(-20 * time.Second))
	maxThreshold := primitive.NewDateTimeFromTime(time.Now().Add(-1 * time.Hour))
	pipeline := mongo.Pipeline{
		// stage 1
		bson.D{{
			Key: "$match",
			Value: bson.D{
				{Key: "tweet_type", Value: bson.D{
					{Key: "$nin", Value: bson.A(noneGrokBson)}, // Convert slice to bson.A
				}},
				{"captured_at", bson.D{{"$lte", threshold}}},
				{"captured_at", bson.D{{"$gte", maxThreshold}}},
			},
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

func (repo *mongoEventRepository) FindAvgScrapeTime(ctx context.Context, since time.Time) (float64, error) {
	pipeline := mongo.Pipeline{
		// 1) match created_at >= cutoff
		{{"$match", bson.D{{"created_at", bson.D{{"$gte", primitive.NewDateTimeFromTime(since)}}}}}},
		// 2) project the difference in ms
		{{"$project", bson.D{{"diffMs", bson.D{{"$subtract", bson.A{"$captured_at", "$created_at"}}}}}}},
		// 3) group to average it
		{{"$group", bson.D{
			bson.E{Key: "_id", Value: nil},
			{"avgMs", bson.D{{"$avg", "$diffMs"}}},
		}}},
		// 4) project into seconds
		{{"$project", bson.D{
			{"_id", 0},
			{"avgSeconds", bson.D{{"$divide", bson.A{"$avgMs", 1000}}}},
		}}},
	}

	// Run aggregation
	cursor, err := repo.conn.Datastore.Collection(tweetIndexCollection).Aggregate(ctx, pipeline, options.Aggregate().SetMaxTime(30*time.Second))
	if err != nil {
		return 0, err
	}
	defer cursor.Close(ctx)

	// Pull out the result
	var result struct {
		AvgSeconds float64 `bson:"avgSeconds"`
	}
	if cursor.Next(ctx) {
		if err := cursor.Decode(&result); err != nil {
			return 0, err
		}
		return result.AvgSeconds, nil
	}
	if err := cursor.Err(); err != nil {
		return 0, err
	}

	// no documents → return zero
	return 0, nil
}

func (repo *mongoEventRepository) FindAvgGrokTime(ctx context.Context, since time.Time) (float64, error) {
	pipeline := mongo.Pipeline{
		// 1) only tweets captured in last 24h
		{{"$match", bson.D{{"captured_at", bson.D{{"$gte", primitive.NewDateTimeFromTime(since)}}}}}},
		// 2) join in the grok.indexes collection on _id
		{{"$lookup", bson.D{
			{"from", grokIndexCollection},
			{"localField", "_id"},
			{"foreignField", "_id"},
			{"as", "grok_data"},
		}}},
		// 3) unwind the array (skip if no grok_data? you can add preserveNullAndEmpty later)
		{{"$unwind", "$grok_data"}},
		// 4) compute the difference in ms between grok_data.created_at and captured_at
		{{"$project", bson.D{{"diffMs", bson.D{{"$subtract", bson.A{"$grok_data.created_at", "$captured_at"}}}}}}},
		// 5) group and average those diffs
		{{"$group", bson.D{
			{"_id", nil},
			{"avgMs", bson.D{{"$avg", "$diffMs"}}},
		}}},
		// 6) project final result in seconds
		{{"$project", bson.D{
			{"_id", 0},
			{"avgSeconds", bson.D{{"$divide", bson.A{"$avgMs", 1000}}}},
		}}},
	}

	// run the aggregation
	cursor, err := repo.conn.Datastore.Collection(tweetIndexCollection).Aggregate(ctx, pipeline, options.Aggregate().SetMaxTime(30*time.Second))
	if err != nil {
		return 0, err
	}
	defer cursor.Close(ctx)

	// decode
	var out struct {
		AvgSeconds float64 `bson:"avgSeconds"`
	}
	if cursor.Next(ctx) {
		if err := cursor.Decode(&out); err != nil {
			return 0, err
		}
		return out.AvgSeconds, nil
	}
	if err := cursor.Err(); err != nil {
		return 0, err
	}
	// no matching docs → zero
	return 0, nil
}

func (repo *mongoEventRepository) FindUniqueUsersSince(ctx context.Context, since time.Time) (int64, error) {
	filter := bson.D{{"created_at", bson.D{{"$gte", primitive.NewDateTimeFromTime(since)}}}}
	ids, err := repo.conn.Datastore.Collection(tweetIndexCollection).Distinct(ctx, "user_id", filter)
	if err != nil {
		return 0, err
	}
	return int64(len(ids)), nil
}

func (repo *mongoEventRepository) CountGrokVsNonGrok(ctx context.Context, since time.Time) (grokCount, nonGrokCount int64, err error) {
	cutoff := primitive.NewDateTimeFromTime(since)

	pipeline := mongo.Pipeline{
		// Stage 1: Filter documents since cutoff
		bson.D{{"$match", bson.D{{"created_at", bson.D{{"$gte", cutoff}}}}}},

		// Stage 2: Single group stage with both counters
		bson.D{{"$group", bson.D{
			{"_id", nil},
			{"grokCount", bson.D{
				{"$sum", bson.D{
					{"$cond", bson.A{
						bson.D{{"$not", bson.D{{"$in", bson.A{"$tweet_type", noneGrokBson}}}}},
						1,
						0,
					}},
				}},
			}},
			{"nonGrokCount", bson.D{
				{"$sum", bson.D{
					{"$cond", bson.A{
						bson.D{{"$in", bson.A{"$tweet_type", noneGrokBson}}},
						1,
						0,
					}},
				}},
			}},
		}}},
	}

	cur, err := repo.conn.Datastore.Collection(tweetIndexCollection).Aggregate(ctx, pipeline)
	if err != nil {
		return 0, 0, err
	}
	defer cur.Close(ctx)

	var res struct {
		GrokCount    int64 `bson:"grokCount"`
		NonGrokCount int64 `bson:"nonGrokCount"`
	}

	if cur.Next(ctx) {
		if err := cur.Decode(&res); err != nil {
			return 0, 0, err
		}
		return res.GrokCount, res.NonGrokCount, nil
	}

	if err := cur.Err(); err != nil {
		return 0, 0, err
	}

	// No documents found
	return 0, 0, nil
}

type latestTweet struct {
	UserID    string    `bson:"_id"`        // grouped _id → user_id
	TweetID   string    `bson:"tweet_id"`   // the tweet’s _id
	CreatedAt time.Time `bson:"created_at"` // the tweet’s created_at
}

func (repo *mongoEventRepository) FindLatestTweetsForUsers(
	ctx context.Context,
	userIds []string,
) ([]latestTweet, error) {
	// 1) Build the aggregation pipeline:
	pipeline := mongo.Pipeline{
		// Stage 1: Match only those whose user_id is in userIds
		{{"$match", bson.D{
			{"user_id", bson.D{{"$in", userIds}}},
		}}},
		// Stage 2: Sort each user’s tweets by created_at descending
		{{"$sort", bson.D{
			{"user_id", 1},
			{"created_at", -1},
		}}},
		// Stage 3: Group by user_id, taking the first (newest) tweet
		{{"$group", bson.D{
			{"_id", "$user_id"},
			{"tweet_id", bson.D{{"$first", "$_id"}}},
			{"created_at", bson.D{{"$first", "$created_at"}}},
		}}},
	}

	// 2) Execute the aggregation
	cursor, err := repo.conn.Datastore.Collection(tweetIndexCollection).Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("aggregation failed: %w", err)
	}
	defer cursor.Close(ctx)

	// 3) Decode results into a slice
	var results []latestTweet
	if err := cursor.All(ctx, &results); err != nil {
		return nil, fmt.Errorf("failed to decode aggregation results: %w", err)
	}
	return results, nil
}
