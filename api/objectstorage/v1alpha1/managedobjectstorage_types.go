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

// MOSNetworkAttachment is a network attachment for an object storage
// service. Unlike the shared NetworkAttachment, only public and private
// types are valid here (utility is not).
type MOSNetworkAttachment struct {
	// +kubebuilder:validation:Pattern=`^[a-zA-Z0-9_-]+$`
	// +kubebuilder:validation:MaxLength=64
	Name string `json:"name"`
	// +kubebuilder:validation:Enum=public;private
	Type string `json:"type"`
	// +kubebuilder:default=IPv4
	// +kubebuilder:validation:Enum=IPv4;IPv6
	Family string `json:"family,omitempty"`
	// UUID of an existing UpCloud SDN network. Required for private attachments
	// when networkRef is not set.
	UUID string `json:"uuid,omitempty"`
	// NetworkRef points at a Network CR in the same namespace; its
	// status.uuid is used.
	NetworkRef *common.LocalObjectReference `json:"networkRef,omitempty"`
}

// Endpoint is a single endpoint of an object storage service, for example
// the public S3 API domain.
type Endpoint struct {
	// +kubebuilder:validation:MaxLength=255
	DomainName string `json:"domainName,omitempty"`
	// +kubebuilder:validation:MaxLength=64
	Type string `json:"type,omitempty"`
	// +kubebuilder:validation:MaxLength=255
	IAMURL string `json:"iamURL,omitempty"`
	// +kubebuilder:validation:MaxLength=255
	STSURL string `json:"stsURL,omitempty"`
}

// ManagedObjectStorageSpec defines the desired state of an UpCloud Managed
// Object Storage service.
// +kubebuilder:validation:XValidation:rule="!has(self.networks) || self.networks.all(n, n.type != 'private' || has(n.uuid) || has(n.networkRef))",message="private networks need uuid or networkRef"
type ManagedObjectStorageSpec struct {
	// Name in UpCloud. Defaults to metadata.name.
	// +kubebuilder:validation:Pattern=`^[a-zA-Z0-9_-]+$`
	// +kubebuilder:validation:MaxLength=64
	Name string `json:"name,omitempty"`
	// Region, for example fi-hel1. Immutable.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="region is immutable"
	Region string `json:"region"`
	// configuredStatus requests the service to be started (default) or
	// stopped. Changes trigger start/shutdown.
	// +kubebuilder:validation:Enum=started;stopped
	// +kubebuilder:default=started
	ConfiguredStatus string `json:"configuredStatus,omitempty"`
	// Network attachments, at most 8. Only public and private types are
	// valid for object storage.
	// +kubebuilder:validation:MaxItems=8
	Networks []MOSNetworkAttachment `json:"networks,omitempty"`
	// +kubebuilder:default=false
	TerminationProtection bool                 `json:"terminationProtection,omitempty"`
	Labels                common.UpCloudLabels `json:"labels,omitempty"`
	// +kubebuilder:default=Delete
	DeletionPolicy common.DeletionPolicy `json:"deletionPolicy,omitempty"`
}

// ManagedObjectStorageStatus defines the observed state of
// ManagedObjectStorage.
type ManagedObjectStorageStatus struct {
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// UUID of the service in UpCloud.
	UUID string `json:"uuid,omitempty"`
	// Current UpCloud operational state, for example running.
	OperationalState string `json:"operationalState,omitempty"`
	// Endpoints reported by the API.
	Endpoints []Endpoint `json:"endpoints,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Region",type=string,JSONPath=`.spec.region`
// +kubebuilder:printcolumn:name="State",type=string,JSONPath=`.status.operationalState`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// ManagedObjectStorage is an UpCloud Managed Object Storage service.
type ManagedObjectStorage struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ManagedObjectStorageSpec   `json:"spec"`
	Status ManagedObjectStorageStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ManagedObjectStorageList contains a list of ManagedObjectStorage.
type ManagedObjectStorageList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ManagedObjectStorage `json:"items"`
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(SchemeGroupVersion, &ManagedObjectStorage{}, &ManagedObjectStorageList{})
		return nil
	})
}

// GetConditions implements reconciler.Object.
func (s *ManagedObjectStorage) GetConditions() []metav1.Condition { return s.Status.Conditions }

// SetConditions implements reconciler.Object.
func (s *ManagedObjectStorage) SetConditions(c []metav1.Condition) { s.Status.Conditions = c }

// GetDeletionPolicy implements reconciler.Object.
func (s *ManagedObjectStorage) GetDeletionPolicy() common.DeletionPolicy {
	return s.Spec.DeletionPolicy
}

// GetExternalID implements reconciler.Object.
func (s *ManagedObjectStorage) GetExternalID() string { return s.Status.UUID }

// ExternalName is the name used in UpCloud.
func (s *ManagedObjectStorage) ExternalName() string {
	if s.Spec.Name != "" {
		return s.Spec.Name
	}
	return s.Name
}

// DesiredConfiguredStatus reports the desired configured status, started by
// default.
func (s *ManagedObjectStorage) DesiredConfiguredStatus() string {
	if s.Spec.ConfiguredStatus == "" {
		return "started"
	}
	return s.Spec.ConfiguredStatus
}
