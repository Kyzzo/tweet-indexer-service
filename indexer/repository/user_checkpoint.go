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
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const collection = "users.checkpoint"

type mongoUserCheckpointRepo struct {
	conn    *mongodb.MongoConnection
	absrepo *persistence.AbstractMongoRepository[*model.UserCheckPoint]
}

type UserCheckPointRepository interface {
	UpdateOrSave(ctx context.Context, checkpoint model.UserCheckPoint) (interface{}, error)
	FindById(ctx context.Context, id string) (*model.UserCheckPoint, error)
	BulkUpsert(ctx context.Context, checkpoints []model.UserCheckPoint) error
}

func NewUserCheckPointRepository(conn *mongodb.MongoConnection) UserCheckPointRepository {
	absrepo := persistence.NewAbstractRepository[*model.UserCheckPoint](conn, collection)
	return &mongoUserCheckpointRepo{
		conn:    conn,
		absrepo: absrepo,
	}
}

func (repo *mongoUserCheckpointRepo) FindById(ctx context.Context, id string) (*model.UserCheckPoint, error) {

	filter := bson.M{"_id": id}

	var checkpoint model.UserCheckPoint
	singleResult := repo.conn.Datastore.Collection(collection).FindOne(ctx, filter)

	if err := singleResult.Decode(&checkpoint); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		logg.Log.Error().Err(err).Msg("Error decoding checkpoint result.")
		return nil, nil
	}

	return &checkpoint, nil
}

func (repo *mongoUserCheckpointRepo) UpdateOrSave(ctx context.Context, checkpoint model.UserCheckPoint) (interface{}, error) {
	opts := options.Update().SetUpsert(true)

	filter := bson.D{{"_id", checkpoint.ID}}
	update := repo.getCheckPointUpdate(checkpoint)

	entry, err := repo.conn.Datastore.Collection(collection).UpdateOne(ctx, filter, update, opts)
	if err != nil {
		logg.Log.Error().Err(err).Msg("Error saving/updating checkpoint.")
		return nil, err
	}

	return entry, nil
}

func (*mongoUserCheckpointRepo) getCheckPointUpdate(checkpoint model.UserCheckPoint) primitive.D {
	return bson.D{{"$set", bson.D{
		{"last_tweet", checkpoint.LastTweet},
		{"last_tweet_created_at", checkpoint.LastTweetCreatedAt},
		{"updated_at", checkpoint.UpdatedAt},
	}}}
}

// / BulkUpsert takes a slice of UserCheckPoint and upserts them all in one BulkWrite
func (repo *mongoUserCheckpointRepo) BulkUpsert(
	ctx context.Context,
	checkpoints []model.UserCheckPoint,
) error {
	if len(checkpoints) == 0 {
		return nil
	}

	// Build a slice of WriteModels (one UpdateOneModel per checkpoint)
	ops := make([]mongo.WriteModel, 0, len(checkpoints))
	now := time.Now()

	for _, cp := range checkpoints {
		filter := bson.M{"_id": cp.ID}
		update := bson.M{"$set": bson.M{
			"last_tweet":            cp.LastTweet,
			"last_tweet_created_at": cp.LastTweetCreatedAt,
			"updated_at":            now,
		}}
		upsertOp := mongo.NewUpdateOneModel().
			SetFilter(filter).
			SetUpdate(update).
			SetUpsert(true)
		ops = append(ops, upsertOp)
	}

	// Grab the raw collection
	coll := repo.conn.Datastore.Collection(collection)

	// Execute BulkWrite (unordered for maximum throughput)
	_, err := coll.BulkWrite(ctx, ops, options.BulkWrite().SetOrdered(false))
	if err != nil {
		logg.Log.Error().Err(err).Msg("bulk upsert checkpoints failed")
		return fmt.Errorf("bulk upsert checkpoints: %w", err)
	}
	return nil
}
