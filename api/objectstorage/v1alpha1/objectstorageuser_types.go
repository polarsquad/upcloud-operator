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

// ObjectStorageUserSpec defines the desired state of an object storage user.
type ObjectStorageUserSpec struct {
	// ServiceRef points at the ManagedObjectStorage the user belongs to.
	// Immutable.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="serviceRef is immutable"
	ServiceRef common.LocalObjectReference `json:"serviceRef"`
	// Username in UpCloud. Defaults to metadata.name. Immutable.
	// +kubebuilder:validation:Pattern=`^[a-zA-Z0-9_-]+$`
	// +kubebuilder:validation:MaxLength=64
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="username is immutable"
	Username string `json:"username,omitempty"`
	// Policies are the names of ObjectStoragePolicy resources in the same
	// service to attach to this user. Each may also be given via PolicyRefs.
	// +kubebuilder:validation:MaxItems=64
	Policies []string `json:"policies,omitempty"`
	// PolicyRefs point at ObjectStoragePolicy CRs in the same namespace;
	// their status.name is attached to the user.
	// +kubebuilder:validation:MaxItems=64
	PolicyRefs []common.LocalObjectReference `json:"policyRefs,omitempty"`
	// +kubebuilder:default=Delete
	DeletionPolicy common.DeletionPolicy `json:"deletionPolicy,omitempty"`
}

// ObjectStorageUserStatus defines the observed state of ObjectStorageUser.
type ObjectStorageUserStatus struct {
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// ServiceUUID of the owning service.
	ServiceUUID string `json:"serviceUUID,omitempty"`
	// Username as stored in UpCloud.
	Username string `json:"username,omitempty"`
	// ARN of the user in UpCloud.
	ARN string `json:"arn,omitempty"`
	// AttachedPolicies are the policy names currently attached in UpCloud.
	AttachedPolicies []string `json:"attachedPolicies,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Username",type=string,JSONPath=`.status.username`
// +kubebuilder:printcolumn:name="ARN",type=string,JSONPath=`.status.arn`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// ObjectStorageUser is an IAM-style user of an object storage service.
type ObjectStorageUser struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ObjectStorageUserSpec   `json:"spec"`
	Status ObjectStorageUserStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ObjectStorageUserList contains a list of ObjectStorageUser.
type ObjectStorageUserList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ObjectStorageUser `json:"items"`
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(SchemeGroupVersion, &ObjectStorageUser{}, &ObjectStorageUserList{})
		return nil
	})
}

// GetConditions implements reconciler.Object.
func (u *ObjectStorageUser) GetConditions() []metav1.Condition { return u.Status.Conditions }

// SetConditions implements reconciler.Object.
func (u *ObjectStorageUser) SetConditions(c []metav1.Condition) { u.Status.Conditions = c }

// GetDeletionPolicy implements reconciler.Object.
func (u *ObjectStorageUser) GetDeletionPolicy() common.DeletionPolicy { return u.Spec.DeletionPolicy }

// GetExternalID implements reconciler.Object. The identity is the username
// within the service.
func (u *ObjectStorageUser) GetExternalID() string { return u.Status.Username }

// ExternalUsername is the username used in UpCloud.
func (u *ObjectStorageUser) ExternalUsername() string {
	if u.Spec.Username != "" {
		return u.Spec.Username
	}
	return u.Name
}

// RefNames returns the referenced ManagedObjectStorage names, for watches.
func (u *ObjectStorageUser) RefNames() []string {
	return []string{u.Spec.ServiceRef.Name}
}

// PolicyRefNames returns the referenced ObjectStoragePolicy CR names, for
// watches.
func (u *ObjectStorageUser) PolicyRefNames() []string {
	out := make([]string, 0, len(u.Spec.PolicyRefs))
	for _, ref := range u.Spec.PolicyRefs {
		out = append(out, ref.Name)
	}
	return out
}
