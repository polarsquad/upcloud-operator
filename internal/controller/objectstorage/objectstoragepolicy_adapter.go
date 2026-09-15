package objectstorage

import (
	"context"
	"fmt"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud/request"
	"sigs.k8s.io/controller-runtime/pkg/client"

	objectstoragev1alpha1 "github.com/polarsquad/upcloud-operator/api/objectstorage/v1alpha1"
	"github.com/polarsquad/upcloud-operator/internal/reconciler"
	"github.com/polarsquad/upcloud-operator/internal/resolve"
	"github.com/polarsquad/upcloud-operator/internal/upcloudapi"
)

// ObjectStoragePolicyAdapter maps ObjectStoragePolicy onto the UpCloud object
// storage policy API. The policy document is immutable in v0.1 (the API only
// offers versioning), so the adapter never modifies it.
type ObjectStoragePolicyAdapter struct {
	API    upcloudapi.ObjectStorageAPI
	Client client.Client
}

var _ reconciler.Adapter[*objectstoragev1alpha1.ObjectStoragePolicy] = (*ObjectStoragePolicyAdapter)(nil)

// serviceUUID resolves the referenced ManagedObjectStorage (which must be
// Ready) on first use and stores the result in Status.
func (a *ObjectStoragePolicyAdapter) serviceUUID(ctx context.Context, p *objectstoragev1alpha1.ObjectStoragePolicy) (string, error) {
	if p.Status.ServiceUUID != "" {
		return p.Status.ServiceUUID, nil
	}
	uuid, err := resolve.Ready(ctx, a.Client, p.Namespace, p.Spec.ServiceRef.Name, &objectstoragev1alpha1.ManagedObjectStorage{})
	if err != nil {
		return "", err
	}
	p.Status.ServiceUUID = uuid
	return uuid, nil
}

// Observe implements reconciler.Adapter.
func (a *ObjectStoragePolicyAdapter) Observe(ctx context.Context, p *objectstoragev1alpha1.ObjectStoragePolicy) (reconciler.Observation, error) {
	svcUUID, err := a.serviceUUID(ctx, p)
	if err != nil {
		return reconciler.Observation{}, err
	}
	pol, err := a.API.GetManagedObjectStoragePolicy(ctx, &request.GetManagedObjectStoragePolicyRequest{ServiceUUID: svcUUID, Name: p.ExternalName()})
	if upcloudapi.IsNotFound(err) {
		return reconciler.Observation{}, nil
	}
	if err != nil {
		return reconciler.Observation{}, fmt.Errorf("get object storage policy: %w", err)
	}
	p.Status.Name = pol.Name
	p.Status.ARN = pol.ARN
	p.Status.DefaultVersionID = pol.DefaultVersionID
	// The document is immutable in v0.1, so existence means up to date.
	return reconciler.Observation{Exists: true, UpToDate: true, Ready: true, Message: "policy " + pol.Name}, nil
}

// Create implements reconciler.Adapter. A Conflict (policy already exists in
// UpCloud) is treated as adoption: the next Observe reconciles it.
func (a *ObjectStoragePolicyAdapter) Create(ctx context.Context, p *objectstoragev1alpha1.ObjectStoragePolicy) error {
	svcUUID, err := a.serviceUUID(ctx, p)
	if err != nil {
		return err
	}
	created, err := a.API.CreateManagedObjectStoragePolicy(ctx, &request.CreateManagedObjectStoragePolicyRequest{
		ServiceUUID: svcUUID,
		Name:        p.ExternalName(),
		Description: p.Spec.Description,
		Document:    p.Spec.Document,
	})
	if err != nil {
		if upcloudapi.IsConflict(err) {
			return nil // adopted; the next Observe reconciles it
		}
		return fmt.Errorf("create object storage policy: %w", err)
	}
	p.Status.Name = created.Name
	p.Status.ARN = created.ARN
	p.Status.DefaultVersionID = created.DefaultVersionID
	return nil
}

// Update implements reconciler.Adapter. The policy document is immutable in
// v0.1, so there is nothing to modify; a missing policy is pending.
func (a *ObjectStoragePolicyAdapter) Update(ctx context.Context, p *objectstoragev1alpha1.ObjectStoragePolicy) error {
	svcUUID, err := a.serviceUUID(ctx, p)
	if err != nil {
		return err
	}
	if _, err := a.API.GetManagedObjectStoragePolicy(ctx, &request.GetManagedObjectStoragePolicyRequest{ServiceUUID: svcUUID, Name: p.ExternalName()}); upcloudapi.IsNotFound(err) {
		return reconciler.ErrPending
	} else if err != nil {
		return fmt.Errorf("get object storage policy: %w", err)
	}
	return nil
}

// Delete implements reconciler.Adapter. A policy still attached to a user is
// refused (409) and reported as pending; a 404 counts as deleted.
func (a *ObjectStoragePolicyAdapter) Delete(ctx context.Context, p *objectstoragev1alpha1.ObjectStoragePolicy) error {
	if p.Status.ServiceUUID == "" {
		return nil
	}
	err := a.API.DeleteManagedObjectStoragePolicy(ctx, &request.DeleteManagedObjectStoragePolicyRequest{
		ServiceUUID: p.Status.ServiceUUID,
		Name:        p.Status.Name,
	})
	switch {
	case err == nil, upcloudapi.IsNotFound(err):
		return nil
	case upcloudapi.IsConflict(err):
		return fmt.Errorf("%w: %s", reconciler.ErrPending, upcloudapi.TitleOf(err))
	default:
		return fmt.Errorf("delete object storage policy: %w", err)
	}
}
