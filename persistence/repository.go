package persistence

import (
	"context"
	"reflect"

	"github.com/jeetdotso/tweet-indexer-service/persistence/mongodb"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type AbstractRepository interface {
	GetID() any
}

type AbstractMongoRepository[T AbstractRepository] struct {
	conn       *mongodb.MongoConnection
	Collection string
}

func NewAbstractRepository[T AbstractRepository](conn *mongodb.MongoConnection, collection string) *AbstractMongoRepository[T] {
	return &AbstractMongoRepository[T]{
		conn:       conn,
		Collection: collection,
	}
}

func (repo *AbstractMongoRepository[T]) InsertOrUpdate(model T) error {
	opts := options.Update().SetUpsert(true)
	filter := bson.D{{"_id", model.GetID()}}
	update := repo.getUpdate(model)
	_, err := repo.conn.Datastore.Collection(repo.Collection).UpdateOne(context.Background(), filter, update, opts)
	if err != nil {
		return err
	}
	return nil
}

func (repo *AbstractMongoRepository[T]) UpdateOrInsertWithCustomCollection(model T, collectionName string) error {
	opts := options.Update().SetUpsert(true)
	filter := bson.D{{"_id", model.GetID()}}
	update := repo.getUpdate(model)
	_, err := repo.conn.Datastore.Collection(collectionName).UpdateOne(context.Background(), filter, update, opts)
	if err != nil {
		return err
	}
	return nil
}

func (repo *AbstractMongoRepository[T]) RenameCollection(collection string, target string, dropTarget bool) error {
	var res = repo.conn.Datastore.RunCommand(context.Background(), bson.D{
		{"renameCollection", collection},
		{"to", target},
		{"dropTarget", dropTarget},
	})

	if res.Err() != nil {
		return res.Err()
	}

	return nil
}

func (*AbstractMongoRepository[T]) getUpdate(model T) bson.D {
	v := reflect.ValueOf(model)
	v = reflect.Indirect(v)
	var elem bson.D
	for i := 0; i < v.NumField(); i++ {
		tag := v.Type().Field(i).Tag.Get("bson")
		if tag == "_id" {
			continue
		}
		elem = append(elem, bson.E{Key: tag, Value: v.Field(i).Interface()})
	}
	return bson.D{
		{"$set", elem},
	}
}

func (repo *AbstractMongoRepository[T]) Count() (int64, error) {
	return repo.conn.Datastore.Collection(repo.Collection).CountDocuments(context.Background(), bson.D{})
}
