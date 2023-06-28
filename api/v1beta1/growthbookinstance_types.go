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

package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GrowthbookInstanceSpec defines the desired state of GrowthbookInstance
type GrowthbookInstanceSpec struct {
	// MongoDB settings
	MongoDB GrowthbookInstanceMongoDB `json:"mongodb,omitempty"`

	// Interval reconciliation
	Interval *metav1.Duration `json:"interval,omitempty"`

	// Prune
	// +kubebuilder:validation:Required"
	Prune bool `json:"prune"`

	// Timeout while reconciling the instance
	// +kubebuilder:default:="5m"
	Timeout *metav1.Duration `json:"timeout,omitempty"`

	// Suspend reconciliation
	Suspend bool `json:"suspend,omitempty"`

	// ResourceSelector defines a selector to select Growthbook resources associated with this instance
	ResourceSelector *metav1.LabelSelector `json:"resourceSelector,omitempty"`
}

// GrowthbookInstanceMongoDB defines how to connect to the growthbook MongoDB
type GrowthbookInstanceMongoDB struct {
	// Address is a MongoDB comptaible URI `mongodb://xxx`
	URI string `json:"uri,omitempty"`

	// Secret is a secret refernece with the MongoDB credentials
	Secret *SecretReference `json:"rootSecret,omitempty"`
}

// GrowthbookInstanceStatus defines the observed state of GrowthbookInstance
type GrowthbookInstanceStatus struct {
	// Conditions holds the conditions for the KeycloakRealm.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// ObservedGeneration is the last generation reconciled by the controller
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// LastReconcileDuration is the total time the reconcile of the realm took
	LastReconcileDuration metav1.Duration `json:"lastReconcileDuration,omitempty"`

	// SubResourceCatalog holds references to all sub resources including GrowthbookFeature and GrowthbookClient associated with this instance
	SubResourceCatalog []ResourceReference `json:"subResourceCatalog,omitempty"`
}

// ResourceReference metadata to lookup another resource
type ResourceReference struct {
	Kind       string `json:"kind,omitempty"`
	Name       string `json:"name,omitempty"`
	APIVersion string `json:"apiVersion,omitempty"`
}

// GrowthbookInstanceNotReady
func GrowthbookInstanceNotReady(clone GrowthbookInstance, reason, message string) GrowthbookInstance {
	setResourceCondition(&clone, ReadyCondition, metav1.ConditionFalse, reason, message)
	return clone
}

// GrowthbookInstanceReady
func GrowthbookInstanceReady(clone GrowthbookInstance, reason, message string) GrowthbookInstance {
	setResourceCondition(&clone, ReadyCondition, metav1.ConditionTrue, reason, message)
	return clone
}

// GetStatusConditions returns a pointer to the Status.Conditions slice
func (in *GrowthbookInstance) GetStatusConditions() *[]metav1.Condition {
	return &in.Status.Conditions
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].status",description=""
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].message",description=""
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description=""

// GrowthbookInstance is the Schema for the GrowthbookInstances API
type GrowthbookInstance struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GrowthbookInstanceSpec   `json:"spec,omitempty"`
	Status GrowthbookInstanceStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// GrowthbookInstanceList contains a list of GrowthbookInstance
type GrowthbookInstanceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GrowthbookInstance `json:"items"`
}

func init() {
	SchemeBuilder.Register(&GrowthbookInstance{}, &GrowthbookInstanceList{})
}
