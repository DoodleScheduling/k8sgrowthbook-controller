package growthbook

import (
	"context"
	"errors"
	"testing"

	"github.com/DoodleScheduling/k8sgrowthbook-controller/api/v1beta1"
	. "github.com/onsi/gomega"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestOrganizationFromV1beta1(t *testing.T) {
	g := NewWithT(t)

	apiSpec := v1beta1.GrowthbookOrganization{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo",
			Namespace: "bar",
		},
		Spec: v1beta1.GrowthbookOrganizationSpec{
			OwnerEmail: "org@org.com",
		},
	}

	f := &Organization{}
	f.FromV1beta1(apiSpec)
	g.Expect(f.OwnerEmail).To(Equal(apiSpec.Spec.OwnerEmail))
	g.Expect(f.Name).To(Equal(apiSpec.Name))
	g.Expect(f.ID).To(Equal(apiSpec.Name))

	apiSpec.Spec.ID = "custom"
	apiSpec.Spec.Name = "custom"
	f.FromV1beta1(apiSpec)
	g.Expect(f.ID).To(Equal(apiSpec.Spec.ID))
	g.Expect(f.Name).To(Equal(apiSpec.Spec.Name))
}

func TestOrganizationCreateIfNotExists(t *testing.T) {
	g := NewWithT(t)

	var insertedDoc Organization
	db := &MockDatabase{
		FindOne: func(ctx context.Context, filter, dst interface{}) error {
			return errors.New("does not exists")
		},
		InsertOne: func(ctx context.Context, doc interface{}) error {
			insertedDoc = doc.(Organization)
			return nil
		},
	}

	Organization := Organization{
		ID: "Organization",
	}

	err := UpdateOrganization(context.TODO(), Organization, db)
	g.Expect(err).To(BeNil())
	g.Expect(insertedDoc.ID).To(Equal(Organization.ID))
	g.Expect(insertedDoc.Members).To(Equal([]OrganizationMember{}))
}

func TestOrganizationNoUpdate(t *testing.T) {
	g := NewWithT(t)

	db := &MockDatabase{
		FindOne: func(ctx context.Context, filter, dst interface{}) error {
			dst.(*Organization).ID = "id"
			return nil
		},
	}

	org := Organization{
		ID: "id",
	}

	err := UpdateOrganization(context.TODO(), org, db)
	g.Expect(err).To(BeNil())
}

func TestOrganizationUpdate(t *testing.T) {
	g := NewWithT(t)

	var updateFilter interface{}
	var updateDoc interface{}
	var find bson.Raw

	db := &MockDatabase{
		FindOne: func(ctx context.Context, filter, dst interface{}) error {
			dst.(*Organization).ID = "id"
			dst.(*Organization).OwnerEmail = "old@mail.com"
			dst.(*Organization).Members = []OrganizationMember{
				{
					ID: "user",
				},
			}

			f, _ := bson.Marshal(dst)
			find = f

			return nil
		},
		UpdateOne: func(ctx context.Context, filter, doc interface{}) error {
			updateFilter = filter
			updateDoc = doc
			return nil
		},
	}

	org := Organization{
		ID:         "id",
		OwnerEmail: "new@mail.com",
	}

	expectedDoc, _ := bson.Marshal(org)
	expectedFilter := bson.M{
		"id": org.ID,
	}

	err := UpdateOrganization(context.TODO(), org, db)
	g.Expect(err).To(BeNil())

	updateDocSet := updateDoc.(primitive.D)
	updateBSON := updateDocSet[0].Value.(bson.Raw)
	newOwnerEmailValue := updateBSON.Lookup("ownerEmail")
	newMemberValue := updateBSON.Lookup("members")

	g.Expect(newOwnerEmailValue).To(Equal(bson.Raw(expectedDoc).Lookup("ownerEmail")))
	g.Expect(newMemberValue).To(Equal(find.Lookup("members")))
	g.Expect(updateFilter).To(Equal(expectedFilter))
}
