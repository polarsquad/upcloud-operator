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

	lb "github.com/polarsquad/upcloud-operator/api/loadbalancer/v1alpha1"
	"github.com/polarsquad/upcloud-operator/internal/reconciler"
	"github.com/polarsquad/upcloud-operator/internal/upcloudapi"
)

// FinalizerLoadBalancer guards LoadBalancer deletion.
const FinalizerLoadBalancer = "loadbalancer.upcloud.polarsquad.com/loadbalancer"

// +kubebuilder:rbac:groups=loadbalancer.upcloud.polarsquad.com,resources=loadbalancers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=loadbalancer.upcloud.polarsquad.com,resources=loadbalancers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=loadbalancer.upcloud.polarsquad.com,resources=loadbalancers/finalizers,verbs=update

// SetupLoadBalancerController registers the LoadBalancer reconciler.
func SetupLoadBalancerController(mgr ctrl.Manager, api upcloudapi.LoadBalancerAPI, opts reconciler.Options) error {
	r := &reconciler.Reconciler[*lb.LoadBalancer]{
		Client:    mgr.GetClient(),
		Adapter:   &LoadBalancerAdapter{API: api, Client: mgr.GetClient()},
		New:       func() *lb.LoadBalancer { return &lb.LoadBalancer{} },
		Finalizer: FinalizerLoadBalancer,
	}
	r.ApplyOptions(opts)
	return ctrl.NewControllerManagedBy(mgr).
		For(&lb.LoadBalancer{}).
		Named("loadbalancer-loadbalancer").
		Complete(r)
}
