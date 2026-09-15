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

// LoadBalancerBackendTLSConfigSpec defines the desired state of a load
// balancer backend TLS configuration.
type LoadBalancerBackendTLSConfigSpec struct {
	// BackendRef points at the owning LoadBalancerBackend. Immutable.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="backendRef is immutable"
	BackendRef common.LocalObjectReference `json:"backendRef"`
	// Name of the TLS config in UpCloud. Immutable.
	// +kubebuilder:validation:Pattern=`^[a-zA-Z0-9_-]+$`
	// +kubebuilder:validation:MaxLength=64
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="name is immutable"
	Name string `json:"name,omitempty"`
	// CertificateBundleRef points at a LoadBalancerCertificateBundle; its
	// status.uuid is used.
	CertificateBundleRef common.LocalObjectReference `json:"certificateBundleRef"`
	// +kubebuilder:default=Delete
	DeletionPolicy common.DeletionPolicy `json:"deletionPolicy,omitempty"`
}

// LoadBalancerBackendTLSConfigStatus defines the observed state of
// LoadBalancerBackendTLSConfig.
type LoadBalancerBackendTLSConfigStatus struct {
	Conditions  []metav1.Condition `json:"conditions,omitempty"`
	ServiceUUID string             `json:"serviceUUID,omitempty"`
	BackendName string             `json:"backendName,omitempty"`
	Name        string             `json:"name,omitempty"`
	// UUID of the attached certificate bundle.
	CertificateBundleUUID string `json:"certificateBundleUUID,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// LoadBalancerBackendTLSConfig is a TLS configuration on a load balancer
// backend.
type LoadBalancerBackendTLSConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   LoadBalancerBackendTLSConfigSpec   `json:"spec"`
	Status LoadBalancerBackendTLSConfigStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// LoadBalancerBackendTLSConfigList contains a list of
// LoadBalancerBackendTLSConfig.
type LoadBalancerBackendTLSConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LoadBalancerBackendTLSConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(SchemeGroupVersion, &LoadBalancerBackendTLSConfig{}, &LoadBalancerBackendTLSConfigList{})
		return nil
	})
}

// GetConditions implements reconciler.Object.
func (c *LoadBalancerBackendTLSConfig) GetConditions() []metav1.Condition { return c.Status.Conditions }

// SetConditions implements reconciler.Object.
func (c *LoadBalancerBackendTLSConfig) SetConditions(cnd []metav1.Condition) {
	c.Status.Conditions = cnd
}

// GetDeletionPolicy implements reconciler.Object.
func (c *LoadBalancerBackendTLSConfig) GetDeletionPolicy() common.DeletionPolicy {
	return c.Spec.DeletionPolicy
}

// GetExternalID implements reconciler.Object.
func (c *LoadBalancerBackendTLSConfig) GetExternalID() string { return c.Status.Name }

// ExternalName is the name used in UpCloud.
func (c *LoadBalancerBackendTLSConfig) ExternalName() string {
	if c.Spec.Name != "" {
		return c.Spec.Name
	}
	return c.Name
}

// RefNames returns the referenced LoadBalancerBackend name, for watches.
func (c *LoadBalancerBackendTLSConfig) RefNames() []string {
	return []string{c.Spec.BackendRef.Name}
}
