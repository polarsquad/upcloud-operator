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

// LoadBalancerFrontendTLSConfigSpec defines the desired state of a frontend
// TLS configuration.
type LoadBalancerFrontendTLSConfigSpec struct {
	// FrontendRef points at the owning LoadBalancerFrontend. Immutable.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="frontendRef is immutable"
	FrontendRef common.LocalObjectReference `json:"frontendRef"`
	// Name of the TLS config in UpCloud. Immutable.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^[a-zA-Z0-9_-]+$`
	// +kubebuilder:validation:MaxLength=64
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="name is immutable"
	Name string `json:"name"`
	// CertificateBundleRef points at the LoadBalancerCertificateBundle to attach.
	// +kubebuilder:validation:Required
	CertificateBundleRef common.LocalObjectReference `json:"certificateBundleRef"`
	// +kubebuilder:default=Delete
	DeletionPolicy common.DeletionPolicy `json:"deletionPolicy,omitempty"`
}

// LoadBalancerFrontendTLSConfigStatus defines the observed state of
// LoadBalancerFrontendTLSConfig.
type LoadBalancerFrontendTLSConfigStatus struct {
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// UUID of the parent load balancer.
	ServiceUUID string `json:"serviceUUID,omitempty"`
	// FrontendName is the owning frontend's name in UpCloud.
	FrontendName string `json:"frontendName,omitempty"`
	// Name of the TLS config in UpCloud.
	Name string `json:"name,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// LoadBalancerFrontendTLSConfig is a load balancer frontend TLS configuration.
type LoadBalancerFrontendTLSConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   LoadBalancerFrontendTLSConfigSpec   `json:"spec"`
	Status LoadBalancerFrontendTLSConfigStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// LoadBalancerFrontendTLSConfigList contains a list of LoadBalancerFrontendTLSConfig.
type LoadBalancerFrontendTLSConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LoadBalancerFrontendTLSConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(SchemeGroupVersion, &LoadBalancerFrontendTLSConfig{}, &LoadBalancerFrontendTLSConfigList{})
		return nil
	})
}

// GetConditions implements reconciler.Object.
func (o *LoadBalancerFrontendTLSConfig) GetConditions() []metav1.Condition {
	return o.Status.Conditions
}

// SetConditions implements reconciler.Object.
func (o *LoadBalancerFrontendTLSConfig) SetConditions(c []metav1.Condition) { o.Status.Conditions = c }

// GetDeletionPolicy implements reconciler.Object.
func (o *LoadBalancerFrontendTLSConfig) GetDeletionPolicy() common.DeletionPolicy {
	return o.Spec.DeletionPolicy
}

// GetExternalID implements reconciler.Object.
func (o *LoadBalancerFrontendTLSConfig) GetExternalID() string { return o.Status.Name }

// ExternalName is the name used in UpCloud.
func (o *LoadBalancerFrontendTLSConfig) ExternalName() string {
	if o.Spec.Name != "" {
		return o.Spec.Name
	}
	return o.Name
}

// RefNames returns the referenced frontend name, for watches.
func (o *LoadBalancerFrontendTLSConfig) RefNames() []string {
	return []string{o.Spec.FrontendRef.Name}
}
