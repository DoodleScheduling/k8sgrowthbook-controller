package growthbook

import (
	"bytes"
	"context"
	"time"

	"github.com/DoodleScheduling/k8sgrowthbook-controller/api/v1beta1"
	"github.com/DoodleScheduling/k8sgrowthbook-controller/internal/storage"
	"go.mongodb.org/mongo-driver/bson"
)

type Organization struct {
	ID          string               `bson:"id"`
	OwnerEmail  string               `bson:"ownerEmail"`
	Name        string               `bson:"name"`
	DateCreated time.Time            `bson:"dateCreated"`
	Members     []OrganizationMember `bson:"members"`
	Revision    int                  `bson:"__v"`
}

type OrganizationMember struct {
	ID   string `bson:"id"`
	Role string `bson:"role"`
}

func (o *Organization) FromV1beta1(org v1beta1.GrowthbookOrganization) *Organization {
	o.Name = org.GetName()
	o.ID = org.GetID()
	o.OwnerEmail = org.Spec.OwnerEmail
	return o
}

func UpdateOrganization(ctx context.Context, org Organization, db storage.Database) error {
	col := db.Collection("organizations")
	filter := bson.M{
		"id": org.ID,
	}

	var existing Organization
	err := col.FindOne(ctx, filter, &existing)

	if err != nil {
		org.DateCreated = time.Now()
		return col.InsertOne(ctx, org)
	}

	existingBson, err := bson.Marshal(existing)
	if err != nil {
		return err
	}

	existing.ID = org.ID
	existing.OwnerEmail = org.OwnerEmail
	existing.Name = org.Name
	existing.ID = org.ID
	existing.Members = org.Members

	updateBson, err := bson.Marshal(existing)
	if err != nil {
		return err
	}

	if bytes.Equal(existingBson, updateBson) {
		return nil
	}

	update := bson.D{
		{Key: "$set", Value: bson.Raw(updateBson)},
	}

	return col.UpdateOne(ctx, filter, update)
}
