package twitterprofile

import (
	"context"
	"fmt"
	"time"

	"github.com/jeetdotso/tweet-indexer-service/logg"
	"github.com/jeetdotso/tweet-indexer-service/persistence"
	"github.com/jeetdotso/tweet-indexer-service/persistence/mongodb"
	twiterTypes "github.com/jeetdotso/tweet-indexer-service/scrape/internals/api/types"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const FetchedCollection = "twitterprofiles_fetched"

type Repository interface {
	/////Fecthed///////////////
	UpdateOrInsertFetched(ctx context.Context, usr *FetchedProfile) error
	FindFetchedByUserId(ctx context.Context, twitterId string) (*FetchedProfile, error)
	FindFetchedUserByTwitterHandle(ctx context.Context, twitterHandle string) (*FetchedProfile, error)
	FindFetchedUserByName(ctx context.Context, usrName string) (*FetchedProfile, error)
	FindFetchedProfiles(ctx context.Context, q *FetchedProfileQuery) (*persistence.Pagination[[]*FetchedProfileFull], error)
	UpsertFetchedProfile(ctx context.Context, u twiterTypes.FollowingUser) error
	ChangeStatusById(ctx context.Context, id string, status int) error
	ChangeStatusMultiple(ctx context.Context, ids []string, status int) error
	UpdateTagById(ctx context.Context, id string, tag string) error
	GetTopHit(ctx context.Context, page, limit int64, tags []string) (*persistence.Pagination[[]*FetchedProfileFull], error)
	GetTopHitRaw(ctx context.Context, limit int64, tags []string) ([]*FetchedProfile, error)
	GetAllByStatus(ctx context.Context, status int) ([]*FetchedProfile, error)
	GetAllByTag(ctx context.Context, tag string) ([]*FetchedProfile, error)
	FindAllByIDs(ctx context.Context, ids []string) ([]FetchedProfile, error)
	GetUniqueTags(ctx context.Context) ([]string, error)
}
type MongoRepository struct {
	conn      *mongodb.MongoConnection
	fetchRepo *persistence.AbstractMongoRepository[*FetchedProfile]
}

func NewRepository(conn *mongodb.MongoConnection) Repository {
	fetchRepo := persistence.NewAbstractRepository[*FetchedProfile](conn, FetchedCollection)
	return &MongoRepository{
		conn:      conn,
		fetchRepo: fetchRepo,
	}
}

// ///////////////Fecthed///////////////
// @Description: This function updates or inserts a fetched profile into the MongoDB collection.
func (repo *MongoRepository) UpdateOrInsertFetched(ctx context.Context, usr *FetchedProfile) error {
	err := repo.fetchRepo.InsertOrUpdate(usr)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error updating or inserting fetched user.")
		return err
	}
	return nil
}

func (repo *MongoRepository) FindFetchedByUserId(ctx context.Context, twitterId string) (*FetchedProfile, error) {
	filter := bson.M{"_id": twitterId}
	var usr FetchedProfile
	res := repo.conn.Datastore.Collection(repo.fetchRepo.Collection).FindOne(ctx, filter)
	if err := res.Decode(&usr); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error decoding fetched user result.")
		return nil, err
	}
	return &usr, nil
}

func (repo *MongoRepository) FindFetchedUserByTwitterHandle(ctx context.Context, twitterHandle string) (*FetchedProfile, error) {
	filter := bson.M{"handle": twitterHandle}
	var usr FetchedProfile
	res := repo.conn.Datastore.Collection(repo.fetchRepo.Collection).FindOne(ctx, filter)
	if err := res.Decode(&usr); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error decoding fetched user result.")
		return nil, err
	}
	return &usr, nil
}

func (repo *MongoRepository) FindFetchedUserByName(ctx context.Context, usrName string) (*FetchedProfile, error) {
	filter := bson.M{"user_name": usrName}
	var usr FetchedProfile
	res := repo.conn.Datastore.Collection(repo.fetchRepo.Collection).FindOne(ctx, filter)
	if err := res.Decode(&usr); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error decoding fetched user result.")
		return nil, err
	}

	return &usr, nil
}

func (repo *MongoRepository) FindFetchedProfiles(
	ctx context.Context,
	q *FetchedProfileQuery,
) (*persistence.Pagination[[]*FetchedProfileFull], error) {

	// 1) Build the base filter
	filter := bson.M{}
	if q.Status != nil {
		filter["status"] = *q.Status
	}
	if len(q.Tags) > 0 {
		filter["tag"] = bson.M{"$in": q.Tags}
	}
	if q.FollowersMin != nil || q.FollowersMax != nil {
		rng := bson.M{}
		if q.FollowersMin != nil {
			rng["$gte"] = *q.FollowersMin
		}
		if q.FollowersMax != nil {
			rng["$lte"] = *q.FollowersMax
		}
		filter["follower_count"] = rng
	}

	// 2) Count total documents (unpaginated)
	total, err := repo.conn.Datastore.
		Collection(repo.fetchRepo.Collection).
		CountDocuments(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("FindFetchedProfilesFull count: %w", err)
	}

	skip := (q.Page - 1) * q.Size
	dir := -1
	if q.Asc {
		dir = 1
	}
	sortField := q.SortField
	if sortField == "" {
		sortField = "captured_at"
	}

	// 3) Build aggregation pipeline
	pipeline := mongo.Pipeline{
		// match stage
		{{"$match", filter}},

		// lookup followers
		{{"$lookup", bson.D{
			{"from", "following.indexes"},
			{"localField", "_id"},
			{"foreignField", "following_id"},
			{"as", "followers_arr"},
		}}},

		// compute followed_by_count
		{{"$addFields", bson.D{
			{"followed_by_count", bson.D{{"$size", "$followers_arr"}}},
		}}},
	}

	// 4) append sort
	pipeline = append(pipeline, bson.D{{
		"$sort", bson.D{{Key: sortField, Value: dir}},
	}})

	// 5) pagination
	pipeline = append(pipeline,
		bson.D{{"$skip", skip}},
		bson.D{{"$limit", q.Size}},
	)

	// 6) cleanup temporary array
	pipeline = append(pipeline, bson.D{{"$project", bson.D{
		{"followers_arr", 0},
	}}})

	// 7) execute aggregation
	cursor, err := repo.conn.Datastore.
		Collection(repo.fetchRepo.Collection).
		Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("FindFetchedProfilesFull aggregate: %w", err)
	}
	defer cursor.Close(ctx)

	// 8) decode into FetchedProfileFull
	var results []*FetchedProfileFull
	for cursor.Next(ctx) {
		var fp FetchedProfileFull
		if err := cursor.Decode(&fp); err != nil {
			return nil, fmt.Errorf("decode full profile: %w", err)
		}
		results = append(results, &fp)
	}
	if err := cursor.Err(); err != nil {
		return nil, fmt.Errorf("cursor error: %w", err)
	}

	// 9) wrap in pagination
	return &persistence.Pagination[[]*FetchedProfileFull]{
		Result:     results,
		Page:       q.Page,
		Size:       q.Size,
		TotalCount: total,
	}, nil
}

func (repo *MongoRepository) UpsertFetchedProfile(ctx context.Context, u twiterTypes.FollowingUser) error {
	now := time.Now().UTC()

	filter := bson.D{{Key: "_id", Value: u.ID}}
	update := bson.D{
		// always overwrite these fields from the API
		{Key: "$set", Value: bson.D{
			{Key: "user_name", Value: u.UserName},
			{Key: "handle", Value: u.Handle},
			{Key: "profile_img", Value: u.ProfileImg},
			{Key: "follower_count", Value: u.Followers},
			{Key: "following_count", Value: u.Following},
			{Key: "verified", Value: u.Verified},
			// bump your updated_at every time
			{Key: "updated_at", Value: now},
		}},
		// only on insert, set these once:
		{Key: "$setOnInsert", Value: bson.D{
			{Key: "captured_at", Value: now},
			{Key: "tag", Value: ""},
			{Key: "status", Value: 0},
		}},
	}
	opts := options.Update().SetUpsert(true)
	_, err := repo.conn.Datastore.
		Collection(repo.fetchRepo.Collection).
		UpdateOne(ctx, filter, update, opts)
	return err
}

func (repo *MongoRepository) ChangeStatusById(ctx context.Context, id string, status int) error {
	filter := bson.M{"_id": id}
	update := bson.M{"$set": bson.M{"status": status}}
	_, err := repo.conn.Datastore.Collection(repo.fetchRepo.Collection).UpdateOne(ctx, filter, update)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error changing status by ID.")
		return err
	}
	return nil
}

func (repo *MongoRepository) GetAllByStatus(ctx context.Context, status int) ([]*FetchedProfile, error) {
	filter := bson.M{"status": status}
	cursor, err := repo.conn.Datastore.Collection(repo.fetchRepo.Collection).Find(ctx, filter)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error getting all by status.")
		return nil, err
	}

	var profiles []*FetchedProfile
	for cursor.Next(ctx) {
		var profile FetchedProfile
		err = cursor.Decode(&profile)
		if err != nil {
			logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error decoding profile.")
			return nil, err
		}
		profiles = append(profiles, &profile)
	}
	return profiles, nil
}

func (repo *MongoRepository) GetAllByTag(ctx context.Context, tag string) ([]*FetchedProfile, error) {
	filter := bson.M{"tag": tag}
	cursor, err := repo.conn.Datastore.Collection(repo.fetchRepo.Collection).Find(ctx, filter)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error getting all by tag.")
		return nil, err
	}

	var profiles []*FetchedProfile
	for cursor.Next(ctx) {
		var profile FetchedProfile
		err = cursor.Decode(&profile)
		if err != nil {
			logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error decoding profile.")
			return nil, err
		}
		profiles = append(profiles, &profile)
	}
	return profiles, nil
}

func (repo *MongoRepository) UpdateTagById(ctx context.Context, id string, tag string) error {
	filter := bson.M{"_id": id}
	update := bson.M{"$set": bson.M{"tag": tag}}
	_, err := repo.conn.Datastore.Collection(repo.fetchRepo.Collection).UpdateOne(ctx, filter, update)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error updating tag by ID.")
		return err
	}
	return nil
}

func (repo *MongoRepository) ChangeStatusMultiple(ctx context.Context, ids []string, status int) error {
	if len(ids) == 0 {
		return fmt.Errorf("ids cannot be empty")
	}

	filter := bson.M{"_id": bson.M{"$in": ids}}
	update := bson.M{"$set": bson.M{"status": status}}
	_, err := repo.conn.Datastore.Collection(repo.fetchRepo.Collection).UpdateMany(ctx, filter, update)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error changing status for multiple IDs.")
		return err
	}
	return nil
}

// following.indexes
// following_id
func (repo *MongoRepository) GetTopHit(
	ctx context.Context,
	page, limit int64,
	tags []string,
) (*persistence.Pagination[[]*FetchedProfileFull], error) {

	// 0) build tag + status filter
	filter := bson.D{}
	if len(tags) > 0 {
		filter = append(filter, bson.E{"tag", bson.D{{"$in", tags}}})
	}
	// only statuses 2 & 3
	filter = append(filter, bson.E{"status", bson.D{{"$in", bson.A{2, 3}}}})

	// 1) count total for pagination
	total, err := repo.conn.Datastore.
		Collection(repo.fetchRepo.Collection).
		CountDocuments(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("count profiles: %w", err)
	}

	skip := (page - 1) * limit

	// 2) build aggregation pipeline
	pipeline := mongo.Pipeline{}

	// – match tags & status
	pipeline = append(pipeline, bson.D{{"$match", filter}})

	// – helper to sort status=2 first
	pipeline = append(pipeline, bson.D{{"$addFields", bson.D{{
		"isNotTwo", bson.D{{
			"$cond", bson.A{
				bson.D{{"$eq", bson.A{"$status", 2}}},
				0,
				1,
			},
		}},
	}}}})

	// – sort by isNotTwo, then follower_count
	pipeline = append(pipeline, bson.D{{"$sort", bson.D{
		{"isNotTwo", 1},
		{"follower_count", -1},
	}}})

	// – pagination
	pipeline = append(pipeline, bson.D{{"$skip", skip}})
	pipeline = append(pipeline, bson.D{{"$limit", limit}})

	// ─── NEW: lookup followers count ─────────────────────────────────────
	pipeline = append(pipeline, bson.D{{"$lookup", bson.D{
		{"from", "following.indexes"},
		{"localField", "_id"},
		{"foreignField", "following_id"},
		{"as", "followers_arr"},
	}}})

	pipeline = append(pipeline, bson.D{{"$addFields", bson.D{
		{"followed_by_count", bson.D{{"$size", "$followers_arr"}}},
	}}})

	// – cleanup helper & lookup array
	pipeline = append(pipeline, bson.D{{"$project", bson.D{
		{"isNotTwo", 0},
		{"followers_arr", 0},
	}}})

	// 3) execute
	cursor, err := repo.conn.Datastore.
		Collection(repo.fetchRepo.Collection).
		Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("aggregate top hits full: %w", err)
	}
	defer cursor.Close(ctx)

	// 4) decode into FetchedProfileFull
	var items []*FetchedProfileFull
	for cursor.Next(ctx) {
		var p FetchedProfileFull
		if err := cursor.Decode(&p); err != nil {
			return nil, fmt.Errorf("decode full profile: %w", err)
		}
		items = append(items, &p)
	}
	if err := cursor.Err(); err != nil {
		return nil, fmt.Errorf("cursor error: %w", err)
	}

	// 5) return new pagination
	return &persistence.Pagination[[]*FetchedProfileFull]{
		Result:     items,
		Page:       page,
		Size:       limit,
		TotalCount: total,
	}, nil
}

func (repo *MongoRepository) GetTopHitRaw(
	ctx context.Context,
	limit int64,
	tags []string,
) ([]*FetchedProfile, error) {
	// 0) match only your tags _and_ only statuses 2 or 3
	filter := bson.D{
		{"tag", bson.D{{"$in", tags}}},
		{"status", bson.D{{"$in", bson.A{2, 3}}}},
	}

	// 1) build pipeline
	pipeline := mongo.Pipeline{
		// only the docs we care about
		{{"$match", filter}},

		// assign a numeric “priority” (0 for status=2, 1 for status=3)
		{{"$addFields", bson.D{{
			"priority", bson.D{{
				"$cond", bson.A{
					bson.D{{"$eq", bson.A{"$status", 2}}},
					0,
					1,
				},
			}},
		}}}},

		// sort by that priority, then by follower_count descending
		{{"$sort", bson.D{
			{"priority", 1},
			{"follower_count", -1},
		}}},

		// optional: cap your result set
		{{"$limit", limit}},

		// clean up our helper field
		{{"$project", bson.D{
			{"priority", 0},
		}}},
	}

	// 2) execute
	cursor, err := repo.conn.Datastore.Collection(repo.fetchRepo.Collection).
		Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("aggregate status 2→3: %w", err)
	}
	defer cursor.Close(ctx)

	var out []*FetchedProfile
	for cursor.Next(ctx) {
		var p FetchedProfile
		if err := cursor.Decode(&p); err != nil {
			return nil, fmt.Errorf("decode profile: %w", err)
		}
		out = append(out, &p)
	}
	if err := cursor.Err(); err != nil {
		return nil, fmt.Errorf("cursor: %w", err)
	}

	return out, nil
}

func (repo *MongoRepository) GetUniqueTags(ctx context.Context) ([]string, error) {
	// Use distinct to get unique tags
	tags, err := repo.conn.Datastore.Collection(repo.fetchRepo.Collection).Distinct(ctx, "tag", bson.M{})
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error getting unique tags.")
		return nil, err
	}

	// Convert interface{} to []string
	var uniqueTags []string
	for _, tag := range tags {
		if strTag, ok := tag.(string); ok {
			uniqueTags = append(uniqueTags, strTag)
		}
	}

	return uniqueTags, nil
}

func (repo *MongoRepository) FindAllByIDs(ctx context.Context, ids []string) ([]FetchedProfile, error) {
	filter := bson.M{"_id": bson.M{"$in": ids}}
	cursor, err := repo.conn.Datastore.Collection(repo.fetchRepo.Collection).Find(ctx, filter, nil)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error finding fetched profiles by ID.")
		return nil, err
	}

	var profiles []FetchedProfile
	for cursor.Next(ctx) {
		var profile FetchedProfile
		err = cursor.Decode(&profile)
		if err != nil {
			logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error decoding fetched profile.")
			continue
		}
		profiles = append(profiles, profile)
	}
	return profiles, nil
}
