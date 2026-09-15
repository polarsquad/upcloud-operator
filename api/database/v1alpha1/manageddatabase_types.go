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
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/polarsquad/upcloud-operator/api/common"
)

// Maintenance defines the window for automatic maintenance operations.
type Maintenance struct {
	// +kubebuilder:validation:Enum=monday;tuesday;wednesday;thursday;friday;saturday;sunday
	DayOfWeek string `json:"dow,omitempty"`
	// +kubebuilder:validation:Pattern=`^\d{2}:\d{2}:\d{2}$`
	Time string `json:"time,omitempty"`
}

// ManagedDatabaseSpec defines the desired state of an UpCloud Managed Database.
// +kubebuilder:validation:XValidation:rule="!has(self.networks) || self.networks.all(n, n.type != 'private' || has(n.uuid) || has(n.networkRef))",message="private networks need uuid or networkRef"
type ManagedDatabaseSpec struct {
	// Engine, for example pg. Immutable.
	// +kubebuilder:validation:Enum=pg;mysql;valkey;opensearch
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="type is immutable"
	Type string `json:"type"`
	// Plan, for example 3x25.
	// +kubebuilder:validation:MinLength=1
	Plan string `json:"plan"`
	// Zone, for example fi-hel1. Immutable.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="zone is immutable"
	Zone string `json:"zone"`
	// Title in UpCloud. Defaults to metadata.name.
	// +kubebuilder:validation:MaxLength=255
	Title string `json:"title,omitempty"`
	// HostNamePrefix for the database host. Create only. Defaults to metadata.name.
	// +kubebuilder:validation:Pattern=`^[a-z0-9-]*$`
	// +kubebuilder:validation:MaxLength=60
	HostNamePrefix string `json:"hostnamePrefix,omitempty"`
	// Engine properties, for example {"version": "16"}. Free-form object.
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Type=object
	Properties *apiextensionsv1.JSON `json:"properties,omitempty"`
	// Maintenance window, applied only when set.
	Maintenance *Maintenance `json:"maintenance,omitempty"`
	// Network attachments, at most 8.
	// +kubebuilder:validation:MaxItems=8
	Networks []common.NetworkAttachment `json:"networks,omitempty"`
	// +kubebuilder:validation:Minimum=0
	AdditionalDiskSpaceGiB int `json:"additionalDiskSpaceGiB,omitempty"`
	// +kubebuilder:default=false
	TerminationProtection bool `json:"terminationProtection,omitempty"`
	// Powered requests the service to be running (true, default) or
	// stopped (false). Changes trigger Start/Shutdown.
	// +kubebuilder:default=true
	Powered *bool                `json:"powered,omitempty"`
	Labels  common.UpCloudLabels `json:"labels,omitempty"`
	// Name of the Secret holding the primary connection details.
	// Defaults to <metadata.name>-connection.
	ConnectionSecretName string `json:"connectionSecretName,omitempty"`
	// +kubebuilder:default=Delete
	DeletionPolicy common.DeletionPolicy `json:"deletionPolicy,omitempty"`
}

// ManagedDatabaseStatus defines the observed state of ManagedDatabase.
type ManagedDatabaseStatus struct {
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// UUID of the service in UpCloud.
	UUID string `json:"uuid,omitempty"`
	// Current UpCloud state, for example running.
	State string `json:"state,omitempty"`
	// PrimaryHost of the service, from the service URI params.
	PrimaryHost string `json:"primaryHost,omitempty"`
	// PrimaryPort of the service, from the service URI params.
	PrimaryPort int `json:"primaryPort,omitempty"`
	// ServiceURIHost is the host part of the service URI.
	ServiceURIHost string `json:"serviceURIHost,omitempty"`
	// Version from the "version" property, when the API reports one.
	Version string `json:"version,omitempty"`
	// NodeCount reported by the API.
	NodeCount int `json:"nodeCount,omitempty"`
	// Powered reported by the API.
	Powered bool `json:"powered,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Type",type=string,JSONPath=`.spec.type`
// +kubebuilder:printcolumn:name="Plan",type=string,JSONPath=`.spec.plan`
// +kubebuilder:printcolumn:name="Zone",type=string,JSONPath=`.spec.zone`
// +kubebuilder:printcolumn:name="State",type=string,JSONPath=`.status.state`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// ManagedDatabase is an UpCloud Managed Database service.
type ManagedDatabase struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ManagedDatabaseSpec   `json:"spec"`
	Status ManagedDatabaseStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ManagedDatabaseList contains a list of ManagedDatabase.
type ManagedDatabaseList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ManagedDatabase `json:"items"`
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(SchemeGroupVersion, &ManagedDatabase{}, &ManagedDatabaseList{})
		return nil
	})
}

// GetConditions implements reconciler.Object.
func (d *ManagedDatabase) GetConditions() []metav1.Condition { return d.Status.Conditions }

// SetConditions implements reconciler.Object.
func (d *ManagedDatabase) SetConditions(c []metav1.Condition) { d.Status.Conditions = c }

// GetDeletionPolicy implements reconciler.Object.
func (d *ManagedDatabase) GetDeletionPolicy() common.DeletionPolicy { return d.Spec.DeletionPolicy }

// GetExternalID implements reconciler.Object.
func (d *ManagedDatabase) GetExternalID() string { return d.Status.UUID }

// ExternalTitle is the title used in UpCloud.
func (d *ManagedDatabase) ExternalTitle() string {
	if d.Spec.Title != "" {
		return d.Spec.Title
	}
	return d.Name
}

// ExternalHostNamePrefix is the hostname prefix sent at create time.
func (d *ManagedDatabase) ExternalHostNamePrefix() string {
	if d.Spec.HostNamePrefix != "" {
		return d.Spec.HostNamePrefix
	}
	return d.Name
}

// ConnectionSecretName is the name of the connection Secret in the namespace.
func (d *ManagedDatabase) ConnectionSecretName() string {
	if d.Spec.ConnectionSecretName != "" {
		return d.Spec.ConnectionSecretName
	}
	return d.Name + "-connection"
}

// DesiredPowered reports the desired powered state, true by default.
func (d *ManagedDatabase) DesiredPowered() bool {
	if d.Spec.Powered == nil {
		return true
	}
	return *d.Spec.Powered
}
