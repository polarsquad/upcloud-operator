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

// GatewayRoute is a local or remote route on a gateway connection.
type GatewayRoute struct {
	// +kubebuilder:validation:MaxLength=64
	Name string `json:"name,omitempty"`
	// StaticNetwork is the destination CIDR.
	// +kubebuilder:validation:MinLength=1
	StaticNetwork string `json:"staticNetwork"`
	// +kubebuilder:default=static
	// +kubebuilder:validation:Enum=static
	Type string `json:"type,omitempty"`
}

// GatewayConnectionSpec defines the desired state of a gateway connection.
type GatewayConnectionSpec struct {
	// GatewayRef references the parent Gateway in the same namespace.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="gatewayRef is immutable"
	GatewayRef common.LocalObjectReference `json:"gatewayRef"`

	// Name in UpCloud. Defaults to metadata.name. Immutable.
	// +kubebuilder:validation:MaxLength=255
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="name is immutable"
	Name string `json:"name,omitempty"`

	// +kubebuilder:default=ipsec
	// +kubebuilder:validation:Enum=ipsec
	Type string `json:"type,omitempty"`

	LocalRoutes  []GatewayRoute `json:"localRoutes,omitempty"`
	RemoteRoutes []GatewayRoute `json:"remoteRoutes,omitempty"`

	// +kubebuilder:default=Delete
	DeletionPolicy common.DeletionPolicy `json:"deletionPolicy,omitempty"`
}

// GatewayConnectionStatus defines the observed state of GatewayConnection.
type GatewayConnectionStatus struct {
	Conditions  []metav1.Condition `json:"conditions,omitempty"`
	GatewayUUID string             `json:"gatewayUUID,omitempty"`
	UUID        string             `json:"uuid,omitempty"`
	Name        string             `json:"name,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Gateway",type=string,JSONPath=`.status.gatewayUUID`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Reason",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].reason`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// GatewayConnection is an UpCloud gateway connection (VPN).
type GatewayConnection struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GatewayConnectionSpec   `json:"spec,omitempty"`
	Status GatewayConnectionStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// GatewayConnectionList contains a list of GatewayConnection.
type GatewayConnectionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GatewayConnection `json:"items"`
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(SchemeGroupVersion, &GatewayConnection{}, &GatewayConnectionList{})
		return nil
	})
}

// GetConditions implements reconciler.Object.
func (c *GatewayConnection) GetConditions() []metav1.Condition { return c.Status.Conditions }

// SetConditions implements reconciler.Object.
func (c *GatewayConnection) SetConditions(v []metav1.Condition) { c.Status.Conditions = v }

// GetDeletionPolicy implements reconciler.Object.
func (c *GatewayConnection) GetDeletionPolicy() common.DeletionPolicy { return c.Spec.DeletionPolicy }

// GetExternalID implements reconciler.Object.
func (c *GatewayConnection) GetExternalID() string { return c.Status.UUID }

// ExternalName is the name used in UpCloud.
func (c *GatewayConnection) ExternalName() string {
	if c.Spec.Name != "" {
		return c.Spec.Name
	}
	return c.Name
}
