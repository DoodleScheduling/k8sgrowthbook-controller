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

// GrowthbookUserSpec defines the desired state of GrowthbookUser
type GrowthbookUserSpec struct {
	Name  string `json:"name,omitempty"`
	ID    string `json:"id,omitempty"`
	Email string `json:"email,omitempty"`

	// Secret is a secret reference to a secret containing the users password
	Secret *SecretReference `json:"secret"`
}

// GetID returns the organization ID which is the resource name if not overwritten by spec.ID
func (o *GrowthbookUser) GetID() string {
	if o.Spec.ID == "" {
		return o.Name
	}

	return o.Spec.ID
}

// GetName returns the organization name which is the resource name if not overwritten by spec.Name
func (o *GrowthbookUser) GetName() string {
	if o.Spec.Name == "" {
		return o.Name
	}

	return o.Spec.ID
}

// +kubebuilder:object:root=true
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description=""

// GrowthbookUser is the Schema for the GrowthbookUsers API
type GrowthbookUser struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec GrowthbookUserSpec `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true

// GrowthbookUserList contains a list of GrowthbookUser
type GrowthbookUserList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GrowthbookUser `json:"items"`
}

func init() {
	SchemeBuilder.Register(&GrowthbookUser{}, &GrowthbookUserList{})
}
