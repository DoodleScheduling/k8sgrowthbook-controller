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
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GrowthbookClientSpec defines the desired state of GrowthbookClient
type GrowthbookClientSpec struct {
	Languages                []string             `json:"languages,omitempty"`
	Name                     string               `json:"name,omitempty"`
	Environment              string               `json:"environment,omitempty"`
	EncryptPayload           bool                 `json:"encryptPayload,omitempty"`
	Project                  string               `json:"project,omitempty"`
	IncludeVisualExperiments bool                 `json:"includeVisualExperiments,omitempty"`
	IncludeDraftExperiments  bool                 `json:"includeDraftExperiments,omitempty"`
	IncludeExperimentNames   bool                 `json:"includeExperimentNames,omitempty"`
	ID                       string               `json:"id,omitempty"`
	TokenSecret              TokenSecretReference `json:"tokenSecret,omitempty"`
}

// GetID returns the client ID which is the resource name if not overwritten by spec.ID
func (c *GrowthbookClient) GetID() string {
	if c.Spec.ID == "" {
		return fmt.Sprintf("%s-%s", c.Name, c.Namespace)
	}

	return c.Spec.ID
}

// GetName returns the client name which is the resource name if not overwritten by spec.Name
func (c *GrowthbookClient) GetName() string {
	if c.Spec.Name == "" {
		return c.Name
	}

	return c.Spec.ID
}

// SecretReference is a named reference to a secret which contains user credentials
type TokenSecretReference struct {
	// Name referrs to the name of the secret, must be located whithin the same namespace
	// +required
	Name string `json:"name,omitempty"`

	// +optional
	// +kubebuilder:default:=token
	TokenField string `json:"tokenField,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description=""

// GrowthbookClient is the Schema for the GrowthbookClients API
type GrowthbookClient struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec GrowthbookClientSpec `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true

// GrowthbookClientList contains a list of GrowthbookClient
type GrowthbookClientList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GrowthbookClient `json:"items"`
}

func init() {
	SchemeBuilder.Register(&GrowthbookClient{}, &GrowthbookClientList{})
}
