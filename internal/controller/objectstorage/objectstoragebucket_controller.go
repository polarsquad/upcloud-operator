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

// FinalizerObjectStorageBucket guards ObjectStorageBucket deletion.
const FinalizerObjectStorageBucket = "objectstorage.upcloud.polarsquad.com/objectstoragebucket"

// +kubebuilder:rbac:groups=objectstorage.upcloud.polarsquad.com,resources=objectstoragebuckets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=objectstorage.upcloud.polarsquad.com,resources=objectstoragebuckets/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=objectstorage.upcloud.polarsquad.com,resources=objectstoragebuckets/finalizers,verbs=update

// SetupObjectStorageBucketController registers the ObjectStorageBucket
// reconciler. Buckets are re-queued when the ManagedObjectStorage they
// reference changes.
func SetupObjectStorageBucketController(mgr ctrl.Manager, api upcloudapi.ObjectStorageAPI) error {
	r := &reconciler.Reconciler[*objectstoragev1alpha1.ObjectStorageBucket]{
		Client:    mgr.GetClient(),
		Adapter:   &ObjectStorageBucketAdapter{API: api, Client: mgr.GetClient()},
		New:       func() *objectstoragev1alpha1.ObjectStorageBucket { return &objectstoragev1alpha1.ObjectStorageBucket{} },
		Finalizer: FinalizerObjectStorageBucket,
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&objectstoragev1alpha1.ObjectStorageBucket{}).
		Watches(&objectstoragev1alpha1.ManagedObjectStorage{}, handler.EnqueueRequestsFromMapFunc(
			reconciler.DependentsOf(mgr.GetClient(),
				func() *objectstoragev1alpha1.ObjectStorageBucketList {
					return &objectstoragev1alpha1.ObjectStorageBucketList{}
				},
				func(l *objectstoragev1alpha1.ObjectStorageBucketList) []*objectstoragev1alpha1.ObjectStorageBucket {
					out := make([]*objectstoragev1alpha1.ObjectStorageBucket, 0, len(l.Items))
					for i := range l.Items {
						out = append(out, &l.Items[i])
					}
					return out
				},
				func(b *objectstoragev1alpha1.ObjectStorageBucket) []string {
					return []string{b.Spec.ServiceRef.Name}
				}))).
		Named("objectstorage-objectstoragebucket").
		Complete(r)
}
