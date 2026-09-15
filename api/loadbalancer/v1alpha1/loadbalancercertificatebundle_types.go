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

// LoadBalancerCertificateBundleSpec defines the desired state of a load
// balancer certificate bundle. Bundles are account-level (not scoped to a
// load balancer).
// +kubebuilder:validation:XValidation:rule="self.type == 'dynamic' ? (has(self.certificateSecretRef) == false) : (has(self.certificateSecretRef))",message="manual and authority bundles need certificateSecretRef, dynamic bundles must not set it"
type LoadBalancerCertificateBundleSpec struct {
	// Name of the bundle in UpCloud.
	// +kubebuilder:validation:Pattern=`^[a-zA-Z0-9_-]+$`
	// +kubebuilder:validation:MaxLength=64
	Name string `json:"name,omitempty"`
	// Type of the bundle. Immutable.
	// +kubebuilder:validation:Enum=manual;dynamic;authority
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="type is immutable"
	Type string `json:"type"`
	// CertificateSecretRef points at a Secret of type kubernetes.io/tls with
	// tls.crt, tls.key and optionally ca.crt (used as intermediates). Required
	// for manual and authority bundles.
	CertificateSecretRef *common.LocalObjectReference `json:"certificateSecretRef,omitempty"`
	// Hostnames the dynamic bundle should cover. Required for dynamic bundles.
	Hostnames []string `json:"hostnames,omitempty"`
	// +kubebuilder:default=Delete
	DeletionPolicy common.DeletionPolicy `json:"deletionPolicy,omitempty"`
}

// LoadBalancerCertificateBundleStatus defines the observed state of
// LoadBalancerCertificateBundle.
type LoadBalancerCertificateBundleStatus struct {
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// UUID of the bundle in UpCloud.
	UUID string `json:"uuid,omitempty"`
	// Current operational state, for example idle.
	OperationalState string `json:"operationalState,omitempty"`
	// notAfter, as reported by the API.
	NotAfter string `json:"notAfter,omitempty"`
	// notBefore, as reported by the API.
	NotBefore string `json:"notBefore,omitempty"`
	// KeyType of the private key, for example rsa.
	KeyType string `json:"keyType,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Type",type=string,JSONPath=`.spec.type`
// +kubebuilder:printcolumn:name="State",type=string,JSONPath=`.status.operationalState`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// LoadBalancerCertificateBundle is an UpCloud load balancer certificate
// bundle.
type LoadBalancerCertificateBundle struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   LoadBalancerCertificateBundleSpec   `json:"spec"`
	Status LoadBalancerCertificateBundleStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// LoadBalancerCertificateBundleList contains a list of
// LoadBalancerCertificateBundle.
type LoadBalancerCertificateBundleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LoadBalancerCertificateBundle `json:"items"`
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(SchemeGroupVersion, &LoadBalancerCertificateBundle{}, &LoadBalancerCertificateBundleList{})
		return nil
	})
}

// GetConditions implements reconciler.Object.
func (b *LoadBalancerCertificateBundle) GetConditions() []metav1.Condition {
	return b.Status.Conditions
}

// SetConditions implements reconciler.Object.
func (b *LoadBalancerCertificateBundle) SetConditions(c []metav1.Condition) { b.Status.Conditions = c }

// GetDeletionPolicy implements reconciler.Object.
func (b *LoadBalancerCertificateBundle) GetDeletionPolicy() common.DeletionPolicy {
	return b.Spec.DeletionPolicy
}

// GetExternalID implements reconciler.Object.
func (b *LoadBalancerCertificateBundle) GetExternalID() string { return b.Status.UUID }

// ExternalName is the name used in UpCloud.
func (b *LoadBalancerCertificateBundle) ExternalName() string {
	if b.Spec.Name != "" {
		return b.Spec.Name
	}
	return b.Name
}
