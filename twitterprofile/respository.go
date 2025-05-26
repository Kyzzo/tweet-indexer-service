package twitterprofile

import (
	"context"

	"github.com/jeetdotso/tweet-indexer-service/logg"
	"github.com/jeetdotso/tweet-indexer-service/persistence"
	"github.com/jeetdotso/tweet-indexer-service/persistence/mongodb"
	twitterTypes "github.com/jeetdotso/tweet-indexer-service/scrape/internals/api/types"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const Collection = "twittterprofiles"

type Repository interface {
	UpdateOrInsert(ctx context.Context, usr *twitterTypes.FollowingUser) error
	FindByUserId(ctx context.Context, twitterId string) (*twitterTypes.FollowingUser, error)
	FindUserByTwitterHandle(ctx context.Context, twitterHandle string) (*twitterTypes.FollowingUser, error)
	FindUserByName(ctx context.Context, usrName string) (*twitterTypes.FollowingUser, error)
	FindAllUsersPagination(ctx context.Context, page, size int64, asc bool, sortedType string) (*persistence.Pagination[[]*twitterTypes.FollowingUser], error)
	FindAllUsersPaginationByNameOrTwitterHandle(ctx context.Context, page, size int64, name string) (*persistence.Pagination[[]*twitterTypes.FollowingUser], error)
	FindTotalCount(ctx context.Context) (int64, error)
	FindAllUsers(ctx context.Context) ([]*twitterTypes.FollowingUser, error)
	FindUsersByIds(ctx context.Context, userIds ...string) ([]*twitterTypes.FollowingUser, error)
	FindByIsCrypto(ctx context.Context, isCrypto bool) ([]*twitterTypes.FollowingUser, error)
	FindByIsCryptoPagination(ctx context.Context, page, size int64, isCrypto bool) (*persistence.Pagination[[]*twitterTypes.FollowingUser], error)
	FinbdByCryptoAndTracking(ctx context.Context, isCrypto, isTracking bool) ([]*twitterTypes.FollowingUser, error)
	FinbdByCryptoAndTrackingPagination(ctx context.Context, page, size int64, isCrypto, isTracking bool) (*persistence.Pagination[[]*twitterTypes.FollowingUser], error)
	FindByIsTracking(ctx context.Context, isTracking bool) ([]*twitterTypes.FollowingUser, error)
	FindByIsTrackingPagination(ctx context.Context, page, size int64, isTracking bool) (*persistence.Pagination[[]*twitterTypes.FollowingUser], error)
}
type MongoRepository struct {
	conn    *mongodb.MongoConnection
	absrepo *persistence.AbstractMongoRepository[*twitterTypes.FollowingUser]
}

func NewRepository(conn *mongodb.MongoConnection) Repository {
	absrepo := persistence.NewAbstractRepository[*twitterTypes.FollowingUser](conn, Collection)
	return &MongoRepository{
		conn:    conn,
		absrepo: absrepo,
	}
}

func (repo *MongoRepository) UpdateOrInsert(ctx context.Context, usr *twitterTypes.FollowingUser) error {
	err := repo.absrepo.InsertOrUpdate(usr)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error updating or inserting user.")
		return err
	}
	return nil
}

func (repo *MongoRepository) FindByUserId(ctx context.Context, twitterId string) (*twitterTypes.FollowingUser, error) {
	filter := bson.M{"_id": twitterId}
	var usr twitterTypes.FollowingUser
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

func (repo *MongoRepository) FindUserByTwitterHandle(ctx context.Context, twitterHandle string) (*twitterTypes.FollowingUser, error) {
	filter := bson.M{"handle": twitterHandle}
	var usr twitterTypes.FollowingUser
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

func (repo *MongoRepository) FindUserByName(ctx context.Context, usrName string) (*twitterTypes.FollowingUser, error) {
	filter := bson.M{"handle": usrName}
	var usr twitterTypes.FollowingUser
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

func (repo *MongoRepository) FindAllUsersPagination(ctx context.Context, page, size int64, asc bool, sortedType string) (*persistence.Pagination[[]*twitterTypes.FollowingUser], error) {
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
	var users []*twitterTypes.FollowingUser
	cursor, err := repo.conn.Datastore.Collection(repo.absrepo.Collection).Find(ctx, bson.M{}, opts)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error finding all users.")
		return nil, err
	}

	for cursor.Next(ctx) {
		var user *twitterTypes.FollowingUser
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

	pag := persistence.NewPagination[[]*twitterTypes.FollowingUser](page, size, users, totalCount)
	return pag, nil
}

func (repo *MongoRepository) FindAllUsersPaginationByNameOrTwitterHandle(ctx context.Context, page, size int64, name string) (*persistence.Pagination[[]*twitterTypes.FollowingUser], error) {
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

	var users []*twitterTypes.FollowingUser
	cursor, err := repo.conn.Datastore.Collection(repo.absrepo.Collection).Find(ctx, filters, opts)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error finding all users.")
		return nil, err
	}

	for cursor.Next(ctx) {
		var user *twitterTypes.FollowingUser
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

	pag := persistence.NewPagination[[]*twitterTypes.FollowingUser](page, size, users, totalCount)
	return pag, nil
}

func (repo *MongoRepository) FindAllUsers(ctx context.Context) ([]*twitterTypes.FollowingUser, error) {
	filter := bson.M{}
	var users []*twitterTypes.FollowingUser
	cursor, err := repo.conn.Datastore.Collection(repo.absrepo.Collection).Find(ctx, filter, nil)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error finding all users.")
		return nil, err
	}

	for cursor.Next(ctx) {
		var user *twitterTypes.FollowingUser
		err = cursor.Decode(&user)
		if err != nil {
			logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error decoding usr.")
			return nil, err
		}
		users = append(users, user)
	}
	return users, nil
}

func (repo *MongoRepository) FindUsersByIds(ctx context.Context, userIds ...string) ([]*twitterTypes.FollowingUser, error) {
	filter := bson.M{"_id": bson.M{"$in": userIds}}
	cursor, err := repo.conn.Datastore.Collection(repo.absrepo.Collection).Find(ctx, filter, nil)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error finding users by ID.")
		return nil, err
	}

	userMap := make(map[string]*twitterTypes.FollowingUser)
	for cursor.Next(ctx) {
		var user *twitterTypes.FollowingUser
		err = cursor.Decode(&user)
		if err != nil {
			logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error decoding user.")
			return nil, err
		}
		userMap[user.ID] = user
	}

	var orderedUsers []*twitterTypes.FollowingUser
	for _, id := range userIds {
		if user, exists := userMap[id]; exists {
			orderedUsers = append(orderedUsers, user)
		}
	}
	return orderedUsers, nil
}

func (repo *MongoRepository) FindByIsCrypto(ctx context.Context, isCrypto bool) ([]*twitterTypes.FollowingUser, error) {
	filter := bson.M{"is_crypto": isCrypto}
	cursor, err := repo.conn.Datastore.Collection(repo.absrepo.Collection).Find(ctx, filter, nil)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error finding users by isCrypto.")
		return nil, err
	}

	var users []*twitterTypes.FollowingUser
	for cursor.Next(ctx) {
		var user *twitterTypes.FollowingUser
		err = cursor.Decode(&user)
		if err != nil {
			logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error decoding user.")
			return nil, err
		}
		users = append(users, user)
	}
	return users, nil
}

func (repo *MongoRepository) FindByIsCryptoPagination(ctx context.Context, page, size int64, isCrypto bool) (*persistence.Pagination[[]*twitterTypes.FollowingUser], error) {
	opts := options.Find()
	if page == 1 {
		opts.SetSkip(0)
		opts.SetLimit(size)
	} else {
		opts.SetSkip((page - 1) * size)
		opts.SetLimit(size)
	}

	filter := bson.M{"is_crypto": isCrypto}
	var users []*twitterTypes.FollowingUser
	cursor, err := repo.conn.Datastore.Collection(repo.absrepo.Collection).Find(ctx, filter, opts)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error finding users by isCrypto.")
		return nil, err
	}

	for cursor.Next(ctx) {
		var user *twitterTypes.FollowingUser
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

	pag := persistence.NewPagination[[]*twitterTypes.FollowingUser](page, size, users, totalCount)
	return pag, nil
}

func (repo *MongoRepository) FinbdByCryptoAndTracking(ctx context.Context, isCrypto, isTracking bool) ([]*twitterTypes.FollowingUser, error) {
	filter := bson.M{"is_crypto": isCrypto, "tracking": isTracking}
	cursor, err := repo.conn.Datastore.Collection(repo.absrepo.Collection).Find(ctx, filter, nil)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error finding users by isCrypto and isTracking.")
		return nil, err
	}

	var users []*twitterTypes.FollowingUser
	for cursor.Next(ctx) {
		var user *twitterTypes.FollowingUser
		err = cursor.Decode(&user)
		if err != nil {
			logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error decoding user.")
			return nil, err
		}
		users = append(users, user)
	}
	return users, nil
}

func (repo *MongoRepository) FinbdByCryptoAndTrackingPagination(ctx context.Context, page, size int64, isCrypto, isTracking bool) (*persistence.Pagination[[]*twitterTypes.FollowingUser], error) {
	opts := options.Find()
	if page == 1 {
		opts.SetSkip(0)
		opts.SetLimit(size)
	} else {
		opts.SetSkip((page - 1) * size)
		opts.SetLimit(size)
	}

	filter := bson.M{"is_crypto": isCrypto, "tracking": isTracking}
	var users []*twitterTypes.FollowingUser
	cursor, err := repo.conn.Datastore.Collection(repo.absrepo.Collection).Find(ctx, filter, opts)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error finding users by isCrypto and isTracking.")
		return nil, err
	}

	for cursor.Next(ctx) {
		var user *twitterTypes.FollowingUser
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

	pag := persistence.NewPagination[[]*twitterTypes.FollowingUser](page, size, users, totalCount)
	return pag, nil
}

func (repo *MongoRepository) FindByIsTracking(ctx context.Context, isTracking bool) ([]*twitterTypes.FollowingUser, error) {
	filter := bson.M{"tracking": isTracking}
	cursor, err := repo.conn.Datastore.Collection(repo.absrepo.Collection).Find(ctx, filter, nil)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error finding users by isTracking.")
		return nil, err
	}

	var users []*twitterTypes.FollowingUser
	for cursor.Next(ctx) {
		var user *twitterTypes.FollowingUser
		err = cursor.Decode(&user)
		if err != nil {
			logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error decoding user.")
			return nil, err
		}
		users = append(users, user)
	}
	return users, nil
}

func (repo *MongoRepository) FindByIsTrackingPagination(ctx context.Context, page, size int64, isTracking bool) (*persistence.Pagination[[]*twitterTypes.FollowingUser], error) {
	opts := options.Find()
	if page == 1 {
		opts.SetSkip(0)
		opts.SetLimit(size)
	} else {
		opts.SetSkip((page - 1) * size)
		opts.SetLimit(size)
	}

	filter := bson.M{"tracking": isTracking}
	var users []*twitterTypes.FollowingUser
	cursor, err := repo.conn.Datastore.Collection(repo.absrepo.Collection).Find(ctx, filter, opts)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error finding users by isTracking.")
		return nil, err
	}

	for cursor.Next(ctx) {
		var user *twitterTypes.FollowingUser
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

	pag := persistence.NewPagination[[]*twitterTypes.FollowingUser](page, size, users, totalCount)
	return pag, nil
}
