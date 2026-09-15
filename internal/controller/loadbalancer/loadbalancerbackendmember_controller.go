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

// FinalizerLoadBalancerBackendMember guards LoadBalancerBackendMember deletion.
const FinalizerLoadBalancerBackendMember = "loadbalancer.upcloud.polarsquad.com/loadbalancerbackendmember"

// +kubebuilder:rbac:groups=loadbalancer.upcloud.polarsquad.com,resources=loadbalancerbackendmembers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=loadbalancer.upcloud.polarsquad.com,resources=loadbalancerbackendmembers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=loadbalancer.upcloud.polarsquad.com,resources=loadbalancerbackendmembers/finalizers,verbs=update

// SetupLoadBalancerBackendMemberController registers the LoadBalancerBackendMember reconciler.
func SetupLoadBalancerBackendMemberController(mgr ctrl.Manager, api upcloudapi.LoadBalancerAPI) error {
	r := &reconciler.Reconciler[*lb.LoadBalancerBackendMember]{
		Client:    mgr.GetClient(),
		Adapter:   &LoadBalancerBackendMemberAdapter{API: api, Client: mgr.GetClient()},
		New:       func() *lb.LoadBalancerBackendMember { return &lb.LoadBalancerBackendMember{} },
		Finalizer: FinalizerLoadBalancerBackendMember,
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&lb.LoadBalancerBackendMember{}).
		Watches(&lb.LoadBalancerBackend{}, handler.EnqueueRequestsFromMapFunc(
			reconciler.DependentsOf(mgr.GetClient(),
				func() *lb.LoadBalancerBackendMemberList {
					return &lb.LoadBalancerBackendMemberList{}
				},
				func(l *lb.LoadBalancerBackendMemberList) []*lb.LoadBalancerBackendMember {
					out := make([]*lb.LoadBalancerBackendMember, 0, len(l.Items))
					for i := range l.Items {
						out = append(out, &l.Items[i])
					}
					return out
				},
				func(o *lb.LoadBalancerBackendMember) []string {
					return o.RefNames()
				}))).
		Named("loadbalancer-loadbalancerbackendmember").
		Complete(r)
}
