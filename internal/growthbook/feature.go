package growthbook

import (
	"bytes"
	"context"
	"time"

	"github.com/DoodleScheduling/growthbook-controller/api/v1beta1"
	"github.com/DoodleScheduling/growthbook-controller/internal/storage"
	"go.mongodb.org/mongo-driver/bson"
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
	Revision            int                           `bson:"__v"`
}

type EnvironmentSetting struct {
	Enabled bool        `bson:"enabled"`
	Rules   interface{} `bson:"rules"`
}

func (f *Feature) FromV1beta1(feature v1beta1.GrowthbookFeature) *Feature {
	f.ID = feature.GetID()
	f.Description = feature.Spec.Description
	f.Tags = feature.Spec.Tags
	f.DefaultValue = feature.Spec.DefaultValue
	f.ValueType = FeatureValueType(feature.Spec.ValueType)

	if f.Environments == nil {
		f.Environments = []string{}
	}

	if f.Tags == nil {
		f.Tags = []string{}
	}

	f.EnvironmentSettings = make(map[string]EnvironmentSetting)
	for _, env := range feature.Spec.Environments {
		f.EnvironmentSettings[env.Name] = EnvironmentSetting{
			Enabled: env.Enabled,
		}
	}

	return f
}

func DeleteFeature(ctx context.Context, feature Feature, db storage.Database) error {
	col := db.Collection("features")
	filter := bson.M{
		"id": feature.ID,
	}

	return col.DeleteOne(ctx, filter)
}

func UpdateFeature(ctx context.Context, feature Feature, db storage.Database) error {
	col := db.Collection("features")
	filter := bson.M{
		"id": feature.ID,
	}

	var existing Feature
	err := col.FindOne(ctx, filter, &existing)

	if err != nil {
		feature.DateCreated = time.Now()
		feature.DateUpdated = feature.DateCreated
		return col.InsertOne(ctx, feature)
	}

	existingBson, err := bson.Marshal(existing)
	if err != nil {
		return err
	}

	existing.ID = feature.ID
	existing.Description = feature.Description
	existing.DefaultValue = feature.DefaultValue
	existing.ValueType = feature.ValueType
	existing.Tags = feature.Tags
	existing.Environments = feature.Environments

	if existing.EnvironmentSettings == nil {
		existing.EnvironmentSettings = make(map[string]EnvironmentSetting)
	}

	for env, settings := range feature.EnvironmentSettings {
		if val, ok := existing.EnvironmentSettings[env]; ok {
			s := existing.EnvironmentSettings[env]
			s.Enabled = val.Enabled
			existing.EnvironmentSettings[env] = s
		} else {
			existing.EnvironmentSettings[env] = EnvironmentSetting{
				Enabled: settings.Enabled,
			}
		}
	}

	for env := range existing.EnvironmentSettings {
		if _, ok := feature.EnvironmentSettings[env]; !ok {
			delete(existing.EnvironmentSettings, env)
		}
	}

	updateBson, err := bson.Marshal(existing)
	if err != nil {
		return err
	}

	if bytes.Equal(existingBson, updateBson) {
		return nil
	}

	existing.DateUpdated = time.Now()
	updateBson, err = bson.Marshal(existing)
	if err != nil {
		return err
	}

	update := bson.D{
		{Key: "$set", Value: bson.Raw(updateBson)},
	}

	return col.UpdateOne(ctx, filter, update)
}
