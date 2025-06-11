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
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const Collection = "twitterprofiles"
const FetchedCollection = "twitterprofiles_fetched"

type Repository interface {
	UpdateOrInsert(ctx context.Context, usr *TwitterProfile) error
	FindByUserId(ctx context.Context, twitterId string) (*TwitterProfile, error)
	FindUserByTwitterHandle(ctx context.Context, twitterHandle string) (*TwitterProfile, error)
	FindUserByName(ctx context.Context, usrName string) (*TwitterProfile, error)
	FindAllUsersPagination(ctx context.Context, page, size int64, asc bool, sortedType string) (*persistence.Pagination[[]*TwitterProfile], error)
	FindAllUsersPaginationByNameOrTwitterHandle(ctx context.Context, page, size int64, name string) (*persistence.Pagination[[]*TwitterProfile], error)
	FindTotalCount(ctx context.Context) (int64, error)
	FindAllUsers(ctx context.Context) ([]*TwitterProfile, error)
	FindUsersByIds(ctx context.Context, userIds ...string) ([]*TwitterProfile, error)
	FindByIsCrypto(ctx context.Context, isCrypto bool) ([]*TwitterProfile, error)
	FindByIsCryptoPagination(ctx context.Context, page, size int64, isCrypto bool) (*persistence.Pagination[[]*TwitterProfile], error)
	FindByCryptoAndTracking(ctx context.Context, isCrypto, isTracking bool) ([]*TwitterProfile, error)
	FindByCryptoAndTrackingPagination(ctx context.Context, page, size int64, isCrypto, isTracking bool) (*persistence.Pagination[[]*TwitterProfile], error)
	FindByIsTracking(ctx context.Context, isTracking bool) ([]*TwitterProfile, error)
	FindByIsTrackingPagination(ctx context.Context, page, size int64, isTracking bool) (*persistence.Pagination[[]*TwitterProfile], error)
	UpsertTwitterProfile(ctx context.Context, u twiterTypes.FollowingUser) error
	FindAllById(ctx context.Context, ids []string) ([]TwitterProfile, error)
	/////Fecthed///////////////
	UpdateOrInsertFetched(ctx context.Context, usr *FetchedProfile) error
	FindFetchedByUserId(ctx context.Context, twitterId string) (*FetchedProfile, error)
	FindFetchedUserByTwitterHandle(ctx context.Context, twitterHandle string) (*FetchedProfile, error)
	FindFetchedUserByName(ctx context.Context, usrName string) (*FetchedProfile, error)
	FindFetchedProfiles(ctx context.Context, q *FetchedProfileQuery) (*persistence.Pagination[[]*FetchedProfile], error)
	UpsertFetchedProfile(ctx context.Context, u twiterTypes.FollowingUser) error
	ChangeStatusById(ctx context.Context, id string, status int) error
	UpdateTagById(ctx context.Context, id string, tag string) error
	GetAllByStatus(ctx context.Context, status int) ([]*FetchedProfile, error)
	GetAllByTag(ctx context.Context, tag string) ([]*FetchedProfile, error)
}
type MongoRepository struct {
	conn      *mongodb.MongoConnection
	absrepo   *persistence.AbstractMongoRepository[*TwitterProfile]
	fetchRepo *persistence.AbstractMongoRepository[*FetchedProfile]
}

func NewRepository(conn *mongodb.MongoConnection) Repository {
	absrepo := persistence.NewAbstractRepository[*TwitterProfile](conn, Collection)
	fetchRepo := persistence.NewAbstractRepository[*FetchedProfile](conn, FetchedCollection)
	return &MongoRepository{
		conn:      conn,
		absrepo:   absrepo,
		fetchRepo: fetchRepo,
	}
}

func (repo *MongoRepository) UpdateOrInsert(ctx context.Context, usr *TwitterProfile) error {
	err := repo.absrepo.InsertOrUpdate(usr)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error updating or inserting user.")
		return err
	}
	return nil
}

func (repo *MongoRepository) FindByUserId(ctx context.Context, twitterId string) (*TwitterProfile, error) {
	filter := bson.M{"_id": twitterId}
	var usr TwitterProfile
	res := repo.conn.Datastore.Collection(repo.absrepo.Collection).FindOne(ctx, filter)
	if err := res.Decode(&usr); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error decoding user result.")
		return nil, err
	}
	return &usr, nil
}

func (repo *MongoRepository) FindUserByTwitterHandle(ctx context.Context, twitterHandle string) (*TwitterProfile, error) {
	filter := bson.M{"handle": twitterHandle}
	var usr TwitterProfile
	res := repo.conn.Datastore.Collection(repo.absrepo.Collection).FindOne(ctx, filter)
	if err := res.Decode(&usr); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error decoding user result.")
		return nil, err
	}
	return &usr, nil
}

func (repo *MongoRepository) FindTotalCount(ctx context.Context) (int64, error) {
	count, err := repo.conn.Datastore.Collection(repo.absrepo.Collection).CountDocuments(ctx, bson.M{})
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (repo *MongoRepository) GetCountByFilter(ctx context.Context, filter bson.M) (int64, error) {
	count, err := repo.conn.Datastore.Collection(repo.absrepo.Collection).CountDocuments(ctx, filter)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error counting documents by filter.")
		return 0, err
	}
	return count, nil
}

func (repo *MongoRepository) FindUserByName(ctx context.Context, usrName string) (*TwitterProfile, error) {
	filter := bson.M{"handle": usrName}
	var usr TwitterProfile
	res := repo.conn.Datastore.Collection(repo.absrepo.Collection).FindOne(ctx, filter)
	if err := res.Decode(&usr); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error decoding user result.")
		return nil, err
	}

	return &usr, nil
}

func (repo *MongoRepository) FindAllUsersPagination(ctx context.Context, page, size int64, asc bool, sortedType string) (*persistence.Pagination[[]*TwitterProfile], error) {
	opts := options.Find()
	if page == 1 {
		opts.SetSkip(0)
		opts.SetLimit(size)
	} else {
		opts.SetSkip((page - 1) * size)
		opts.SetLimit(size)
	}

	opts.SetSort(bson.D{{sortedType, -1}})

	if asc {
		opts.SetSort(bson.D{{sortedType, 1}})

	}
	var users []*TwitterProfile
	cursor, err := repo.conn.Datastore.Collection(repo.absrepo.Collection).Find(ctx, bson.M{}, opts)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error finding all users.")
		return nil, err
	}

	for cursor.Next(ctx) {
		var user *TwitterProfile
		err = cursor.Decode(&user)
		if err != nil {
			logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error decoding usr.")
			return nil, err
		}
		users = append(users, user)
	}
	totalCount, err := repo.FindTotalCount(ctx)
	if err != nil {
		return nil, err
	}

	pag := persistence.NewPagination[[]*TwitterProfile](page, size, users, totalCount)
	return pag, nil
}

func (repo *MongoRepository) FindAllUsersPaginationByNameOrTwitterHandle(ctx context.Context, page, size int64, name string) (*persistence.Pagination[[]*TwitterProfile], error) {
	opts := options.Find()
	if page == 1 {
		opts.SetSkip(0)
		opts.SetLimit(size)
	} else {
		opts.SetSkip((page - 1) * size)
		opts.SetLimit(size)
	}
	filters := bson.M{
		"$or": []bson.M{
			{"user_name": primitive.Regex{Pattern: ".*" + name + ".*", Options: "i"}},
			{"handle": primitive.Regex{Pattern: ".*" + name + ".*", Options: "i"}},
		},
	}
	//TODO: sort methodaligy

	var users []*TwitterProfile
	cursor, err := repo.conn.Datastore.Collection(repo.absrepo.Collection).Find(ctx, filters, opts)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error finding all users.")
		return nil, err
	}

	for cursor.Next(ctx) {
		var user *TwitterProfile
		err = cursor.Decode(&user)
		if err != nil {
			logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error decoding usr.")
			return nil, err
		}
		users = append(users, user)
	}
	totalCount, err := repo.FindTotalCount(ctx)
	if err != nil {
		return nil, err
	}

	pag := persistence.NewPagination[[]*TwitterProfile](page, size, users, totalCount)
	return pag, nil
}

func (repo *MongoRepository) FindAllUsers(ctx context.Context) ([]*TwitterProfile, error) {
	filter := bson.M{}
	var users []*TwitterProfile
	cursor, err := repo.conn.Datastore.Collection(repo.absrepo.Collection).Find(ctx, filter, nil)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error finding all users.")
		return nil, err
	}

	for cursor.Next(ctx) {
		var user *TwitterProfile
		err = cursor.Decode(&user)
		if err != nil {
			logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error decoding usr.")
			return nil, err
		}
		users = append(users, user)
	}
	return users, nil
}

func (repo *MongoRepository) FindUsersByIds(ctx context.Context, userIds ...string) ([]*TwitterProfile, error) {
	filter := bson.M{"_id": bson.M{"$in": userIds}}
	cursor, err := repo.conn.Datastore.Collection(repo.absrepo.Collection).Find(ctx, filter, nil)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error finding users by ID.")
		return nil, err
	}

	userMap := make(map[string]*TwitterProfile)
	for cursor.Next(ctx) {
		var user *TwitterProfile
		err = cursor.Decode(&user)
		if err != nil {
			logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error decoding user.")
			return nil, err
		}
		userMap[user.ID] = user
	}

	var orderedUsers []*TwitterProfile
	for _, id := range userIds {
		if user, exists := userMap[id]; exists {
			orderedUsers = append(orderedUsers, user)
		}
	}
	return orderedUsers, nil
}

func (repo *MongoRepository) FindByIsCrypto(ctx context.Context, isCrypto bool) ([]*TwitterProfile, error) {
	filter := bson.M{"is_crypto": isCrypto}
	sortedType := "captured_at"
	cursor, err := repo.conn.Datastore.Collection(repo.absrepo.Collection).Find(ctx, filter, options.Find().SetSort(bson.D{{sortedType, -1}}))
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error finding users by isCrypto.")
		return nil, err
	}

	var users []*TwitterProfile
	for cursor.Next(ctx) {
		var user *TwitterProfile
		err = cursor.Decode(&user)
		if err != nil {
			logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error decoding user.")
			return nil, err
		}
		users = append(users, user)
	}
	return users, nil
}

func (repo *MongoRepository) FindByIsCryptoPagination(ctx context.Context, page, size int64, isCrypto bool) (*persistence.Pagination[[]*TwitterProfile], error) {
	opts := options.Find()
	if page == 1 {
		opts.SetSkip(0)
		opts.SetLimit(size)
	} else {
		opts.SetSkip((page - 1) * size)
		opts.SetLimit(size)
	}
	sortedType := "captured_at"
	opts.SetSort(bson.D{{sortedType, -1}})
	filter := bson.M{"is_crypto": isCrypto}
	var users []*TwitterProfile
	cursor, err := repo.conn.Datastore.Collection(repo.absrepo.Collection).Find(ctx, filter, opts)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error finding users by isCrypto.")
		return nil, err
	}

	for cursor.Next(ctx) {
		var user *TwitterProfile
		err = cursor.Decode(&user)
		if err != nil {
			logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error decoding user.")
			return nil, err
		}
		users = append(users, user)
	}
	totalCount, err := repo.GetCountByFilter(ctx, bson.M{"is_crypto": isCrypto})
	if err != nil {
		return nil, err
	}

	pag := persistence.NewPagination[[]*TwitterProfile](page, size, users, totalCount)
	return pag, nil
}

func (repo *MongoRepository) FindByCryptoAndTracking(ctx context.Context, isCrypto, isTracking bool) ([]*TwitterProfile, error) {
	filter := bson.M{"is_crypto": isCrypto, "tracking": isTracking}
	cursor, err := repo.conn.Datastore.Collection(repo.absrepo.Collection).Find(ctx, filter, nil)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error finding users by isCrypto and isTracking.")
		return nil, err
	}

	var users []*TwitterProfile
	for cursor.Next(ctx) {
		var user *TwitterProfile
		err = cursor.Decode(&user)
		if err != nil {
			logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error decoding user.")
			return nil, err
		}
		users = append(users, user)
	}
	return users, nil
}

func (repo *MongoRepository) FindByCryptoAndTrackingPagination(ctx context.Context, page, size int64, isCrypto, isTracking bool) (*persistence.Pagination[[]*TwitterProfile], error) {
	opts := options.Find()
	if page == 1 {
		opts.SetSkip(0)
		opts.SetLimit(size)
	} else {
		opts.SetSkip((page - 1) * size)
		opts.SetLimit(size)
	}
	sortedType := "captured_at"
	opts.SetSort(bson.D{{sortedType, -1}})
	filter := bson.M{"is_crypto": isCrypto, "tracking": isTracking}
	var users []*TwitterProfile
	cursor, err := repo.conn.Datastore.Collection(repo.absrepo.Collection).Find(ctx, filter, opts)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error finding users by isCrypto and isTracking.")
		return nil, err
	}

	for cursor.Next(ctx) {
		var user *TwitterProfile
		err = cursor.Decode(&user)
		if err != nil {
			logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error decoding user.")
			return nil, err
		}
		users = append(users, user)
	}
	totalCount, err := repo.GetCountByFilter(ctx, bson.M{"is_crypto": isCrypto, "tracking": isTracking})
	if err != nil {
		return nil, err
	}

	pag := persistence.NewPagination[[]*TwitterProfile](page, size, users, totalCount)
	return pag, nil
}

func (repo *MongoRepository) FindByIsTracking(ctx context.Context, isTracking bool) ([]*TwitterProfile, error) {
	filter := bson.M{"tracking": isTracking}
	cursor, err := repo.conn.Datastore.Collection(repo.absrepo.Collection).Find(ctx, filter, nil)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error finding users by isTracking.")
		return nil, err
	}

	var users []*TwitterProfile
	for cursor.Next(ctx) {
		var user *TwitterProfile
		err = cursor.Decode(&user)
		if err != nil {
			logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error decoding user.")
			return nil, err
		}
		users = append(users, user)
	}
	return users, nil
}

func (repo *MongoRepository) FindByIsTrackingPagination(ctx context.Context, page, size int64, isTracking bool) (*persistence.Pagination[[]*TwitterProfile], error) {
	opts := options.Find()
	if page == 1 {
		opts.SetSkip(0)
		opts.SetLimit(size)
	} else {
		opts.SetSkip((page - 1) * size)
		opts.SetLimit(size)
	}

	filter := bson.M{"tracking": isTracking}
	sortedType := "captured_at"
	opts.SetSort(bson.D{{sortedType, -1}})
	var users []*TwitterProfile
	cursor, err := repo.conn.Datastore.Collection(repo.absrepo.Collection).Find(ctx, filter, opts)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error finding users by isTracking.")
		return nil, err
	}

	for cursor.Next(ctx) {
		var user *TwitterProfile
		err = cursor.Decode(&user)
		if err != nil {
			logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error decoding user.")
			return nil, err
		}
		users = append(users, user)
	}
	totalCount, err := repo.GetCountByFilter(ctx, bson.M{"tracking": isTracking})
	if err != nil {
		return nil, err
	}

	pag := persistence.NewPagination[[]*TwitterProfile](page, size, users, totalCount)
	return pag, nil
}

func (repo *MongoRepository) UpsertTwitterProfile(ctx context.Context, u twiterTypes.FollowingUser) error {
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
			{Key: "is_crypto", Value: false},
			{Key: "tracking", Value: false},
		}},
	}

	opts := options.Update().SetUpsert(true)
	_, err := repo.conn.Datastore.
		Collection(repo.absrepo.Collection).
		UpdateOne(ctx, filter, update, opts)
	return err
}

func (repo *MongoRepository) FindAllById(ctx context.Context, ids []string) ([]TwitterProfile, error) {
	filter := bson.M{"_id": bson.M{"$in": ids}}
	cursor, err := repo.conn.Datastore.Collection(repo.absrepo.Collection).Find(ctx, filter, nil)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error finding users by ID.")
		return nil, err
	}

	var users []TwitterProfile
	for cursor.Next(ctx) {
		var user TwitterProfile
		err = cursor.Decode(&user)
		if err != nil {
			logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error decoding user.")
			return nil, err
		}
		users = append(users, user)
	}
	return users, nil
}

// ///////////////Fecthed///////////////
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
	fmt.Println("Sort field:", sortField, "Direction:", dir)
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
