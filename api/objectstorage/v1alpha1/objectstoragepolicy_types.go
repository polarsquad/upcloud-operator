/*
Copyright 2026 Polar Squad.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/polarsquad/upcloud-operator/api/common"
)

// ObjectStoragePolicySpec defines the desired state of an object storage
// policy (an S3-style IAM policy document).
type ObjectStoragePolicySpec struct {
	// ServiceRef points at the ManagedObjectStorage the policy belongs to.
	// Immutable.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="serviceRef is immutable"
	ServiceRef common.LocalObjectReference `json:"serviceRef"`
	// Name in UpCloud. Defaults to metadata.name. Immutable.
	// +kubebuilder:validation:MaxLength=64
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="name is immutable"
	Name string `json:"name,omitempty"`
	// Description of the policy.
	// +kubebuilder:validation:MaxLength=255
	Description string `json:"description,omitempty"`
	// Document is the policy JSON, sent to UpCloud as a raw string.
	// Immutable in v0.1 (the API only offers versioning).
	// +kubebuilder:validation:XValidation:rule="self.size() > 2",message="document must be a non-empty JSON string"
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="document is immutable in v0.1"
	Document string `json:"document"`
	// +kubebuilder:default=Delete
	DeletionPolicy common.DeletionPolicy `json:"deletionPolicy,omitempty"`
}

// ObjectStoragePolicyStatus defines the observed state of ObjectStoragePolicy.
type ObjectStoragePolicyStatus struct {
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// ServiceUUID of the owning service.
	ServiceUUID string `json:"serviceUUID,omitempty"`
	// Name as stored in UpCloud.
	Name string `json:"name,omitempty"`
	// ARN of the policy in UpCloud.
	ARN string `json:"arn,omitempty"`
	// DefaultVersionID reported by the API.
	DefaultVersionID string `json:"defaultVersionID,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Policy",type=string,JSONPath=`.status.name`
// +kubebuilder:printcolumn:name="ARN",type=string,JSONPath=`.status.arn`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// ObjectStoragePolicy is an S3-style policy document attached to an
// object storage service.
type ObjectStoragePolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ObjectStoragePolicySpec   `json:"spec"`
	Status ObjectStoragePolicyStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ObjectStoragePolicyList contains a list of ObjectStoragePolicy.
type ObjectStoragePolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ObjectStoragePolicy `json:"items"`
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(SchemeGroupVersion, &ObjectStoragePolicy{}, &ObjectStoragePolicyList{})
		return nil
	})
}

// GetConditions implements reconciler.Object.
func (p *ObjectStoragePolicy) GetConditions() []metav1.Condition { return p.Status.Conditions }

// SetConditions implements reconciler.Object.
func (p *ObjectStoragePolicy) SetConditions(c []metav1.Condition) { p.Status.Conditions = c }

// GetDeletionPolicy implements reconciler.Object.
func (p *ObjectStoragePolicy) GetDeletionPolicy() common.DeletionPolicy { return p.Spec.DeletionPolicy }

// GetExternalID implements reconciler.Object. The identity is the policy
// name within the service (the UUID is not used).
func (p *ObjectStoragePolicy) GetExternalID() string { return p.Status.Name }

// ExternalName is the policy name used in UpCloud.
func (p *ObjectStoragePolicy) ExternalName() string {
	if p.Spec.Name != "" {
		return p.Spec.Name
	}
	return p.Name
}
