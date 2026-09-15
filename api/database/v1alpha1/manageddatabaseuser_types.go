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

// PGUserAccessControl holds the PostgreSQL access control for a user.
type PGUserAccessControl struct {
	// AllowReplication for the user.
	AllowReplication *bool `json:"allowReplication,omitempty"`
}

// ValkeyUserAccessControl holds the Valkey access control for a user.
type ValkeyUserAccessControl struct {
	// Allowed patterns for ACL categories.
	Categories []string `json:"categories,omitempty"`
	// Allowed patterns for pub/sub channels.
	Channels []string `json:"channels,omitempty"`
	// Allowed commands.
	Commands []string `json:"commands,omitempty"`
	// Allowed key patterns.
	Keys []string `json:"keys,omitempty"`
}

// OpenSearchRule is one index permission rule for an OpenSearch user.
type OpenSearchRule struct {
	// Index the rule applies to.
	Index string `json:"index"`
	// +kubebuilder:validation:Enum=admin;deny;readwrite;read;write
	Permission string `json:"permission"`
}

// OpenSearchUserAccessControl holds the OpenSearch access control for a user.
type OpenSearchUserAccessControl struct {
	// Index permission rules.
	Rules []OpenSearchRule `json:"rules,omitempty"`
}

// ManagedDatabaseUserSpec defines the desired state of a ManagedDatabaseUser.
type ManagedDatabaseUserSpec struct {
	// ServiceRef points at the ManagedDatabase this user belongs to.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="serviceRef is immutable"
	ServiceRef common.LocalObjectReference `json:"serviceRef"`
	// Username in UpCloud. Defaults to metadata.name.
	// +kubebuilder:validation:Pattern=`^[a-zA-Z0-9_-]+$`
	// +kubebuilder:validation:MaxLength=64
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="username is immutable"
	Username string `json:"username,omitempty"`
	// PasswordSecretRef selects the password to set. When nil the API
	// generates one and it is stored in the connection Secret.
	PasswordSecretRef *common.SecretKeySelector `json:"passwordSecretRef,omitempty"`
	// Authentication type, MySQL services only.
	// +kubebuilder:validation:Enum=caching_sha2_password;mysql_native_password
	Authentication string `json:"authentication,omitempty"`
	// PGAccessControl for PostgreSQL services.
	PGAccessControl *PGUserAccessControl `json:"pgAccessControl,omitempty"`
	// ValkeyAccessControl for Valkey services.
	ValkeyAccessControl *ValkeyUserAccessControl `json:"valkeyAccessControl,omitempty"`
	// OpenSearchAccessControl for OpenSearch services.
	OpenSearchAccessControl *OpenSearchUserAccessControl `json:"opensearchAccessControl,omitempty"`
	// Name of the Secret holding the user credentials.
	// Defaults to <metadata.name>-credentials.
	ConnectionSecretName string `json:"connectionSecretName,omitempty"`
	// +kubebuilder:default=Delete
	DeletionPolicy common.DeletionPolicy `json:"deletionPolicy,omitempty"`
}

// ManagedDatabaseUserStatus defines the observed state of ManagedDatabaseUser.
type ManagedDatabaseUserStatus struct {
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// UUID of the parent ManagedDatabase service.
	ServiceUUID string `json:"serviceUUID,omitempty"`
	// Username in UpCloud.
	Username string `json:"username,omitempty"`
	// User type: primary or normal.
	Type string `json:"type,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Service",type=string,JSONPath=`.spec.serviceRef.name`
// +kubebuilder:printcolumn:name="Username",type=string,JSONPath=`.spec.username`
// +kubebuilder:printcolumn:name="Type",type=string,JSONPath=`.status.type`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// ManagedDatabaseUser is a user of an UpCloud Managed Database service.
type ManagedDatabaseUser struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ManagedDatabaseUserSpec   `json:"spec"`
	Status ManagedDatabaseUserStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ManagedDatabaseUserList contains a list of ManagedDatabaseUser.
type ManagedDatabaseUserList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ManagedDatabaseUser `json:"items"`
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(SchemeGroupVersion, &ManagedDatabaseUser{}, &ManagedDatabaseUserList{})
		return nil
	})
}

// GetConditions implements reconciler.Object.
func (u *ManagedDatabaseUser) GetConditions() []metav1.Condition { return u.Status.Conditions }

// SetConditions implements reconciler.Object.
func (u *ManagedDatabaseUser) SetConditions(c []metav1.Condition) { u.Status.Conditions = c }

// GetDeletionPolicy implements reconciler.Object.
func (u *ManagedDatabaseUser) GetDeletionPolicy() common.DeletionPolicy { return u.Spec.DeletionPolicy }

// GetExternalID implements reconciler.Object.
func (u *ManagedDatabaseUser) GetExternalID() string { return u.Status.Username }

// ExternalUsername is the username used in UpCloud.
func (u *ManagedDatabaseUser) ExternalUsername() string {
	if u.Spec.Username != "" {
		return u.Spec.Username
	}
	return u.Name
}

// ConnectionSecretName is the name of the credentials Secret in the namespace.
func (u *ManagedDatabaseUser) ConnectionSecretName() string {
	if u.Spec.ConnectionSecretName != "" {
		return u.Spec.ConnectionSecretName
	}
	return u.Name + "-credentials"
}
