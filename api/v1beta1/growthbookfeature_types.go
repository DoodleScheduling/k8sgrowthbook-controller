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

// +kubebuilder:validation:Enum=boolean;string;number;json
type FeatureValueType string

var (
	FeatureValueTypeBoolean FeatureValueType = "boolean"
	FeatureValueTypeString  FeatureValueType = "string"
	FeatureValueTypeNumber  FeatureValueType = "number"
	FeatureValueTypeJSON    FeatureValueType = "json"
)

// +kubebuilder:validation:Enum=all;none;any
type SavedGroupTargetingMatch string

var (
	SavedGroupTargetingMatchAll  SavedGroupTargetingMatch = "all"
	SavedGroupTargetingMatchNone SavedGroupTargetingMatch = "none"
	SavedGroupTargetingMatchAny  SavedGroupTargetingMatch = "any"
)

// +kubebuilder:validation:Enum=force;rollout;experiment
type FeatureRuleType string

var (
	FeatureRuleTypeForce      FeatureRuleType = "force"
	FeatureRuleTypeRollout    FeatureRuleType = "rollout"
	FeatureRuleTypeExperiment FeatureRuleType = "experiment"
)

type FeatureRule struct {
	ID                     string                `json:"id,omitempty"`
	Type                   FeatureRuleType       `json:"type,omitempty"`
	Description            string                `json:"description,omitempty"`
	Condition              string                `json:"condition,omitempty"`
	Enabled                bool                  `json:"enabled,omitempty"`
	ScheduleRules          []ScheduleRule        `json:"scheduleRules,omitempty"`
	SavedGroups            []SavedGroupTargeting `json:"savedGroups,omitempty"`
	Prerequisites          []FeaturePrerequisite `json:"prerequisites,omitempty"`
	Value                  string                `json:"value,omitempty"`
	Coverage               string                `json:"coverage,omitempty"`
	HashAttribute          string                `json:"hashAttribute,omitempty"`
	TrackingKey            string                `json:"trackingKey,omitempty"`
	FallbackAttribute      *string               `json:"fallbackAttribute,omitempty"`
	DisableStickyBucketing *bool                 `json:"disableStickyBucketing,omitempty"`
	BucketVersion          *int64                `json:"bucketVersion,omitempty"`
	MinBucketVersion       *int64                `json:"minBucketVersion,omitempty"`
	Namespace              *NamespaceValue       `json:"namespace,omitempty"`
	Values                 []ExperimentValue     `json:"values,omitempty"`
}

type FeaturePrerequisite struct {
	ID        string `json:"id,omitempty"`
	Condition string `json:"condition,omitempty"`
}

type ScheduleRule struct {
	Timestamp string `json:"timestamp,omitempty"`
	Enabled   bool   `json:"enabled,omitempty"`
}

type SavedGroupTargeting struct {
	Match SavedGroupTargetingMatch `json:"match,omitempty"`
	IDs   []string                 `json:"ids,omitempty"`
}

type ExperimentValue struct {
	Value  string  `json:"value,omitempty"`
	Weight int64   `json:"weight,omitempty"`
	Name   *string `json:"name,omitempty"`
}

type NamespaceValue struct {
	Enabled bool    `json:"enabled,omitempty"`
	Name    string  `json:"name,omitempty"`
	Range   []int64 `json:"range,omitempty"`
}

// GetID returns the feature ID which is the resource name if not overwritten by spec.ID
func (f *GrowthbookFeature) GetID() string {
	if f.Spec.ID == "" {
		return f.Name
	}

	return f.Spec.ID
}

// Environment defines a grothbook environment
type Environment struct {
	Name    string        `json:"name,omitempty"`
	Enabled bool          `json:"enabled,omitempty"`
	Rules   []FeatureRule `json:"rules,omitempty"`
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
