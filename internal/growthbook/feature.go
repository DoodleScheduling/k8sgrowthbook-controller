package growthbook

import (
	"context"
	"time"

	"github.com/DoodleScheduling/k8sgrowthbook-controller/api/v1beta1"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type FeatureValueType string

var (
	FeatureValueTypeBoolean FeatureValueType = "boolean"
	FeatureValueTypeString  FeatureValueType = "string"
	FeatureValueTypeNumber  FeatureValueType = "number"
	FeatureValueTypeJSON    FeatureValueType = "json"
)

type Feature struct {
	ID                  string                        `bson:"id"`
	Owner               string                        `bson:"owner"`
	Description         string                        `bson:"description"`
	Tags                []string                      `bson:"tags"`
	DefaultValue        string                        `bson:"defaultValue"`
	ValueType           FeatureValueType              `bson:"valueType"`
	Organization        string                        `bson:"organization"`
	Environments        []string                      `bson:"environment"`
	EnvironmentSettings map[string]EnvironmentSetting `bson:"environmentSettings"`
	DateCreated         time.Time                     `bson:"dateCreated"`
	DateUpdated         time.Time                     `bson:"dateUpdated"`
	Archived            bool                          `bson:"archived"`
}

type EnvironmentSetting struct {
	Enabled bool `bson:"enabled"`
}

func (f *Feature) FromV1beta1(feature v1beta1.GrowthbookFeature) *Feature {
	f.ID = feature.GetID()
	f.Description = feature.Spec.Description
	f.Tags = feature.Spec.Tags
	f.DefaultValue = feature.Spec.DefaultValue
	f.ValueType = FeatureValueType(feature.Spec.ValueType)

	f.Environments = nil
	for _, env := range feature.Spec.Environments {
		f.Environments = append(f.Environments, env.Name)
	}

	f.EnvironmentSettings = make(map[string]EnvironmentSetting)
	for _, env := range feature.Spec.Environments {
		f.EnvironmentSettings[env.Name] = EnvironmentSetting{
			Enabled: env.Enabled,
		}
	}

	return f
}

func UpdateFeature(ctx context.Context, feature Feature, db *mongo.Database) error {
	col := db.Collection("features")
	filter := bson.M{
		"id": feature.ID,
	}

	var existing Feature
	err := col.FindOne(ctx, filter).Decode(&existing)

	if err != nil {
		feature.DateCreated = time.Now()
		_, err := col.InsertOne(ctx, feature)
		return err
	}

	feature.DateUpdated = time.Now()
	update := bson.D{{Key: "$set", Value: feature}}
	_, err = col.UpdateOne(ctx, filter, update)
	return err
}
