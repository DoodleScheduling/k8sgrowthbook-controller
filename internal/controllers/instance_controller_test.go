package controllers

import (
	"context"
	"fmt"
	"time"

	"github.com/DoodleScheduling/k8sgrowthbook-controller/api/v1beta1"
	"github.com/DoodleScheduling/k8sgrowthbook-controller/internal/growthbook"
	"github.com/DoodleScheduling/k8sgrowthbook-controller/internal/storage"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// MockProvider returns a storage.Database for testing
func MockProvider(ctx context.Context, instance v1beta1.GrowthbookInstance, username, password string) (storage.Database, error) {
	// See test "reconciling a GrowthbookInstance with a timeout"
	if instance.Spec.Timeout != nil && instance.Spec.Timeout.Duration == 500*time.Millisecond {
		<-ctx.Done()
		return nil, ctx.Err()
	}

	// For testing the controller logic the storage adapter just does nothing and returns no error
	if instance.Spec.MongoDB.URI == "" {
		return &growthbook.MockDatabase{
			FindOne: func(ctx context.Context, filter interface{}, dst interface{}) error {
				return nil
			},
			InsertOne: func(ctx context.Context, doc interface{}) error {
				return nil
			},
			UpdateOne: func(ctx context.Context, filter interface{}, doc interface{}) error {
				return nil
			},
			DeleteMany: func(ctx context.Context, filter interface{}) error {
				return nil
			},
			DeleteOne: func(ctx context.Context, filter interface{}) error {
				return nil
			},
		}, nil
	}

	return MongoDBProvider(ctx, instance, username, password)
}

var _ = Describe("GrowthbookInstance controller", func() {
	const (
		timeout  = time.Second * 20
		interval = time.Millisecond * 600
	)

	When("reconciling a GrowthbookInstance with referencing organizations", func() {
		name := fmt.Sprintf("growthbookinstance-%s", randStringRunes(5))
		nameOrg := fmt.Sprintf("growthbookorganization-%s", randStringRunes(5))
		nameAnotherOrg := fmt.Sprintf("growthbookorganization-%s", randStringRunes(5))

		It("Should update status.catalog with all resources found", func() {
			By("By creating a new GrowthbookInstance")
			ctx := context.Background()

			gi := &v1beta1.GrowthbookInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: "default",
				},
				Spec: v1beta1.GrowthbookInstanceSpec{
					MongoDB: v1beta1.GrowthbookInstanceMongoDB{},
					ResourceSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"instance": name,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, gi)).Should(Succeed())

			By("By creating a new GrowthbookOrganization matching instance=test-instance")
			gf := &v1beta1.GrowthbookOrganization{
				ObjectMeta: metav1.ObjectMeta{
					Name:      nameOrg,
					Namespace: "default",
					Labels: map[string]string{
						"instance": name,
					},
				},
				Spec: v1beta1.GrowthbookOrganizationSpec{
					OwnerEmail: "admin@org.com",
					ResourceSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"org": nameOrg,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, gf)).Should(Succeed())

			By("By creating a new GrowthbookOrganization not matching instance=test-instance")
			gf2 := &v1beta1.GrowthbookOrganization{
				ObjectMeta: metav1.ObjectMeta{
					Name:      nameAnotherOrg,
					Namespace: "default",
				},
				Spec: v1beta1.GrowthbookOrganizationSpec{
					OwnerEmail:       "admin@another.com",
					ResourceSelector: &metav1.LabelSelector{},
				},
			}
			Expect(k8sClient.Create(ctx, gf2)).Should(Succeed())

			instanceLookupKey := types.NamespacedName{Name: name, Namespace: "default"}
			reconciledInstance := &v1beta1.GrowthbookInstance{}

			expectedStatus := v1beta1.GrowthbookInstanceStatus{
				ObservedGeneration: int64(1),
				SubResourceCatalog: []v1beta1.ResourceReference{
					{
						Kind:       "GrowthbookOrganization",
						APIVersion: "growthbook.infra.doodle.com/v1beta1",
						Name:       nameOrg,
					},
				},
				Conditions: []metav1.Condition{
					{
						Type:               v1beta1.ReadyCondition,
						Status:             "True",
						ObservedGeneration: 0,
						Reason:             "Synchronized",
						Message:            "instance successfully reconciled",
					},
				},
			}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, instanceLookupKey, reconciledInstance)
				if err != nil {
					return false
				}

				return needStatus(reconciledInstance, &expectedStatus)
			}, timeout, interval).Should(BeTrue())

			Expect(reconciledInstance.Status.SubResourceCatalog).To(Equal(expectedStatus.SubResourceCatalog))
		})

		nameFeature := fmt.Sprintf("growthbookfeature-%s", randStringRunes(5))
		nameAnotherFeature := fmt.Sprintf("growthbookfeature-%s", randStringRunes(5))

		It("Should update status.catalog with new GrowthbookFeatures added", func() {
			By("By creating a new GrowthbookFeature matching organization org=test-org")
			gf := &v1beta1.GrowthbookFeature{
				ObjectMeta: metav1.ObjectMeta{
					Name:      nameFeature,
					Namespace: "default",
					Labels: map[string]string{
						"org":      nameOrg,
						"instance": name,
					},
				},
				Spec: v1beta1.GrowthbookFeatureSpec{
					Tags: []string{"tag"},
				},
			}
			Expect(k8sClient.Create(ctx, gf)).Should(Succeed())

			By("By creating a new GrowthbookFeature not matching org=test-org")
			gf2 := &v1beta1.GrowthbookFeature{
				ObjectMeta: metav1.ObjectMeta{
					Name:      nameAnotherFeature,
					Namespace: "default",
					Labels: map[string]string{
						"instance": name,
					},
				},
				Spec: v1beta1.GrowthbookFeatureSpec{
					Tags: []string{"tag"},
				},
			}
			Expect(k8sClient.Create(ctx, gf2)).Should(Succeed())

			instanceLookupKey := types.NamespacedName{Name: name, Namespace: "default"}
			reconciledInstance := &v1beta1.GrowthbookInstance{}

			expectedStatus := v1beta1.GrowthbookInstanceStatus{
				ObservedGeneration: int64(1),
				SubResourceCatalog: []v1beta1.ResourceReference{
					{
						Kind:       "GrowthbookOrganization",
						APIVersion: "growthbook.infra.doodle.com/v1beta1",
						Name:       nameOrg,
					},
					{
						Kind:       "GrowthbookFeature",
						APIVersion: "growthbook.infra.doodle.com/v1beta1",
						Name:       nameFeature,
					},
				},
				Conditions: []metav1.Condition{
					{
						Type:               v1beta1.ReadyCondition,
						Status:             "True",
						ObservedGeneration: 0,
						Reason:             "Synchronized",
						Message:            "instance successfully reconciled",
					},
				},
			}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, instanceLookupKey, reconciledInstance)
				if err != nil {
					return false
				}

				return needStatus(reconciledInstance, &expectedStatus)
			}, timeout, interval).Should(BeTrue())

			Expect(reconciledInstance.Status.SubResourceCatalog).To(Equal(expectedStatus.SubResourceCatalog))
		})

		instanceAnotherName := fmt.Sprintf("growthbookinstance-%s", randStringRunes(5))

		It("Creates a new instance with matching all resources", func() {
			By("By creating a new GrowthbookInstance")
			ctx := context.Background()

			gi := &v1beta1.GrowthbookInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      instanceAnotherName,
					Namespace: "default",
				},
				Spec: v1beta1.GrowthbookInstanceSpec{
					MongoDB:          v1beta1.GrowthbookInstanceMongoDB{},
					ResourceSelector: &metav1.LabelSelector{},
				},
			}
			Expect(k8sClient.Create(ctx, gi)).Should(Succeed())

			instanceLookupKey := types.NamespacedName{Name: instanceAnotherName, Namespace: "default"}
			reconciledInstance := &v1beta1.GrowthbookInstance{}

			expectedStatus := v1beta1.GrowthbookInstanceStatus{
				ObservedGeneration: int64(1),
				SubResourceCatalog: []v1beta1.ResourceReference{
					{
						Kind:       "GrowthbookOrganization",
						APIVersion: "growthbook.infra.doodle.com/v1beta1",
						Name:       nameOrg,
					},
					{
						Kind:       "GrowthbookOrganization",
						APIVersion: "growthbook.infra.doodle.com/v1beta1",
						Name:       nameAnotherOrg,
					},
					{
						Kind:       "GrowthbookFeature",
						APIVersion: "growthbook.infra.doodle.com/v1beta1",
						Name:       nameFeature,
					},
					{
						Kind:       "GrowthbookFeature",
						APIVersion: "growthbook.infra.doodle.com/v1beta1",
						Name:       nameAnotherFeature,
					},
				},
				Conditions: []metav1.Condition{
					{
						Type:               v1beta1.ReadyCondition,
						Status:             "True",
						ObservedGeneration: 0,
						Reason:             "Synchronized",
						Message:            "instance successfully reconciled",
					},
				},
			}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, instanceLookupKey, reconciledInstance)
				if err != nil {
					return false
				}

				return needStatus(reconciledInstance, &expectedStatus)
			}, timeout, interval).Should(BeTrue())

			Expect(reconciledInstance.Status.SubResourceCatalog).To(Equal(expectedStatus.SubResourceCatalog))
		})

		It("Should update status.catalog with new GrowthbookClients added", func() {
			nameClientSecret := fmt.Sprintf("clientsecret-%s", randStringRunes(5))

			By("By creating a client token secret")
			secret := &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      nameClientSecret,
					Namespace: "default",
				},
				Data: map[string][]byte{
					"token": []byte("token"),
				},
			}
			Expect(k8sClient.Create(ctx, secret)).Should(Succeed())

			By("By creating a new GrowthbookClient matching organization org=test-org")
			nameClient := fmt.Sprintf("growthbookclient-%s", randStringRunes(5))
			gc := &v1beta1.GrowthbookClient{
				ObjectMeta: metav1.ObjectMeta{
					Name:      nameClient,
					Namespace: "default",
					Labels: map[string]string{
						"org":      nameOrg,
						"instance": name,
					},
				},
				Spec: v1beta1.GrowthbookClientSpec{
					TokenSecret: &v1beta1.TokenSecretReference{
						Name: nameClientSecret,
					},
				},
			}
			Expect(k8sClient.Create(ctx, gc)).Should(Succeed())

			By("By creating a new GrowthbookClient not matching org=test-org")
			gc2 := &v1beta1.GrowthbookClient{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("growthbookclient-%s", randStringRunes(5)),
					Namespace: "default",
					Labels: map[string]string{
						"instance": name,
					},
				},
				Spec: v1beta1.GrowthbookClientSpec{
					TokenSecret: &v1beta1.TokenSecretReference{
						Name: "does-not-exists",
					},
				},
			}
			Expect(k8sClient.Create(ctx, gc2)).Should(Succeed())

			instanceLookupKey := types.NamespacedName{Name: name, Namespace: "default"}
			reconciledInstance := &v1beta1.GrowthbookInstance{}

			expectedStatus := v1beta1.GrowthbookInstanceStatus{
				ObservedGeneration: int64(1),
				SubResourceCatalog: []v1beta1.ResourceReference{
					{
						Kind:       "GrowthbookOrganization",
						APIVersion: "growthbook.infra.doodle.com/v1beta1",
						Name:       nameOrg,
					},
					{
						Kind:       "GrowthbookFeature",
						APIVersion: "growthbook.infra.doodle.com/v1beta1",
						Name:       nameFeature,
					},
					{
						Kind:       "GrowthbookClient",
						APIVersion: "growthbook.infra.doodle.com/v1beta1",
						Name:       nameClient,
					},
				},
				Conditions: []metav1.Condition{
					{
						Type:               v1beta1.ReadyCondition,
						Status:             "True",
						ObservedGeneration: 0,
						Reason:             "Synchronized",
						Message:            "instance successfully reconciled",
					},
				},
			}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, instanceLookupKey, reconciledInstance)
				if err != nil {
					return false
				}

				return needStatus(reconciledInstance, &expectedStatus)
			}, timeout, interval).Should(BeTrue())

			Expect(reconciledInstance.Status.SubResourceCatalog).To(Equal(expectedStatus.SubResourceCatalog))
		})
	})

	When("Creating a new GrowthbookClient", func() {
		It("Should fail if spec.secret is not specified", func() {
			By("By creating a new GrowthbookClient")
			gu := &v1beta1.GrowthbookClient{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("growthbookclient-%s", randStringRunes(5)),
					Namespace: "default",
					Labels: map[string]string{
						"instance": "test-instance",
					},
				},
				Spec: v1beta1.GrowthbookClientSpec{},
			}
			Expect(k8sClient.Create(ctx, gu)).Should(Not(Succeed()))
		})
	})

	When("Creating a new GrowthbookUser", func() {
		It("Should fail if spec.secret is not specified", func() {
			By("By creating a new GrowthbookUser")
			gu := &v1beta1.GrowthbookUser{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("growthbookuser-%s", randStringRunes(5)),
					Namespace: "default",
					Labels: map[string]string{
						"instance": "test-instance",
					},
				},
				Spec: v1beta1.GrowthbookUserSpec{},
			}

			Expect(k8sClient.Create(ctx, gu)).Should(Not(Succeed()))
		})
	})

	When("reconciling a GrowthbookInstance with referencing users", func() {
		name := fmt.Sprintf("growthbookinstance-%s", randStringRunes(5))
		nameUser := fmt.Sprintf("growthbookuser-%s", randStringRunes(5))
		nameSecret := fmt.Sprintf("usersecret-%s", randStringRunes(5))

		It("Should update status.catalog with all resources found but condition is failed because not secret was found", func() {
			By("By creating a new GrowthbookInstance")
			ctx := context.Background()

			gi := &v1beta1.GrowthbookInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: "default",
				},
				Spec: v1beta1.GrowthbookInstanceSpec{
					MongoDB: v1beta1.GrowthbookInstanceMongoDB{},
					ResourceSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"instance": name,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, gi)).Should(Succeed())

			By("By creating a new GrowthbookUser matching instance=test-instance")
			gu := &v1beta1.GrowthbookUser{
				ObjectMeta: metav1.ObjectMeta{
					Name:      nameUser,
					Namespace: "default",
					Labels: map[string]string{
						"instance": name,
					},
				},
				Spec: v1beta1.GrowthbookUserSpec{
					Secret: &v1beta1.SecretReference{
						Name: nameSecret,
					},
				},
			}
			Expect(k8sClient.Create(ctx, gu)).Should(Succeed())

			By("By creating a new GrowthbookUser not matching instance=test-instance")
			gu2 := &v1beta1.GrowthbookUser{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("growthbookuser-%s", randStringRunes(5)),
					Namespace: "default",
				},
				Spec: v1beta1.GrowthbookUserSpec{
					Secret: &v1beta1.SecretReference{
						Name: "user-secret",
					},
				},
			}
			Expect(k8sClient.Create(ctx, gu2)).Should(Succeed())

			instanceLookupKey := types.NamespacedName{Name: name, Namespace: "default"}
			reconciledInstance := &v1beta1.GrowthbookInstance{}

			expectedStatus := v1beta1.GrowthbookInstanceStatus{
				ObservedGeneration: int64(1),
				SubResourceCatalog: []v1beta1.ResourceReference{
					{
						Kind:       "GrowthbookUser",
						APIVersion: "growthbook.infra.doodle.com/v1beta1",
						Name:       nameUser,
					},
				},
				Conditions: []metav1.Condition{
					{
						Type:               v1beta1.ReadyCondition,
						Status:             "False",
						ObservedGeneration: 0,
						Reason:             "Failed",
						Message:            fmt.Sprintf("failed reconciling users: referencing secret was not found: Secret \"%s\" not found", nameSecret),
					},
				},
			}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, instanceLookupKey, reconciledInstance)
				if err != nil {
					return false
				}

				return needStatus(reconciledInstance, &expectedStatus)
			}, timeout, interval).Should(BeTrue())

			Expect(reconciledInstance.Status.SubResourceCatalog).To(Equal(expectedStatus.SubResourceCatalog))
		})

		It("Should update condition to successful if the user credentials have been added", func() {
			By("By creating a new set of credentials")
			secret := &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      nameSecret,
					Namespace: "default",
				},
				Data: map[string][]byte{
					"password": []byte("password"),
				},
			}
			Expect(k8sClient.Create(ctx, secret)).Should(Succeed())

			instanceLookupKey := types.NamespacedName{Name: name, Namespace: "default"}
			reconciledInstance := &v1beta1.GrowthbookInstance{}

			expectedStatus := v1beta1.GrowthbookInstanceStatus{
				ObservedGeneration: int64(1),
				SubResourceCatalog: []v1beta1.ResourceReference{
					{
						Kind:       "GrowthbookUser",
						APIVersion: "growthbook.infra.doodle.com/v1beta1",
						Name:       nameUser,
					},
				},
				Conditions: []metav1.Condition{
					{
						Type:               v1beta1.ReadyCondition,
						Status:             "True",
						ObservedGeneration: 0,
						Reason:             "Synchronized",
						Message:            "instance successfully reconciled",
					},
				},
			}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, instanceLookupKey, reconciledInstance)
				if err != nil {
					return false
				}

				return needStatus(reconciledInstance, &expectedStatus)
			}, timeout, interval).Should(BeTrue())

			Expect(reconciledInstance.Status.SubResourceCatalog).To(Equal(expectedStatus.SubResourceCatalog))
		})
	})

	When("reconciling a GrowthbookInstance with a timeout", func() {
		name := fmt.Sprintf("growthbookinstance-%s", randStringRunes(5))

		It("Should update status condition to False while timeout was reached", func() {
			By("By creating a new GrowthbookInstance")
			ctx := context.Background()

			gi := &v1beta1.GrowthbookInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: "default",
				},
				Spec: v1beta1.GrowthbookInstanceSpec{
					MongoDB: v1beta1.GrowthbookInstanceMongoDB{},
					Timeout: &metav1.Duration{Duration: time.Millisecond * 500},
				},
			}
			Expect(k8sClient.Create(ctx, gi)).Should(Succeed())

			instanceLookupKey := types.NamespacedName{Name: name, Namespace: "default"}
			reconciledInstance := &v1beta1.GrowthbookInstance{}

			expectedStatus := v1beta1.GrowthbookInstanceStatus{
				ObservedGeneration: int64(1),
				Conditions: []metav1.Condition{
					{
						Type:               v1beta1.ReadyCondition,
						Status:             "False",
						ObservedGeneration: 0,
						Reason:             "Failed",
						Message:            "context deadline exceeded",
					},
				},
			}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, instanceLookupKey, reconciledInstance)
				if err != nil {
					return false
				}

				return needStatus(reconciledInstance, &expectedStatus)
			}, timeout, interval).Should(BeTrue())

			Expect(reconciledInstance.Status.SubResourceCatalog).To(Equal(expectedStatus.SubResourceCatalog))
		})
	})

	When("reconciling a GrowthbookInstance which is suspended", func() {
		name := fmt.Sprintf("growthbookinstance-%s", randStringRunes(5))

		It("Should skip reconciling", func() {
			By("By creating a new GrowthbookInstance")
			ctx := context.Background()

			gi := &v1beta1.GrowthbookInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: "default",
				},
				Spec: v1beta1.GrowthbookInstanceSpec{
					MongoDB: v1beta1.GrowthbookInstanceMongoDB{},
					Suspend: true,
				},
			}
			Expect(k8sClient.Create(ctx, gi)).Should(Succeed())

			instanceLookupKey := types.NamespacedName{Name: name, Namespace: "default"}
			reconciledInstance := &v1beta1.GrowthbookInstance{}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, instanceLookupKey, reconciledInstance)
				if err != nil {
					return false
				}

				return reconciledInstance.Status.ObservedGeneration == 0
			}, timeout, interval).Should(BeTrue())
		})
	})

	When("reconciling a GrowthbookInstance with an invalid mongodb uri provided", func() {
		It("Should fail with a failed status condition", func() {
			By("By creating a new GrowthbookInstance")
			ctx := context.Background()
			name := fmt.Sprintf("growthbookinstance-%s", randStringRunes(5))

			gi := &v1beta1.GrowthbookInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: "default",
				},
				Spec: v1beta1.GrowthbookInstanceSpec{
					MongoDB: v1beta1.GrowthbookInstanceMongoDB{
						URI: "x://invalid",
					},
				},
			}
			Expect(k8sClient.Create(ctx, gi)).Should(Succeed())

			instanceLookupKey := types.NamespacedName{Name: name, Namespace: "default"}
			reconciledInstance := &v1beta1.GrowthbookInstance{}

			expectedStatus := v1beta1.GrowthbookInstanceStatus{
				ObservedGeneration: int64(1),
				Conditions: []metav1.Condition{
					{
						Type:               v1beta1.ReadyCondition,
						Status:             "False",
						ObservedGeneration: 0,
						Reason:             "Failed",
						Message:            "error parsing uri: scheme must be \"mongodb\" or \"mongodb+srv\"",
					},
				},
			}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, instanceLookupKey, reconciledInstance)
				if err != nil {
					return false
				}

				return needStatus(reconciledInstance, &expectedStatus)
			}, timeout, interval).Should(BeTrue())

			Expect(reconciledInstance.Status.SubResourceCatalog).To(Equal(expectedStatus.SubResourceCatalog))
		})
	})

	When("garbae collecting resources other than GrowthbookInstance", func() {
		name := fmt.Sprintf("growthbookinstance-%s", randStringRunes(5))
		nameOrg := fmt.Sprintf("growthbookorganization-%s", randStringRunes(5))

		It("should delete a GrowthbookOrganization without pruning", func() {
			By("By creating a new GrowthbookInstance")
			ctx := context.Background()

			gi := &v1beta1.GrowthbookInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: "default",
				},
				Spec: v1beta1.GrowthbookInstanceSpec{
					MongoDB: v1beta1.GrowthbookInstanceMongoDB{},
					ResourceSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"instance": name,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, gi)).Should(Succeed())

			By("By creating a new GrowthbookOrganization matching instance=test-instance")
			gorg := &v1beta1.GrowthbookOrganization{
				ObjectMeta: metav1.ObjectMeta{
					Name:      nameOrg,
					Namespace: "default",
					Labels: map[string]string{
						"instance": name,
					},
				},
				Spec: v1beta1.GrowthbookOrganizationSpec{
					OwnerEmail: "admin@org.com",
					ResourceSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"org": nameOrg,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, gorg)).Should(Succeed())

			orgLookupKey := types.NamespacedName{Name: nameOrg, Namespace: "default"}
			reconciledOrganization := &v1beta1.GrowthbookOrganization{}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, orgLookupKey, reconciledOrganization)
				if err != nil {
					return false
				}

				return len(reconciledOrganization.Finalizers) == 1 &&
					reconciledOrganization.Finalizers[0] == fmt.Sprintf("finalizers.doodle.com/%s.default", name)
			}, timeout, interval).Should(BeTrue())

			By("By deleting the GrowthbookOrganization")
			Expect(k8sClient.Delete(ctx, gorg)).Should(Succeed())

			Eventually(func() bool {
				err := k8sClient.Get(ctx, orgLookupKey, reconciledOrganization)
				return err != nil
			}, timeout, interval).Should(BeTrue())

			By("By creating the same GrowthbookOrganization matching instance=test-instance again but with a different id")
			gorg2 := &v1beta1.GrowthbookOrganization{
				ObjectMeta: metav1.ObjectMeta{
					Name:      nameOrg,
					Namespace: "default",
					Labels: map[string]string{
						"instance": name,
					},
				},
				Spec: v1beta1.GrowthbookOrganizationSpec{
					ID:         "another-id",
					OwnerEmail: "admin@org.com",
					ResourceSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"org": nameOrg,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, gorg2)).Should(Succeed())

			instanceLookupKey := types.NamespacedName{Name: name, Namespace: "default"}
			reconciledInstance := &v1beta1.GrowthbookInstance{}
			expectedStatus := v1beta1.GrowthbookInstanceStatus{
				ObservedGeneration: int64(1),
				SubResourceCatalog: []v1beta1.ResourceReference{
					{
						Kind:       "GrowthbookOrganization",
						APIVersion: "growthbook.infra.doodle.com/v1beta1",
						Name:       nameOrg,
					},
				},
				Conditions: []metav1.Condition{
					{
						Type:               v1beta1.ReadyCondition,
						Status:             "True",
						ObservedGeneration: 0,
						Reason:             "Synchronized",
						Message:            "instance successfully reconciled",
					},
				},
			}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, instanceLookupKey, reconciledInstance)
				if err != nil {
					return false
				}

				return needStatus(reconciledInstance, &expectedStatus) &&
					len(reconciledInstance.Finalizers) == 1 &&
					reconciledInstance.Finalizers[0] == "finalizers.doodle.com"
			}, timeout, interval).Should(BeTrue())

		})

		It("should delete a GrowthbookOrganization with pruning", func() {
			By("By setting spec.prune=true on the GrowtbookInstance")
			instanceLookupKey := types.NamespacedName{Name: name, Namespace: "default"}
			reconciledInstance := &v1beta1.GrowthbookInstance{}
			Expect(k8sClient.Get(ctx, instanceLookupKey, reconciledInstance)).Should(Succeed())

			reconciledInstance.Spec.Prune = true
			Expect(k8sClient.Update(ctx, reconciledInstance)).Should(Succeed())

			orgLookupKey := types.NamespacedName{Name: nameOrg, Namespace: "default"}
			reconciledOrganization := &v1beta1.GrowthbookOrganization{}
			Expect(k8sClient.Get(ctx, orgLookupKey, reconciledOrganization)).Should(Succeed())

			By("By deleting the GrowthbookOrganization")
			Expect(k8sClient.Delete(ctx, reconciledOrganization)).Should(Succeed())

			Eventually(func() bool {
				err := k8sClient.Get(ctx, orgLookupKey, reconciledOrganization)
				return err != nil
			}, timeout, interval).Should(BeTrue())
		})
	})

	When("garbage collecting GrowthbookInstance", func() {
		name := fmt.Sprintf("growthbookinstance-%s", randStringRunes(5))
		nameOrg := fmt.Sprintf("growthbookorganization-%s", randStringRunes(5))

		It("should remove the growthbooks finalizer from all related resources with pruning", func() {
			By("By creating a new GrowthbookInstance")
			ctx := context.Background()

			gi := &v1beta1.GrowthbookInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: "default",
				},
				Spec: v1beta1.GrowthbookInstanceSpec{
					Prune:   true,
					MongoDB: v1beta1.GrowthbookInstanceMongoDB{},
					ResourceSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"instance": name,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, gi)).Should(Succeed())

			By("By creating a new GrowthbookOrganization matching instance=test-instance")
			gorg := &v1beta1.GrowthbookOrganization{
				ObjectMeta: metav1.ObjectMeta{
					Name:      nameOrg,
					Namespace: "default",
					Labels: map[string]string{
						"instance": name,
					},
				},
				Spec: v1beta1.GrowthbookOrganizationSpec{
					OwnerEmail: "admin@org.com",
					ResourceSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"org": nameOrg,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, gorg)).Should(Succeed())

			orgLookupKey := types.NamespacedName{Name: nameOrg, Namespace: "default"}
			reconciledOrganization := &v1beta1.GrowthbookOrganization{}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, orgLookupKey, reconciledOrganization)
				if err != nil {
					return false
				}

				return len(reconciledOrganization.Finalizers) == 1 &&
					reconciledOrganization.Finalizers[0] == fmt.Sprintf("finalizers.doodle.com/%s.default", name)
			}, timeout, interval).Should(BeTrue())

			By("By deleting the GrowthbookInstance")
			Expect(k8sClient.Delete(ctx, gi)).Should(Succeed())

			instanceLookupKey := types.NamespacedName{Name: name, Namespace: "default"}
			reconciledInstance := &v1beta1.GrowthbookInstance{}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, instanceLookupKey, reconciledInstance)
				return err != nil
			}, timeout, interval).Should(BeTrue())

			By("By making sure the finalizer from the GrowthbookOrganization is removed")
			Eventually(func() bool {
				err := k8sClient.Get(ctx, orgLookupKey, reconciledOrganization)
				if err != nil {
					return false
				}

				return len(reconciledOrganization.Finalizers) == 0
			}, timeout, interval).Should(BeTrue())
		})
	})

	It("should remove the growthbooks finalizer from all related resources without pruning", func() {
		name := fmt.Sprintf("growthbookinstance-%s", randStringRunes(5))
		nameOrg := fmt.Sprintf("growthbookorganization-%s", randStringRunes(5))

		By("By creating a new GrowthbookInstance")
		ctx := context.Background()

		gi := &v1beta1.GrowthbookInstance{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: "default",
			},
			Spec: v1beta1.GrowthbookInstanceSpec{
				MongoDB: v1beta1.GrowthbookInstanceMongoDB{},
				ResourceSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"instance": name,
					},
				},
			},
		}
		Expect(k8sClient.Create(ctx, gi)).Should(Succeed())

		By("By creating a new GrowthbookOrganization matching instance=test-instance")
		gorg := &v1beta1.GrowthbookOrganization{
			ObjectMeta: metav1.ObjectMeta{
				Name:      nameOrg,
				Namespace: "default",
				Labels: map[string]string{
					"instance": name,
				},
			},
			Spec: v1beta1.GrowthbookOrganizationSpec{
				OwnerEmail: "admin@org.com",
				ResourceSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"org": nameOrg,
					},
				},
			},
		}
		Expect(k8sClient.Create(ctx, gorg)).Should(Succeed())

		orgLookupKey := types.NamespacedName{Name: nameOrg, Namespace: "default"}
		reconciledOrganization := &v1beta1.GrowthbookOrganization{}

		Eventually(func() bool {
			err := k8sClient.Get(ctx, orgLookupKey, reconciledOrganization)
			if err != nil {
				return false
			}

			return len(reconciledOrganization.Finalizers) == 1 &&
				reconciledOrganization.Finalizers[0] == fmt.Sprintf("finalizers.doodle.com/%s.default", name)
		}, timeout, interval).Should(BeTrue())

		By("By deleting the GrowthbookInstance")
		Expect(k8sClient.Delete(ctx, gi)).Should(Succeed())

		instanceLookupKey := types.NamespacedName{Name: name, Namespace: "default"}
		reconciledInstance := &v1beta1.GrowthbookInstance{}

		Eventually(func() bool {
			err := k8sClient.Get(ctx, instanceLookupKey, reconciledInstance)
			return err != nil
		}, timeout, interval).Should(BeTrue())

		By("By making sure the finalizer from the GrowthbookOrganization is removed")
		Eventually(func() bool {
			err := k8sClient.Get(ctx, orgLookupKey, reconciledOrganization)
			if err != nil {
				return false
			}

			return len(reconciledOrganization.Finalizers) == 0
		}, timeout, interval).Should(BeTrue())
	})
})

func needStatus(reconciledInstance *v1beta1.GrowthbookInstance, expectedStatus *v1beta1.GrowthbookInstanceStatus) bool {
	return reconciledInstance.Status.ObservedGeneration != 0 &&
		reconciledInstance.Status.Conditions[0].Reason == expectedStatus.Conditions[0].Reason &&
		reconciledInstance.Status.LastReconcileDuration.Duration > 0 &&
		len(reconciledInstance.Status.Conditions) > 0 &&
		len(reconciledInstance.Status.SubResourceCatalog) == len(expectedStatus.SubResourceCatalog) &&
		reconciledInstance.Status.Conditions[0].Type == expectedStatus.Conditions[0].Type &&
		reconciledInstance.Status.Conditions[0].Status == expectedStatus.Conditions[0].Status &&
		reconciledInstance.Status.Conditions[0].ObservedGeneration == expectedStatus.Conditions[0].ObservedGeneration &&
		reconciledInstance.Status.Conditions[0].Reason == expectedStatus.Conditions[0].Reason &&
		reconciledInstance.Status.Conditions[0].Message == expectedStatus.Conditions[0].Message
}
