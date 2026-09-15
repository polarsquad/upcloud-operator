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

// GatewayAddress is a named address on the gateway.
type GatewayAddress struct {
	// Name of the address (e.g. "public", "vpn"). Create only.
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
}

// GatewaySpec defines the desired state of an UpCloud network gateway.
// +kubebuilder:validation:XValidation:rule="has(self.routerRef) != has(self.routerUUID)",message="exactly one of routerRef and routerUUID must be set"
type GatewaySpec struct {
	// Name in UpCloud. Defaults to metadata.name.
	// +kubebuilder:validation:Pattern=`^[a-zA-Z0-9_-]+$`
	// +kubebuilder:validation:MaxLength=64
	Name string `json:"name,omitempty"`

	// Zone the gateway runs in. Immutable.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="zone is immutable"
	Zone string `json:"zone"`

	// Plan (e.g. "small"). Immutable.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="plan is immutable"
	Plan string `json:"plan"`

	// Features the gateway must provide. At least one. Immutable.
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:Items:Enum=nat,vpn
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="features are immutable; recreate the Gateway"
	Features []string `json:"features"`

	// RouterRef references a Router CR in the same namespace. Exactly one of
	// routerRef and routerUUID must be set. Immutable.
	RouterRef *common.LocalObjectReference `json:"routerRef,omitempty"`

	// RouterUUID is an explicit UpCloud router UUID. Exactly one of routerRef
	// and routerUUID must be set. Immutable.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="routerUUID is immutable"
	RouterUUID string `json:"routerUUID,omitempty"`

	// ConfiguredStatus is "started" or "stopped".
	// +kubebuilder:default=started
	// +kubebuilder:validation:Enum=started;stopped
	ConfiguredStatus string `json:"configuredStatus"`

	// Addresses are the named addresses to request (e.g. public, vpn).
	// MaxItems 1, create only.
	// +kubebuilder:validation:MaxItems=1
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="addresses are immutable; recreate the Gateway"
	Addresses []GatewayAddress `json:"addresses,omitempty"`

	Labels common.UpCloudLabels `json:"labels,omitempty"`

	// +kubebuilder:default=Delete
	DeletionPolicy common.DeletionPolicy `json:"deletionPolicy,omitempty"`
}

// GatewayStatus defines the observed state of Gateway.
type GatewayStatus struct {
	Conditions       []metav1.Condition `json:"conditions,omitempty"`
	UUID             string             `json:"uuid,omitempty"`
	OperationalState string             `json:"operationalState,omitempty"`
	Addresses        []GatewayAddress   `json:"addresses,omitempty"`
	RouterUUID       string             `json:"routerUUID,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="UUID",type=string,JSONPath=`.status.uuid`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Reason",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].reason`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Gateway is an UpCloud network gateway (NAT / VPN).
type Gateway struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GatewaySpec   `json:"spec,omitempty"`
	Status GatewayStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// GatewayList contains a list of Gateway.
type GatewayList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Gateway `json:"items"`
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(SchemeGroupVersion, &Gateway{}, &GatewayList{})
		return nil
	})
}

// GetConditions implements reconciler.Object.
func (g *Gateway) GetConditions() []metav1.Condition { return g.Status.Conditions }

// SetConditions implements reconciler.Object.
func (g *Gateway) SetConditions(c []metav1.Condition) { g.Status.Conditions = c }

// GetDeletionPolicy implements reconciler.Object.
func (g *Gateway) GetDeletionPolicy() common.DeletionPolicy { return g.Spec.DeletionPolicy }

// GetExternalID implements reconciler.Object.
func (g *Gateway) GetExternalID() string { return g.Status.UUID }

// ExternalName is the name used in UpCloud.
func (g *Gateway) ExternalName() string {
	if g.Spec.Name != "" {
		return g.Spec.Name
	}
	return g.Name
}
