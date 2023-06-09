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
)

// +kubebuilder:rbac:groups=growthbook.infra.doodle.com,resources=growthbookinstances,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=growthbook.infra.doodle.com,resources=growthbookinstances/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=growthbook.infra.doodle.com,resources=growthbookfeatures,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=growthbook.infra.doodle.com,resources=growthbookfeatures/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=growthbook.infra.doodle.com,resources=growthbookclients,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=growthbook.infra.doodle.com,resources=growthbookclients/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

const (
	secretIndexKey = ".metadata.secret"
)

// GrowthbookInstance reconciles a GrowthbookInstance object
type GrowthbookInstanceReconciler struct {
	client.Client
	Log      logr.Logger
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

type GrowthbookInstanceReconcilerOptions struct {
	MaxConcurrentReconciles int
}

// SetupWithManager adding controllers
func (r *GrowthbookInstanceReconciler) SetupWithManager(mgr ctrl.Manager, opts GrowthbookInstanceReconcilerOptions) error {
	// Index the GrowthbookInstance by the Secret references they point at
	/*if err := mgr.GetFieldIndexer().IndexField(context.TODO(), &v1beta1.GrowthbookInstance{}, secretIndexKey,
		func(o client.Object) []string {
			// The referenced admin secret gets indexed
			instance := o.(*v1beta1.GrowthbookInstance)
			keys := []string{
				fmt.Sprintf("%s/%s", instance.GetNamespace(), instance.Spec.AuthSecret.Name),
			}

			// As well as an attempt to index all field secret references
			b, err := json.Marshal(instance.Spec.instance)
			if err != nil {
				//TODO error handling
				return keys
			}

			results := r.secretRegex.FindAllSubmatch(b, -1)
			for _, result := range results {
				if len(result) > 1 {
					keys = append(keys, fmt.Sprintf("%s/%s", instance.GetNamespace(), string(result[1])))
				}
			}

			return keys
		},
	); err != nil {
		return err
	}*/

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1beta1.GrowthbookInstance{}, builder.WithPredicates(
			predicate.GenerationChangedPredicate{},
		)).
		Watches(
			&source.Kind{Type: &corev1.Secret{}},
			handler.EnqueueRequestsFromMapFunc(r.requestsForSecretChange),
		).
		Watches(
			&source.Kind{Type: &v1beta1.GrowthbookFeature{}},
			handler.EnqueueRequestsFromMapFunc(r.requestsForGrowthbookFeatureChange),
		).
		Watches(
			&source.Kind{Type: &v1beta1.GrowthbookClient{}},
			handler.EnqueueRequestsFromMapFunc(r.requestsForGrowthbookClientChange),
		).
		WithOptions(controller.Options{MaxConcurrentReconciles: opts.MaxConcurrentReconciles}).
		Complete(r)
}

func (r *GrowthbookInstanceReconciler) requestsForSecretChange(o client.Object) []reconcile.Request {
	sectet, ok := o.(*corev1.Secret)
	if !ok {
		panic(fmt.Sprintf("expected a Secret, got %T", o))
	}

	ctx := context.Background()
	var list v1beta1.GrowthbookInstanceList
	if err := r.List(ctx, &list, client.MatchingFields{
		secretIndexKey: objectKey(sectet).String(),
	}); err != nil {
		return nil
	}

	var reqs []reconcile.Request
	for _, instance := range list.Items {
		r.Log.Info("referenced secret from a GrowthbookInstance changed detected", "namespace", instance.GetNamespace(), "instance-name", instance.GetName())
		reqs = append(reqs, reconcile.Request{NamespacedName: objectKey(&instance)})
	}

	return reqs
}

func (r *GrowthbookInstanceReconciler) requestsForGrowthbookFeatureChange(o client.Object) []reconcile.Request {
	_, ok := o.(*v1beta1.GrowthbookFeature)
	if !ok {
		panic(fmt.Sprintf("expected a GrowthbookFeature, got %T", o))
	}

	ctx := context.Background()
	var list v1beta1.GrowthbookInstanceList
	if err := r.List(ctx, &list); err != nil {
		return nil
	}

	var reqs []reconcile.Request
	/*for _, instance := range list.Items {
		if matches(instance.Labels, client.Spec.instanceSelector) {
			r.Log.Info("change of growthbook client referencing instance detected", "namespace", instance.GetNamespace(), "instance", instance.GetName(), "feature-name", feature.GetName())
			reqs = append(reqs, reconcile.Request{NamespacedName: objectKey(&instance)})
		}
	}*/

	return reqs
}

func (r *GrowthbookInstanceReconciler) requestsForGrowthbookClientChange(o client.Object) []reconcile.Request {
	_, ok := o.(*v1beta1.GrowthbookClient)
	if !ok {
		panic(fmt.Sprintf("expected a GrowthbookClient, got %T", o))
	}

	ctx := context.Background()
	var list v1beta1.GrowthbookInstanceList
	if err := r.List(ctx, &list); err != nil {
		return nil
	}

	var reqs []reconcile.Request
	/*for _, instance := range list.Items {
		if matches(instance.Labels, client.Spec.instanceSelector) {
			r.Log.Info("change of growthbook client referencing instance detected", "namespace", instance.GetNamespace(), "instance", instance.GetName(), "feature-name", feature.GetName())
			reqs = append(reqs, reconcile.Request{NamespacedName: objectKey(&instance)})
		}
	}*/

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
	instance, err = r.reconcile(ctx, instance, logger)
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
		usr, pw, err = getSecret(ctx, r.Client, instance)
		if err != nil {
			return instance, err
		}
	}

	opts := options.Client().ApplyURI(instance.Spec.MongoDB.URI)
	if usr != "" || pw != "" {
		opts.SetAuth(options.Credential{
			Username: usr,
			Password: pw,
		})
	}

	mongoClient, err := mongo.NewClient(opts)
	if err != nil {
		return instance, err
	}

	db := mongoClient.Database("s")

	instance.Status.SubResourceCatalog = []v1beta1.ResourceReference{}

	instance, err = r.reconcileFeatures(ctx, instance, db, logger)
	if err != nil {
		return instance, err
	}

	/*instance, err = r.reconcileClients(ctx, instance, db, logger)
	if err != nil {
		return instance, err
	}*/

	return instance, err
}

func (r *GrowthbookInstanceReconciler) reconcileFeatures(ctx context.Context, instance v1beta1.GrowthbookInstance, db *mongo.Database, logger logr.Logger) (v1beta1.GrowthbookInstance, error) {
	var features v1beta1.GrowthbookFeatureList
	selector, err := metav1.LabelSelectorAsSelector(instance.Spec.ResourceSelector)
	if err != nil {
		return instance, err
	}

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
		f := growthbook.Feature{}
		f.FromV1beta1(feature)

		if err := growthbook.UpdateFeature(ctx, f, db); err != nil {
			return instance, err
		}
	}

	return instance, nil
}

/*
func (r *GrowthbookInstanceReconciler) reconcileClients(ctx context.Context, instance v1beta1.GrowthbookInstance, db *mongo.Database, logger logr.Logger) (v1beta1.GrowthbookInstance, error) {
	var clients v1beta1.GrowthbookClientList
	selector, err := metav1.LabelSelectorAsSelector(instance.Spec.ResourceSelector)
	if err != nil {
		return instance, err
	}

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
		if err := r.reconcileFeature(ctx, client, db); err != nil {
			//log only
		}
	}

	return instance, nil
}*/
/*func (r *GrowthbookInstanceReconciler) reconcileClient(ctx context.Context, client v1beta1.GrowthbookClient, db *mongo.Database) (v1beta1.GrowthbookInstance, error) {
}*/

func getSecret(ctx context.Context, c client.Client, instance v1beta1.GrowthbookInstance) (string, string, error) {
	// Fetch referencing root secret
	secret := &corev1.Secret{}
	secretName := types.NamespacedName{
		Namespace: instance.Namespace,
		Name:      instance.Spec.MongoDB.Secret.Name,
	}
	err := c.Get(ctx, secretName, secret)

	// Failed to fetch referenced secret, requeue immediately
	if err != nil {
		return "", "", fmt.Errorf("referencing secret was not found: %w", err)
	}

	usr, pw, err := extractCredentials(instance.Spec.MongoDB.Secret, secret)
	if err != nil {
		return usr, pw, fmt.Errorf("credentials field not found in referenced rootSecret: %w", err)
	}

	return usr, pw, err
}

func extractCredentials(credentials *v1beta1.SecretReference, secret *corev1.Secret) (string, string, error) {
	var (
		user string
		pw   string
	)

	if val, ok := secret.Data[credentials.UserField]; !ok {
		return "", "", errors.New("defined username field not found in secret")
	} else {
		user = string(val)
	}

	if val, ok := secret.Data[credentials.PasswordField]; !ok {
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
