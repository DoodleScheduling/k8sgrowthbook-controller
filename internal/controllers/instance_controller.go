/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	v1beta1 "github.com/DoodleScheduling/k8sgrowthbook-controller/api/v1beta1"
	"github.com/DoodleScheduling/k8sgrowthbook-controller/internal/growthbook"
	"github.com/DoodleScheduling/k8sgrowthbook-controller/internal/storage"
	"github.com/DoodleScheduling/k8sgrowthbook-controller/internal/storage/mongodb"
)

// +kubebuilder:rbac:groups=growthbook.infra.doodle.com,resources=growthbookinstances,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=growthbook.infra.doodle.com,resources=growthbookinstances/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=growthbook.infra.doodle.com,resources=growthbookorganizations,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=growthbook.infra.doodle.com,resources=growthbookusers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=growthbook.infra.doodle.com,resources=growthbookfeatures,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=growthbook.infra.doodle.com,resources=growthbookclients,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

const (
	secretIndexKey = ".metadata.secret"
	usersIndexKey  = ".metadata.users"
	orgsIndexKey   = ".metadata.orgs"
)

// MongoDBProvider returns a storage.Database for MongoDB
func MongoDBProvider(ctx context.Context, instance v1beta1.GrowthbookInstance, username, password string) (storage.Database, error) {
	opts := options.Client().ApplyURI(instance.Spec.MongoDB.URI)
	if username != "" || password != "" {
		opts.SetAuth(options.Credential{
			Username: username,
			Password: password,
		})
	}

	opts.SetAppName("k8sgrowthbook-controller")
	mongoClient, err := mongo.NewClient(opts)
	if err != nil {
		return nil, err
	}

	u, err := url.Parse(instance.Spec.MongoDB.URI)
	if err != nil {
		return nil, err
	}

	dbName := strings.TrimLeft(u.Path, "/")
	db := mongoClient.Database(dbName)

	if err := mongoClient.Connect(ctx); err != nil {
		return nil, fmt.Errorf("failed connecting to mongodb: %w", err)
	}

	return mongodb.New(db), nil
}

// GrowthbookInstance reconciles a GrowthbookInstance object
type GrowthbookInstanceReconciler struct {
	client.Client
	Log              logr.Logger
	Scheme           *runtime.Scheme
	Recorder         record.EventRecorder
	DatabaseProvider func(ctx context.Context, instance v1beta1.GrowthbookInstance, username, password string) (storage.Database, error)
}

type GrowthbookInstanceReconcilerOptions struct {
	MaxConcurrentReconciles int
}

// SetupWithManager adding controllers
func (r *GrowthbookInstanceReconciler) SetupWithManager(mgr ctrl.Manager, opts GrowthbookInstanceReconcilerOptions) error {
	// Index the GrowthbookInstance by the Secret references they point at
	if err := mgr.GetFieldIndexer().IndexField(context.TODO(), &v1beta1.GrowthbookInstance{}, secretIndexKey,
		func(o client.Object) []string {
			// The referenced admin secret gets indexed
			instance := o.(*v1beta1.GrowthbookInstance)
			keys := []string{}

			if instance.Spec.MongoDB.Secret != nil {
				keys = []string{
					fmt.Sprintf("%s/%s", instance.GetNamespace(), instance.Spec.MongoDB.Secret.Name),
				}
			}

			var users v1beta1.GrowthbookUserList
			selector, err := metav1.LabelSelectorAsSelector(instance.Spec.ResourceSelector)
			if err != nil {
				return keys
			}

			err = r.Client.List(context.TODO(), &users, client.InNamespace(instance.Namespace), client.MatchingLabelsSelector{Selector: selector})
			if err != nil {
				return keys
			}

			for _, user := range users.Items {
				if user.Spec.Secret == nil {
					continue
				}

				keys = append(keys, fmt.Sprintf("%s/%s", instance.GetNamespace(), user.Spec.Secret.Name))
			}

			var clients v1beta1.GrowthbookClientList
			selector, err = metav1.LabelSelectorAsSelector(instance.Spec.ResourceSelector)
			if err != nil {
				return keys
			}

			err = r.Client.List(context.TODO(), &clients, client.InNamespace(instance.Namespace), client.MatchingLabelsSelector{Selector: selector})
			if err != nil {
				return keys
			}

			for _, client := range clients.Items {
				if client.Spec.TokenSecret == nil {
					continue
				}

				keys = append(keys, fmt.Sprintf("%s/%s", instance.GetNamespace(), client.Spec.TokenSecret.Name))
			}

			return keys
		},
	); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1beta1.GrowthbookInstance{}, builder.WithPredicates(
			predicate.GenerationChangedPredicate{},
		)).
		Watches(
			&source.Kind{Type: &corev1.Secret{}},
			handler.EnqueueRequestsFromMapFunc(r.requestsForChangeByField(secretIndexKey)),
		).
		Watches(
			&source.Kind{Type: &v1beta1.GrowthbookUser{}},
			handler.EnqueueRequestsFromMapFunc(r.requestsForChangeBySelector),
		).
		Watches(
			&source.Kind{Type: &v1beta1.GrowthbookOrganization{}},
			handler.EnqueueRequestsFromMapFunc(r.requestsForChangeBySelector),
		).
		Watches(
			&source.Kind{Type: &v1beta1.GrowthbookClient{}},
			handler.EnqueueRequestsFromMapFunc(r.requestsForChangeBySelector),
		).
		Watches(
			&source.Kind{Type: &v1beta1.GrowthbookFeature{}},
			handler.EnqueueRequestsFromMapFunc(r.requestsForChangeBySelector),
		).
		WithOptions(controller.Options{MaxConcurrentReconciles: opts.MaxConcurrentReconciles}).
		Complete(r)
}

func (r *GrowthbookInstanceReconciler) requestsForChangeByField(field string) handler.MapFunc {
	return func(o client.Object) []reconcile.Request {
		ctx := context.Background()
		var list v1beta1.GrowthbookInstanceList
		if err := r.List(ctx, &list, client.MatchingFields{
			field: objectKey(o).String(),
		}); err != nil {
			return nil
		}

		var reqs []reconcile.Request
		for _, instance := range list.Items {
			r.Log.Info("change of referenced resource detected", "namespace", instance.GetNamespace(), "instance-name", instance.GetName(), "resource-kind", o.GetObjectKind().GroupVersionKind().Kind, "resource-name", o.GetName())
			reqs = append(reqs, reconcile.Request{NamespacedName: objectKey(&instance)})
		}

		return reqs
	}
}

func (r *GrowthbookInstanceReconciler) requestsForChangeBySelector(o client.Object) []reconcile.Request {
	ctx := context.Background()
	var list v1beta1.GrowthbookInstanceList
	if err := r.List(ctx, &list, client.InNamespace(o.GetNamespace())); err != nil {
		return nil
	}

	var reqs []reconcile.Request
	for _, instance := range list.Items {
		if matches(o.GetLabels(), instance.Spec.ResourceSelector) {
			r.Log.Info("change of referenced resource detected", "namespace", o.GetNamespace(), "name", o.GetName(), "kind", o.GetObjectKind().GroupVersionKind().Kind, "instance-name", instance.GetName())
			reqs = append(reqs, reconcile.Request{NamespacedName: objectKey(&instance)})
		}
	}

	return reqs
}

// Reconcile GrowthbookInstances
func (r *GrowthbookInstanceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := r.Log.WithValues("Namespace", req.Namespace, "Name", req.NamespacedName, "req", req)
	logger.Info("reconciling GrowthbookInstance")

	// Fetch the GrowthbookInstance instance
	instance := v1beta1.GrowthbookInstance{}

	err := r.Client.Get(ctx, req.NamespacedName, &instance)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	if instance.Spec.Suspend {
		return ctrl.Result{}, nil
	}

	start := time.Now()

	reconcileContext := ctx
	if instance.Spec.Timeout != nil {
		c, cancel := context.WithTimeout(ctx, instance.Spec.Timeout.Duration)
		defer cancel()
		reconcileContext = c
	}

	instance, err = r.reconcile(reconcileContext, instance, logger)
	res := ctrl.Result{}

	done := time.Now()

	instance.Status.LastReconcileDuration = metav1.Duration{
		Duration: done.Sub(start),
	}

	instance.Status.ObservedGeneration = instance.GetGeneration()

	if err != nil {
		r.Recorder.Event(&instance, "Normal", "error", err.Error())
		res = ctrl.Result{Requeue: true}
		instance = v1beta1.GrowthbookInstanceNotReady(instance, v1beta1.FailedReason, err.Error())
	} else {
		if instance.Spec.Interval != nil {
			res = ctrl.Result{
				RequeueAfter: instance.Spec.Interval.Duration,
			}
		}

		msg := "instance successfully reconciled"
		r.Recorder.Event(&instance, "Normal", "info", msg)
		instance = v1beta1.GrowthbookInstanceReady(instance, v1beta1.SynchronizedReason, msg)
	}

	// Update status after reconciliation.
	if err := r.patchStatus(ctx, &instance); err != nil {
		logger.Error(err, "unable to update status after reconciliation")
		return ctrl.Result{Requeue: true}, err
	}

	return res, err
}

func (r *GrowthbookInstanceReconciler) reconcile(ctx context.Context, instance v1beta1.GrowthbookInstance, logger logr.Logger) (v1beta1.GrowthbookInstance, error) {
	msg := "reconcile instance progressing"
	r.Recorder.Event(&instance, "Normal", "info", msg)
	instance = v1beta1.GrowthbookInstanceNotReady(instance, v1beta1.ProgressingReason, msg)
	if err := r.patchStatus(ctx, &instance); err != nil {
		return instance, err
	}

	var err error
	var usr, pw string
	if instance.Spec.MongoDB.Secret != nil {
		usr, pw, err = r.getUsernamePassword(ctx, instance, instance.Spec.MongoDB.Secret)
		if err != nil {
			return instance, err
		}
	}

	db, err := r.DatabaseProvider(ctx, instance, usr, pw)
	if err != nil {
		return instance, err
	}

	instance.Status.SubResourceCatalog = []v1beta1.ResourceReference{}

	instance, err = r.reconcileUsers(ctx, instance, db, logger)
	if err != nil {
		return instance, fmt.Errorf("failed reconciling users: %w", err)
	}

	instance, orgs, err := r.reconcileOrganizations(ctx, instance, db, logger)
	if err != nil {
		return instance, fmt.Errorf("failed reconciling organizations: %w", err)
	}

	for _, org := range orgs {
		instance, err = r.reconcileFeatures(ctx, instance, org, db, logger)
		if err != nil {
			return instance, fmt.Errorf("failed reconciling features: %w", err)
		}

		instance, err = r.reconcileClients(ctx, instance, org, db, logger)
		if err != nil {
			return instance, fmt.Errorf("failed reconciling clients: %w", err)
		}
	}

	return instance, err
}

func (r *GrowthbookInstanceReconciler) reconcileOrganizations(ctx context.Context, instance v1beta1.GrowthbookInstance, db storage.Database, logger logr.Logger) (v1beta1.GrowthbookInstance, []v1beta1.GrowthbookOrganization, error) {
	var orgs v1beta1.GrowthbookOrganizationList
	selector, err := metav1.LabelSelectorAsSelector(instance.Spec.ResourceSelector)
	if err != nil {
		return instance, nil, err
	}

	err = r.Client.List(ctx, &orgs, client.InNamespace(instance.Namespace), client.MatchingLabelsSelector{Selector: selector})
	if err != nil {
		return instance, nil, err
	}

	for _, org := range orgs.Items {
		instance.Status.SubResourceCatalog = append(instance.Status.SubResourceCatalog, v1beta1.ResourceReference{
			Kind:       org.Kind,
			Name:       org.Name,
			APIVersion: org.APIVersion,
		})
	}

	for _, org := range orgs.Items {
		o := growthbook.Organization{}
		o.FromV1beta1(org)

		for _, binding := range org.Spec.Users {
			var users v1beta1.GrowthbookUserList
			selector, err := metav1.LabelSelectorAsSelector(binding.Selector)
			if err != nil {
				return instance, nil, err
			}

			err = r.Client.List(ctx, &users, client.InNamespace(instance.Namespace), client.MatchingLabelsSelector{Selector: selector})
			if err != nil {
				return instance, nil, err
			}

			for _, user := range users.Items {
				o.Members = append(o.Members, growthbook.OrganizationMember{
					ID:   user.GetID(),
					Role: binding.Role,
				})
			}
		}

		if err := growthbook.UpdateOrganization(ctx, o, db); err != nil {
			return instance, nil, err
		}
	}

	return instance, orgs.Items, nil
}

func (r *GrowthbookInstanceReconciler) reconcileFeatures(ctx context.Context, instance v1beta1.GrowthbookInstance, org v1beta1.GrowthbookOrganization, db storage.Database, logger logr.Logger) (v1beta1.GrowthbookInstance, error) {
	var features v1beta1.GrowthbookFeatureList
	selector, err := metav1.LabelSelectorAsSelector(org.Spec.ResourceSelector)
	if err != nil {
		return instance, err
	}

	instanceSelector, err := metav1.LabelSelectorAsSelector(instance.Spec.ResourceSelector)
	if err != nil {
		return instance, err
	}

	req, _ := instanceSelector.Requirements()
	selector.Add(req...)

	err = r.Client.List(ctx, &features, client.InNamespace(instance.Namespace), client.MatchingLabelsSelector{Selector: selector})
	if err != nil {
		return instance, err
	}

	for _, feature := range features.Items {
		instance.Status.SubResourceCatalog = append(instance.Status.SubResourceCatalog, v1beta1.ResourceReference{
			Kind:       feature.Kind,
			Name:       feature.Name,
			APIVersion: feature.APIVersion,
		})
	}

	for _, feature := range features.Items {
		f := growthbook.Feature{
			Owner:        "k8sgrowthbook-controller",
			Organization: org.GetID(),
		}

		f.FromV1beta1(feature)

		if err := growthbook.UpdateFeature(ctx, f, db); err != nil {
			return instance, err
		}
	}

	return instance, nil
}

func (r *GrowthbookInstanceReconciler) reconcileUsers(ctx context.Context, instance v1beta1.GrowthbookInstance, db storage.Database, logger logr.Logger) (v1beta1.GrowthbookInstance, error) {
	var users v1beta1.GrowthbookUserList
	selector, err := metav1.LabelSelectorAsSelector(instance.Spec.ResourceSelector)
	if err != nil {
		return instance, err
	}

	err = r.Client.List(ctx, &users, client.InNamespace(instance.Namespace), client.MatchingLabelsSelector{Selector: selector})
	if err != nil {
		return instance, err
	}

	for _, user := range users.Items {
		instance.Status.SubResourceCatalog = append(instance.Status.SubResourceCatalog, v1beta1.ResourceReference{
			Kind:       user.Kind,
			Name:       user.Name,
			APIVersion: user.APIVersion,
		})
	}

	for _, user := range users.Items {
		username, password, err := r.getOptionalUsernamePassword(ctx, instance, user.Spec.Secret)
		if err != nil {
			return instance, err
		}

		u := growthbook.User{}
		if username != "" {
			u.Email = username
		}

		if err := u.FromV1beta1(user).SetPassword(ctx, db, password); err != nil {
			return instance, err
		}

		if err := growthbook.UpdateUser(ctx, u, db); err != nil {
			return instance, err
		}
	}

	return instance, nil
}

func (r *GrowthbookInstanceReconciler) reconcileClients(ctx context.Context, instance v1beta1.GrowthbookInstance, org v1beta1.GrowthbookOrganization, db storage.Database, logger logr.Logger) (v1beta1.GrowthbookInstance, error) {
	var clients v1beta1.GrowthbookClientList
	selector, err := metav1.LabelSelectorAsSelector(org.Spec.ResourceSelector)
	if err != nil {
		return instance, err
	}

	instanceSelector, err := metav1.LabelSelectorAsSelector(instance.Spec.ResourceSelector)
	if err != nil {
		return instance, err
	}

	req, _ := instanceSelector.Requirements()
	selector.Add(req...)

	err = r.Client.List(ctx, &clients, client.InNamespace(instance.Namespace), client.MatchingLabelsSelector{Selector: selector})
	if err != nil {
		return instance, err
	}

	for _, client := range clients.Items {
		instance.Status.SubResourceCatalog = append(instance.Status.SubResourceCatalog, v1beta1.ResourceReference{
			Kind:       client.Kind,
			Name:       client.Name,
			APIVersion: client.APIVersion,
		})
	}

	for _, client := range clients.Items {
		token, err := r.getClientToken(ctx, client)
		if err != nil {
			return instance, err
		}

		if token[:4] != "sdk-" {
			token = fmt.Sprintf("sdk-%s", token)
		}

		s := growthbook.SDKConnection{
			Organization: org.GetID(),
			Key:          token,
		}

		s.FromV1beta1(client)

		if err := growthbook.UpdateSDKConnection(ctx, s, db); err != nil {
			return instance, err
		}
	}

	return instance, nil
}

func (r *GrowthbookInstanceReconciler) getSecret(ctx context.Context, ref types.NamespacedName) (*corev1.Secret, error) {
	secret := &corev1.Secret{}
	err := r.Client.Get(ctx, ref, secret)

	if err != nil {
		return nil, fmt.Errorf("referencing secret was not found: %w", err)
	}

	return secret, nil
}

func (r *GrowthbookInstanceReconciler) getClientToken(ctx context.Context, client v1beta1.GrowthbookClient) (string, error) {
	if client.Spec.TokenSecret == nil {
		return "", errors.New("no secret reference provided")
	}

	secret, err := r.getSecret(ctx, types.NamespacedName{
		Namespace: client.Namespace,
		Name:      client.Spec.TokenSecret.Name,
	})

	if err != nil {
		return "", err
	}

	tokenFieldName := "token"
	if client.Spec.TokenSecret.TokenField != "" {
		tokenFieldName = client.Spec.TokenSecret.TokenField
	}

	if val, ok := secret.Data[tokenFieldName]; !ok {
		return "", errors.New("defined token field not found in secret")
	} else {
		return string(val), nil
	}
}

func (r *GrowthbookInstanceReconciler) getUsernamePassword(ctx context.Context, instance v1beta1.GrowthbookInstance, secretReference *v1beta1.SecretReference) (string, string, error) {
	if secretReference == nil {
		return "", "", errors.New("no secret reference provided")
	}

	secret, err := r.getSecret(ctx, types.NamespacedName{
		Namespace: instance.Namespace,
		Name:      secretReference.Name,
	})

	if err != nil {
		return "", "", err
	}

	var (
		user string
		pw   string
	)

	if val, ok := secret.Data[secretReference.UserField]; !ok {
		return "", "", errors.New("defined username field not found in secret")
	} else {
		user = string(val)
	}

	if val, ok := secret.Data[secretReference.PasswordField]; !ok {
		return "", "", errors.New("defined password field not found in secret")
	} else {
		pw = string(val)
	}

	return user, pw, nil
}

func (r *GrowthbookInstanceReconciler) getOptionalUsernamePassword(ctx context.Context, instance v1beta1.GrowthbookInstance, secretReference *v1beta1.SecretReference) (string, string, error) {
	if secretReference == nil {
		return "", "", errors.New("no secret reference provided")
	}

	secret, err := r.getSecret(ctx, types.NamespacedName{
		Namespace: instance.Namespace,
		Name:      secretReference.Name,
	})

	if err != nil {
		return "", "", err
	}

	var (
		user string
		pw   string
	)

	if val, ok := secret.Data[secretReference.UserField]; ok {
		user = string(val)
	}

	if val, ok := secret.Data[secretReference.PasswordField]; !ok {
		return "", "", errors.New("defined password field not found in secret")
	} else {
		pw = string(val)
	}

	return user, pw, nil
}

func (r *GrowthbookInstanceReconciler) patchStatus(ctx context.Context, instance *v1beta1.GrowthbookInstance) error {
	key := client.ObjectKeyFromObject(instance)
	latest := &v1beta1.GrowthbookInstance{}
	if err := r.Client.Get(ctx, key, latest); err != nil {
		return err
	}

	return r.Client.Status().Patch(ctx, instance, client.MergeFrom(latest))
}

// objectKey returns client.ObjectKey for the object.
func objectKey(object metav1.Object) client.ObjectKey {
	return client.ObjectKey{
		Namespace: object.GetNamespace(),
		Name:      object.GetName(),
	}
}

func matches(labels map[string]string, selector *metav1.LabelSelector) bool {
	if selector == nil {
		return true
	}

	for kS, vS := range selector.MatchLabels {
		var match bool
		for kL, vL := range selector.MatchLabels {
			if kS == kL && vS == vL {
				match = true
			}
		}

		if !match {
			return false
		}
	}

	return true
}
