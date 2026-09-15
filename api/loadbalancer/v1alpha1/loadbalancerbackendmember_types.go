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

// LoadBalancerBackendMemberSpec defines the desired state of a load balancer
// backend member.
// +kubebuilder:validation:XValidation:rule="self.type == 'static' ? has(self.ip) : true",message="static members need ip"
type LoadBalancerBackendMemberSpec struct {
	// BackendRef points at the owning LoadBalancerBackend. Immutable.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="backendRef is immutable"
	BackendRef common.LocalObjectReference `json:"backendRef"`
	// Name of the member in UpCloud. Immutable.
	// +kubebuilder:validation:Pattern=`^[a-zA-Z0-9_-]+$`
	// +kubebuilder:validation:MaxLength=64
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="name is immutable"
	Name string `json:"name,omitempty"`
	// +kubebuilder:validation:Enum=static;dynamic
	Type string `json:"type"`
	// IP address of the member. Required for static members.
	IP string `json:"ip,omitempty"`
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	Port int `json:"port"`
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	// +kubebuilder:default=1
	Weight int `json:"weight,omitempty"`
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:default=1000
	MaxSessions int `json:"maxSessions,omitempty"`
	// +kubebuilder:default=true
	Enabled bool `json:"enabled,omitempty"`
	// +kubebuilder:default=Delete
	DeletionPolicy common.DeletionPolicy `json:"deletionPolicy,omitempty"`
}

// LoadBalancerBackendMemberStatus defines the observed state of
// LoadBalancerBackendMember.
type LoadBalancerBackendMemberStatus struct {
	Conditions  []metav1.Condition `json:"conditions,omitempty"`
	ServiceUUID string             `json:"serviceUUID,omitempty"`
	// Name of the backend this member belongs to.
	BackendName string `json:"backendName,omitempty"`
	Name        string `json:"name,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// LoadBalancerBackendMember is a single server behind a load balancer
// backend.
type LoadBalancerBackendMember struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   LoadBalancerBackendMemberSpec   `json:"spec"`
	Status LoadBalancerBackendMemberStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// LoadBalancerBackendMemberList contains a list of LoadBalancerBackendMember.
type LoadBalancerBackendMemberList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LoadBalancerBackendMember `json:"items"`
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(SchemeGroupVersion, &LoadBalancerBackendMember{}, &LoadBalancerBackendMemberList{})
		return nil
	})
}

// GetConditions implements reconciler.Object.
func (m *LoadBalancerBackendMember) GetConditions() []metav1.Condition { return m.Status.Conditions }

// SetConditions implements reconciler.Object.
func (m *LoadBalancerBackendMember) SetConditions(c []metav1.Condition) { m.Status.Conditions = c }

// GetDeletionPolicy implements reconciler.Object.
func (m *LoadBalancerBackendMember) GetDeletionPolicy() common.DeletionPolicy {
	return m.Spec.DeletionPolicy
}

// GetExternalID implements reconciler.Object.
func (m *LoadBalancerBackendMember) GetExternalID() string { return m.Status.Name }

// ExternalName is the name used in UpCloud.
func (m *LoadBalancerBackendMember) ExternalName() string {
	if m.Spec.Name != "" {
		return m.Spec.Name
	}
	return m.Name
}

// RefNames returns the referenced LoadBalancerBackend name, for watches.
func (m *LoadBalancerBackendMember) RefNames() []string {
	return []string{m.Spec.BackendRef.Name}
}
