/*
Copyright 2026 Polar Squad.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package network

import (
	ctrl "sigs.k8s.io/controller-runtime"

	networkv1alpha1 "github.com/polarsquad/upcloud-operator/api/network/v1alpha1"
	"github.com/polarsquad/upcloud-operator/internal/reconciler"
	"github.com/polarsquad/upcloud-operator/internal/upcloudapi"
)

// FinalizerFloatingIP guards FloatingIP deletion.
const FinalizerFloatingIP = "network.upcloud.polarsquad.com/floatingip"

// +kubebuilder:rbac:groups=network.upcloud.polarsquad.com,resources=floatingips,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=network.upcloud.polarsquad.com,resources=floatingips/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=network.upcloud.polarsquad.com,resources=floatingips/finalizers,verbs=update

// SetupFloatingIPController registers the FloatingIP reconciler.
func SetupFloatingIPController(mgr ctrl.Manager, api upcloudapi.NetworkAPI, opts reconciler.Options) error {
	r := &reconciler.Reconciler[*networkv1alpha1.FloatingIP]{
		Client:    mgr.GetClient(),
		Adapter:   &FloatingIPAdapter{API: api},
		New:       func() *networkv1alpha1.FloatingIP { return &networkv1alpha1.FloatingIP{} },
		Finalizer: FinalizerFloatingIP,
	}
	r.ApplyOptions(opts)
	return ctrl.NewControllerManagedBy(mgr).
		For(&networkv1alpha1.FloatingIP{}).
		Named("network-floatingip").
		Complete(r)
}
