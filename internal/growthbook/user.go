package growthbook

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/DoodleScheduling/k8sgrowthbook-controller/api/v1beta1"
	"github.com/DoodleScheduling/k8sgrowthbook-controller/internal/storage"
	"go.mongodb.org/mongo-driver/bson"
	"golang.org/x/crypto/scrypt"
)

const saltLen = 16
const hashLen = 64

type User struct {
	ID           string `bson:"id"`
	Email        string `bson:"email"`
	Name         string `bson:"name"`
	PasswordHash string `bson:"passwordHash"`
	Revision     int    `bson:"__v"`
}

func (u *User) FromV1beta1(user v1beta1.GrowthbookUser) *User {
	u.Name = user.GetName()
	u.ID = user.GetID()
	u.Email = user.Spec.Email
	return u
}

// SetPassword is compatible to https://github.com/growthbook/growthbook/blob/bbe5e54d00c8f9c8a7a575f78bf11ab1dc85cd24/packages/back-end/src/services/users.ts#L16
func (u *User) SetPassword(ctx context.Context, db storage.Database, password string) error {
	col := db.Collection("users")
	filter := bson.M{
		"id": u.ID,
	}

	var existing User
	err := col.FindOne(ctx, filter, &existing)
	var salt string

	if err != nil || strings.Split(existing.PasswordHash, ":")[0] == "" {
		buf := make([]byte, saltLen)
		_, err := rand.Read(buf)
		if err != nil {
			return err
		}

		salt = hex.EncodeToString(buf)
	} else {
		s := strings.Split(existing.PasswordHash, ":")
		salt = s[0]
	}

	// nodejs crypt.scrypt uses N=16384 by default
	p, err := scrypt.Key([]byte(password), []byte(salt), 16384, 8, 1, hashLen)
	if err != nil {
		return err
	}

	u.PasswordHash = fmt.Sprintf("%s:%s", salt, hex.EncodeToString(p))

	return nil
}

func UpdateUser(ctx context.Context, user User, db storage.Database) error {
	col := db.Collection("users")
	filter := bson.M{
		"id": user.ID,
	}

	var existing User
	err := col.FindOne(ctx, filter, &existing)

	if err != nil {
		return col.InsertOne(ctx, user)
	}

	existingBson, err := bson.Marshal(existing)
	if err != nil {
		return err
	}

	existing.ID = user.ID
	existing.Email = user.Email
	existing.Name = user.Name

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
