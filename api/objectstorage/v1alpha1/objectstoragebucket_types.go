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

// ObjectStorageBucketSpec defines the desired state of an object storage
// bucket.
type ObjectStorageBucketSpec struct {
	// ServiceRef points at the ManagedObjectStorage the bucket belongs to.
	// Immutable.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="serviceRef is immutable"
	ServiceRef common.LocalObjectReference `json:"serviceRef"`
	// Name in UpCloud. Defaults to metadata.name. Immutable.
	// +kubebuilder:validation:Pattern=`^[a-z0-9][a-z0-9.-]{1,61}[a-z0-9]$`
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="name is immutable"
	Name string `json:"name,omitempty"`
	// +kubebuilder:default=Delete
	DeletionPolicy common.DeletionPolicy `json:"deletionPolicy,omitempty"`
}

// ObjectStorageBucketStatus defines the observed state of
// ObjectStorageBucket.
type ObjectStorageBucketStatus struct {
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// ServiceUUID of the owning service.
	ServiceUUID string `json:"serviceUUID,omitempty"`
	// Name as stored in UpCloud.
	Name string `json:"name,omitempty"`
	// TotalObjects reported by the API.
	TotalObjects int `json:"totalObjects,omitempty"`
	// TotalSizeBytes reported by the API.
	TotalSizeBytes int `json:"totalSizeBytes,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Bucket",type=string,JSONPath=`.status.name`
// +kubebuilder:printcolumn:name="Objects",type=integer,JSONPath=`.status.totalObjects`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// ObjectStorageBucket is a bucket of an object storage service.
type ObjectStorageBucket struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ObjectStorageBucketSpec   `json:"spec"`
	Status ObjectStorageBucketStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ObjectStorageBucketList contains a list of ObjectStorageBucket.
type ObjectStorageBucketList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ObjectStorageBucket `json:"items"`
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(SchemeGroupVersion, &ObjectStorageBucket{}, &ObjectStorageBucketList{})
		return nil
	})
}

// GetConditions implements reconciler.Object.
func (b *ObjectStorageBucket) GetConditions() []metav1.Condition { return b.Status.Conditions }

// SetConditions implements reconciler.Object.
func (b *ObjectStorageBucket) SetConditions(c []metav1.Condition) { b.Status.Conditions = c }

// GetDeletionPolicy implements reconciler.Object.
func (b *ObjectStorageBucket) GetDeletionPolicy() common.DeletionPolicy { return b.Spec.DeletionPolicy }

// GetExternalID implements reconciler.Object. The identity is the bucket
// name within the service.
func (b *ObjectStorageBucket) GetExternalID() string { return b.Status.Name }

// ExternalName is the bucket name used in UpCloud.
func (b *ObjectStorageBucket) ExternalName() string {
	if b.Spec.Name != "" {
		return b.Spec.Name
	}
	return b.Name
}

// RefNames returns the referenced ManagedObjectStorage names, for watches.
func (b *ObjectStorageBucket) RefNames() []string {
	return []string{b.Spec.ServiceRef.Name}
}
