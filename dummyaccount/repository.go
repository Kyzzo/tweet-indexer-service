package dummyaccount

import (
	"context"
	"time"

	"github.com/jeetdotso/tweet-indexer-service/logg"
	"github.com/jeetdotso/tweet-indexer-service/persistence"
	"github.com/jeetdotso/tweet-indexer-service/persistence/mongodb"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const Collection = "dummy.accounts"

type Repository interface {
	UpdateOrInsert(ctx context.Context, pl *SelAccount) error
	FindById(ctx context.Context, id string) (*SelAccount, error)
	FindNext(ctx context.Context) (*SelAccount, error)
	FindInUse(ctx context.Context, group int) (*SelAccount, error)
	FindGroupNumber(ctx context.Context) (int, error)
	ResetAll(ctx context.Context) error
	FindAllByGroup(ctx context.Context, group int) ([]*SelAccount, error)
	UpdateAllToGroup(ctx context.Context, group int) error
}
type MongoRepository struct {
	conn    *mongodb.MongoConnection
	absrepo *persistence.AbstractMongoRepository[*SelAccount]
}

func NewRepository(conn *mongodb.MongoConnection) Repository {
	absrepo := persistence.NewAbstractRepository[*SelAccount](conn, Collection)
	return &MongoRepository{
		conn:    conn,
		absrepo: absrepo,
	}
}

func (repo *MongoRepository) UpdateOrInsert(ctx context.Context, pl *SelAccount) error {
	err := repo.absrepo.InsertOrUpdate(pl)
	if err != nil {
		return err
	}
	return nil
}

func (repo *MongoRepository) FindById(ctx context.Context, id string) (*SelAccount, error) {
	filter := bson.M{"username": id}
	var pool SelAccount
	res := repo.conn.Datastore.Collection(repo.absrepo.Collection).FindOne(ctx, filter)
	if err := res.Decode(&pool); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error decoding scraper account.")
		return nil, err
	}
	return &pool, nil
}

func (repo *MongoRepository) FindNext(ctx context.Context) (*SelAccount, error) {
	// find next (last updated account) in group 0
	filter := bson.M{
		"in_use": false,
		"group":  bson.M{"$in": bson.A{0}},
	}

	opts := options.FindOne().SetSort(bson.M{"updated_at": 1})

	var account SelAccount
	err := repo.conn.Datastore.Collection(repo.absrepo.Collection).FindOne(ctx, filter, opts).Decode(&account)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}

	return &account, nil
}

func (repo *MongoRepository) FindInUse(ctx context.Context, group int) (*SelAccount, error) {
	filter := bson.M{"in_use": true, "group": group}

	var account SelAccount
	err := repo.conn.Datastore.Collection(repo.absrepo.Collection).FindOne(ctx, filter).Decode(&account)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}

	return &account, nil
}

func (repo *MongoRepository) FindGroupNumber(ctx context.Context) (int, error) {
	filter := bson.M{}

	opts := options.FindOne().SetSort(bson.M{"group": -1})

	var selAccount SelAccount

	err := repo.conn.Datastore.Collection(repo.absrepo.Collection).FindOne(ctx, filter, opts).Decode(&selAccount)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return 0, nil
		}
		return 0, err
	}

	return selAccount.Group, nil
}

func (repo *MongoRepository) ResetAll(ctx context.Context) error {
	filter := bson.M{"in_use": true}

	update := bson.M{
		"$set": bson.M{
			"in_use":     false,
			"updated_at": time.Now(),
		},
	}

	opts := options.Update()

	_, err := repo.conn.Datastore.Collection(repo.absrepo.Collection).UpdateMany(ctx, filter, update, opts)
	if err != nil {
		return err
	}

	return nil
}

func (repo *MongoRepository) FindAllByGroup(ctx context.Context, group int) ([]*SelAccount, error) {
	filter := bson.M{"group": group}

	cursor, err := repo.conn.Datastore.Collection(repo.absrepo.Collection).Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var accounts []*SelAccount
	for cursor.Next(ctx) {
		var account SelAccount
		if err := cursor.Decode(&account); err != nil {
			return nil, err
		}
		accounts = append(accounts, &account)
	}

	return accounts, nil
}

func (repo *MongoRepository) UpdateAllToGroup(ctx context.Context, group int) error {
	filter := bson.M{}

	update := bson.M{
		"$set": bson.M{
			"group":      group,
			"updated_at": time.Now(),
		},
	}

	opts := options.Update()

	_, err := repo.conn.Datastore.Collection(repo.absrepo.Collection).UpdateMany(ctx, filter, update, opts)
	if err != nil {
		return err
	}

	return nil
}
