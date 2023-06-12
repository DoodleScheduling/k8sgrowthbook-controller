package growthbook

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"github.com/DoodleScheduling/k8sgrowthbook-controller/api/v1beta1"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/crypto/scrypt"
)

const saltLen = 16
const hashLen = 64

type User struct {
	ID           string `bson:"id"`
	Email        string `bson:"email"`
	Name         string `bson:"name"`
	PasswordHash string `bson:"passwordHash"`
}

func (u *User) FromV1beta1(user v1beta1.GrowthbookUser) *User {
	u.Name = user.GetName()
	u.ID = user.GetID()
	u.Email = user.Spec.Email
	return u
}

func (u *User) SetPassword(password string) error {
	buf := make([]byte, saltLen)
	_, err := rand.Read(buf)
	if err != nil {
		return err
	}

	p, err := scrypt.Key([]byte(password), buf, 32768, 8, 1, hashLen)
	if err != nil {
		return err
	}

	u.PasswordHash = fmt.Sprintf("%s:%s", hex.EncodeToString(buf), hex.EncodeToString(p))

	return nil
}

func UpdateUser(ctx context.Context, org User, db *mongo.Database) error {
	col := db.Collection("users")
	filter := bson.M{
		"id": org.ID,
	}

	var existing User
	err := col.FindOne(ctx, filter).Decode(&existing)

	if err != nil {
		_, err := col.InsertOne(ctx, org)
		return err
	}

	update := bson.D{{Key: "$set", Value: org}}
	_, err = col.UpdateOne(ctx, filter, update)
	return err
}
