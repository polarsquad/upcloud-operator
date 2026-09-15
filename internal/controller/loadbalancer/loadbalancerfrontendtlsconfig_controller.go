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

// FinalizerLoadBalancerFrontendTLSConfig guards LoadBalancerFrontendTLSConfig deletion.
const FinalizerLoadBalancerFrontendTLSConfig = "loadbalancer.upcloud.polarsquad.com/loadbalancerfrontendtlsconfig"

// +kubebuilder:rbac:groups=loadbalancer.upcloud.polarsquad.com,resources=loadbalancerfrontendtlsconfigs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=loadbalancer.upcloud.polarsquad.com,resources=loadbalancerfrontendtlsconfigs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=loadbalancer.upcloud.polarsquad.com,resources=loadbalancerfrontendtlsconfigs/finalizers,verbs=update

// SetupLoadBalancerFrontendTLSConfigController registers the LoadBalancerFrontendTLSConfig reconciler.
func SetupLoadBalancerFrontendTLSConfigController(mgr ctrl.Manager, api upcloudapi.LoadBalancerAPI) error {
	r := &reconciler.Reconciler[*lb.LoadBalancerFrontendTLSConfig]{
		Client:    mgr.GetClient(),
		Adapter:   &LoadBalancerFrontendTLSConfigAdapter{API: api, Client: mgr.GetClient()},
		New:       func() *lb.LoadBalancerFrontendTLSConfig { return &lb.LoadBalancerFrontendTLSConfig{} },
		Finalizer: FinalizerLoadBalancerFrontendTLSConfig,
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&lb.LoadBalancerFrontendTLSConfig{}).
		Watches(&lb.LoadBalancerFrontend{}, handler.EnqueueRequestsFromMapFunc(
			reconciler.DependentsOf(mgr.GetClient(),
				func() *lb.LoadBalancerFrontendTLSConfigList {
					return &lb.LoadBalancerFrontendTLSConfigList{}
				},
				func(l *lb.LoadBalancerFrontendTLSConfigList) []*lb.LoadBalancerFrontendTLSConfig {
					out := make([]*lb.LoadBalancerFrontendTLSConfig, 0, len(l.Items))
					for i := range l.Items {
						out = append(out, &l.Items[i])
					}
					return out
				},
				func(o *lb.LoadBalancerFrontendTLSConfig) []string {
					return o.RefNames()
				}))).
		Watches(&lb.LoadBalancerCertificateBundle{}, handler.EnqueueRequestsFromMapFunc(
			reconciler.DependentsOf(mgr.GetClient(),
				func() *lb.LoadBalancerFrontendTLSConfigList {
					return &lb.LoadBalancerFrontendTLSConfigList{}
				},
				func(l *lb.LoadBalancerFrontendTLSConfigList) []*lb.LoadBalancerFrontendTLSConfig {
					out := make([]*lb.LoadBalancerFrontendTLSConfig, 0, len(l.Items))
					for i := range l.Items {
						out = append(out, &l.Items[i])
					}
					return out
				},
				func(o *lb.LoadBalancerFrontendTLSConfig) []string {
					return o.RefNames()
				}))).
		Named("loadbalancer-loadbalancerfrontendtlsconfig").
		Complete(r)
}
