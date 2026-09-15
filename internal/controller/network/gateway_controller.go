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

// FinalizerGateway guards Gateway deletion.
const FinalizerGateway = "network.upcloud.polarsquad.com/gateway"

// +kubebuilder:rbac:groups=network.upcloud.polarsquad.com,resources=gateways,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=network.upcloud.polarsquad.com,resources=gateways/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=network.upcloud.polarsquad.com,resources=gateways/finalizers,verbs=update

// SetupGatewayController registers the Gateway reconciler. Gateways are
// re-queued when the Router they reference changes.
func SetupGatewayController(mgr ctrl.Manager, api upcloudapi.GatewayAPI, opts reconciler.Options) error {
	r := &reconciler.Reconciler[*networkv1alpha1.Gateway]{
		Client:    mgr.GetClient(),
		Adapter:   &GatewayAdapter{API: api, Client: mgr.GetClient()},
		New:       func() *networkv1alpha1.Gateway { return &networkv1alpha1.Gateway{} },
		Finalizer: FinalizerGateway,
	}
	r.ApplyOptions(opts)
	return ctrl.NewControllerManagedBy(mgr).
		For(&networkv1alpha1.Gateway{}).
		Watches(&networkv1alpha1.Router{}, handler.EnqueueRequestsFromMapFunc(
			reconciler.DependentsOf(mgr.GetClient(),
				func() *networkv1alpha1.GatewayList { return &networkv1alpha1.GatewayList{} },
				func(l *networkv1alpha1.GatewayList) []*networkv1alpha1.Gateway {
					out := make([]*networkv1alpha1.Gateway, 0, len(l.Items))
					for i := range l.Items {
						out = append(out, &l.Items[i])
					}
					return out
				},
				func(g *networkv1alpha1.Gateway) []string {
					if g.Spec.RouterRef == nil {
						return nil
					}
					return []string{g.Spec.RouterRef.Name}
				},
			)),
		).
		Named("network-gateway").
		Complete(r)
}
