package growthbook

import (
	"context"
	"time"

	"github.com/DoodleScheduling/k8sgrowthbook-controller/api/v1beta1"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type SDKConnection struct {
	ID                       string    `bson:"id"`
	Key                      string    `bson:"key"`
	Languages                []string  `bson:"languages"`
	Name                     string    `bson:"name"`
	Environment              string    `bson:"environment"`
	EncryptPayload           bool      `bson:"encryptPayload"`
	Organization             string    `bson:"organization"`
	Project                  string    `bson:"project"`
	IncludeVisualExperiments bool      `bson:"includeVisualExperiments"`
	IncludeDraftExperiments  bool      `bson:"includeDraftExperiments"`
	IncludeExperimentNames   bool      `bson:"includeExperimentNames"`
	DateCreated              time.Time `bson:"dateCreated"`
	DateUpdated              time.Time `bson:"dateUpdated"`
}

func (s *SDKConnection) FromV1beta1(client v1beta1.GrowthbookClient) *SDKConnection {
	s.ID = client.GetID()
	s.Name = client.GetName()
	s.Languages = client.Spec.Languages
	s.Environment = client.Spec.Environment
	s.EncryptPayload = client.Spec.EncryptPayload
	s.Project = client.Spec.Project
	s.IncludeVisualExperiments = client.Spec.IncludeVisualExperiments
	s.IncludeDraftExperiments = client.Spec.IncludeDraftExperiments
	s.IncludeExperimentNames = client.Spec.IncludeExperimentNames
	return s
}

func UpdateSDKConnection(ctx context.Context, sdkconnection SDKConnection, db *mongo.Database) error {
	col := db.Collection("sdkconnections")
	filter := bson.M{
		"id": sdkconnection.ID,
	}

	var existing SDKConnection
	err := col.FindOne(ctx, filter).Decode(&existing)

	if err != nil {
		sdkconnection.DateCreated = time.Now()
		_, err := col.InsertOne(ctx, sdkconnection)
		return err
	}

	sdkconnection.DateUpdated = time.Now()
	update := bson.D{{Key: "$set", Value: sdkconnection}}
	_, err = col.UpdateOne(ctx, filter, update)
	return err
}
