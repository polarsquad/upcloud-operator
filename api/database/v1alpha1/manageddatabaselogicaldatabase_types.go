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

// ManagedDatabaseLogicalDatabaseSpec defines the desired state of a
// ManagedDatabaseLogicalDatabase.
type ManagedDatabaseLogicalDatabaseSpec struct {
	// ServiceRef points at the ManagedDatabase this logical database
	// belongs to.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="serviceRef is immutable"
	ServiceRef common.LocalObjectReference `json:"serviceRef"`
	// Name in UpCloud. Defaults to metadata.name.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="name is immutable"
	Name string `json:"name,omitempty"`
	// LCCollate is the default string sort order. Create only.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="lcCollate is immutable"
	LCCollate string `json:"lcCollate,omitempty"`
	// LCCType is the default character classification. Create only.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="lcCtype is immutable"
	LCCType string `json:"lcCtype,omitempty"`
	// +kubebuilder:default=Delete
	DeletionPolicy common.DeletionPolicy `json:"deletionPolicy,omitempty"`
}

// ManagedDatabaseLogicalDatabaseStatus defines the observed state of
// ManagedDatabaseLogicalDatabase.
type ManagedDatabaseLogicalDatabaseStatus struct {
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// UUID of the parent ManagedDatabase service.
	ServiceUUID string `json:"serviceUUID,omitempty"`
	// Name in UpCloud.
	Name string `json:"name,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Service",type=string,JSONPath=`.spec.serviceRef.name`
// +kubebuilder:printcolumn:name="Name",type=string,JSONPath=`.status.name`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// ManagedDatabaseLogicalDatabase is a logical database of an UpCloud
// Managed Database service.
type ManagedDatabaseLogicalDatabase struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ManagedDatabaseLogicalDatabaseSpec   `json:"spec"`
	Status ManagedDatabaseLogicalDatabaseStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ManagedDatabaseLogicalDatabaseList contains a list of
// ManagedDatabaseLogicalDatabase.
type ManagedDatabaseLogicalDatabaseList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ManagedDatabaseLogicalDatabase `json:"items"`
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(SchemeGroupVersion, &ManagedDatabaseLogicalDatabase{}, &ManagedDatabaseLogicalDatabaseList{})
		return nil
	})
}

// GetConditions implements reconciler.Object.
func (d *ManagedDatabaseLogicalDatabase) GetConditions() []metav1.Condition {
	return d.Status.Conditions
}

// SetConditions implements reconciler.Object.
func (d *ManagedDatabaseLogicalDatabase) SetConditions(c []metav1.Condition) { d.Status.Conditions = c }

// GetDeletionPolicy implements reconciler.Object.
func (d *ManagedDatabaseLogicalDatabase) GetDeletionPolicy() common.DeletionPolicy {
	return d.Spec.DeletionPolicy
}

// GetExternalID implements reconciler.Object.
func (d *ManagedDatabaseLogicalDatabase) GetExternalID() string { return d.Status.Name }

// ExternalName is the logical database name used in UpCloud.
func (d *ManagedDatabaseLogicalDatabase) ExternalName() string {
	if d.Spec.Name != "" {
		return d.Spec.Name
	}
	return d.Name
}
