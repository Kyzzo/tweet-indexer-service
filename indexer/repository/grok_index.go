package repository

import (
	"context"
	"fmt"
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
	GetGrokChart(ctx context.Context, start, end time.Time, interval string, categories []int) (*model.GrokChartResponse, error)
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

// GetGrokChart returns counts of GrokEvent documents per interval between start and end.
func (repo *mongoGrokEventRepository) GetGrokChart(ctx context.Context, start, end time.Time, interval string, categories []int) (*model.GrokChartResponse, error) {
	// 1) parse interval into unit & binSize
	var unit string
	var binSize int32
	var step time.Duration
	switch interval {
	case "5min":
		unit, binSize, step = "minute", 5, 5*time.Minute
	case "1H":
		unit, binSize, step = "hour", 1, time.Hour
	case "2H":
		unit, binSize, step = "hour", 2, 2*time.Hour
	case "3H":
		unit, binSize, step = "hour", 3, 3*time.Hour
	case "1D":
		unit, binSize, step = "day", 1, 24*time.Hour
	case "2D":
		unit, binSize, step = "day", 2, 2*24*time.Hour
	case "1W":
		unit, binSize, step = "week", 1, 7*24*time.Hour
	default:
		return nil, fmt.Errorf("unsupported interval %q", interval)
	}

	// 2) total count in [start,end)
	matchFilter := bson.M{
		"created_at": bson.M{"$gte": start, "$lt": end},
		"is_shill":   true,
	}
	if len(categories) > 0 {
		matchFilter["category"] = bson.M{"$in": categories}
	}

	total, err := repo.conn.Datastore.
		Collection(grokIndexCollection).
		CountDocuments(ctx, matchFilter)
	if err != nil {
		return nil, fmt.Errorf("count total events: %w", err)
	}

	// 3) build aggregation pipeline
	pipeline := mongo.Pipeline{
		// match time window and filter by categories if provided
		{{"$match", matchFilter}},
		// group into buckets
		{{"$group", bson.D{
			{"_id", bson.D{{
				"$dateTrunc", bson.D{
					{"date", "$created_at"},
					{"unit", unit},
					{"binSize", binSize},
					{"timezone", "UTC"},
				},
			}}},
			{"count", bson.D{{"$sum", 1}}},
		}}},
		// project into our shape
		{{"$project", bson.D{
			{"_id", 0},
			{"timestamp", "$_id"},
			{"unix", bson.D{{"$toLong", "$_id"}}},
			{"count", 1},
		}}},
		// order ascending
		{{"$sort", bson.D{{"timestamp", 1}}}},
	}

	cursor, err := repo.conn.Datastore.
		Collection(grokIndexCollection).
		Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("aggregate chart data: %w", err)
	}
	defer cursor.Close(ctx)

	// 4) decode buckets
	var data []model.GrokChartData
	for cursor.Next(ctx) {
		var d model.GrokChartData
		if err := cursor.Decode(&d); err != nil {
			return nil, fmt.Errorf("decode bucket: %w", err)
		}
		data = append(data, d)
	}
	if err := cursor.Err(); err != nil {
		return nil, fmt.Errorf("cursor error: %w", err)
	}

	// build a map from timestamp to count
	countMap := make(map[time.Time]int, len(data))
	for _, d := range data {
		// ensure timestamps are UTC-truncated to the bucket
		ts := d.Timestamp.UTC().Truncate(step)
		countMap[ts] = d.Count
	}

	// generate full time series
	full := make([]model.GrokChartData, 0, int(end.Sub(start)/step)+1)
	// start from the bucket boundary at or before `start`
	t := start.UTC().Truncate(step)
	// advance until we reach `end`
	for !t.After(end.UTC()) {
		full = append(full, model.GrokChartData{
			Timestamp: t,
			Unix:      t.UnixMilli(),
			Count:     countMap[t], // zero if missing
		})
		t = t.Add(step)
	}

	// 5) assemble response
	return &model.GrokChartResponse{
		ChartData:  full,
		TotalCount: total,
		Start:      start,
		End:        end,
		Interval:   interval,
	}, nil
}






