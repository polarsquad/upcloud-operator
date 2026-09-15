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

	networkv1alpha1 "github.com/polarsquad/upcloud-operator/api/network/v1alpha1"
	"github.com/polarsquad/upcloud-operator/internal/reconciler"
	"github.com/polarsquad/upcloud-operator/internal/upcloudapi"
)

// FinalizerRouter guards Router deletion.
const FinalizerRouter = "network.upcloud.polarsquad.com/router"

// +kubebuilder:rbac:groups=network.upcloud.polarsquad.com,resources=routers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=network.upcloud.polarsquad.com,resources=routers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=network.upcloud.polarsquad.com,resources=routers/finalizers,verbs=update

// SetupRouterController registers the Router reconciler.
func SetupRouterController(mgr ctrl.Manager, api upcloudapi.NetworkAPI) error {
	r := &reconciler.Reconciler[*networkv1alpha1.Router]{
		Client:    mgr.GetClient(),
		Adapter:   &RouterAdapter{API: api},
		New:       func() *networkv1alpha1.Router { return &networkv1alpha1.Router{} },
		Finalizer: FinalizerRouter,
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&networkv1alpha1.Router{}).
		Named("network-router").
		Complete(r)
}
