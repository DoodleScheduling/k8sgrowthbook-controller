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

// GrowthbookFeatureSpec defines the desired state of GrowthbookFeature
type GrowthbookFeatureSpec struct {
	ID           string           `json:"id,omitempty"`
	Description  string           `json:"description,omitempty"`
	Tags         []string         `json:"tags,omitempty"`
	DefaultValue string           `json:"defaultValue,omitempty"`
	ValueType    FeatureValueType `json:"valueType,omitempty"`
	// +kubebuilder:default:={{name: dev, enabled: true}}
	Environments []Environment `json:"environments,omitempty"`
}

type FeatureValueType string

var (
	FeatureValueTypeBoolean FeatureValueType = "boolean"
	FeatureValueTypeString  FeatureValueType = "string"
	FeatureValueTypeNumber  FeatureValueType = "number"
	FeatureValueTypeJSON    FeatureValueType = "json"
)

// GetID returns the feature ID which is the resource name if not overwritten by spec.ID
func (f *GrowthbookFeature) GetID() string {
	if f.Spec.ID == "" {
		return f.Name
	}

	return f.Spec.ID
}

// Environment defines a grothbook environment
type Environment struct {
	Name    string `json:"name,omitempty"`
	Enabled bool   `json:"enabled,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description=""

// GrowthbookFeature is the Schema for the GrowthbookFeatures API
type GrowthbookFeature struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec GrowthbookFeatureSpec `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true

// GrowthbookFeatureList contains a list of GrowthbookFeature
type GrowthbookFeatureList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GrowthbookFeature `json:"items"`
}

func init() {
	SchemeBuilder.Register(&GrowthbookFeature{}, &GrowthbookFeatureList{})
}
