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

package network

import (
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/handler"

	networkv1alpha1 "github.com/polarsquad/upcloud-operator/api/network/v1alpha1"
	"github.com/polarsquad/upcloud-operator/internal/reconciler"
	"github.com/polarsquad/upcloud-operator/internal/upcloudapi"
)

// FinalizerNetwork guards Network deletion.
const FinalizerNetwork = "network.upcloud.polarsquad.com/network"

// +kubebuilder:rbac:groups=network.upcloud.polarsquad.com,resources=networks,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=network.upcloud.polarsquad.com,resources=networks/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=network.upcloud.polarsquad.com,resources=networks/finalizers,verbs=update

// SetupNetworkController registers the Network reconciler. Networks are
// re-queued when the Router they reference changes.
func SetupNetworkController(mgr ctrl.Manager, api upcloudapi.NetworkAPI, opts reconciler.Options) error {
	r := &reconciler.Reconciler[*networkv1alpha1.Network]{
		Client:    mgr.GetClient(),
		Adapter:   &NetworkAdapter{API: api, Client: mgr.GetClient()},
		New:       func() *networkv1alpha1.Network { return &networkv1alpha1.Network{} },
		Finalizer: FinalizerNetwork,
	}
	r.ApplyOptions(opts)
	return ctrl.NewControllerManagedBy(mgr).
		For(&networkv1alpha1.Network{}).
		Watches(&networkv1alpha1.Router{}, handler.EnqueueRequestsFromMapFunc(
			reconciler.DependentsOf(mgr.GetClient(),
				func() *networkv1alpha1.NetworkList { return &networkv1alpha1.NetworkList{} },
				func(l *networkv1alpha1.NetworkList) []*networkv1alpha1.Network {
					out := make([]*networkv1alpha1.Network, 0, len(l.Items))
					for i := range l.Items {
						out = append(out, &l.Items[i])
					}
					return out
				},
				func(n *networkv1alpha1.Network) []string {
					if n.Spec.RouterRef == nil {
						return nil
					}
					return []string{n.Spec.RouterRef.Name}
				}))).
		Named("network-network").
		Complete(r)
}
