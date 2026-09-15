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

// FinalizerObjectStorageAccessKey guards ObjectStorageAccessKey deletion.
const FinalizerObjectStorageAccessKey = "objectstorage.upcloud.polarsquad.com/objectstorageaccesskey"

// +kubebuilder:rbac:groups=objectstorage.upcloud.polarsquad.com,resources=objectstorageaccesskeys,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=objectstorage.upcloud.polarsquad.com,resources=objectstorageaccesskeys/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=objectstorage.upcloud.polarsquad.com,resources=objectstorageaccesskeys/finalizers,verbs=update

// SetupObjectStorageAccessKeyController registers the ObjectStorageAccessKey
// reconciler. Keys are re-queued when the ObjectStorageUser they reference
// changes.
func SetupObjectStorageAccessKeyController(mgr ctrl.Manager, api upcloudapi.ObjectStorageAPI) error {
	r := &reconciler.Reconciler[*objectstoragev1alpha1.ObjectStorageAccessKey]{
		Client:  mgr.GetClient(),
		Adapter: &ObjectStorageAccessKeyAdapter{API: api, Client: mgr.GetClient()},
		New: func() *objectstoragev1alpha1.ObjectStorageAccessKey {
			return &objectstoragev1alpha1.ObjectStorageAccessKey{}
		},
		Finalizer: FinalizerObjectStorageAccessKey,
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&objectstoragev1alpha1.ObjectStorageAccessKey{}).
		Watches(&objectstoragev1alpha1.ObjectStorageUser{}, handler.EnqueueRequestsFromMapFunc(
			reconciler.DependentsOf(mgr.GetClient(),
				func() *objectstoragev1alpha1.ObjectStorageAccessKeyList {
					return &objectstoragev1alpha1.ObjectStorageAccessKeyList{}
				},
				func(l *objectstoragev1alpha1.ObjectStorageAccessKeyList) []*objectstoragev1alpha1.ObjectStorageAccessKey {
					out := make([]*objectstoragev1alpha1.ObjectStorageAccessKey, 0, len(l.Items))
					for i := range l.Items {
						out = append(out, &l.Items[i])
					}
					return out
				},
				func(k *objectstoragev1alpha1.ObjectStorageAccessKey) []string {
					return []string{k.Spec.UserRef.Name}
				}))).
		Named("objectstorage-objectstorageaccesskey").
		Complete(r)
}
