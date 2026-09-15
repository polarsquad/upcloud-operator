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

// ObjectStorageAccessKeySpec defines the desired state of an object storage
// access key (an S3-style secret key pair for a user).
type ObjectStorageAccessKeySpec struct {
	// UserRef points at the ObjectStorageUser the key belongs to. Immutable.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="userRef is immutable"
	UserRef common.LocalObjectReference `json:"userRef"`
	// Status of the key, Active (default) or Inactive.
	// +kubebuilder:validation:Enum=Active;Inactive
	// +kubebuilder:default=Active
	Status string `json:"status,omitempty"`
	// SecretName is the name of the Secret holding the S3 credentials.
	// Defaults to <metadata.name>-s3.
	// +kubebuilder:validation:MaxLength=253
	SecretName string `json:"secretName,omitempty"`
	// +kubebuilder:default=Delete
	DeletionPolicy common.DeletionPolicy `json:"deletionPolicy,omitempty"`
}

// ObjectStorageAccessKeyStatus defines the observed state of
// ObjectStorageAccessKey.
type ObjectStorageAccessKeyStatus struct {
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// ServiceUUID of the owning service.
	ServiceUUID string `json:"serviceUUID,omitempty"`
	// Username the key belongs to.
	Username string `json:"username,omitempty"`
	// AccessKeyID as assigned by UpCloud.
	AccessKeyID string `json:"accessKeyID,omitempty"`
	// CreatedAt reported by the API, in RFC3339.
	CreatedAt string `json:"createdAt,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="AccessKeyID",type=string,JSONPath=`.status.accessKeyID`
// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.spec.status`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// ObjectStorageAccessKey is an S3-style access key for an object storage
// user. The secret is written to a Secret in the same namespace once, at
// create time.
type ObjectStorageAccessKey struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ObjectStorageAccessKeySpec   `json:"spec"`
	Status ObjectStorageAccessKeyStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ObjectStorageAccessKeyList contains a list of ObjectStorageAccessKey.
type ObjectStorageAccessKeyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ObjectStorageAccessKey `json:"items"`
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(SchemeGroupVersion, &ObjectStorageAccessKey{}, &ObjectStorageAccessKeyList{})
		return nil
	})
}

// GetConditions implements reconciler.Object.
func (k *ObjectStorageAccessKey) GetConditions() []metav1.Condition { return k.Status.Conditions }

// SetConditions implements reconciler.Object.
func (k *ObjectStorageAccessKey) SetConditions(c []metav1.Condition) { k.Status.Conditions = c }

// GetDeletionPolicy implements reconciler.Object.
func (k *ObjectStorageAccessKey) GetDeletionPolicy() common.DeletionPolicy {
	return k.Spec.DeletionPolicy
}

// GetExternalID implements reconciler.Object. The identity is the
// AccessKeyID.
func (k *ObjectStorageAccessKey) GetExternalID() string { return k.Status.AccessKeyID }

// SecretKeyName is the name of the Secret holding the S3 credentials.
func (k *ObjectStorageAccessKey) SecretKeyName() string {
	if k.Spec.SecretName != "" {
		return k.Spec.SecretName
	}
	return k.Name + "-s3"
}

// RefNames returns the referenced ObjectStorageUser names, for watches.
func (k *ObjectStorageAccessKey) RefNames() []string {
	return []string{k.Spec.UserRef.Name}
}
