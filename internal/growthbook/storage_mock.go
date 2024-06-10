package growthbook

import (
	"context"
	"errors"

	"github.com/DoodleScheduling/growthbook-controller/internal/storage"
)

var (
	_ storage.Database   = &MockDatabase{}
	_ storage.Collection = &MockCollection{}
)

type MockDisconnect struct {
}

func (d *MockDisconnect) Disconnect(ctx context.Context) error {
	return nil
}

type MockDatabase struct {
	FindOne    func(ctx context.Context, filter interface{}) (storage.Decoder, error)
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

type MockResult struct {
	decode func(v interface{}) error
}

func (r *MockResult) Decode(dst interface{}) error {
	if r.decode != nil {
		return r.decode(dst)
	}

	return nil
}

type MockCollection struct {
	db *MockDatabase
}

func (c *MockCollection) FindOne(ctx context.Context, filter interface{}) (storage.Decoder, error) {
	if c.db.FindOne == nil {
		return nil, errors.New("no mock func for findOne provided")
	}

	return c.db.FindOne(ctx, filter)
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
