package growthbook

import (
	"context"

	"github.com/DoodleScheduling/k8sgrowthbook-controller/api/v1beta1"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type Feature struct {
	ID           string   `bson:"id,omitempty"`
	Description  string   `bson:"description,omitempty"`
	Tags         []string `bson:"tags,omitempty"`
	DefaultValue string   `bson:"defaultValue,omitempty"`
	ValueType    string   `bson:"valueType,omitempty"`
	Organization string   `bson:"organization,omitempty"`
	Environments []string `bson:"environment,omitempty"`
}

func (f *Feature) FromV1beta1(feature v1beta1.GrowthbookFeature) {
	f.ID = feature.Spec.ID
	f.Description = feature.Spec.Description
	f.Tags = feature.Spec.Tags
	f.DefaultValue = feature.Spec.DefaultValue
	f.ValueType = feature.Spec.ValueType
	f.Organization = feature.Spec.Organization

	f.Environments = nil
	for _, env := range feature.Spec.Environments {
		f.Environments = append(f.Environments, env.Name)
	}
}

func UpdateFeature(ctx context.Context, feature Feature, db *mongo.Database) error {
	col := db.Collection("feature")
	filter := bson.M{
		"name": feature.ID,
	}

	count, err := col.CountDocuments(ctx, filter)

	if err != nil {
		return nil
	}

	if count == 0 {
		_, err := col.InsertOne(ctx, feature)
		return err
	}

	update := bson.D{{Key: "$set", Value: feature}}
	_, err = col.UpdateOne(ctx, filter, update)
	return err
}
