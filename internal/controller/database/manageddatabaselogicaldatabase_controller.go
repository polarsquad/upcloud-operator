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
	"sigs.k8s.io/controller-runtime/pkg/handler"

	databasev1alpha1 "github.com/polarsquad/upcloud-operator/api/database/v1alpha1"
	"github.com/polarsquad/upcloud-operator/internal/reconciler"
	"github.com/polarsquad/upcloud-operator/internal/upcloudapi"
)

// FinalizerManagedDatabaseLogicalDatabase guards
// ManagedDatabaseLogicalDatabase deletion.
const FinalizerManagedDatabaseLogicalDatabase = "database.upcloud.polarsquad.com/manageddatabaselogicaldatabase"

// +kubebuilder:rbac:groups=database.upcloud.polarsquad.com,resources=manageddatabaselogicaldatabases,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=database.upcloud.polarsquad.com,resources=manageddatabaselogicaldatabases/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=database.upcloud.polarsquad.com,resources=manageddatabaselogicaldatabases/finalizers,verbs=update

// SetupManagedDatabaseLogicalDatabaseController registers the
// ManagedDatabaseLogicalDatabase reconciler. Logical databases are re-queued
// when the ManagedDatabase they reference changes.
func SetupManagedDatabaseLogicalDatabaseController(mgr ctrl.Manager, api upcloudapi.DatabaseAPI, opts reconciler.Options) error {
	r := &reconciler.Reconciler[*databasev1alpha1.ManagedDatabaseLogicalDatabase]{
		Client:  mgr.GetClient(),
		Adapter: &LogicalDatabaseAdapter{API: api, Client: mgr.GetClient()},
		New: func() *databasev1alpha1.ManagedDatabaseLogicalDatabase {
			return &databasev1alpha1.ManagedDatabaseLogicalDatabase{}
		},
		Finalizer: FinalizerManagedDatabaseLogicalDatabase,
	}
	r.ApplyOptions(opts)
	return ctrl.NewControllerManagedBy(mgr).
		For(&databasev1alpha1.ManagedDatabaseLogicalDatabase{}).
		Watches(&databasev1alpha1.ManagedDatabase{}, handler.EnqueueRequestsFromMapFunc(
			reconciler.DependentsOf(mgr.GetClient(),
				func() *databasev1alpha1.ManagedDatabaseLogicalDatabaseList {
					return &databasev1alpha1.ManagedDatabaseLogicalDatabaseList{}
				},
				func(l *databasev1alpha1.ManagedDatabaseLogicalDatabaseList) []*databasev1alpha1.ManagedDatabaseLogicalDatabase {
					out := make([]*databasev1alpha1.ManagedDatabaseLogicalDatabase, 0, len(l.Items))
					for i := range l.Items {
						out = append(out, &l.Items[i])
					}
					return out
				},
				func(d *databasev1alpha1.ManagedDatabaseLogicalDatabase) []string {
					return []string{d.Spec.ServiceRef.Name}
				}))).
		Named("database-manageddatabaselogicaldatabase").
		Complete(r)
}
