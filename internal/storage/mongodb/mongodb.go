package mongodb

import (
	"context"

	"github.com/DoodleScheduling/growthbook-controller/internal/storage"
	"go.mongodb.org/mongo-driver/mongo"
)

var (
	_ storage.Database   = &Database{}
	_ storage.Collection = &Collection{}
)

func New(db *mongo.Database) *Database {
	return &Database{
		db: db,
	}
}

type Database struct {
	db *mongo.Database
}

func (d *Database) Collection(collName string) storage.Collection {
	return &Collection{
		collection: d.db.Collection(collName),
	}
}

type Collection struct {
	collection *mongo.Collection
}

func (c *Collection) FindOne(ctx context.Context, filter interface{}, dst interface{}) error {
	return c.collection.FindOne(ctx, filter).Decode(dst)
}

func (c *Collection) DeleteOne(ctx context.Context, filter interface{}) error {
	_, err := c.collection.DeleteOne(ctx, filter)
	return err
}

func (c *Collection) InsertOne(ctx context.Context, doc interface{}) error {
	_, err := c.collection.InsertOne(ctx, doc)
	return err
}

func (c *Collection) UpdateOne(ctx context.Context, filter interface{}, doc interface{}) error {
	_, err := c.collection.UpdateOne(ctx, filter, doc)
	return err
}

func (c *Collection) DeleteMany(ctx context.Context, filter interface{}) error {
	_, err := c.collection.DeleteMany(ctx, filter)
	return err
}
