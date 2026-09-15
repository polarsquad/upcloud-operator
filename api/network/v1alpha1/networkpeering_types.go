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

// NetworkPeeringSpec defines the desired state of an UpCloud network peering.
// The local network is a Network CR in the same namespace; the peer network is
// referenced by UpCloud UUID so it may live in another account.
type NetworkPeeringSpec struct {
	// Name of the peering in UpCloud. Defaults to metadata.name.
	// +kubebuilder:validation:MaxLength=255
	Name string `json:"name,omitempty"`
	// NetworkRef references the local Network CR in the same namespace.
	// Immutable: changing the local network is a different peering.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="networkRef is immutable"
	NetworkRef common.LocalObjectReference `json:"networkRef"`
	// PeerNetworkUUID is the UpCloud UUID of the peer network. It may be in
	// another account. Immutable: changing the peer is a different peering.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="peerNetworkUUID is immutable"
	PeerNetworkUUID string `json:"peerNetworkUUID"`
	// ConfiguredStatus toggles the peering active or disabled.
	// +kubebuilder:validation:Enum=active;disabled
	// +kubebuilder:default=active
	ConfiguredStatus string               `json:"configuredStatus,omitempty"`
	Labels           common.UpCloudLabels `json:"labels,omitempty"`
	// +kubebuilder:default=Delete
	DeletionPolicy common.DeletionPolicy `json:"deletionPolicy,omitempty"`
}

// NetworkPeeringStatus defines the observed state of NetworkPeering.
type NetworkPeeringStatus struct {
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// UUID of the peering in UpCloud.
	UUID string `json:"uuid,omitempty"`
	// State as reported by the API (active, pending-peer, provisioning, ...).
	State string `json:"state,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="UUID",type=string,JSONPath=`.status.uuid`
// +kubebuilder:printcolumn:name="State",type=string,JSONPath=`.status.state`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// NetworkPeering is an UpCloud network peering between a local Network and a
// peer network (by UUID).
type NetworkPeering struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NetworkPeeringSpec   `json:"spec,omitempty"`
	Status NetworkPeeringStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// NetworkPeeringList contains a list of NetworkPeering.
type NetworkPeeringList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NetworkPeering `json:"items"`
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(SchemeGroupVersion, &NetworkPeering{}, &NetworkPeeringList{})
		return nil
	})
}

// GetConditions implements reconciler.Object.
func (p *NetworkPeering) GetConditions() []metav1.Condition { return p.Status.Conditions }

// SetConditions implements reconciler.Object.
func (p *NetworkPeering) SetConditions(c []metav1.Condition) { p.Status.Conditions = c }

// GetDeletionPolicy implements reconciler.Object.
func (p *NetworkPeering) GetDeletionPolicy() common.DeletionPolicy { return p.Spec.DeletionPolicy }

// GetExternalID implements reconciler.Object.
func (p *NetworkPeering) GetExternalID() string { return p.Status.UUID }

// ExternalName is the name used in UpCloud.
func (p *NetworkPeering) ExternalName() string {
	if p.Spec.Name != "" {
		return p.Spec.Name
	}
	return p.Name
}
