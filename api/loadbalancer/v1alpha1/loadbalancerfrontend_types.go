/*
Copyright 2026 Polar Squad.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or distributed under the License is
distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
KIND, either express or implied. See the License for the specific language
governing permissions and limitations under the License.
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/polarsquad/upcloud-operator/api/common"
)

// FrontendNetwork is one network attachment on a LoadBalancerFrontend. The
// name must match a network on the parent LoadBalancer. It is immutable: the
// API cannot modify the networks of an existing frontend.
type FrontendNetwork struct {
	// Name is a reference to a network on the parent LoadBalancer.
	// +kubebuilder:validation:Required
	Name string `json:"name"`
}

// LoadBalancerFrontendProperties mirrors the UpCloud frontend properties.
type LoadBalancerFrontendProperties struct {
	// +kubebuilder:validation:Minimum=0
	TimeoutClient int `json:"timeoutClient,omitempty"`
	// +optional
	InboundProxyProtocol *bool `json:"inboundProxyProtocol,omitempty"`
	// +optional
	HTTP2Enabled *bool `json:"http2Enabled,omitempty"`
}

// LoadBalancerFrontendSpec defines the desired state of a load balancer
// frontend.
type LoadBalancerFrontendSpec struct {
	// LoadBalancerRef points at the owning LoadBalancer. Immutable.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="loadBalancerRef is immutable"
	LoadBalancerRef common.LocalObjectReference `json:"loadBalancerRef"`
	// Name of the frontend in UpCloud. Immutable.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^[a-zA-Z0-9_-]+$`
	// +kubebuilder:validation:MaxLength=64
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="name is immutable"
	Name string `json:"name"`
	// Mode is the frontend mode.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=http;tcp
	Mode string `json:"mode"`
	// Port is the frontend port, 1-65535.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	Port int `json:"port"`
	// DefaultBackendRef points at the default LoadBalancerBackend. Resolves to
	// its status.name.
	// +kubebuilder:validation:Required
	DefaultBackendRef common.LocalObjectReference `json:"defaultBackendRef"`
	// Networks is the frontend's network attachment (names of the parent's
	// networks). If omitted, the API attaches all of the parent's networks.
	// Immutable.
	// +optional
	Networks []FrontendNetwork `json:"networks,omitempty"`
	// Properties are the frontend's tunable properties.
	// +optional
	Properties *LoadBalancerFrontendProperties `json:"properties,omitempty"`
	// +kubebuilder:default=Delete
	DeletionPolicy common.DeletionPolicy `json:"deletionPolicy,omitempty"`
}

// LoadBalancerFrontendStatus defines the observed state of LoadBalancerFrontend.
type LoadBalancerFrontendStatus struct {
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// UUID of the parent load balancer.
	ServiceUUID string `json:"serviceUUID,omitempty"`
	// Name of the frontend in UpCloud.
	Name string `json:"name,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// LoadBalancerFrontend is a load balancer frontend.
type LoadBalancerFrontend struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   LoadBalancerFrontendSpec   `json:"spec"`
	Status LoadBalancerFrontendStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// LoadBalancerFrontendList contains a list of LoadBalancerFrontend.
type LoadBalancerFrontendList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LoadBalancerFrontend `json:"items"`
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(SchemeGroupVersion, &LoadBalancerFrontend{}, &LoadBalancerFrontendList{})
		return nil
	})
}

// GetConditions implements reconciler.Object.
func (o *LoadBalancerFrontend) GetConditions() []metav1.Condition { return o.Status.Conditions }

// SetConditions implements reconciler.Object.
func (o *LoadBalancerFrontend) SetConditions(c []metav1.Condition) { o.Status.Conditions = c }

// GetDeletionPolicy implements reconciler.Object.
func (o *LoadBalancerFrontend) GetDeletionPolicy() common.DeletionPolicy {
	return o.Spec.DeletionPolicy
}

// GetExternalID implements reconciler.Object.
func (o *LoadBalancerFrontend) GetExternalID() string { return o.Status.Name }

// ExternalName is the name used in UpCloud.
func (o *LoadBalancerFrontend) ExternalName() string {
	if o.Spec.Name != "" {
		return o.Spec.Name
	}
	return o.Name
}

// RefNames returns the referenced parent names, for watches.
func (o *LoadBalancerFrontend) RefNames() []string {
	return []string{o.Spec.LoadBalancerRef.Name, o.Spec.DefaultBackendRef.Name}
}
