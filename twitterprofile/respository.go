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
	UpdateOrInsert(ctx context.Context, usr *twitterTypes.User) error
	FindByUserId(ctx context.Context, twitterId string) (*twitterTypes.User, error)
	FindUserByTwitterHandle(ctx context.Context, twitterHandle string) (*twitterTypes.User, error)
	FindUserByName(ctx context.Context, usrName string) (*twitterTypes.User, error)
	FindAllUsersPagination(ctx context.Context, page, size int64, asc bool, sortedType string) (*persistence.Pagination[[]*twitterTypes.User], error)
	FindAllUsersPaginationByNameOrTwitterHandle(ctx context.Context, page, size int64, name string) (*persistence.Pagination[[]*twitterTypes.User], error)
	FindTotalCount(ctx context.Context) (int64, error)
	FindAllUsers(ctx context.Context) ([]*twitterTypes.User, error)
	FindUsersByIds(ctx context.Context, userIds ...string) ([]*twitterTypes.User, error)
}
type MongoRepository struct {
	conn    *mongodb.MongoConnection
	absrepo *persistence.AbstractMongoRepository[*twitterTypes.User]
}

func NewRepository(conn *mongodb.MongoConnection) Repository {
	absrepo := persistence.NewAbstractRepository[*twitterTypes.User](conn, Collection)
	return &MongoRepository{
		conn:    conn,
		absrepo: absrepo,
	}
}

func (repo *MongoRepository) UpdateOrInsert(ctx context.Context, usr *twitterTypes.User) error {
	err := repo.absrepo.InsertOrUpdate(usr)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error updating or inserting user.")
		return err
	}
	return nil
}

func (repo *MongoRepository) FindByUserId(ctx context.Context, twitterId string) (*twitterTypes.User, error) {
	filter := bson.M{"_id": twitterId}
	var usr twitterTypes.User
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

func (repo *MongoRepository) FindUserByTwitterHandle(ctx context.Context, twitterHandle string) (*twitterTypes.User, error) {
	filter := bson.M{"handle": twitterHandle}
	var usr twitterTypes.User
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

func (repo *MongoRepository) FindUserByName(ctx context.Context, usrName string) (*twitterTypes.User, error) {
	filter := bson.M{"handle": usrName}
	var usr twitterTypes.User
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

func (repo *MongoRepository) FindAllUsersPagination(ctx context.Context, page, size int64, asc bool, sortedType string) (*persistence.Pagination[[]*twitterTypes.User], error) {
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
	var users []*twitterTypes.User
	cursor, err := repo.conn.Datastore.Collection(repo.absrepo.Collection).Find(ctx, bson.M{}, opts)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error finding all users.")
		return nil, err
	}

	for cursor.Next(ctx) {
		var user *twitterTypes.User
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

	pag := persistence.NewPagination[[]*twitterTypes.User](page, size, users, totalCount)
	return pag, nil
}

func (repo *MongoRepository) FindAllUsersPaginationByNameOrTwitterHandle(ctx context.Context, page, size int64, name string) (*persistence.Pagination[[]*twitterTypes.User], error) {
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

	var users []*twitterTypes.User
	cursor, err := repo.conn.Datastore.Collection(repo.absrepo.Collection).Find(ctx, filters, opts)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error finding all users.")
		return nil, err
	}

	for cursor.Next(ctx) {
		var user *twitterTypes.User
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

	pag := persistence.NewPagination[[]*twitterTypes.User](page, size, users, totalCount)
	return pag, nil
}

func (repo *MongoRepository) FindAllUsers(ctx context.Context) ([]*twitterTypes.User, error) {
	filter := bson.M{}
	var users []*twitterTypes.User
	cursor, err := repo.conn.Datastore.Collection(repo.absrepo.Collection).Find(ctx, filter, nil)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error finding all users.")
		return nil, err
	}

	for cursor.Next(ctx) {
		var user *twitterTypes.User
		err = cursor.Decode(&user)
		if err != nil {
			logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error decoding usr.")
			return nil, err
		}
		users = append(users, user)
	}
	return users, nil
}

func (repo *MongoRepository) FindUsersByIds(ctx context.Context, userIds ...string) ([]*twitterTypes.User, error) {
	filter := bson.M{"_id": bson.M{"$in": userIds}}
	cursor, err := repo.conn.Datastore.Collection(repo.absrepo.Collection).Find(ctx, filter, nil)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error finding users by ID.")
		return nil, err
	}

	userMap := make(map[string]*twitterTypes.User)
	for cursor.Next(ctx) {
		var user *twitterTypes.User
		err = cursor.Decode(&user)
		if err != nil {
			logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error decoding user.")
			return nil, err
		}
		userMap[user.ID] = user
	}

	var orderedUsers []*twitterTypes.User
	for _, id := range userIds {
		if user, exists := userMap[id]; exists {
			orderedUsers = append(orderedUsers, user)
		}
	}
	return orderedUsers, nil
}
