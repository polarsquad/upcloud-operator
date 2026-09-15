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

// LoadBalancerResolverSpec defines the desired state of a load balancer
// resolver.
type LoadBalancerResolverSpec struct {
	// LoadBalancerRef points at the owning LoadBalancer. Immutable.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="loadBalancerRef is immutable"
	LoadBalancerRef common.LocalObjectReference `json:"loadBalancerRef"`
	// Name of the resolver in UpCloud. Immutable.
	// +kubebuilder:validation:Pattern=`^[a-zA-Z0-9_-]+$`
	// +kubebuilder:validation:MaxLength=64
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="name is immutable"
	Name string `json:"name,omitempty"`
	// Nameservers, at least one.
	// +kubebuilder:validation:MinItems=1
	Nameservers []string `json:"nameservers"`
	// Retries, how many times to retry a lookup.
	// +kubebuilder:validation:Minimum=0
	Retries int `json:"retries,omitempty"`
	// Timeout for a lookup in seconds.
	// +kubebuilder:validation:Minimum=0
	Timeout int `json:"timeout,omitempty"`
	// TimeoutRetry in seconds.
	// +kubebuilder:validation:Minimum=0
	TimeoutRetry int `json:"timeoutRetry,omitempty"`
	// CacheValid, how long to cache a positive answer, in seconds.
	// +kubebuilder:validation:Minimum=0
	CacheValid int `json:"cacheValid,omitempty"`
	// CacheInvalid, how long to cache a negative answer, in seconds.
	// +kubebuilder:validation:Minimum=0
	CacheInvalid int `json:"cacheInvalid,omitempty"`
	// +kubebuilder:default=Delete
	DeletionPolicy common.DeletionPolicy `json:"deletionPolicy,omitempty"`
}

// LoadBalancerResolverStatus defines the observed state of LoadBalancerResolver.
type LoadBalancerResolverStatus struct {
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// UUID of the parent load balancer.
	ServiceUUID string `json:"serviceUUID,omitempty"`
	// Name of the resolver in UpCloud.
	Name string `json:"name,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// LoadBalancerResolver is a DNS resolver for a load balancer.
type LoadBalancerResolver struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   LoadBalancerResolverSpec   `json:"spec"`
	Status LoadBalancerResolverStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// LoadBalancerResolverList contains a list of LoadBalancerResolver.
type LoadBalancerResolverList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LoadBalancerResolver `json:"items"`
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(SchemeGroupVersion, &LoadBalancerResolver{}, &LoadBalancerResolverList{})
		return nil
	})
}

// GetConditions implements reconciler.Object.
func (r *LoadBalancerResolver) GetConditions() []metav1.Condition { return r.Status.Conditions }

// SetConditions implements reconciler.Object.
func (r *LoadBalancerResolver) SetConditions(c []metav1.Condition) { r.Status.Conditions = c }

// GetDeletionPolicy implements reconciler.Object.
func (r *LoadBalancerResolver) GetDeletionPolicy() common.DeletionPolicy {
	return r.Spec.DeletionPolicy
}

// GetExternalID implements reconciler.Object.
func (r *LoadBalancerResolver) GetExternalID() string { return r.Status.Name }

// ExternalName is the name used in UpCloud.
func (r *LoadBalancerResolver) ExternalName() string {
	if r.Spec.Name != "" {
		return r.Spec.Name
	}
	return r.Name
}

// RefNames returns the referenced LoadBalancer name, for watches.
func (r *LoadBalancerResolver) RefNames() []string {
	return []string{r.Spec.LoadBalancerRef.Name}
}
