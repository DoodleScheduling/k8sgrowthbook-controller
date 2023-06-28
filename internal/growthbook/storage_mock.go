package growthbook

import (
	"context"
	"errors"

	"github.com/DoodleScheduling/k8sgrowthbook-controller/internal/storage"
)

var (
	_ storage.Database   = &MockDatabase{}
	_ storage.Collection = &MockCollection{}
)

type MockDatabase struct {
	FindOne    func(ctx context.Context, filter interface{}, dst interface{}) error
	DeleteOne  func(ctx context.Context, filter interface{}) error
	InsertOne  func(ctx context.Context, doc interface{}) error
	UpdateOne  func(ctx context.Context, filter interface{}, doc interface{}) error
	DeleteMany func(ctx context.Context, filter interface{}) error
}

func (d *MockDatabase) Collection(collName string) storage.Collection {
	return &MockCollection{
		db: d,
	}
}

type MockCollection struct {
	db *MockDatabase
}

func (c *MockCollection) FindOne(ctx context.Context, filter interface{}, dst interface{}) error {
	if c.db.FindOne == nil {
		return errors.New("no mock func for findOne provided")
	}

	return c.db.FindOne(ctx, filter, dst)
}

func (c *MockCollection) DeleteOne(ctx context.Context, filter interface{}) error {
	if c.db.DeleteOne == nil {
		return errors.New("no mock func for deleteOne provided")
	}

	return c.db.DeleteOne(ctx, filter)
}

func (c *MockCollection) InsertOne(ctx context.Context, doc interface{}) error {
	if c.db.InsertOne == nil {
		return errors.New("no mock func for insertOne provided")
	}

	return c.db.InsertOne(ctx, doc)
}

func (c *MockCollection) UpdateOne(ctx context.Context, filter interface{}, doc interface{}) error {
	if c.db.UpdateOne == nil {
		return errors.New("no mock func for updateOne provided")
	}

	return c.db.UpdateOne(ctx, filter, doc)
}

func (c *MockCollection) DeleteMany(ctx context.Context, filter interface{}) error {
	if c.db.DeleteMany == nil {
		return errors.New("no mock func for deleteMany provided")
	}

	return c.db.DeleteMany(ctx, filter)
}
