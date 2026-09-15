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

	objectstoragev1alpha1 "github.com/polarsquad/upcloud-operator/api/objectstorage/v1alpha1"
	"github.com/polarsquad/upcloud-operator/internal/reconciler"
	"github.com/polarsquad/upcloud-operator/internal/upcloudapi"
)

// FinalizerManagedObjectStorage guards ManagedObjectStorage deletion.
const FinalizerManagedObjectStorage = "objectstorage.upcloud.polarsquad.com/managedobjectstorage"

// +kubebuilder:rbac:groups=objectstorage.upcloud.polarsquad.com,resources=managedobjectstorages,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=objectstorage.upcloud.polarsquad.com,resources=managedobjectstorages/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=objectstorage.upcloud.polarsquad.com,resources=managedobjectstorages/finalizers,verbs=update

// SetupManagedObjectStorageController registers the ManagedObjectStorage
// reconciler.
func SetupManagedObjectStorageController(mgr ctrl.Manager, api upcloudapi.ObjectStorageAPI) error {
	r := &reconciler.Reconciler[*objectstoragev1alpha1.ManagedObjectStorage]{
		Client:  mgr.GetClient(),
		Adapter: &ManagedObjectStorageAdapter{API: api, Client: mgr.GetClient()},
		New: func() *objectstoragev1alpha1.ManagedObjectStorage {
			return &objectstoragev1alpha1.ManagedObjectStorage{}
		},
		Finalizer: FinalizerManagedObjectStorage,
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&objectstoragev1alpha1.ManagedObjectStorage{}).
		Named("objectstorage-managedobjectstorage").
		Complete(r)
}
