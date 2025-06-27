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
	FindFetchedProfiles(ctx context.Context, q *FetchedProfileQuery) (*persistence.Pagination[[]*FetchedProfile], error)
	UpsertFetchedProfile(ctx context.Context, u twiterTypes.FollowingUser) error
	ChangeStatusById(ctx context.Context, id string, status int) error
	ChangeStatusMultiple(ctx context.Context, ids []string, status int) error
	UpdateTagById(ctx context.Context, id string, tag string) error
	GetTopHit(ctx context.Context, page, limit int64, tags []string) (*persistence.Pagination[[]*FetchedProfile], error)
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
) (*persistence.Pagination[[]*FetchedProfile], error) {
	// 1) Build the filter document
	filter := bson.M{}

	if q.Status != nil {
		filter["status"] = *q.Status
	}
	if len(q.Tags) > 0 {
		// since Profile.Tag is a single string,
		// this matches any profile whose tag is in the list
		filter["tag"] = bson.M{"$in": q.Tags}
	}
	// follower count range
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

	// 2) Build the find options (skip, limit, sort)
	opts := options.Find().
		SetSkip((q.Page - 1) * q.Size).
		SetLimit(q.Size)

	// determine sort direction
	dir := -1
	if q.Asc {
		dir = 1
	}
	// default to captured_at if no field given
	sortField := q.SortField
	if sortField == "" {
		sortField = "captured_at"
	}
	opts.SetSort(bson.D{{Key: sortField, Value: dir}})
	// 3) Execute the query
	cursor, err := repo.conn.Datastore.
		Collection(repo.fetchRepo.Collection).
		Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("FindFetchedProfiles: %w", err)
	}
	defer cursor.Close(ctx)

	var users []*FetchedProfile
	for cursor.Next(ctx) {
		var u FetchedProfile
		if err := cursor.Decode(&u); err != nil {
			return nil, fmt.Errorf("FindFetchedProfiles decode: %w", err)
		}
		users = append(users, &u)
	}

	// 4) Get total count for pagination metadata
	total, err := repo.conn.Datastore.
		Collection(repo.fetchRepo.Collection).
		CountDocuments(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("FindFetchedProfiles count: %w", err)
	}

	// 5) Wrap it up in your pagination type
	pag := persistence.NewPagination[[]*FetchedProfile](
		q.Page, q.Size, users, total,
	)
	return pag, nil
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

func (repo *MongoRepository) GetTopHit(
	ctx context.Context,
	page, limit int64,
	tags []string,
) (*persistence.Pagination[[]*FetchedProfile], error) {
	// 0) build a BSON filter for tags
	var filter bson.D
	if len(tags) > 0 {
		filter = bson.D{{"tag", bson.D{{"$in", tags}}}}
	} else {
		filter = bson.D{} // match everything
	}
	filter = append(filter, bson.E{Key: "status", Value: bson.D{{Key: "$in", Value: bson.A{2, 3}}}})
	// filter = append(filter, bson.E{Key: "status", Value: bson.D{{Key: "$in", Value: bson.A{2, 3}}}})
	// 1) count total matching documents (for pagination)
	total, err := repo.conn.Datastore.Collection(repo.fetchRepo.Collection).CountDocuments(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("count profiles: %w", err)
	}

	// 2) compute skip
	skip := (page - 1) * limit

	// 3) build our aggregation pipeline
	pipeline := mongo.Pipeline{}

	// – optional tag‐filter
	if len(filter) > 0 {
		pipeline = append(pipeline, bson.D{{"$match", filter}})
	}

	// – synthesize isNotTwo = 0 if status==2 else 1
	pipeline = append(pipeline, bson.D{{
		"$addFields", bson.D{{
			"isNotTwo", bson.D{{
				"$cond", bson.A{
					bson.D{{"$eq", bson.A{"$status", 2}}},
					0,
					1,
				},
			}},
		}},
	}})

	// – sort first by isNotTwo asc, then follower_count desc
	pipeline = append(pipeline, bson.D{{
		"$sort", bson.D{
			{"isNotTwo", 1},
			{"follower_count", -1},
		},
	}})

	// – pagination: skip then limit
	pipeline = append(pipeline, bson.D{{"$skip", skip}})
	pipeline = append(pipeline, bson.D{{"$limit", limit}})

	// – remove our helper field
	pipeline = append(pipeline, bson.D{{"$project", bson.D{
		{"isNotTwo", 0},
	}}})

	// 4) execute it
	cursor, err := repo.conn.Datastore.Collection(repo.fetchRepo.Collection).Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("aggregate top hits: %w", err)
	}
	defer cursor.Close(ctx)

	var pageItems []*FetchedProfile
	for cursor.Next(ctx) {
		var p FetchedProfile
		if err := cursor.Decode(&p); err != nil {
			return nil, fmt.Errorf("decode profile: %w", err)
		}
		pageItems = append(pageItems, &p)
	}
	if err := cursor.Err(); err != nil {
		return nil, fmt.Errorf("cursor error: %w", err)
	}

	return &persistence.Pagination[[]*FetchedProfile]{
		Result:     pageItems,
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
