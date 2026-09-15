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

package network

import (
	ctrl "sigs.k8s.io/controller-runtime"

	networkv1alpha1 "github.com/polarsquad/upcloud-operator/api/network/v1alpha1"
	"github.com/polarsquad/upcloud-operator/internal/reconciler"
	"github.com/polarsquad/upcloud-operator/internal/upcloudapi"
)

// FinalizerNetworkPeering guards NetworkPeering deletion.
const FinalizerNetworkPeering = "network.upcloud.polarsquad.com/networkpeering"

// +kubebuilder:rbac:groups=network.upcloud.polarsquad.com,resources=networkpeerings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=network.upcloud.polarsquad.com,resources=networkpeerings/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=network.upcloud.polarsquad.com,resources=networkpeerings/finalizers,verbs=update

// SetupNetworkPeeringController registers the NetworkPeering reconciler.
func SetupNetworkPeeringController(mgr ctrl.Manager, api upcloudapi.NetworkAPI, opts reconciler.Options) error {
	r := &reconciler.Reconciler[*networkv1alpha1.NetworkPeering]{
		Client:    mgr.GetClient(),
		Adapter:   &NetworkPeeringAdapter{API: api, Client: mgr.GetClient()},
		New:       func() *networkv1alpha1.NetworkPeering { return &networkv1alpha1.NetworkPeering{} },
		Finalizer: FinalizerNetworkPeering,
	}
	r.ApplyOptions(opts)
	return ctrl.NewControllerManagedBy(mgr).
		For(&networkv1alpha1.NetworkPeering{}).
		Named("network-networkpeering").
		Complete(r)
}
