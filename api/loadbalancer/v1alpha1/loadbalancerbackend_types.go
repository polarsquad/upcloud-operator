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

// LoadBalancerBackendProperties mirrors upcloud.LoadBalancerBackendProperties.
// Only the fields the user sets are sent (pointers are nil when unset).
type LoadBalancerBackendProperties struct {
	// +kubebuilder:validation:Minimum=0
	TimeoutServer int `json:"timeoutServer,omitempty"`
	// +kubebuilder:validation:Minimum=0
	TimeoutTunnel        int   `json:"timeoutTunnel,omitempty"`
	HealthCheckTLSVerify *bool `json:"healthCheckTLSVerify,omitempty"`
	// +kubebuilder:validation:Enum=off;tcp;http
	HealthCheckType string `json:"healthCheckType,omitempty"`
	// +kubebuilder:validation:Minimum=0
	HealthCheckInterval int `json:"healthCheckInterval,omitempty"`
	// +kubebuilder:validation:Minimum=0
	HealthCheckFall int `json:"healthCheckFall,omitempty"`
	// +kubebuilder:validation:Minimum=0
	HealthCheckRise int    `json:"healthCheckRise,omitempty"`
	HealthCheckURL  string `json:"healthCheckURL,omitempty"`
	// +kubebuilder:validation:Minimum=0
	HealthCheckExpectedStatus int    `json:"healthCheckExpectedStatus,omitempty"`
	StickySessionCookieName   string `json:"stickySessionCookieName,omitempty"`
	// +kubebuilder:validation:Enum=v1;v2
	OutboundProxyProtocol string `json:"outboundProxyProtocol,omitempty"`
	TLSEnabled            *bool  `json:"tlsEnabled,omitempty"`
	TLSVerify             *bool  `json:"tlsVerify,omitempty"`
	TLSUseSystemCA        *bool  `json:"tlsUseSystemCA,omitempty"`
	HTTP2Enabled          *bool  `json:"http2Enabled,omitempty"`
}

// LoadBalancerBackendSpec defines the desired state of a load balancer
// backend.
type LoadBalancerBackendSpec struct {
	// LoadBalancerRef points at the owning LoadBalancer. Immutable.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="loadBalancerRef is immutable"
	LoadBalancerRef common.LocalObjectReference `json:"loadBalancerRef"`
	// Name of the backend in UpCloud. Immutable.
	// +kubebuilder:validation:Pattern=`^[a-zA-Z0-9_-]+$`
	// +kubebuilder:validation:MaxLength=64
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="name is immutable"
	Name string `json:"name,omitempty"`
	// ResolverRef points at a LoadBalancerResolver; its status.name is used.
	ResolverRef *common.LocalObjectReference   `json:"resolverRef,omitempty"`
	Properties  *LoadBalancerBackendProperties `json:"properties,omitempty"`
	// +kubebuilder:default=Delete
	DeletionPolicy common.DeletionPolicy `json:"deletionPolicy,omitempty"`
}

// LoadBalancerBackendStatus defines the observed state of LoadBalancerBackend.
type LoadBalancerBackendStatus struct {
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// UUID of the parent load balancer.
	ServiceUUID string `json:"serviceUUID,omitempty"`
	// Name of the backend in UpCloud.
	Name string `json:"name,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// LoadBalancerBackend is a load balancer backend.
type LoadBalancerBackend struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   LoadBalancerBackendSpec   `json:"spec"`
	Status LoadBalancerBackendStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// LoadBalancerBackendList contains a list of LoadBalancerBackend.
type LoadBalancerBackendList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LoadBalancerBackend `json:"items"`
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(SchemeGroupVersion, &LoadBalancerBackend{}, &LoadBalancerBackendList{})
		return nil
	})
}

// GetConditions implements reconciler.Object.
func (b *LoadBalancerBackend) GetConditions() []metav1.Condition { return b.Status.Conditions }

// SetConditions implements reconciler.Object.
func (b *LoadBalancerBackend) SetConditions(c []metav1.Condition) { b.Status.Conditions = c }

// GetDeletionPolicy implements reconciler.Object.
func (b *LoadBalancerBackend) GetDeletionPolicy() common.DeletionPolicy {
	return b.Spec.DeletionPolicy
}

// GetExternalID implements reconciler.Object.
func (b *LoadBalancerBackend) GetExternalID() string { return b.Status.Name }

// ExternalName is the name used in UpCloud.
func (b *LoadBalancerBackend) ExternalName() string {
	if b.Spec.Name != "" {
		return b.Spec.Name
	}
	return b.Name
}

// RefNames returns the referenced LoadBalancer name, for watches.
func (b *LoadBalancerBackend) RefNames() []string {
	return []string{b.Spec.LoadBalancerRef.Name}
}
