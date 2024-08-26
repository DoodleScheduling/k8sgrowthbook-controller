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
	"golang.org/x/exp/slices"
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
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1beta1 "github.com/DoodleScheduling/growthbook-controller/api/v1beta1"
	"github.com/DoodleScheduling/growthbook-controller/internal/growthbook"
	"github.com/DoodleScheduling/growthbook-controller/internal/storage"
	"github.com/DoodleScheduling/growthbook-controller/internal/storage/mongodb"
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
	owner          = "growthbook-controller"
)

// MongoDBProvider returns a storage.Database for MongoDB
func MongoDBProvider(ctx context.Context, instance v1beta1.GrowthbookInstance, username, password string) (storage.Disconnector, storage.Database, error) {
	opts := options.Client().ApplyURI(instance.Spec.MongoDB.URI)
	if username != "" || password != "" {
		opts.SetAuth(options.Credential{
			Username: username,
			Password: password,
		})
	}

	opts.SetAppName("growthbook-controller")
	mongoClient, err := mongo.Connect(ctx, opts)
	if err != nil {
		return nil, nil, fmt.Errorf("failed connecting to mongodb: %w", err)
	}

	u, err := url.Parse(instance.Spec.MongoDB.URI)
	if err != nil {
		return nil, nil, err
	}

	dbName := strings.TrimLeft(u.Path, "/")
	db := mongoClient.Database(dbName)

	return mongoClient, mongodb.New(db), nil
}

// GrowthbookInstance reconciles a GrowthbookInstance object
type GrowthbookInstanceReconciler struct {
	client.Client
	Log              logr.Logger
	Scheme           *runtime.Scheme
	Recorder         record.EventRecorder
	DatabaseProvider func(ctx context.Context, instance v1beta1.GrowthbookInstance, username, password string) (storage.Disconnector, storage.Database, error)
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
			&corev1.Secret{},
			handler.EnqueueRequestsFromMapFunc(r.requestsForChangeByField(secretIndexKey)),
		).
		Watches(
			&v1beta1.GrowthbookUser{},
			handler.EnqueueRequestsFromMapFunc(r.requestsForChangeBySelector),
		).
		Watches(
			&v1beta1.GrowthbookOrganization{},
			handler.EnqueueRequestsFromMapFunc(r.requestsForChangeBySelector),
		).
		Watches(
			&v1beta1.GrowthbookClient{},
			handler.EnqueueRequestsFromMapFunc(r.requestsForChangeBySelector),
		).
		Watches(
			&v1beta1.GrowthbookFeature{},
			handler.EnqueueRequestsFromMapFunc(r.requestsForChangeBySelector),
		).
		WithOptions(controller.Options{MaxConcurrentReconciles: opts.MaxConcurrentReconciles}).
		Complete(r)
}

func (r *GrowthbookInstanceReconciler) requestsForChangeByField(field string) handler.MapFunc {
	return func(ctx context.Context, o client.Object) []reconcile.Request {
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

func (r *GrowthbookInstanceReconciler) requestsForChangeBySelector(ctx context.Context, o client.Object) []reconcile.Request {
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

	// examine DeletionTimestamp to determine if object is under deletion
	if err := r.addFinalizer(ctx, v1beta1.Finalizer, metav1.PartialObjectMetadata{TypeMeta: instance.TypeMeta, ObjectMeta: instance.ObjectMeta}); err != nil {
		return ctrl.Result{}, err
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
		if !instance.DeletionTimestamp.IsZero() {
			if err := r.removeFinalizer(ctx, v1beta1.Finalizer, metav1.PartialObjectMetadata{TypeMeta: instance.TypeMeta, ObjectMeta: instance.ObjectMeta}); err != nil {
				return res, err
			} else {
				return ctrl.Result{}, nil
			}
		}

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
	//TODO there is a test race condition with this one, leaving for now
	/*msg := "reconcile instance progressing"
	r.Recorder.Event(&instance, "Normal", "info", msg)
	instance = v1beta1.GrowthbookInstanceNotReady(instance, v1beta1.ProgressingReason, msg)
	if err := r.patchStatus(ctx, &instance); err != nil {
		return instance, err
	}*/

	var err error
	var usr, pw string
	if instance.Spec.MongoDB.Secret != nil {
		usr, pw, err = r.getUsernamePassword(ctx, instance, instance.Spec.MongoDB.Secret)
		if err != nil {
			return instance, err
		}
	}

	disconnector, db, err := r.DatabaseProvider(ctx, instance, usr, pw)
	if err != nil {
		return instance, err
	}

	defer func() {
		ctx, cancel := context.WithTimeout(context.TODO(), time.Second*10)
		defer cancel()
		if err := disconnector.Disconnect(ctx); err != nil {
			logger.Error(err, "failed disconnecting mongodb")
		}
	}()

	instance.Status.SubResourceCatalog = []v1beta1.ResourceReference{}

	instance, err = r.reconcileUsers(ctx, instance, db)
	if err != nil {
		return instance, fmt.Errorf("failed reconciling users: %w", err)
	}

	instance, orgs, err := r.reconcileOrganizations(ctx, instance, db)
	if err != nil {
		return instance, fmt.Errorf("failed reconciling organizations: %w", err)
	}

	for _, org := range orgs {
		instance, err = r.reconcileFeatures(ctx, instance, org, db)
		if err != nil {
			return instance, fmt.Errorf("failed reconciling features: %w", err)
		}

		instance, err = r.reconcileClients(ctx, instance, org, db)
		if err != nil {
			return instance, fmt.Errorf("failed reconciling clients: %w", err)
		}
	}

	return instance, err
}

func (r *GrowthbookInstanceReconciler) reconcileOrganizations(ctx context.Context, instance v1beta1.GrowthbookInstance, db storage.Database) (v1beta1.GrowthbookInstance, []v1beta1.GrowthbookOrganization, error) {
	var orgs v1beta1.GrowthbookOrganizationList
	finalizerName := fmt.Sprintf("%s/%s.%s", v1beta1.Finalizer, instance.Name, instance.Namespace)

	selector, err := metav1.LabelSelectorAsSelector(instance.Spec.ResourceSelector)
	if err != nil {
		return instance, nil, err
	}

	err = r.Client.List(ctx, &orgs, client.InNamespace(instance.Namespace), client.MatchingLabelsSelector{Selector: selector})
	if err != nil {
		return instance, nil, err
	}

	if instance.DeletionTimestamp.IsZero() {
		for _, org := range orgs.Items {
			if err := r.addFinalizer(ctx, finalizerName, metav1.PartialObjectMetadata{TypeMeta: org.TypeMeta, ObjectMeta: org.ObjectMeta}); err != nil {
				return instance, nil, err
			}

			if org.DeletionTimestamp.IsZero() {
				instance = updateResourceCatalog(instance, &org)
			}
		}
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

		if org.DeletionTimestamp.IsZero() && instance.DeletionTimestamp.IsZero() {
			if err := growthbook.UpdateOrganization(ctx, o, db); err != nil {
				return instance, nil, err
			}
		} else {
			if instance.Spec.Prune {
				if err := growthbook.DeleteOrganization(ctx, o, db); err != nil {
					return instance, nil, err
				}
			}

			if err := r.removeFinalizer(ctx, finalizerName, metav1.PartialObjectMetadata{TypeMeta: org.TypeMeta, ObjectMeta: org.ObjectMeta}); err != nil {
				return instance, nil, err
			}
		}
	}

	return instance, orgs.Items, nil
}

func (r *GrowthbookInstanceReconciler) reconcileFeatures(ctx context.Context, instance v1beta1.GrowthbookInstance, org v1beta1.GrowthbookOrganization, db storage.Database) (v1beta1.GrowthbookInstance, error) {
	var features v1beta1.GrowthbookFeatureList
	selector, err := metav1.LabelSelectorAsSelector(org.Spec.ResourceSelector)
	if err != nil {
		return instance, err
	}

	finalizerName := fmt.Sprintf("%s/%s.%s", v1beta1.Finalizer, instance.Name, instance.Namespace)
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

	if instance.DeletionTimestamp.IsZero() {
		for _, feature := range features.Items {
			if err := r.addFinalizer(ctx, finalizerName, metav1.PartialObjectMetadata{TypeMeta: feature.TypeMeta, ObjectMeta: feature.ObjectMeta}); err != nil {
				return instance, err
			}

			if feature.DeletionTimestamp.IsZero() {
				instance = updateResourceCatalog(instance, &feature)
			}
		}
	}

	for _, feature := range features.Items {
		f := growthbook.Feature{
			Owner:        owner,
			Organization: org.GetID(),
		}

		f.FromV1beta1(feature)

		if feature.DeletionTimestamp.IsZero() && instance.DeletionTimestamp.IsZero() {
			if err := growthbook.UpdateFeature(ctx, f, db); err != nil {
				return instance, err
			}
		} else {
			if instance.Spec.Prune {
				if err := growthbook.DeleteFeature(ctx, f, db); err != nil {
					return instance, err
				}
			}

			if err := r.removeFinalizer(ctx, finalizerName, metav1.PartialObjectMetadata{TypeMeta: feature.TypeMeta, ObjectMeta: feature.ObjectMeta}); err != nil {
				return instance, err
			}
		}
	}

	return instance, nil
}

func (r *GrowthbookInstanceReconciler) addFinalizer(ctx context.Context, finalizerName string, obj metav1.PartialObjectMetadata) error {
	if !obj.GetDeletionTimestamp().IsZero() {
		return nil
	}

	controllerutil.AddFinalizer(&obj, finalizerName)
	if err := r.patch(ctx, &obj); err != nil {
		return err
	}

	return nil
}

func (r *GrowthbookInstanceReconciler) removeFinalizer(ctx context.Context, finalizerName string, obj metav1.PartialObjectMetadata) error {
	controllerutil.RemoveFinalizer(&obj, finalizerName)
	if err := r.patch(ctx, &obj); err != nil {
		return err
	}

	return nil
}

func (r *GrowthbookInstanceReconciler) reconcileUsers(ctx context.Context, instance v1beta1.GrowthbookInstance, db storage.Database) (v1beta1.GrowthbookInstance, error) {
	var users v1beta1.GrowthbookUserList
	finalizerName := fmt.Sprintf("%s/%s.%s", v1beta1.Finalizer, instance.Name, instance.Namespace)

	selector, err := metav1.LabelSelectorAsSelector(instance.Spec.ResourceSelector)
	if err != nil {
		return instance, err
	}

	err = r.Client.List(ctx, &users, client.InNamespace(instance.Namespace), client.MatchingLabelsSelector{Selector: selector})
	if err != nil {
		return instance, err
	}

	if instance.DeletionTimestamp.IsZero() {
		for _, user := range users.Items {
			if err := r.addFinalizer(ctx, finalizerName, metav1.PartialObjectMetadata{TypeMeta: user.TypeMeta, ObjectMeta: user.ObjectMeta}); err != nil {
				return instance, err
			}

			if user.DeletionTimestamp.IsZero() {
				instance = updateResourceCatalog(instance, &user)
			}
		}
	}

	for _, user := range users.Items {
		u := growthbook.User{}
		u.FromV1beta1(user)

		if user.DeletionTimestamp.IsZero() && instance.DeletionTimestamp.IsZero() {
			username, password, err := r.getOptionalUsernamePassword(ctx, instance, user.Spec.Secret)
			if err != nil {
				return instance, err
			}

			if username != "" {
				u.Name = username
			}

			if err := u.SetPassword(ctx, db, password); err != nil {
				return instance, err
			}

			if err := growthbook.UpdateUser(ctx, u, db); err != nil {
				return instance, err
			}
		} else {
			if instance.Spec.Prune {
				if err := growthbook.DeleteUser(ctx, u, db); err != nil {
					return instance, err
				}
			}

			if err := r.removeFinalizer(ctx, finalizerName, metav1.PartialObjectMetadata{TypeMeta: user.TypeMeta, ObjectMeta: user.ObjectMeta}); err != nil {
				return instance, err
			}
		}
	}

	return instance, nil
}

func (r *GrowthbookInstanceReconciler) reconcileClients(ctx context.Context, instance v1beta1.GrowthbookInstance, org v1beta1.GrowthbookOrganization, db storage.Database) (v1beta1.GrowthbookInstance, error) {
	var clients v1beta1.GrowthbookClientList
	finalizerName := fmt.Sprintf("%s/%s.%s", v1beta1.Finalizer, instance.Name, instance.Namespace)

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

	if instance.DeletionTimestamp.IsZero() {
		for _, client := range clients.Items {
			if err := r.addFinalizer(ctx, finalizerName, metav1.PartialObjectMetadata{TypeMeta: client.TypeMeta, ObjectMeta: client.ObjectMeta}); err != nil {
				return instance, err
			}

			if client.DeletionTimestamp.IsZero() {
				instance = updateResourceCatalog(instance, &client)
			}
		}
	}

	for _, client := range clients.Items {
		s := growthbook.SDKConnection{
			Organization: org.GetID(),
		}

		s.FromV1beta1(client)

		if client.DeletionTimestamp.IsZero() && instance.DeletionTimestamp.IsZero() {
			token, err := r.getClientToken(ctx, client)
			if err != nil {
				return instance, err
			}

			if token[:4] != "sdk-" {
				token = fmt.Sprintf("sdk-%s", token)
			}

			s.Key = token

			if err := growthbook.UpdateSDKConnection(ctx, s, db); err != nil {
				return instance, err
			}
		} else {
			if instance.Spec.Prune {
				if err := growthbook.DeleteSDKConnection(ctx, s, db); err != nil {
					return instance, err
				}
			}

			if err := r.removeFinalizer(ctx, finalizerName, metav1.PartialObjectMetadata{TypeMeta: client.TypeMeta, ObjectMeta: client.ObjectMeta}); err != nil {
				return instance, err
			}
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

func updateResourceCatalog(instance v1beta1.GrowthbookInstance, resource client.Object) v1beta1.GrowthbookInstance {
	resRef := v1beta1.ResourceReference{
		Kind:       resource.GetObjectKind().GroupVersionKind().Kind,
		Name:       resource.GetName(),
		APIVersion: fmt.Sprintf("%s/%s", resource.GetObjectKind().GroupVersionKind().Group, resource.GetObjectKind().GroupVersionKind().Version),
	}

	if !slices.Contains(instance.Status.SubResourceCatalog, resRef) {
		instance.Status.SubResourceCatalog = append(instance.Status.SubResourceCatalog, resRef)
	}

	return instance
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

func (r *GrowthbookInstanceReconciler) patch(ctx context.Context, obj *metav1.PartialObjectMetadata) error {
	key := client.ObjectKeyFromObject(obj)
	latest := &metav1.PartialObjectMetadata{
		TypeMeta: obj.TypeMeta,
	}

	if err := r.Client.Get(ctx, key, latest); err != nil {
		return err
	}

	return r.Client.Patch(ctx, obj, client.MergeFrom(latest))
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
		for kL, vL := range labels {
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
