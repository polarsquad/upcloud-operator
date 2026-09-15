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

// ObjectStorageCustomDomainSpec defines the desired state of a custom domain
// on an object storage service.
type ObjectStorageCustomDomainSpec struct {
	// ServiceRef points at the ManagedObjectStorage the domain is bound to.
	// Immutable.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="serviceRef is immutable"
	ServiceRef common.LocalObjectReference `json:"serviceRef"`
	// DomainName to serve, for example objects.example.com.
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([a-z0-9-]*[a-z0-9])?(\.[a-z0-9]([a-z0-9-]*[a-z0-9])?)*$`
	// +kubebuilder:validation:MaxLength=253
	DomainName string `json:"domainName"`
	// Type of the endpoint the domain binds to.
	// +kubebuilder:validation:Enum=public
	// +kubebuilder:default=public
	Type string `json:"type,omitempty"`
	// +kubebuilder:default=Delete
	DeletionPolicy common.DeletionPolicy `json:"deletionPolicy,omitempty"`
}

// ObjectStorageCustomDomainStatus defines the observed state of
// ObjectStorageCustomDomain.
type ObjectStorageCustomDomainStatus struct {
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// ServiceUUID of the owning service.
	ServiceUUID string `json:"serviceUUID,omitempty"`
	// DomainName as stored in UpCloud.
	DomainName string `json:"domainName,omitempty"`
	// Type as stored in UpCloud.
	Type string `json:"type,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Domain",type=string,JSONPath=`.status.domainName`
// +kubebuilder:printcolumn:name="Type",type=string,JSONPath=`.status.type`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// ObjectStorageCustomDomain is a custom domain bound to an object storage
// service endpoint.
type ObjectStorageCustomDomain struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ObjectStorageCustomDomainSpec   `json:"spec"`
	Status ObjectStorageCustomDomainStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ObjectStorageCustomDomainList contains a list of ObjectStorageCustomDomain.
type ObjectStorageCustomDomainList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ObjectStorageCustomDomain `json:"items"`
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(SchemeGroupVersion, &ObjectStorageCustomDomain{}, &ObjectStorageCustomDomainList{})
		return nil
	})
}

// GetConditions implements reconciler.Object.
func (d *ObjectStorageCustomDomain) GetConditions() []metav1.Condition { return d.Status.Conditions }

// SetConditions implements reconciler.Object.
func (d *ObjectStorageCustomDomain) SetConditions(c []metav1.Condition) { d.Status.Conditions = c }

// GetDeletionPolicy implements reconciler.Object.
func (d *ObjectStorageCustomDomain) GetDeletionPolicy() common.DeletionPolicy {
	return d.Spec.DeletionPolicy
}

// GetExternalID implements reconciler.Object. The identity is the domain
// name within the service.
func (d *ObjectStorageCustomDomain) GetExternalID() string { return d.Status.DomainName }

// RefNames returns the referenced ManagedObjectStorage names, for watches.
func (d *ObjectStorageCustomDomain) RefNames() []string {
	return []string{d.Spec.ServiceRef.Name}
}
