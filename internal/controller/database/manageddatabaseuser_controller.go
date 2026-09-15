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

// FinalizerManagedDatabaseUser guards ManagedDatabaseUser deletion.
const FinalizerManagedDatabaseUser = "database.upcloud.polarsquad.com/manageddatabaseuser"

// +kubebuilder:rbac:groups=database.upcloud.polarsquad.com,resources=manageddatabaseusers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=database.upcloud.polarsquad.com,resources=manageddatabaseusers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=database.upcloud.polarsquad.com,resources=manageddatabaseusers/finalizers,verbs=update

// SetupManagedDatabaseUserController registers the ManagedDatabaseUser
// reconciler. Users are re-queued when the ManagedDatabase they reference
// changes.
func SetupManagedDatabaseUserController(mgr ctrl.Manager, api upcloudapi.DatabaseAPI, opts reconciler.Options) error {
	r := &reconciler.Reconciler[*databasev1alpha1.ManagedDatabaseUser]{
		Client:    mgr.GetClient(),
		Adapter:   &ManagedDatabaseUserAdapter{API: api, Client: mgr.GetClient()},
		New:       func() *databasev1alpha1.ManagedDatabaseUser { return &databasev1alpha1.ManagedDatabaseUser{} },
		Finalizer: FinalizerManagedDatabaseUser,
	}
	r.ApplyOptions(opts)
	return ctrl.NewControllerManagedBy(mgr).
		For(&databasev1alpha1.ManagedDatabaseUser{}).
		Watches(&databasev1alpha1.ManagedDatabase{}, handler.EnqueueRequestsFromMapFunc(
			reconciler.DependentsOf(mgr.GetClient(),
				func() *databasev1alpha1.ManagedDatabaseUserList { return &databasev1alpha1.ManagedDatabaseUserList{} },
				func(l *databasev1alpha1.ManagedDatabaseUserList) []*databasev1alpha1.ManagedDatabaseUser {
					out := make([]*databasev1alpha1.ManagedDatabaseUser, 0, len(l.Items))
					for i := range l.Items {
						out = append(out, &l.Items[i])
					}
					return out
				},
				func(u *databasev1alpha1.ManagedDatabaseUser) []string {
					return []string{u.Spec.ServiceRef.Name}
				}))).
		Named("database-manageddatabaseuser").
		Complete(r)
}
