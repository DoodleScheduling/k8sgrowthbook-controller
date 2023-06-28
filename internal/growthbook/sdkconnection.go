package growthbook

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"strings"
	"time"

	"github.com/DoodleScheduling/k8sgrowthbook-controller/api/v1beta1"
	"github.com/DoodleScheduling/k8sgrowthbook-controller/internal/storage"
	"go.mongodb.org/mongo-driver/bson"
)

type SDKConnection struct {
	ID                       string             `bson:"id"`
	Key                      string             `bson:"key"`
	Languages                []string           `bson:"languages"`
	Name                     string             `bson:"name"`
	Environment              string             `bson:"environment"`
	EncryptPayload           bool               `bson:"encryptPayload"`
	EncryptionKey            string             `bson:"encryptionKey"`
	Organization             string             `bson:"organization"`
	Project                  string             `bson:"project"`
	IncludeVisualExperiments bool               `bson:"includeVisualExperiments"`
	IncludeDraftExperiments  bool               `bson:"includeDraftExperiments"`
	IncludeExperimentNames   bool               `bson:"includeExperimentNames"`
	DateCreated              time.Time          `bson:"dateCreated"`
	DateUpdated              time.Time          `bson:"dateUpdated"`
	Proxy                    SDKConnectionProxy `bson:"proxy"`
	Revision                 int                `bson:"__v"`
}

type SDKConnectionProxy struct {
	SigningKey string `bson:"signingKey"`
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

	if s.Languages == nil {
		s.Languages = []string{}
	}

	return s
}

func DeleteSDKConnection(ctx context.Context, sdkconnection SDKConnection, db storage.Database) error {
	col := db.Collection("sdkconnections")
	filter := bson.M{
		"id": sdkconnection.ID,
	}

	return col.DeleteOne(ctx, filter)
}

func UpdateSDKConnection(ctx context.Context, sdkconnection SDKConnection, db storage.Database) error {
	col := db.Collection("sdkconnections")
	filter := bson.M{
		"id": sdkconnection.ID,
	}

	clearPayloadCache := func() error {
		//growthbook caches the response payload, clear it
		filter = bson.M{
			"environment":  sdkconnection.Environment,
			"organization": sdkconnection.Organization,
		}

		return db.Collection("sdkpayloads").DeleteMany(ctx, filter)
	}

	var existing SDKConnection
	err := col.FindOne(ctx, filter, &existing)

	if err != nil {
		sdkconnection.DateCreated = time.Now()
		sdkconnection.DateUpdated = sdkconnection.DateCreated

		encryptionKey, err := generateKey("", 32)
		if err != nil {
			return err
		}

		signingKey, err := generateKey("", 32)
		if err != nil {
			return err
		}

		sdkconnection.EncryptionKey = encryptionKey
		sdkconnection.Proxy.SigningKey = signingKey

		err = col.InsertOne(ctx, sdkconnection)
		if err != nil {
			return err
		}

		return clearPayloadCache()
	}

	existingBson, err := bson.Marshal(existing)
	if err != nil {
		return err
	}

	existing.ID = sdkconnection.ID
	existing.Key = sdkconnection.Key
	existing.Languages = sdkconnection.Languages
	existing.Name = sdkconnection.Name
	existing.Environment = sdkconnection.Environment
	existing.EncryptPayload = sdkconnection.EncryptPayload
	existing.Organization = sdkconnection.Organization
	existing.Project = sdkconnection.Project
	existing.IncludeVisualExperiments = sdkconnection.IncludeVisualExperiments
	existing.IncludeDraftExperiments = sdkconnection.IncludeDraftExperiments
	existing.IncludeDraftExperiments = sdkconnection.IncludeDraftExperiments

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

	err = col.UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}

	return clearPayloadCache()
}

func generateKey(prefix string, n int) (string, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}

	e := base64.StdEncoding.EncodeToString(b)
	return prefix + strings.Replace(strings.Replace(strings.Replace(e, "=", "", -1), "/", "", -1), "+", "", -1), nil
}
