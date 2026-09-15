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

// FinalizerGatewayTunnel guards GatewayTunnel deletion.
const FinalizerGatewayTunnel = "network.upcloud.polarsquad.com/gatewaytunnel"

// +kubebuilder:rbac:groups=network.upcloud.polarsquad.com,resources=gatewaytunnels,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=network.upcloud.polarsquad.com,resources=gatewaytunnels/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=network.upcloud.polarsquad.com,resources=gatewaytunnels/finalizers,verbs=update

// SetupGatewayTunnelController registers the GatewayTunnel reconciler. Tunnels
// are re-queued when the parent GatewayConnection changes.
func SetupGatewayTunnelController(mgr ctrl.Manager, api upcloudapi.GatewayAPI) error {
	r := &reconciler.Reconciler[*networkv1alpha1.GatewayTunnel]{
		Client:    mgr.GetClient(),
		Adapter:   &GatewayTunnelAdapter{API: api, Client: mgr.GetClient()},
		New:       func() *networkv1alpha1.GatewayTunnel { return &networkv1alpha1.GatewayTunnel{} },
		Finalizer: FinalizerGatewayTunnel,
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&networkv1alpha1.GatewayTunnel{}).
		Watches(&networkv1alpha1.GatewayConnection{}, handler.EnqueueRequestsFromMapFunc(
			reconciler.DependentsOf(mgr.GetClient(),
				func() *networkv1alpha1.GatewayTunnelList { return &networkv1alpha1.GatewayTunnelList{} },
				func(l *networkv1alpha1.GatewayTunnelList) []*networkv1alpha1.GatewayTunnel {
					out := make([]*networkv1alpha1.GatewayTunnel, 0, len(l.Items))
					for i := range l.Items {
						out = append(out, &l.Items[i])
					}
					return out
				},
				func(t *networkv1alpha1.GatewayTunnel) []string {
					return []string{t.Spec.ConnectionRef.Name}
				},
			)),
		).
		Named("network-gatewaytunnel").
		Complete(r)
}
