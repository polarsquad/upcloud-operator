/*
Copyright 2026 Polar Squad.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
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

// GatewayTunnelIPSecAuth holds the tunnel's IPSec authentication. The PSK is
// read from a Secret at create time only; the API never returns it, so PSK
// drift is undetectable.
type GatewayTunnelIPSecAuth struct {
	// +kubebuilder:default=psk
	// +kubebuilder:validation:Enum=psk
	Type string `json:"type,omitempty"`
	// PskSecretRef selects the pre-shared key from a Secret.
	PskSecretRef common.SecretKeySelector `json:"pskSecretRef"`
}

// GatewayTunnelIPSec is the tunnel's IPSec configuration.
type GatewayTunnelIPSec struct {
	Authentication            GatewayTunnelIPSecAuth `json:"authentication"`
	RekeyTime                 int                    `json:"rekeyTime,omitempty"`
	ChildRekeyTime            int                    `json:"childRekeyTime,omitempty"`
	DpdDelay                  int                    `json:"dpdDelay,omitempty"`
	DpdTimeout                int                    `json:"dpdTimeout,omitempty"`
	IkeLifetime               int                    `json:"ikeLifetime,omitempty"`
	Phase1Algorithms          []string               `json:"phase1Algorithms,omitempty"`
	Phase1IntegrityAlgorithms []string               `json:"phase1IntegrityAlgorithms,omitempty"`
	Phase1DhGroupNumbers      []int                  `json:"phase1DhGroupNumbers,omitempty"`
	Phase2Algorithms          []string               `json:"phase2Algorithms,omitempty"`
	Phase2IntegrityAlgorithms []string               `json:"phase2IntegrityAlgorithms,omitempty"`
	Phase2DhGroupNumbers      []int                  `json:"phase2DhGroupNumbers,omitempty"`
}

// GatewayTunnelSpec defines the desired state of a gateway tunnel.
type GatewayTunnelSpec struct {
	// ConnectionRef references the parent GatewayConnection. Immutable.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="connectionRef is immutable"
	ConnectionRef common.LocalObjectReference `json:"connectionRef"`

	// Name in UpCloud. Defaults to metadata.name. Immutable.
	// +kubebuilder:validation:MaxLength=255
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="name is immutable"
	Name string `json:"name,omitempty"`

	// LocalAddressName must match a Gateway.status.addresses[].name.
	// +kubebuilder:validation:MinLength=1
	LocalAddressName string `json:"localAddressName"`

	// RemoteAddress is the peer's public IP.
	// +kubebuilder:validation:MinLength=1
	RemoteAddress string `json:"remoteAddress"`

	// +kubebuilder:validation:Optional
	IPSec *GatewayTunnelIPSec `json:"ipsec,omitempty"`

	// +kubebuilder:default=Delete
	DeletionPolicy common.DeletionPolicy `json:"deletionPolicy,omitempty"`
}

// GatewayTunnelStatus defines the observed state of GatewayTunnel.
type GatewayTunnelStatus struct {
	Conditions       []metav1.Condition `json:"conditions,omitempty"`
	GatewayUUID      string             `json:"gatewayUUID,omitempty"`
	ConnectionUUID   string             `json:"connectionUUID,omitempty"`
	UUID             string             `json:"uuid,omitempty"`
	OperationalState string             `json:"operationalState,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Connection",type=string,JSONPath=`.status.connectionUUID`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Reason",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].reason`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// GatewayTunnel is an UpCloud gateway IPSec VPN tunnel.
type GatewayTunnel struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GatewayTunnelSpec   `json:"spec,omitempty"`
	Status GatewayTunnelStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// GatewayTunnelList contains a list of GatewayTunnel.
type GatewayTunnelList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GatewayTunnel `json:"items"`
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(SchemeGroupVersion, &GatewayTunnel{}, &GatewayTunnelList{})
		return nil
	})
}

// GetConditions implements reconciler.Object.
func (t *GatewayTunnel) GetConditions() []metav1.Condition { return t.Status.Conditions }

// SetConditions implements reconciler.Object.
func (t *GatewayTunnel) SetConditions(v []metav1.Condition) { t.Status.Conditions = v }

// GetDeletionPolicy implements reconciler.Object.
func (t *GatewayTunnel) GetDeletionPolicy() common.DeletionPolicy { return t.Spec.DeletionPolicy }

// GetExternalID implements reconciler.Object.
func (t *GatewayTunnel) GetExternalID() string { return t.Status.UUID }

// ExternalName is the name used in UpCloud.
func (t *GatewayTunnel) ExternalName() string {
	if t.Spec.Name != "" {
		return t.Spec.Name
	}
	return t.Name
}
