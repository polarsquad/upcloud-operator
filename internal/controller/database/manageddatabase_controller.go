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

package database

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	databasev1alpha1 "github.com/polarsquad/upcloud-operator/api/database/v1alpha1"
)

// ManagedDatabaseReconciler reconciles a ManagedDatabase object.
//
// The adapter-based implementation replaces this scaffold in the follow-up
// task; the RBAC markers below are final for the group and feed
// config/rbac/role.yaml via `make manifests`.
type ManagedDatabaseReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=database.upcloud.polarsquad.com,resources=manageddatabases,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=database.upcloud.polarsquad.com,resources=manageddatabases/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=database.upcloud.polarsquad.com,resources=manageddatabases/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *ManagedDatabaseReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = logf.FromContext(ctx)

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ManagedDatabaseReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&databasev1alpha1.ManagedDatabase{}).
		Named("database-manageddatabase").
		Complete(r)
}
