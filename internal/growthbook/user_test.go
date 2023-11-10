package growthbook

import (
	"context"
	"errors"
	"testing"

	"github.com/DoodleScheduling/growthbook-controller/api/v1beta1"
	. "github.com/onsi/gomega"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestUserFromV1beta1(t *testing.T) {
	g := NewWithT(t)

	apiSpec := v1beta1.GrowthbookUser{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo",
			Namespace: "bar",
		},
		Spec: v1beta1.GrowthbookUserSpec{
			Email: "org@org.com",
		},
	}

	f := &User{}
	f.FromV1beta1(apiSpec)
	g.Expect(f.Email).To(Equal(apiSpec.Spec.Email))
	g.Expect(f.Name).To(Equal(apiSpec.Name))
	g.Expect(f.ID).To(Equal(apiSpec.Name))

	apiSpec.Spec.ID = "custom"
	apiSpec.Spec.Name = "custom"
	f.FromV1beta1(apiSpec)
	g.Expect(f.ID).To(Equal(apiSpec.Spec.ID))
	g.Expect(f.Name).To(Equal(apiSpec.Spec.Name))
}

func TestUserDelete(t *testing.T) {
	g := NewWithT(t)

	var deleteFilter bson.M
	db := &MockDatabase{
		DeleteOne: func(ctx context.Context, filter interface{}) error {
			deleteFilter = filter.(bson.M)
			return nil
		},
	}

	user := User{
		ID: "user",
	}

	err := DeleteUser(context.TODO(), user, db)
	g.Expect(err).To(BeNil())
	g.Expect(deleteFilter).To(Equal(bson.M{
		"id": "user",
	}))
}

func TestUserCreateIfNotExists(t *testing.T) {
	g := NewWithT(t)

	var insertedDoc User
	db := &MockDatabase{
		FindOne: func(ctx context.Context, filter, dst interface{}) error {
			return errors.New("does not exists")
		},
		InsertOne: func(ctx context.Context, doc interface{}) error {
			insertedDoc = doc.(User)
			return nil
		},
	}

	User := User{
		ID: "User",
	}

	err := UpdateUser(context.TODO(), User, db)
	g.Expect(err).To(BeNil())
	g.Expect(insertedDoc.ID).To(Equal(User.ID))
}

func TestUserNoUpdate(t *testing.T) {
	g := NewWithT(t)

	db := &MockDatabase{
		FindOne: func(ctx context.Context, filter, dst interface{}) error {
			dst.(*User).ID = "id"
			return nil
		},
	}

	user := User{
		ID: "id",
	}

	err := UpdateUser(context.TODO(), user, db)
	g.Expect(err).To(BeNil())
}

func TestUserUpdate(t *testing.T) {
	g := NewWithT(t)

	var updateFilter interface{}
	var updateDoc interface{}

	db := &MockDatabase{
		FindOne: func(ctx context.Context, filter, dst interface{}) error {
			dst.(*User).ID = "id"
			dst.(*User).Email = "old@org.com"
			return nil
		},
		UpdateOne: func(ctx context.Context, filter, doc interface{}) error {
			updateFilter = filter
			updateDoc = doc
			return nil
		},
	}

	user := User{
		ID:    "id",
		Email: "new@org.com",
	}

	expectedDoc, _ := bson.Marshal(user)
	expectedFilter := bson.M{
		"id": user.ID,
	}

	err := UpdateUser(context.TODO(), user, db)
	g.Expect(err).To(BeNil())

	updateDocSet := updateDoc.(primitive.D)
	updateBSON := updateDocSet[0].Value.(bson.Raw)
	newEmailValue := updateBSON.Lookup("email")

	g.Expect(newEmailValue).To(Equal(bson.Raw(expectedDoc).Lookup("email")))
	g.Expect(updateFilter).To(Equal(expectedFilter))
}

func TestUserSetPasswordIfEmpty(t *testing.T) {
	g := NewWithT(t)

	db := &MockDatabase{
		FindOne: func(ctx context.Context, filter, dst interface{}) error {
			dst.(*User).ID = "id"
			dst.(*User).Email = "old@org.com"
			return nil
		},
	}

	user := User{
		ID: "id",
	}

	g.Expect(user.SetPassword(context.TODO(), db, "new-password")).To(BeNil())
	g.Expect(len(user.PasswordHash)).To(Equal(161))
}

func TestUserSetPassword(t *testing.T) {
	g := NewWithT(t)

	var passwordHash = "edde9778227cd6f1d6c19989a38f4bba:9f95ef055ac8e16aa7186afbc13de15f9810a97fc9334755c3c7dd22b0a8408c373dd44ce7df9a8065f70ada23f57e4c981976ddea4cd3b24945afb44229aada"

	db := &MockDatabase{
		FindOne: func(ctx context.Context, filter, dst interface{}) error {
			dst.(*User).ID = "id"
			dst.(*User).PasswordHash = passwordHash
			dst.(*User).Email = "old@org.com"
			return nil
		},
	}

	user := User{
		ID: "id",
	}

	g.Expect(user.SetPassword(context.TODO(), db, "new-password")).To(BeNil())
	g.Expect(user.PasswordHash).To(Equal(passwordHash))
	g.Expect(user.SetPassword(context.TODO(), db, "another-password")).To(BeNil())
	g.Expect(user.PasswordHash).To(Not(Equal(passwordHash)))
}
