package integration_test

import (
	"context"
	"testing"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud/request"
	. "github.com/onsi/gomega"

	"github.com/polarsquad/upcloud-operator/api/common"
	objectstoragev1 "github.com/polarsquad/upcloud-operator/api/objectstorage/v1alpha1"
	"github.com/polarsquad/upcloud-operator/internal/controller/objectstorage"
	"github.com/polarsquad/upcloud-operator/internal/reconciler"
	"github.com/polarsquad/upcloud-operator/internal/upcloudapi"
	"github.com/polarsquad/upcloud-operator/internal/upcloudapi/fake"
)

func TestParentFirstObjectStorageBucketRetriesUntilEmpty(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()
	c, api := pairClient(t), fake.NewObjectStorageAPI()
	service := &objectstoragev1.ManagedObjectStorage{
		ObjectMeta: pairMeta("parent-object-storage"),
		Spec:       objectstoragev1.ManagedObjectStorageSpec{Region: "europe-1"},
	}
	bucket := &objectstoragev1.ObjectStorageBucket{
		ObjectMeta: pairMeta("child-bucket"),
		Spec: objectstoragev1.ObjectStorageBucketSpec{
			ServiceRef: common.LocalObjectReference{Name: service.Name}, Name: "parent-first-bucket",
		},
	}
	s := &reconciler.Reconciler[*objectstoragev1.ManagedObjectStorage]{
		Client: c, Adapter: &objectstorage.ManagedObjectStorageAdapter{Client: c, API: api},
		New:       func() *objectstoragev1.ManagedObjectStorage { return &objectstoragev1.ManagedObjectStorage{} },
		Finalizer: objectstorage.FinalizerManagedObjectStorage, PendingRequeue: retryDelay,
	}
	b := &reconciler.Reconciler[*objectstoragev1.ObjectStorageBucket]{
		Client: c, Adapter: &objectstorage.ObjectStorageBucketAdapter{Client: c, API: api},
		New:       func() *objectstoragev1.ObjectStorageBucket { return &objectstoragev1.ObjectStorageBucket{} },
		Finalizer: objectstorage.FinalizerObjectStorageBucket, PendingRequeue: retryDelay,
	}
	createReady(t, s, service)
	createReady(t, b, bucket)
	serviceID, bucketName := service.Status.UUID, bucket.Status.Name
	g.Expect(bucket.Status.ServiceUUID).To(Equal(serviceID))
	requestDeletion(t, c, service)
	// force=false refuses a non-empty service. No injected error is needed:
	// the bucket created by its controller is the actual fake-API blocker.
	for range 2 {
		requirePending(t, s, service)
		requireLive(t, c, bucket)
		cloud, err := api.GetManagedObjectStorage(ctx, &request.GetManagedObjectStorageRequest{UUID: serviceID})
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(upcloudapi.HasUID(cloud.Labels, service.UID)).To(BeTrue())
		metrics, err := api.GetManagedObjectStorageBucketMetrics(ctx, &request.GetManagedObjectStorageBucketMetricsRequest{ServiceUUID: serviceID})
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(metrics).To(HaveLen(1))
		g.Expect(metrics[0].Name).To(Equal(bucketName))
	}

	requestDeletion(t, c, bucket)
	// An accepted bucket delete is pending until another pass confirms 404.
	requirePending(t, b, bucket)
	metrics, err := api.GetManagedObjectStorageBucketMetrics(ctx, &request.GetManagedObjectStorageBucketMetricsRequest{ServiceUUID: serviceID})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(metrics).To(BeEmpty(), "the captured bucket identity must leave the cloud")

	// The parent can now drain even before the bucket CR loses its finalizer.
	// Accepted service deletion also requires a later confirmation pass.
	requirePending(t, s, service)
	requireGone(t, s, service)
	g.Expect(bucket.Status.ServiceUUID).To(Equal(serviceID))
	requireGone(t, b, bucket) // saved identity works after the parent CR is gone
	_, err = api.GetManagedObjectStorage(ctx, &request.GetManagedObjectStorageRequest{UUID: serviceID})
	g.Expect(upcloudapi.IsNotFound(err)).To(BeTrue())
	_, err = api.GetManagedObjectStorageBucketMetrics(ctx, &request.GetManagedObjectStorageBucketMetricsRequest{ServiceUUID: serviceID})
	g.Expect(upcloudapi.IsNotFound(err)).To(BeTrue())
	const deleteService = "DeleteManagedObjectStorage"
	g.Expect(deleteCalls(api.Calls)).To(Equal([]string{
		deleteService, deleteService, "DeleteManagedObjectStorageBucket",
		deleteService, "DeleteManagedObjectStorageBucket",
	}))
}
