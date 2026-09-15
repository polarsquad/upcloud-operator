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

// LoadBalancerNetworkAttachment is a network attachment for a load balancer.
// Only public and private types, and IPv4, are valid.
type LoadBalancerNetworkAttachment struct {
	// +kubebuilder:validation:Pattern=`^[a-zA-Z0-9_-]+$`
	// +kubebuilder:validation:MaxLength=64
	Name string `json:"name"`
	// +kubebuilder:validation:Enum=public;private
	Type string `json:"type"`
	// +kubebuilder:default=IPv4
	// +kubebuilder:validation:Enum=IPv4
	Family string `json:"family,omitempty"`
	// UUID of an existing UpCloud SDN network. Required for private
	// attachments when networkRef is not set.
	UUID string `json:"uuid,omitempty"`
	// NetworkRef points at a Network CR in the same namespace; its
	// status.uuid is used.
	NetworkRef *common.LocalObjectReference `json:"networkRef,omitempty"`
}

// Maintenance schedules the load balancer maintenance window.
type Maintenance struct {
	// +kubebuilder:validation:Enum=monday;tuesday;wednesday;thursday;friday;saturday;sunday
	DOW string `json:"dow,omitempty"`
	// Maintenance time in HH:mm format.
	// +kubebuilder:validation:MaxLength=5
	Time string `json:"time,omitempty"`
}

// LBStatusNetwork is a single network of a load balancer, as reported by the
// API.
type LBStatusNetwork struct {
	// +kubebuilder:validation:MaxLength=64
	Name string `json:"name"`
	// +kubebuilder:validation:MaxLength=16
	Type string `json:"type,omitempty"`
	// +kubebuilder:validation:MaxLength=16
	Family string `json:"family,omitempty"`
	// +kubebuilder:validation:MaxLength=255
	DNSName     string   `json:"dnsName,omitempty"`
	IPAddresses []string `json:"ipAddresses,omitempty"`
}

// LoadBalancerSpec defines the desired state of an UpCloud Load Balancer.
// +kubebuilder:validation:XValidation:rule="!has(self.networks) || self.networks.all(n, n.type != 'private' || has(n.uuid) || has(n.networkRef))",message="private networks need uuid or networkRef"
type LoadBalancerSpec struct {
	// Name in UpCloud. Defaults to metadata.name.
	// +kubebuilder:validation:Pattern=`^[a-zA-Z0-9_-]+$`
	// +kubebuilder:validation:MaxLength=64
	Name string `json:"name,omitempty"`
	// Plan, for example 0-4.
	// +kubebuilder:validation:MinLength=1
	Plan string `json:"plan"`
	// Zone, for example fi-hel1. Immutable.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="zone is immutable"
	Zone string `json:"zone"`
	// Network attachments, 1 to 8. Immutable as a whole.
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=8
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="networks are immutable"
	Networks []LoadBalancerNetworkAttachment `json:"networks"`
	// configuredStatus requests the load balancer to be started (default) or
	// stopped. Changes trigger start/shutdown.
	// +kubebuilder:validation:Enum=started;stopped
	// +kubebuilder:default=started
	ConfiguredStatus string `json:"configuredStatus,omitempty"`
	// Maintenance window.
	Maintenance Maintenance          `json:"maintenance,omitempty"`
	Labels      common.UpCloudLabels `json:"labels,omitempty"`
	// +kubebuilder:default=Delete
	DeletionPolicy common.DeletionPolicy `json:"deletionPolicy,omitempty"`
}

// LoadBalancerStatus defines the observed state of LoadBalancer.
type LoadBalancerStatus struct {
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// UUID of the load balancer in UpCloud.
	UUID string `json:"uuid,omitempty"`
	// Current UpCloud operational state, for example running.
	OperationalState string `json:"operationalState,omitempty"`
	// Networks reported by the API.
	Networks []LBStatusNetwork `json:"networks,omitempty"`
	// Number of nodes the API reports.
	Nodes int `json:"nodes,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Zone",type=string,JSONPath=`.spec.zone`
// +kubebuilder:printcolumn:name="State",type=string,JSONPath=`.status.operationalState`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// LoadBalancer is an UpCloud Load Balancer service.
type LoadBalancer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   LoadBalancerSpec   `json:"spec"`
	Status LoadBalancerStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// LoadBalancerList contains a list of LoadBalancer.
type LoadBalancerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LoadBalancer `json:"items"`
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(SchemeGroupVersion, &LoadBalancer{}, &LoadBalancerList{})
		return nil
	})
}

// GetConditions implements reconciler.Object.
func (s *LoadBalancer) GetConditions() []metav1.Condition { return s.Status.Conditions }

// SetConditions implements reconciler.Object.
func (s *LoadBalancer) SetConditions(c []metav1.Condition) { s.Status.Conditions = c }

// GetDeletionPolicy implements reconciler.Object.
func (s *LoadBalancer) GetDeletionPolicy() common.DeletionPolicy {
	return s.Spec.DeletionPolicy
}

// GetExternalID implements reconciler.Object.
func (s *LoadBalancer) GetExternalID() string { return s.Status.UUID }

// ExternalName is the name used in UpCloud.
func (s *LoadBalancer) ExternalName() string {
	if s.Spec.Name != "" {
		return s.Spec.Name
	}
	return s.Name
}

// DesiredConfiguredStatus reports the desired configured status, started by
// default.
func (s *LoadBalancer) DesiredConfiguredStatus() string {
	if s.Spec.ConfiguredStatus == "" {
		return "started"
	}
	return s.Spec.ConfiguredStatus
}
