package growthbook

import (
	"context"
	"time"

	"github.com/DoodleScheduling/k8sgrowthbook-controller/api/v1beta1"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type Organization struct {
	ID          string               `bson:"id"`
	OwnerEmail  string               `bson:"ownerEmail"`
	Name        string               `bson:"name"`
	DateCreated time.Time            `bson:"dateCreated"`
	Members     []OrganizationMember `bson:"members"`
}

type OrganizationMember struct {
	ID   string `bson:"id"`
	Role string `bson:"role"`
}

func (o *Organization) FromV1beta1(org v1beta1.GrowthbookOrganization) *Organization {
	o.Name = org.GetName()
	o.ID = org.GetID()
	return o
}

func UpdateOrganization(ctx context.Context, org Organization, db *mongo.Database) error {
	col := db.Collection("organizations")
	filter := bson.M{
		"id": org.ID,
	}

	var existing Organization
	err := col.FindOne(ctx, filter).Decode(&existing)

	if err != nil {
		org.DateCreated = time.Now()
		_, err := col.InsertOne(ctx, org)
		return err
	}

	update := bson.D{{Key: "$set", Value: org}}
	_, err = col.UpdateOne(ctx, filter, update)
	return err
}
