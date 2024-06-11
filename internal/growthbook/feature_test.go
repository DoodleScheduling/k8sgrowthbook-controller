package growthbook

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DoodleScheduling/growthbook-controller/api/v1beta1"
	"github.com/DoodleScheduling/growthbook-controller/internal/storage"
	. "github.com/onsi/gomega"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestFeatureFromV1beta1(t *testing.T) {
	g := NewWithT(t)

	apiSpec := v1beta1.GrowthbookFeature{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo",
			Namespace: "bar",
		},
		Spec: v1beta1.GrowthbookFeatureSpec{
			Tags:         []string{"foo", "bar"},
			DefaultValue: "true",
			ValueType:    v1beta1.FeatureValueTypeBoolean,
			Description:  "foo",
			Environments: []v1beta1.Environment{
				{
					Name:    "production",
					Enabled: true,
				},
				{
					Name: "dev",
				},
				{
					Name:    "other",
					Enabled: true,
					Rules: []v1beta1.FeatureRule{
						{
							Type:  v1beta1.FeatureRuleTypeForce,
							Value: "false",
						},
					},
				},
			},
		},
	}

	f := &Feature{}
	f.FromV1beta1(apiSpec)
	g.Expect(f.ID).To(Equal(apiSpec.Name))
	g.Expect(f.Description).To(Equal(apiSpec.Spec.Description))
	g.Expect(f.Tags).To(Equal(apiSpec.Spec.Tags))
	g.Expect(f.DefaultValue).To(Equal(apiSpec.Spec.DefaultValue))
	g.Expect(string(f.ValueType)).To(Equal(string(apiSpec.Spec.ValueType)))
	g.Expect(f.EnvironmentSettings["production"].Enabled).To(BeTrue())
	g.Expect(f.EnvironmentSettings["dev"].Enabled).To(BeFalse())
	g.Expect(f.EnvironmentSettings["other"].Enabled).To(BeTrue())
	g.Expect(f.EnvironmentSettings["other"].Rules).To(Equal([]FeatureRule{{Type: FeatureRuleTypeForce, Value: "false"}}))

	apiSpec.Spec.ID = "custom"
	f.FromV1beta1(apiSpec)
	g.Expect(f.ID).To(Equal(apiSpec.Spec.ID))
}

func TestFeatureDelete(t *testing.T) {
	g := NewWithT(t)

	var deleteFilter bson.M
	db := &MockDatabase{
		DeleteOne: func(ctx context.Context, filter interface{}) error {
			deleteFilter = filter.(bson.M)
			return nil
		},
	}

	feature := Feature{
		ID: "feature",
	}

	err := DeleteFeature(context.TODO(), feature, db)
	g.Expect(err).To(BeNil())
	g.Expect(deleteFilter).To(Equal(bson.M{
		"id": "feature",
	}))
}

func TestFeatureCreateIfNotExists(t *testing.T) {
	g := NewWithT(t)

	var insertedDoc Feature
	db := &MockDatabase{
		FindOne: func(ctx context.Context, filter interface{}) (storage.Decoder, error) {
			return nil, errors.New("does not exists")
		},
		InsertOne: func(ctx context.Context, doc interface{}) error {
			insertedDoc = doc.(Feature)
			return nil
		},
	}

	feature := Feature{
		ID: "feature",
	}

	err := UpdateFeature(context.TODO(), feature, db)
	g.Expect(err).To(BeNil())
	g.Expect(insertedDoc.ID).To(Equal(feature.ID))
}

func TestFeatureNoUpdate(t *testing.T) {
	g := NewWithT(t)

	db := &MockDatabase{
		FindOne: func(ctx context.Context, filter interface{}) (storage.Decoder, error) {
			return &MockResult{
				decode: func(dst interface{}) error {
					dst.(*Feature).ID = "id"
					dst.(*Feature).EnvironmentSettings = map[string]EnvironmentSetting{
						"dev": {
							Rules: []FeatureRule{
								{
									ID:    "existing_rule",
									Type:  "force",
									Value: "test",
								},
							},
						},
					}
					return nil
				},
			}, nil
		},
	}

	feature := Feature{
		ID: "id",
		EnvironmentSettings: map[string]EnvironmentSetting{
			"dev": {},
		},
	}

	err := UpdateFeature(context.TODO(), feature, db)
	g.Expect(err).To(BeNil())
}

func TestFeatureUpdate(t *testing.T) {
	g := NewWithT(t)

	var updateFilter interface{}
	var updateDoc interface{}

	db := &MockDatabase{
		FindOne: func(ctx context.Context, filter interface{}) (storage.Decoder, error) {
			return &MockResult{
				decode: func(dst interface{}) error {
					dst.(*Feature).ID = "id"
					dst.(*Feature).DefaultValue = "current-value"
					dst.(*Feature).EnvironmentSettings = map[string]EnvironmentSetting{
						"dev": {
							Rules: []FeatureRule{
								{
									ID:    "existing_rule",
									Type:  "force",
									Value: "test",
								},
								{
									Type:  "rollout",
									Value: "another-rule",
								},
							},
						},
					}

					return nil
				},
			}, nil
		},
		UpdateOne: func(ctx context.Context, filter, doc interface{}) error {
			updateFilter = filter
			updateDoc = doc
			return nil
		},
	}

	feature := Feature{
		ID:           "id",
		DefaultValue: "new-value",
		EnvironmentSettings: map[string]EnvironmentSetting{
			"dev": {
				Rules: []FeatureRule{
					{
						Type:     "rollout",
						Coverage: 0.1,
					},
				},
			},
		},
	}

	expectedDoc, _ := bson.Marshal(feature)
	expectedFilter := bson.M{
		"id": feature.ID,
	}

	beforeUpdate := time.Now().Add(time.Duration(-1) * time.Hour)
	err := UpdateFeature(context.TODO(), feature, db)
	g.Expect(err).To(BeNil())

	updateDocSet := updateDoc.(primitive.D)
	updateBSON := updateDocSet[0].Value.(bson.Raw)
	newDefaultValue := updateBSON.Lookup("defaultValue")
	newDateUpdatedValue := updateBSON.Lookup("dateUpdated")
	newEnvironmentSettings := updateBSON.Lookup("environmentSettings")
	expectedEnvironmentSettings, _ := bson.Marshal(Feature{
		EnvironmentSettings: map[string]EnvironmentSetting{
			"dev": {
				Rules: []FeatureRule{
					{
						ID:    "existing_rule",
						Type:  "force",
						Value: "test",
					},
					{
						Type:     "rollout",
						Coverage: 0.1,
					},
				},
			},
		}})

	g.Expect(newEnvironmentSettings).To(Equal(bson.Raw(expectedEnvironmentSettings).Lookup("environmentSettings")))

	g.Expect(newDefaultValue).To(Equal(bson.Raw(expectedDoc).Lookup("defaultValue")))
	dateUpdated := newDateUpdatedValue.Time()

	g.Expect(dateUpdated.After(beforeUpdate)).To(BeTrue())
	g.Expect(updateFilter).To(Equal(expectedFilter))
}
