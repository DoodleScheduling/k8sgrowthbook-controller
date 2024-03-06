package growthbook

import (
	"bytes"
	"context"
	"strconv"
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
	Enabled bool          `bson:"enabled"`
	Rules   []FeatureRule `bson:"rules"`
}

type SavedGroupTargetingMatch string

var (
	SavedGroupTargetingMatchAll  SavedGroupTargetingMatch = "all"
	SavedGroupTargetingMatchNone SavedGroupTargetingMatch = "none"
	SavedGroupTargetingMatchAny  SavedGroupTargetingMatch = "any"
)

type FeatureRuleType string

var (
	FeatureRuleTypeForce      FeatureRuleType = "force"
	FeatureRuleTypeRollout    FeatureRuleType = "rollout"
	FeatureRuleTypeExperiment FeatureRuleType = "experiment"
)

type FeatureRule struct {
	ID                     string                `bson:"id,omitempty"`
	Type                   FeatureRuleType       `bson:"type,omitempty"`
	Description            string                `bson:"description,omitempty"`
	Condition              string                `bson:"condition,omitempty"`
	Enabled                bool                  `bson:"enabled,omitempty"`
	ScheduleRules          []ScheduleRule        `bson:"scheduleRules,omitempty"`
	SavedGroups            []SavedGroupTargeting `bson:"savedGroups,omitempty"`
	Prerequisites          []FeaturePrerequisite `bson:"prerequisites,omitempty"`
	Value                  string                `bson:"value,omitempty"`
	Coverage               float64               `bson:"coverage,omitempty"`
	HashAttribute          string                `bson:"hashAttribute,omitempty"`
	TrackingKey            string                `bson:"trackingKey,omitempty"`
	FallbackAttribute      *string               `bson:"fallbackAttribute,omitempty"`
	DisableStickyBucketing *bool                 `bson:"disableStickyBucketing,omitempty"`
	BucketVersion          *int64                `bson:"bucketVersion,omitempty"`
	MinBucketVersion       *int64                `bson:"minBucketVersion,omitempty"`
	Namespace              *NamespaceValue       `bson:"namespace,omitempty"`
	Values                 []ExperimentValue     `bson:"values,omitempty"`
}

type FeaturePrerequisite struct {
	ID        string `bson:"id,omitempty"`
	Condition string `bson:"condition,omitempty"`
}

type ScheduleRule struct {
	Timestamp string `bson:"timestamp,omitempty"`
	Enabled   bool   `bson:"enabled,omitempty"`
}

type SavedGroupTargeting struct {
	Match SavedGroupTargetingMatch `bson:"match,omitempty"`
	IDs   []string                 `bson:"ids,omitempty"`
}

type ExperimentValue struct {
	Value  string  `bson:"value,omitempty"`
	Weight int64   `bson:"weight,omitempty"`
	Name   *string `bson:"name,omitempty"`
}

type NamespaceValue struct {
	Enabled bool    `bson:"enabled,omitempty"`
	Name    string  `bson:"name,omitempty"`
	Range   []int64 `bson:"range,omitempty"`
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
		var rules []FeatureRule
		for _, rule := range env.Rules {
			var scheduleRules []ScheduleRule
			for _, scheduleRule := range rule.ScheduleRules {
				scheduleRules = append(scheduleRules, ScheduleRule{
					Timestamp: scheduleRule.Timestamp,
					Enabled:   scheduleRule.Enabled,
				})
			}

			var savedGroups []SavedGroupTargeting
			for _, savedGroup := range rule.SavedGroups {
				savedGroups = append(savedGroups, SavedGroupTargeting{
					Match: SavedGroupTargetingMatch(savedGroup.Match),
					IDs:   savedGroup.IDs,
				})
			}

			var featurePrerequisites []FeaturePrerequisite
			for _, featurePrerequisite := range rule.Prerequisites {
				featurePrerequisites = append(featurePrerequisites, FeaturePrerequisite{
					ID:        featurePrerequisite.ID,
					Condition: featurePrerequisite.Condition,
				})
			}

			var experimentValues []ExperimentValue
			for _, experimentValue := range rule.Values {
				experimentValues = append(experimentValues, ExperimentValue{
					Value:  experimentValue.Value,
					Weight: experimentValue.Weight,
					Name:   experimentValue.Name,
				})
			}

			coverage, _ := strconv.ParseFloat(rule.Coverage, 64)
			storeRule := FeatureRule{
				Type:                   FeatureRuleType(rule.Type),
				Description:            rule.Description,
				Condition:              rule.Condition,
				Enabled:                rule.Enabled,
				ScheduleRules:          scheduleRules,
				SavedGroups:            savedGroups,
				Prerequisites:          featurePrerequisites,
				Value:                  rule.Value,
				Coverage:               coverage,
				HashAttribute:          rule.HashAttribute,
				TrackingKey:            rule.TrackingKey,
				FallbackAttribute:      rule.FallbackAttribute,
				DisableStickyBucketing: rule.DisableStickyBucketing,
				BucketVersion:          rule.BucketVersion,
				MinBucketVersion:       rule.MinBucketVersion,
				Values:                 experimentValues,
			}

			if rule.Namespace != nil {
				storeRule.Namespace = &NamespaceValue{
					Enabled: rule.Namespace.Enabled,
					Name:    rule.Namespace.Name,
					Range:   rule.Namespace.Range,
				}
			}

			rules = append(rules, storeRule)
		}

		settings := EnvironmentSetting{
			Enabled: env.Enabled,
		}

		if env.Rules != nil {
			settings.Rules = rules
		}

		f.EnvironmentSettings[env.Name] = settings
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
		if existingSettings, ok := existing.EnvironmentSettings[env]; ok {
			s := existing.EnvironmentSettings[env]
			s.Enabled = settings.Enabled
			s.Rules = mergeRules(existingSettings.Rules, settings.Rules)
			existing.EnvironmentSettings[env] = s
		} else {
			existing.EnvironmentSettings[env] = EnvironmentSetting{
				Enabled: settings.Enabled,
				Rules:   settings.Rules,
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

func mergeRules(existing, spec []FeatureRule) []FeatureRule {
	var rules []FeatureRule
	for _, rule := range existing {
		if rule.ID != "" {
			rules = append(rules, rule)
		}
	}

	return append(rules, spec...)
}
