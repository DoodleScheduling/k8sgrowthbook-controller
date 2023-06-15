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
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	ReadyCondition     = "Ready"
	SynchronizedReason = "Synchronized"
	ProgressingReason  = "Progressing"
	FailedReason       = "Failed"
)

// ConditionalResource is a resource with conditions
type conditionalResource interface {
	GetStatusConditions() *[]metav1.Condition
}

// setResourceCondition sets the given condition with the given status,
// reason and message on a resource.
func setResourceCondition(resource conditionalResource, condition string, status metav1.ConditionStatus, reason, message string) {
	conditions := resource.GetStatusConditions()

	newCondition := metav1.Condition{
		Type:    condition,
		Status:  status,
		Reason:  reason,
		Message: message,
	}

	apimeta.SetStatusCondition(conditions, newCondition)
}

// SecretReference is a named reference to a secret which contains user credentials
type SecretReference struct {
	// Name referrs to the name of the secret, must be located whithin the same namespace
	Name string `json:"name"`

	// +optional
	// +kubebuilder:default:=username
	UserField string `json:"userField,omitempty"`

	// +optional
	// +kubebuilder:default:=password
	PasswordField string `json:"passwordField,omitempty"`
}
