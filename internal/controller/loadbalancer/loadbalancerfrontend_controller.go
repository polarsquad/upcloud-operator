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

// FinalizerLoadBalancerFrontend guards LoadBalancerFrontend deletion.
const FinalizerLoadBalancerFrontend = "loadbalancer.upcloud.polarsquad.com/loadbalancerfrontend"

// +kubebuilder:rbac:groups=loadbalancer.upcloud.polarsquad.com,resources=loadbalancerfrontends,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=loadbalancer.upcloud.polarsquad.com,resources=loadbalancerfrontends/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=loadbalancer.upcloud.polarsquad.com,resources=loadbalancerfrontends/finalizers,verbs=update

// SetupLoadBalancerFrontendController registers the LoadBalancerFrontend reconciler.
func SetupLoadBalancerFrontendController(mgr ctrl.Manager, api upcloudapi.LoadBalancerAPI, opts reconciler.Options) error {
	r := &reconciler.Reconciler[*lb.LoadBalancerFrontend]{
		Client:    mgr.GetClient(),
		Adapter:   &LoadBalancerFrontendAdapter{API: api, Client: mgr.GetClient()},
		New:       func() *lb.LoadBalancerFrontend { return &lb.LoadBalancerFrontend{} },
		Finalizer: FinalizerLoadBalancerFrontend,
	}
	r.ApplyOptions(opts)
	return ctrl.NewControllerManagedBy(mgr).
		For(&lb.LoadBalancerFrontend{}).
		Watches(&lb.LoadBalancer{}, handler.EnqueueRequestsFromMapFunc(
			reconciler.DependentsOf(mgr.GetClient(),
				func() *lb.LoadBalancerFrontendList {
					return &lb.LoadBalancerFrontendList{}
				},
				func(l *lb.LoadBalancerFrontendList) []*lb.LoadBalancerFrontend {
					out := make([]*lb.LoadBalancerFrontend, 0, len(l.Items))
					for i := range l.Items {
						out = append(out, &l.Items[i])
					}
					return out
				},
				func(o *lb.LoadBalancerFrontend) []string {
					return o.RefNames()
				}))).
		Watches(&lb.LoadBalancerBackend{}, handler.EnqueueRequestsFromMapFunc(
			reconciler.DependentsOf(mgr.GetClient(),
				func() *lb.LoadBalancerFrontendList {
					return &lb.LoadBalancerFrontendList{}
				},
				func(l *lb.LoadBalancerFrontendList) []*lb.LoadBalancerFrontend {
					out := make([]*lb.LoadBalancerFrontend, 0, len(l.Items))
					for i := range l.Items {
						out = append(out, &l.Items[i])
					}
					return out
				},
				func(o *lb.LoadBalancerFrontend) []string {
					return o.RefNames()
				}))).
		Named("loadbalancer-loadbalancerfrontend").
		Complete(r)
}
