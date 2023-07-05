package storage

import "context"

type Disconnector interface {
	Disconnect(ctx context.Context) error
}

type Database interface {
	Collection(collName string) Collection
}

type Collection interface {
	FindOne(ctx context.Context, filter interface{}, dst interface{}) error
	DeleteOne(ctx context.Context, filter interface{}) error
	InsertOne(ctx context.Context, doc interface{}) error
	UpdateOne(ctx context.Context, filter interface{}, doc interface{}) error
	DeleteMany(ctx context.Context, filter interface{}) error
}
