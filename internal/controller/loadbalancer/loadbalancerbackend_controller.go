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

package loadbalancer

import (
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/handler"

	lb "github.com/polarsquad/upcloud-operator/api/loadbalancer/v1alpha1"
	"github.com/polarsquad/upcloud-operator/internal/reconciler"
	"github.com/polarsquad/upcloud-operator/internal/upcloudapi"
)

// FinalizerLoadBalancerBackend guards LoadBalancerBackend deletion.
const FinalizerLoadBalancerBackend = "loadbalancer.upcloud.polarsquad.com/loadbalancerbackend"

// +kubebuilder:rbac:groups=loadbalancer.upcloud.polarsquad.com,resources=loadbalancerbackends,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=loadbalancer.upcloud.polarsquad.com,resources=loadbalancerbackends/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=loadbalancer.upcloud.polarsquad.com,resources=loadbalancerbackends/finalizers,verbs=update

// SetupLoadBalancerBackendController registers the LoadBalancerBackend reconciler.
func SetupLoadBalancerBackendController(mgr ctrl.Manager, api upcloudapi.LoadBalancerAPI) error {
	r := &reconciler.Reconciler[*lb.LoadBalancerBackend]{
		Client:    mgr.GetClient(),
		Adapter:   &LoadBalancerBackendAdapter{API: api, Client: mgr.GetClient()},
		New:       func() *lb.LoadBalancerBackend { return &lb.LoadBalancerBackend{} },
		Finalizer: FinalizerLoadBalancerBackend,
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&lb.LoadBalancerBackend{}).
		Watches(&lb.LoadBalancer{}, handler.EnqueueRequestsFromMapFunc(
			reconciler.DependentsOf(mgr.GetClient(),
				func() *lb.LoadBalancerBackendList {
					return &lb.LoadBalancerBackendList{}
				},
				func(l *lb.LoadBalancerBackendList) []*lb.LoadBalancerBackend {
					out := make([]*lb.LoadBalancerBackend, 0, len(l.Items))
					for i := range l.Items {
						out = append(out, &l.Items[i])
					}
					return out
				},
				func(o *lb.LoadBalancerBackend) []string {
					return o.RefNames()
				}))).
		Named("loadbalancer-loadbalancerbackend").
		Complete(r)
}
