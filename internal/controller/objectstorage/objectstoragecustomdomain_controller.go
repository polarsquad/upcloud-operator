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

// FinalizerObjectStorageCustomDomain guards ObjectStorageCustomDomain
// deletion.
const FinalizerObjectStorageCustomDomain = "objectstorage.upcloud.polarsquad.com/objectstoragecustomdomain"

// +kubebuilder:rbac:groups=objectstorage.upcloud.polarsquad.com,resources=objectstoragecustomdomains,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=objectstorage.upcloud.polarsquad.com,resources=objectstoragecustomdomains/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=objectstorage.upcloud.polarsquad.com,resources=objectstoragecustomdomains/finalizers,verbs=update

// SetupObjectStorageCustomDomainController registers the
// ObjectStorageCustomDomain reconciler. Custom domains are re-queued when the
// ManagedObjectStorage they reference changes.
func SetupObjectStorageCustomDomainController(mgr ctrl.Manager, api upcloudapi.ObjectStorageAPI) error {
	r := &reconciler.Reconciler[*objectstoragev1alpha1.ObjectStorageCustomDomain]{
		Client:  mgr.GetClient(),
		Adapter: &ObjectStorageCustomDomainAdapter{API: api, Client: mgr.GetClient()},
		New: func() *objectstoragev1alpha1.ObjectStorageCustomDomain {
			return &objectstoragev1alpha1.ObjectStorageCustomDomain{}
		},
		Finalizer: FinalizerObjectStorageCustomDomain,
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&objectstoragev1alpha1.ObjectStorageCustomDomain{}).
		Watches(&objectstoragev1alpha1.ManagedObjectStorage{}, handler.EnqueueRequestsFromMapFunc(
			reconciler.DependentsOf(mgr.GetClient(),
				func() *objectstoragev1alpha1.ObjectStorageCustomDomainList {
					return &objectstoragev1alpha1.ObjectStorageCustomDomainList{}
				},
				func(l *objectstoragev1alpha1.ObjectStorageCustomDomainList) []*objectstoragev1alpha1.ObjectStorageCustomDomain {
					out := make([]*objectstoragev1alpha1.ObjectStorageCustomDomain, 0, len(l.Items))
					for i := range l.Items {
						out = append(out, &l.Items[i])
					}
					return out
				},
				func(d *objectstoragev1alpha1.ObjectStorageCustomDomain) []string {
					return []string{d.Spec.ServiceRef.Name}
				}))).
		Named("objectstorage-objectstoragecustomdomain").
		Complete(r)
}
