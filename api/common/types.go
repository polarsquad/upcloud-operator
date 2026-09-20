package common

// DeletionPolicy decides what happens to the UpCloud resource when the CR is deleted.
// +kubebuilder:validation:Enum=Delete;Orphan
type DeletionPolicy string

const (
	// DeletionPolicyDelete removes the UpCloud resource.
	DeletionPolicyDelete DeletionPolicy = "Delete"
	// DeletionPolicyOrphan leaves the UpCloud resource in place.
	DeletionPolicyOrphan DeletionPolicy = "Orphan"
)

// LocalObjectReference names an object of a known kind in the same namespace.
type LocalObjectReference struct {
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
}

// SecretKeySelector selects a key of a Secret in the same namespace.
type SecretKeySelector struct {
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
	// +kubebuilder:validation:MinLength=1
	Key string `json:"key"`
}

// NetworkAttachment mirrors the UpCloud network object accepted by Managed
// Database, Managed Object Storage and Load Balancer create requests.
type NetworkAttachment struct {
	// Name of the attachment, unique within the service.
	// +kubebuilder:validation:Pattern=`^[a-zA-Z0-9_-]+$`
	// +kubebuilder:validation:MaxLength=64
	Name string `json:"name"`
	// +kubebuilder:validation:Enum=public;private;utility
	Type string `json:"type"`
	// +kubebuilder:default=IPv4
	// +kubebuilder:validation:Enum=IPv4;IPv6
	Family string `json:"family,omitempty"`
	// UUID of an existing UpCloud SDN network. Private attachments need uuid or networkRef.
	UUID string `json:"uuid,omitempty"`
	// NetworkRef points at a Network CR in the same namespace; its status.uuid is used.
	NetworkRef *LocalObjectReference `json:"networkRef,omitempty"`
}

// UpCloudLabels is the user-controlled part of the labels set on an UpCloud resource.
// The operator also sets services.k8s.upcloud/managed-by, /namespace and /uid; a
// value given here overrides the first two but never services.k8s.upcloud/uid.
type UpCloudLabels map[string]string
