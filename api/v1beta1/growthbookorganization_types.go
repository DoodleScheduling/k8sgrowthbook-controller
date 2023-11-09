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

// GrowthbookOrganizationSpec defines the desired state of GrowthbookOrganization
type GrowthbookOrganizationSpec struct {
	Name       string `json:"name,omitempty"`
	ID         string `json:"id,omitempty"`
	OwnerEmail string `json:"ownerEmail,omitempty"`

	// Users defines a selector and a role which should be assigned to an organization
	Users []*GrowthbookOrganizationUser `json:"users,omitempty"`

	// ResourceSelector defines a selector to select Growthbook resources associated with this organization
	ResourceSelector *metav1.LabelSelector `json:"resourceSelector,omitempty"`
}

// GrowthbookOrganizationUser defines which users are assigned to what organization with what role
type GrowthbookOrganizationUser struct {
	Selector *metav1.LabelSelector `json:"selector,omitempty"`
	Role     string                `json:"role,omitempty"`
}

// GetID returns the organization ID which is the resource name if not overwritten by spec.ID
func (o *GrowthbookOrganization) GetID() string {
	if o.Spec.ID == "" {
		return o.Name
	}

	return o.Spec.ID
}

// GetName returns the organization name which is the resource name if not overwritten by spec.Name
func (o *GrowthbookOrganization) GetName() string {
	if o.Spec.Name == "" {
		return o.Name
	}

	return o.Spec.Name
}

// +kubebuilder:object:root=true
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description=""

// GrowthbookOrganization is the Schema for the GrowthbookOrganizations API
type GrowthbookOrganization struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec GrowthbookOrganizationSpec `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true

// GrowthbookOrganizationList contains a list of GrowthbookOrganization
type GrowthbookOrganizationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GrowthbookOrganization `json:"items"`
}

func init() {
	SchemeBuilder.Register(&GrowthbookOrganization{}, &GrowthbookOrganizationList{})
}
