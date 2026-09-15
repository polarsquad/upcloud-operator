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
	ctrl "sigs.k8s.io/controller-runtime"

	databasev1alpha1 "github.com/polarsquad/upcloud-operator/api/database/v1alpha1"
	"github.com/polarsquad/upcloud-operator/internal/reconciler"
	"github.com/polarsquad/upcloud-operator/internal/upcloudapi"
)

// FinalizerManagedDatabase guards ManagedDatabase deletion.
const FinalizerManagedDatabase = "database.upcloud.polarsquad.com/manageddatabase"

// +kubebuilder:rbac:groups=database.upcloud.polarsquad.com,resources=manageddatabases,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=database.upcloud.polarsquad.com,resources=manageddatabases/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=database.upcloud.polarsquad.com,resources=manageddatabases/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch

// SetupManagedDatabaseController registers the ManagedDatabase reconciler.
// The service is polled for state changes; there are no watch sources.
func SetupManagedDatabaseController(mgr ctrl.Manager, api upcloudapi.DatabaseAPI, opts reconciler.Options) error {
	r := &reconciler.Reconciler[*databasev1alpha1.ManagedDatabase]{
		Client:    mgr.GetClient(),
		Adapter:   &ManagedDatabaseAdapter{API: api, Client: mgr.GetClient()},
		New:       func() *databasev1alpha1.ManagedDatabase { return &databasev1alpha1.ManagedDatabase{} },
		Finalizer: FinalizerManagedDatabase,
	}
	r.ApplyOptions(opts)
	return ctrl.NewControllerManagedBy(mgr).
		For(&databasev1alpha1.ManagedDatabase{}).
		Named("database-manageddatabase").
		Complete(r)
}
