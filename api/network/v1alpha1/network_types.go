package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/polarsquad/upcloud-operator/api/common"
)

// IPNetwork is one subnet of an SDN network.
type IPNetwork struct {
	// Address in CIDR notation, for example 10.0.0.0/24.
	// +kubebuilder:validation:MinLength=1
	Address string `json:"address"`
	// +kubebuilder:default=IPv4
	// +kubebuilder:validation:Enum=IPv4;IPv6
	Family string `json:"family,omitempty"`
	// +kubebuilder:default=true
	DHCP *bool `json:"dhcp,omitempty"`
	// +kubebuilder:default=false
	DHCPDefaultRoute *bool `json:"dhcpDefaultRoute,omitempty"`
	DHCPDns          []string `json:"dhcpDns,omitempty"`
	DHCPRoutes       []string `json:"dhcpRoutes,omitempty"`
	// Gateway address. UpCloud picks one when omitted.
	Gateway string `json:"gateway,omitempty"`
}

// NetworkSpec defines the desired state of an UpCloud SDN private network.
type NetworkSpec struct {
	// Name in UpCloud. Defaults to metadata.name.
	// +kubebuilder:validation:MaxLength=255
	Name string `json:"name,omitempty"`
	// Zone, for example fi-hel1. Immutable.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="zone is immutable"
	Zone string `json:"zone"`
	// +kubebuilder:validation:MinItems=1
	IPNetworks []IPNetwork `json:"ipNetworks"`
	// RouterRef attaches the network to a Router CR in the same namespace.
	RouterRef *common.LocalObjectReference `json:"routerRef,omitempty"`
	// RouterUUID attaches the network to an existing UpCloud router by UUID.
	RouterUUID string `json:"routerUUID,omitempty"`
	Labels     common.UpCloudLabels `json:"labels,omitempty"`
	// +kubebuilder:default=Delete
	DeletionPolicy common.DeletionPolicy `json:"deletionPolicy,omitempty"`
}

// NetworkStatus defines the observed state of Network.
type NetworkStatus struct {
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// UUID of the network in UpCloud.
	UUID string `json:"uuid,omitempty"`
	// RouterUUID currently attached in UpCloud.
	RouterUUID string `json:"routerUUID,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Zone",type=string,JSONPath=`.spec.zone`
// +kubebuilder:printcolumn:name="UUID",type=string,JSONPath=`.status.uuid`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Reason",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].reason`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Network is an UpCloud SDN private network.
type Network struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NetworkSpec   `json:"spec"`
	Status NetworkStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// NetworkList contains a list of Network.
type NetworkList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Network `json:"items"`
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(SchemeGroupVersion, &Network{}, &NetworkList{})
		return nil
	})
}

// GetConditions implements reconciler.Object.
func (n *Network) GetConditions() []metav1.Condition { return n.Status.Conditions }

// SetConditions implements reconciler.Object.
func (n *Network) SetConditions(c []metav1.Condition) { n.Status.Conditions = c }

// GetDeletionPolicy implements reconciler.Object.
func (n *Network) GetDeletionPolicy() common.DeletionPolicy { return n.Spec.DeletionPolicy }

// GetExternalID implements reconciler.Object.
func (n *Network) GetExternalID() string { return n.Status.UUID }

// ExternalName is the name used in UpCloud.
func (n *Network) ExternalName() string {
	if n.Spec.Name != "" {
		return n.Spec.Name
	}
	return n.Name
}
