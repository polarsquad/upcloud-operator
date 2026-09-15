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

// FinalizerLoadBalancerFrontendRule guards LoadBalancerFrontendRule deletion.
const FinalizerLoadBalancerFrontendRule = "loadbalancer.upcloud.polarsquad.com/loadbalancerfrontendrule"

// +kubebuilder:rbac:groups=loadbalancer.upcloud.polarsquad.com,resources=loadbalancerfrontendrules,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=loadbalancer.upcloud.polarsquad.com,resources=loadbalancerfrontendrules/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=loadbalancer.upcloud.polarsquad.com,resources=loadbalancerfrontendrules/finalizers,verbs=update

// SetupLoadBalancerFrontendRuleController registers the LoadBalancerFrontendRule reconciler.
func SetupLoadBalancerFrontendRuleController(mgr ctrl.Manager, api upcloudapi.LoadBalancerAPI, opts reconciler.Options) error {
	r := &reconciler.Reconciler[*lb.LoadBalancerFrontendRule]{
		Client:    mgr.GetClient(),
		Adapter:   &LoadBalancerFrontendRuleAdapter{API: api, Client: mgr.GetClient()},
		New:       func() *lb.LoadBalancerFrontendRule { return &lb.LoadBalancerFrontendRule{} },
		Finalizer: FinalizerLoadBalancerFrontendRule,
	}
	r.ApplyOptions(opts)
	return ctrl.NewControllerManagedBy(mgr).
		For(&lb.LoadBalancerFrontendRule{}).
		Watches(&lb.LoadBalancerFrontend{}, handler.EnqueueRequestsFromMapFunc(
			reconciler.DependentsOf(mgr.GetClient(),
				func() *lb.LoadBalancerFrontendRuleList {
					return &lb.LoadBalancerFrontendRuleList{}
				},
				func(l *lb.LoadBalancerFrontendRuleList) []*lb.LoadBalancerFrontendRule {
					out := make([]*lb.LoadBalancerFrontendRule, 0, len(l.Items))
					for i := range l.Items {
						out = append(out, &l.Items[i])
					}
					return out
				},
				func(o *lb.LoadBalancerFrontendRule) []string {
					return o.RefNames()
				}))).
		Watches(&lb.LoadBalancerBackend{}, handler.EnqueueRequestsFromMapFunc(
			reconciler.DependentsOf(mgr.GetClient(),
				func() *lb.LoadBalancerFrontendRuleList {
					return &lb.LoadBalancerFrontendRuleList{}
				},
				func(l *lb.LoadBalancerFrontendRuleList) []*lb.LoadBalancerFrontendRule {
					out := make([]*lb.LoadBalancerFrontendRule, 0, len(l.Items))
					for i := range l.Items {
						out = append(out, &l.Items[i])
					}
					return out
				},
				func(o *lb.LoadBalancerFrontendRule) []string {
					return o.RefNames()
				}))).
		Named("loadbalancer-loadbalancerfrontendrule").
		Complete(r)
}
