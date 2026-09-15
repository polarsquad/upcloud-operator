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

package objectstorage

import (
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/handler"

	objectstoragev1alpha1 "github.com/polarsquad/upcloud-operator/api/objectstorage/v1alpha1"
	"github.com/polarsquad/upcloud-operator/internal/reconciler"
	"github.com/polarsquad/upcloud-operator/internal/upcloudapi"
)

// FinalizerObjectStorageUser guards ObjectStorageUser deletion.
const FinalizerObjectStorageUser = "objectstorage.upcloud.polarsquad.com/objectstorageuser"

// +kubebuilder:rbac:groups=objectstorage.upcloud.polarsquad.com,resources=objectstorageusers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=objectstorage.upcloud.polarsquad.com,resources=objectstorageusers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=objectstorage.upcloud.polarsquad.com,resources=objectstorageusers/finalizers,verbs=update

// SetupObjectStorageUserController registers the ObjectStorageUser
// reconciler. Users are re-queued when the ManagedObjectStorage they
// reference or the ObjectStoragePolicy CRs they use change.
func SetupObjectStorageUserController(mgr ctrl.Manager, api upcloudapi.ObjectStorageAPI, opts reconciler.Options) error {
	r := &reconciler.Reconciler[*objectstoragev1alpha1.ObjectStorageUser]{
		Client:    mgr.GetClient(),
		Adapter:   &ObjectStorageUserAdapter{API: api, Client: mgr.GetClient()},
		New:       func() *objectstoragev1alpha1.ObjectStorageUser { return &objectstoragev1alpha1.ObjectStorageUser{} },
		Finalizer: FinalizerObjectStorageUser,
	}
	r.ApplyOptions(opts)
	return ctrl.NewControllerManagedBy(mgr).
		For(&objectstoragev1alpha1.ObjectStorageUser{}).
		Watches(&objectstoragev1alpha1.ManagedObjectStorage{}, handler.EnqueueRequestsFromMapFunc(
			reconciler.DependentsOf(mgr.GetClient(),
				func() *objectstoragev1alpha1.ObjectStorageUserList {
					return &objectstoragev1alpha1.ObjectStorageUserList{}
				},
				func(l *objectstoragev1alpha1.ObjectStorageUserList) []*objectstoragev1alpha1.ObjectStorageUser {
					out := make([]*objectstoragev1alpha1.ObjectStorageUser, 0, len(l.Items))
					for i := range l.Items {
						out = append(out, &l.Items[i])
					}
					return out
				},
				func(u *objectstoragev1alpha1.ObjectStorageUser) []string {
					return []string{u.Spec.ServiceRef.Name}
				}))).
		Watches(&objectstoragev1alpha1.ObjectStoragePolicy{}, handler.EnqueueRequestsFromMapFunc(
			reconciler.DependentsOf(mgr.GetClient(),
				func() *objectstoragev1alpha1.ObjectStorageUserList {
					return &objectstoragev1alpha1.ObjectStorageUserList{}
				},
				func(l *objectstoragev1alpha1.ObjectStorageUserList) []*objectstoragev1alpha1.ObjectStorageUser {
					out := make([]*objectstoragev1alpha1.ObjectStorageUser, 0, len(l.Items))
					for i := range l.Items {
						out = append(out, &l.Items[i])
					}
					return out
				},
				func(u *objectstoragev1alpha1.ObjectStorageUser) []string {
					return u.PolicyRefNames()
				}))).
		Named("objectstorage-objectstorageuser").
		Complete(r)
}
