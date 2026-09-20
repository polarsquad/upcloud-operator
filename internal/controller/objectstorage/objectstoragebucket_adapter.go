package objectstorage

import (
	"context"
	"fmt"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud"
	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud/request"
	"sigs.k8s.io/controller-runtime/pkg/client"

	objectstoragev1alpha1 "github.com/polarsquad/upcloud-operator/api/objectstorage/v1alpha1"
	"github.com/polarsquad/upcloud-operator/internal/reconciler"
	"github.com/polarsquad/upcloud-operator/internal/resolve"
	"github.com/polarsquad/upcloud-operator/internal/upcloudapi"
)

// bucketPageSize is the page size used when paging through bucket metrics.
const bucketPageSize = 100

// ObjectStorageBucketAdapter maps ObjectStorageBucket onto the UpCloud object
// storage bucket API. Buckets are observed via the paginated bucket metrics
// endpoint.
type ObjectStorageBucketAdapter struct {
	API    upcloudapi.ObjectStorageAPI
	Client client.Client
}

var _ reconciler.Adapter[*objectstoragev1alpha1.ObjectStorageBucket] = (*ObjectStorageBucketAdapter)(nil)

// serviceUUID resolves the referenced ManagedObjectStorage (which must be
// Ready) on first use and stores the result in Status.
func (a *ObjectStorageBucketAdapter) serviceUUID(ctx context.Context, b *objectstoragev1alpha1.ObjectStorageBucket) (string, error) {
	if b.Status.ServiceUUID != "" {
		return b.Status.ServiceUUID, nil
	}
	uuid, err := resolve.Ready(ctx, a.Client, b.Namespace, b.Spec.ServiceRef.Name, &objectstoragev1alpha1.ManagedObjectStorage{})
	if err != nil {
		return "", err
	}
	b.Status.ServiceUUID = uuid
	return uuid, nil
}

// findBucket pages through the bucket metrics until a page is empty, looking
// for the named bucket. A deleted bucket is treated as absent.
func (a *ObjectStorageBucketAdapter) findBucket(ctx context.Context, svcUUID, name string) (*upcloud.ManagedObjectStorageBucketMetrics, bool, error) {
	for page := 1; ; page++ {
		list, err := a.API.GetManagedObjectStorageBucketMetrics(ctx, &request.GetManagedObjectStorageBucketMetricsRequest{
			ServiceUUID: svcUUID,
			Page:        &request.Page{Size: bucketPageSize, Number: page},
		})
		if err != nil {
			return nil, false, fmt.Errorf("get bucket metrics: %w", err)
		}
		if len(list) == 0 {
			return nil, false, nil
		}
		for i := range list {
			if list[i].Name == name && !list[i].Deleted {
				return &list[i], true, nil
			}
		}
	}
}

// Observe implements reconciler.Adapter. A missing (or deleted) bucket
// reports Exists: false.
func (a *ObjectStorageBucketAdapter) Observe(ctx context.Context, b *objectstoragev1alpha1.ObjectStorageBucket) (reconciler.Observation, error) {
	svcUUID, err := a.serviceUUID(ctx, b)
	if err != nil {
		return reconciler.Observation{}, err
	}
	metrics, found, err := a.findBucket(ctx, svcUUID, b.ExternalName())
	if err != nil {
		return reconciler.Observation{}, err
	}
	if !found {
		return reconciler.Observation{}, nil
	}
	b.Status.Name = metrics.Name
	b.Status.TotalObjects = metrics.TotalObjects
	b.Status.TotalSizeBytes = metrics.TotalSizeBytes
	return reconciler.Observation{Exists: true, UpToDate: true, Ready: true, Message: "bucket " + metrics.Name}, nil
}

// Create implements reconciler.Adapter. A Conflict (bucket already exists in
// UpCloud) is treated as adoption: the next Observe reconciles it.
func (a *ObjectStorageBucketAdapter) Create(ctx context.Context, b *objectstoragev1alpha1.ObjectStorageBucket) error {
	svcUUID, err := a.serviceUUID(ctx, b)
	if err != nil {
		return err
	}
	created, err := a.API.CreateManagedObjectStorageBucket(ctx, &request.CreateManagedObjectStorageBucketRequest{
		ServiceUUID: svcUUID,
		Name:        b.ExternalName(),
	})
	if err != nil {
		if upcloudapi.IsConflict(err) {
			return nil // adopted; the next Observe reconciles it
		}
		return fmt.Errorf("create object storage bucket: %w", err)
	}
	b.Status.Name = created.Name
	return nil
}

// Update implements reconciler.Adapter. Buckets have no mutable fields, so a
// missing bucket is the only thing to report (pending).
func (a *ObjectStorageBucketAdapter) Update(ctx context.Context, b *objectstoragev1alpha1.ObjectStorageBucket) error {
	svcUUID, err := a.serviceUUID(ctx, b)
	if err != nil {
		return err
	}
	if _, found, err := a.findBucket(ctx, svcUUID, b.ExternalName()); err != nil {
		return err
	} else if !found {
		return reconciler.ErrPending
	}
	return nil
}

// Delete implements reconciler.Adapter. A non-empty bucket is refused (409)
// and reported as pending with the API title until the user empties it;
// a 404 counts as deleted; a successful delete is pending until gone.
func (a *ObjectStorageBucketAdapter) Delete(ctx context.Context, b *objectstoragev1alpha1.ObjectStorageBucket) error {
	if b.Status.ServiceUUID == "" || b.Status.Name == "" {
		return nil
	}
	err := a.API.DeleteManagedObjectStorageBucket(ctx, &request.DeleteManagedObjectStorageBucketRequest{
		ServiceUUID: b.Status.ServiceUUID,
		Name:        b.ExternalName(),
	})
	switch {
	case err == nil:
		return reconciler.ErrPending
	case upcloudapi.IsNotFound(err):
		return nil
	case upcloudapi.IsConflict(err):
		return fmt.Errorf("%w: %s", reconciler.ErrPending, upcloudapi.TitleOf(err))
	default:
		return fmt.Errorf("delete object storage bucket: %w", err)
	}
}
