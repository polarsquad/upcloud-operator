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

// StaticRoute is a user-defined route on a router.
type StaticRoute struct {
	// +kubebuilder:validation:MaxLength=64
	Name string `json:"name,omitempty"`
	// Route is the destination CIDR.
	// +kubebuilder:validation:MinLength=1
	Route string `json:"route"`
	// Nexthop is the next-hop IP address.
	// +kubebuilder:validation:MinLength=1
	Nexthop string `json:"nexthop"`
}

// RouterSpec defines the desired state of an UpCloud SDN router.
type RouterSpec struct {
	// Name in UpCloud. Defaults to metadata.name.
	// +kubebuilder:validation:MaxLength=255
	Name         string               `json:"name,omitempty"`
	StaticRoutes []StaticRoute        `json:"staticRoutes,omitempty"`
	Labels       common.UpCloudLabels `json:"labels,omitempty"`
	// +kubebuilder:default=Delete
	DeletionPolicy common.DeletionPolicy `json:"deletionPolicy,omitempty"`
}

// RouterStatus defines the observed state of Router.
type RouterStatus struct {
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	UUID       string             `json:"uuid,omitempty"`
	// AttachedNetworks lists UUIDs of networks attached in UpCloud.
	AttachedNetworks []string `json:"attachedNetworks,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="UUID",type=string,JSONPath=`.status.uuid`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Reason",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].reason`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Router is an UpCloud SDN router.
type Router struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RouterSpec   `json:"spec,omitempty"`
	Status RouterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// RouterList contains a list of Router.
type RouterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Router `json:"items"`
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(SchemeGroupVersion, &Router{}, &RouterList{})
		return nil
	})
}

// GetConditions implements reconciler.Object.
func (r *Router) GetConditions() []metav1.Condition { return r.Status.Conditions }

// SetConditions implements reconciler.Object.
func (r *Router) SetConditions(c []metav1.Condition) { r.Status.Conditions = c }

// GetDeletionPolicy implements reconciler.Object.
func (r *Router) GetDeletionPolicy() common.DeletionPolicy { return r.Spec.DeletionPolicy }

// GetExternalID implements reconciler.Object.
func (r *Router) GetExternalID() string { return r.Status.UUID }

// ExternalName is the name used in UpCloud.
func (r *Router) ExternalName() string {
	if r.Spec.Name != "" {
		return r.Spec.Name
	}
	return r.Name
}
