package list

import (
	"context"

	"github.com/jeetdotso/tweet-indexer-service/logg"
	"github.com/jeetdotso/tweet-indexer-service/persistence"
	"github.com/jeetdotso/tweet-indexer-service/persistence/mongodb"
	twitterTypes "github.com/jeetdotso/tweet-indexer-service/scrape/internals/api/types"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const listCollection = "list"

type ListRepository interface {
	SaveOrUpdate(ctx context.Context, blockIndex twitterTypes.FollowingUser)
	FindAllByUserId(ctx context.Context, userId string) ([]twitterTypes.FollowingUser, error)
	FindAllByID(ctx context.Context, id string) (*twitterTypes.FollowingUser, error)
	InList(ctx context.Context, username string) (bool, error)
	FindAll(ctx context.Context) ([]twitterTypes.FollowingUser, error)
}
type mongoListRepository struct {
	conn    *mongodb.MongoConnection
	absrepo *persistence.AbstractMongoRepository[*twitterTypes.FollowingUser]
}

// NewRatingRepository creates a new instance of RatingRepository.
func NewListRepo(conn *mongodb.MongoConnection) ListRepository {
	absrepo := persistence.NewAbstractRepository[*twitterTypes.FollowingUser](conn, listCollection)
	return &mongoListRepository{
		conn:    conn,
		absrepo: absrepo,
	}
}

func (repo *mongoListRepository) InList(ctx context.Context, username string) (bool, error) {
	filter := bson.M{"handle": username}
	var index twitterTypes.FollowingUser
	res := repo.conn.Datastore.Collection(listCollection).FindOne(ctx, filter)
	if err := res.Decode(&index); err != nil {
		if err == mongo.ErrNoDocuments {
			return false, nil
		}
		logg.Log.Error().Err(err).Msg("Error decoding index result.")
		return false, err
	}
	if index.ID == "" {
		return false, nil
	}
	return true, nil
}
func (repo *mongoListRepository) FindAllByID(ctx context.Context, id string) (*twitterTypes.FollowingUser, error) {
	filter := bson.M{"_id": id}
	var index twitterTypes.FollowingUser
	res := repo.conn.Datastore.Collection(listCollection).FindOne(ctx, filter)
	if err := res.Decode(&index); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		logg.Log.Error().Err(err).Msg("Error decoding index result.")
		return nil, err
	}
	return &index, nil
}

func (repo *mongoListRepository) SaveOrUpdate(ctx context.Context, tweetIndex twitterTypes.FollowingUser) {
	collection := repo.conn.Datastore.Collection(listCollection)

	_, err := collection.InsertOne(ctx, tweetIndex)
	if err != nil {
		logg.Log.Error().Err(err).Msg("Error updating/inserting block index.")
		return
	}

}

func (repo *mongoListRepository) FindAllByUserId(ctx context.Context, userId string) ([]twitterTypes.FollowingUser, error) {
	filter := bson.M{"_id": userId}
	findOpt := options.Find()
	findOpt.SetSort(bson.D{{"created_at", 1}})

	var tweetIdxs []twitterTypes.FollowingUser

	cursor, err := repo.conn.Datastore.Collection(listCollection).Find(ctx, filter, findOpt)
	if err != nil {
		logg.Log.Error().Err(err).Msg("Error finding all block indexes by stv id.")
		return nil, err
	}

	for cursor.Next(ctx) {
		var tweetIdx twitterTypes.FollowingUser
		err = cursor.Decode(&tweetIdx)
		if err != nil {
			logg.Log.Error().Err(err).Msg("Error decoding block index.")
			return nil, err
		}
		tweetIdxs = append(tweetIdxs, tweetIdx)
	}

	return tweetIdxs, nil
}

func (repo *mongoListRepository) FindAll(ctx context.Context) ([]twitterTypes.FollowingUser, error) {
	findOpt := options.Find()

	var blockIdxs []twitterTypes.FollowingUser

	cursor, err := repo.conn.Datastore.Collection(listCollection).Find(ctx, findOpt)
	if err != nil {
		logg.Log.Error().Err(err).Msg("Error finding all block indexes.")
		return nil, err
	}

	for cursor.Next(ctx) {
		var blockIdx twitterTypes.FollowingUser
		err = cursor.Decode(&blockIdx)
		if err != nil {
			logg.Log.Error().Err(err).Msg("Error decoding block index.")
			return nil, err
		}
		blockIdxs = append(blockIdxs, blockIdx)
	}

	return blockIdxs, nil
}
