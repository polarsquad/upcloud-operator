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

// FinalizerGatewayConnection guards GatewayConnection deletion.
const FinalizerGatewayConnection = "network.upcloud.polarsquad.com/gatewayconnection"

// +kubebuilder:rbac:groups=network.upcloud.polarsquad.com,resources=gatewayconnections,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=network.upcloud.polarsquad.com,resources=gatewayconnections/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=network.upcloud.polarsquad.com,resources=gatewayconnections/finalizers,verbs=update

// SetupGatewayConnectionController registers the GatewayConnection reconciler.
// Connections are re-queued when the parent Gateway changes.
func SetupGatewayConnectionController(mgr ctrl.Manager, api upcloudapi.GatewayAPI, opts reconciler.Options) error {
	r := &reconciler.Reconciler[*networkv1alpha1.GatewayConnection]{
		Client:    mgr.GetClient(),
		Adapter:   &GatewayConnectionAdapter{API: api, Client: mgr.GetClient()},
		New:       func() *networkv1alpha1.GatewayConnection { return &networkv1alpha1.GatewayConnection{} },
		Finalizer: FinalizerGatewayConnection,
	}
	r.ApplyOptions(opts)
	return ctrl.NewControllerManagedBy(mgr).
		For(&networkv1alpha1.GatewayConnection{}).
		Watches(&networkv1alpha1.Gateway{}, handler.EnqueueRequestsFromMapFunc(
			reconciler.DependentsOf(mgr.GetClient(),
				func() *networkv1alpha1.GatewayConnectionList { return &networkv1alpha1.GatewayConnectionList{} },
				func(l *networkv1alpha1.GatewayConnectionList) []*networkv1alpha1.GatewayConnection {
					out := make([]*networkv1alpha1.GatewayConnection, 0, len(l.Items))
					for i := range l.Items {
						out = append(out, &l.Items[i])
					}
					return out
				},
				func(c *networkv1alpha1.GatewayConnection) []string {
					return []string{c.Spec.GatewayRef.Name}
				},
			)),
		).
		Named("network-gatewayconnection").
		Complete(r)
}
