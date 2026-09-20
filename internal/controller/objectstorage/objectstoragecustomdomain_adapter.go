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

// ObjectStorageCustomDomainAdapter maps ObjectStorageCustomDomain onto the
// UpCloud object storage custom domain API.
type ObjectStorageCustomDomainAdapter struct {
	API    upcloudapi.ObjectStorageAPI
	Client client.Client
}

var _ reconciler.Adapter[*objectstoragev1alpha1.ObjectStorageCustomDomain] = (*ObjectStorageCustomDomainAdapter)(nil)

// serviceUUID resolves the referenced ManagedObjectStorage (which must be
// Ready) on first use and stores the result in Status.
func (a *ObjectStorageCustomDomainAdapter) serviceUUID(ctx context.Context, d *objectstoragev1alpha1.ObjectStorageCustomDomain) (string, error) {
	if d.Status.ServiceUUID != "" {
		return d.Status.ServiceUUID, nil
	}
	uuid, err := resolve.Ready(ctx, a.Client, d.Namespace, d.Spec.ServiceRef.Name, &objectstoragev1alpha1.ManagedObjectStorage{})
	if err != nil {
		return "", err
	}
	d.Status.ServiceUUID = uuid
	return uuid, nil
}

func desiredDomainType(d *objectstoragev1alpha1.ObjectStorageCustomDomain) string {
	if d.Spec.Type != "" {
		return d.Spec.Type
	}
	return EndpointTypePublic
}

// Observe implements reconciler.Adapter. A missing domain reports Exists: false.
func (a *ObjectStorageCustomDomainAdapter) Observe(ctx context.Context, d *objectstoragev1alpha1.ObjectStorageCustomDomain) (reconciler.Observation, error) {
	svcUUID, err := a.serviceUUID(ctx, d)
	if err != nil {
		return reconciler.Observation{}, err
	}
	domain, err := a.API.GetManagedObjectStorageCustomDomain(ctx, &request.GetManagedObjectStorageCustomDomainRequest{
		ServiceUUID: svcUUID,
		DomainName:  d.Spec.DomainName,
	})
	if upcloudapi.IsNotFound(err) {
		return reconciler.Observation{}, nil
	}
	if err != nil {
		return reconciler.Observation{}, fmt.Errorf("get object storage custom domain: %w", err)
	}
	d.Status.DomainName = domain.DomainName
	d.Status.Type = domain.Type
	upToDate := domain.Type == desiredDomainType(d)
	return reconciler.Observation{Exists: true, UpToDate: upToDate, Ready: true, Message: "domain " + domain.DomainName}, nil
}

// Create implements reconciler.Adapter. A Conflict (domain already exists in
// UpCloud) is treated as adoption: the next Observe reconciles it.
func (a *ObjectStorageCustomDomainAdapter) Create(ctx context.Context, d *objectstoragev1alpha1.ObjectStorageCustomDomain) error {
	svcUUID, err := a.serviceUUID(ctx, d)
	if err != nil {
		return err
	}
	err = a.API.CreateManagedObjectStorageCustomDomain(ctx, &request.CreateManagedObjectStorageCustomDomainRequest{
		ServiceUUID: svcUUID,
		DomainName:  d.Spec.DomainName,
		Type:        desiredDomainType(d),
	})
	if err != nil {
		if upcloudapi.IsConflict(err) {
			return nil // adopted; the next Observe reconciles it
		}
		return fmt.Errorf("create object storage custom domain: %w", err)
	}
	d.Status.DomainName = d.Spec.DomainName
	d.Status.Type = desiredDomainType(d)
	return nil
}

// Update implements reconciler.Adapter. A type drift is pushed with
// ModifyManagedObjectStorageCustomDomain.
func (a *ObjectStorageCustomDomainAdapter) Update(ctx context.Context, d *objectstoragev1alpha1.ObjectStorageCustomDomain) error {
	svcUUID, err := a.serviceUUID(ctx, d)
	if err != nil {
		return err
	}
	if _, err := a.API.GetManagedObjectStorageCustomDomain(ctx, &request.GetManagedObjectStorageCustomDomainRequest{
		ServiceUUID: svcUUID,
		DomainName:  d.Spec.DomainName,
	}); upcloudapi.IsNotFound(err) {
		return reconciler.ErrPending
	} else if err != nil {
		return fmt.Errorf("get object storage custom domain: %w", err)
	}
	modified, err := a.API.ModifyManagedObjectStorageCustomDomain(ctx, &request.ModifyManagedObjectStorageCustomDomainRequest{
		ServiceUUID:  svcUUID,
		DomainName:   d.Spec.DomainName,
		CustomDomain: request.ModifyCustomDomain{DomainName: d.Spec.DomainName, Type: desiredDomainType(d)},
	})
	if err != nil {
		return fmt.Errorf("modify object storage custom domain: %w", err)
	}
	d.Status.Type = modified.Type
	return nil
}

// Delete implements reconciler.Adapter. A 404 counts as deleted; conflicts
// are pending.
func (a *ObjectStorageCustomDomainAdapter) Delete(ctx context.Context, d *objectstoragev1alpha1.ObjectStorageCustomDomain) error {
	if d.Status.ServiceUUID == "" || d.Status.DomainName == "" {
		return nil
	}
	err := a.API.DeleteManagedObjectStorageCustomDomain(ctx, &request.DeleteManagedObjectStorageCustomDomainRequest{
		ServiceUUID: d.Status.ServiceUUID,
		DomainName:  d.Status.DomainName,
	})
	switch {
	case err == nil, upcloudapi.IsNotFound(err):
		return nil
	case upcloudapi.IsConflict(err):
		return fmt.Errorf("%w: %s", reconciler.ErrPending, upcloudapi.TitleOf(err))
	default:
		return fmt.Errorf("delete object storage custom domain: %w", err)
	}
}
