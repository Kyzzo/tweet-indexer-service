package twitterprofile

import (
	"context"
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
}
type MongoRepository struct {
	conn    *mongodb.MongoConnection
	absrepo *persistence.AbstractMongoRepository[*TwitterProfile]
}

func NewRepository(conn *mongodb.MongoConnection) Repository {
	absrepo := persistence.NewAbstractRepository[*TwitterProfile](conn, Collection)
	return &MongoRepository{
		conn:    conn,
		absrepo: absrepo,
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
