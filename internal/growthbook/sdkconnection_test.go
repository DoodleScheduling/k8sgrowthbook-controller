package growthbook

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/DoodleScheduling/k8sgrowthbook-controller/api/v1beta1"
	. "github.com/onsi/gomega"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestSDKConnectionFromV1beta1(t *testing.T) {
	g := NewWithT(t)

	apiSpec := v1beta1.GrowthbookClient{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo",
			Namespace: "bar",
		},
		Spec: v1beta1.GrowthbookClientSpec{
			Languages:                []string{"go"},
			Environment:              "test",
			EncryptPayload:           true,
			Project:                  "test",
			IncludeVisualExperiments: true,
			IncludeDraftExperiments:  true,
			IncludeExperimentNames:   true,
		},
	}

	f := &SDKConnection{}
	f.FromV1beta1(apiSpec)
	g.Expect(f.Languages).To(Equal(apiSpec.Spec.Languages))
	g.Expect(f.Environment).To(Equal(apiSpec.Spec.Environment))
	g.Expect(f.EncryptPayload).To(Equal(apiSpec.Spec.EncryptPayload))
	g.Expect(f.Project).To(Equal(apiSpec.Spec.Project))
	g.Expect(f.IncludeVisualExperiments).To(Equal(apiSpec.Spec.IncludeVisualExperiments))
	g.Expect(f.IncludeDraftExperiments).To(Equal(apiSpec.Spec.IncludeDraftExperiments))
	g.Expect(f.IncludeExperimentNames).To(Equal(apiSpec.Spec.IncludeExperimentNames))
	g.Expect(f.Name).To(Equal(apiSpec.Name))
	g.Expect(f.ID).To(Equal(fmt.Sprintf("%s-%s", apiSpec.Name, apiSpec.Namespace)))

	apiSpec.Spec.ID = "custom"
	apiSpec.Spec.Name = "custom"
	f.FromV1beta1(apiSpec)
	g.Expect(f.ID).To(Equal(apiSpec.Spec.ID))
	g.Expect(f.Name).To(Equal(apiSpec.Spec.Name))
}

func TestSDKConnectionCreateIfNotExists(t *testing.T) {
	g := NewWithT(t)

	var insertedDoc SDKConnection
	db := &MockDatabase{
		FindOne: func(ctx context.Context, filter, dst interface{}) error {
			return errors.New("does not exists")
		},
		InsertOne: func(ctx context.Context, doc interface{}) error {
			insertedDoc = doc.(SDKConnection)
			return nil
		},
		DeleteMany: func(ctx context.Context, filter interface{}) error {
			return nil
		},
	}

	SDKConnection := SDKConnection{
		ID: "SDKConnection",
	}

	err := UpdateSDKConnection(context.TODO(), SDKConnection, db)
	g.Expect(err).To(BeNil())
	g.Expect(insertedDoc.ID).To(Equal(SDKConnection.ID))
}

func TestSDKConnectionNoUpdate(t *testing.T) {
	g := NewWithT(t)

	db := &MockDatabase{
		FindOne: func(ctx context.Context, filter, dst interface{}) error {
			dst.(*SDKConnection).ID = "id"
			return nil
		},
	}

	sdkconnection := SDKConnection{
		ID: "id",
	}

	err := UpdateSDKConnection(context.TODO(), sdkconnection, db)
	g.Expect(err).To(BeNil())
}

func TestSDKConnectionUpdate(t *testing.T) {
	g := NewWithT(t)

	var updateFilter interface{}
	var updateDoc interface{}

	db := &MockDatabase{
		FindOne: func(ctx context.Context, filter, dst interface{}) error {
			dst.(*SDKConnection).ID = "id"
			return nil
		},
		UpdateOne: func(ctx context.Context, filter, doc interface{}) error {
			updateFilter = filter
			updateDoc = doc
			return nil
		},
		DeleteMany: func(ctx context.Context, filter interface{}) error {
			return nil
		},
	}

	sdkconnection := SDKConnection{
		ID:             "id",
		EncryptPayload: true,
	}

	expectedDoc, _ := bson.Marshal(sdkconnection)
	expectedFilter := bson.M{
		"id": sdkconnection.ID,
	}

	beforeUpdate := time.Now().Add(time.Duration(-1) * time.Hour)
	err := UpdateSDKConnection(context.TODO(), sdkconnection, db)
	g.Expect(err).To(BeNil())

	updateDocSet := updateDoc.(primitive.D)
	updateBSON := updateDocSet[0].Value.(bson.Raw)
	newEncryptPayloadValue := updateBSON.Lookup("encryptPayload")
	newDateUpdatedValue := updateBSON.Lookup("dateUpdated")

	g.Expect(newEncryptPayloadValue).To(Equal(bson.Raw(expectedDoc).Lookup("encryptPayload")))
	dateUpdated := newDateUpdatedValue.Time()

	g.Expect(dateUpdated.After(beforeUpdate)).To(BeTrue())
	g.Expect(updateFilter).To(Equal(expectedFilter))
}
