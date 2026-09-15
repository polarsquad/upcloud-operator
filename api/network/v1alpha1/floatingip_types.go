/*
Copyright 2026 Polar Squad.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to writing, software
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

// FloatingIPSpec defines the desired state of an UpCloud floating IP address.
// The IP is account-level (not scoped to a server) and is allocated in a zone.
type FloatingIPSpec struct {
	// Zone, for example fi-hel1. Immutable.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="zone is immutable"
	Zone string `json:"zone"`
	// Family of the address. Immutable.
	// +kubebuilder:validation:Enum=IPv4;IPv6
	// +kubebuilder:default=IPv4
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="family is immutable"
	Family string `json:"family,omitempty"`
	// Access of the address. Immutable.
	// +kubebuilder:validation:Enum=private;public;utility
	// +kubebuilder:default=public
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="access is immutable"
	Access string `json:"access,omitempty"`
	// PTRRecord is the reverse DNS (rDNS) name for the address.
	PTRRecord string `json:"ptrRecord,omitempty"`
	// ReleasePolicy controls what happens to the address if it is released
	// outside the operator.
	// +kubebuilder:validation:Enum=keep;release
	// +kubebuilder:default=keep
	ReleasePolicy string `json:"releasePolicy,omitempty"`
	// +kubebuilder:default=Delete
	DeletionPolicy common.DeletionPolicy `json:"deletionPolicy,omitempty"`
}

// FloatingIPStatus defines the observed state of FloatingIP.
type FloatingIPStatus struct {
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// Address is the allocated IP, the identity of this FloatingIP in UpCloud.
	Address string `json:"address,omitempty"`
	// MAC of the address.
	MAC string `json:"mac,omitempty"`
	// PartOfPlan is true when the address is part of the server's plan rather
	// than an add-on.
	PartOfPlan bool `json:"partOfPlan,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Address",type=string,JSONPath=`.status.address`
// +kubebuilder:printcolumn:name="Zone",type=string,JSONPath=`.spec.zone`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// FloatingIP is an UpCloud floating (unbound) IP address.
type FloatingIP struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   FloatingIPSpec   `json:"spec,omitempty"`
	Status FloatingIPStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// FloatingIPList contains a list of FloatingIP.
type FloatingIPList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []FloatingIP `json:"items"`
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(SchemeGroupVersion, &FloatingIP{}, &FloatingIPList{})
		return nil
	})
}

// GetConditions implements reconciler.Object.
func (f *FloatingIP) GetConditions() []metav1.Condition { return f.Status.Conditions }

// SetConditions implements reconciler.Object.
func (f *FloatingIP) SetConditions(c []metav1.Condition) { f.Status.Conditions = c }

// GetDeletionPolicy implements reconciler.Object.
func (f *FloatingIP) GetDeletionPolicy() common.DeletionPolicy { return f.Spec.DeletionPolicy }

// GetExternalID implements reconciler.Object. The IP address is the identity.
func (f *FloatingIP) GetExternalID() string { return f.Status.Address }
