package objectstorage

import (
	"context"
	"errors"
	"testing"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud/request"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/polarsquad/upcloud-operator/api/common"
	objectstoragev1alpha1 "github.com/polarsquad/upcloud-operator/api/objectstorage/v1alpha1"
	"github.com/polarsquad/upcloud-operator/internal/reconciler"
	"github.com/polarsquad/upcloud-operator/internal/upcloudapi/fake"
)

func newBucket(name string) *objectstoragev1alpha1.ObjectStorageBucket {
	return &objectstoragev1alpha1.ObjectStorageBucket{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: testNS, UID: "bucket-uid"},
		Spec: objectstoragev1alpha1.ObjectStorageBucketSpec{
			ServiceRef: common.LocalObjectReference{Name: parentName},
		},
	}
}

func TestBucketCreateObservePagination(t *testing.T) {
	g := NewWithT(t)
	api := fake.NewObjectStorageAPI()
	c := newMOSClient(t)
	readyService(g, c, api)
	a := &ObjectStorageBucketAdapter{API: api, Client: c}
	b := newBucket("my-bucket")
	ctx := context.Background()

	obs, err := a.Observe(ctx, b)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(obs.Exists).To(BeFalse())

	g.Expect(a.Create(ctx, b)).To(Succeed())
	g.Expect(b.Status.ServiceUUID).To(Equal(parentUUID))
	g.Expect(b.Status.Name).To(Equal("my-bucket"))

	// Add several buckets in the fake so the metrics endpoint returns more
	// than one entry; the adapter must page and find "my-bucket".
	for _, extra := range []string{"a", "b", "c"} {
		_, err := api.CreateManagedObjectStorageBucket(ctx, &request.CreateManagedObjectStorageBucketRequest{ServiceUUID: parentUUID, Name: extra})
		g.Expect(err).NotTo(HaveOccurred())
	}

	obs, err = a.Observe(ctx, b)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(obs.Exists).To(BeTrue())
	g.Expect(obs.Ready).To(BeTrue())
	g.Expect(b.Status.Name).To(Equal("my-bucket"))
}

func TestBucketDeleteNonEmptyPending(t *testing.T) {
	g := NewWithT(t)
	api := fake.NewObjectStorageAPI()
	c := newMOSClient(t)
	readyService(g, c, api)
	a := &ObjectStorageBucketAdapter{API: api, Client: c}
	b := newBucket("full-bucket")
	ctx := context.Background()

	g.Expect(a.Create(ctx, b)).To(Succeed())
	// Make the bucket non-empty so deletion is refused.
	api.Buckets[parentUUID][0].TotalObjects = 5

	err := a.Delete(ctx, b)
	g.Expect(err).To(MatchError(ContainSubstring("not empty")))
	// Keep the finalizer pending until the user has emptied the bucket.
	g.Expect(errors.Is(err, reconciler.ErrPending)).To(BeTrue())
	g.Expect(api.Buckets[parentUUID]).To(HaveLen(1))
	api.Buckets[parentUUID][0].TotalObjects = 0
	g.Expect(errors.Is(a.Delete(ctx, b), reconciler.ErrPending)).To(BeTrue())
	g.Expect(a.Delete(ctx, b)).To(Succeed())
}

func TestBucketDeletePendingWhenEmpty(t *testing.T) {
	g := NewWithT(t)
	api := fake.NewObjectStorageAPI()
	c := newMOSClient(t)
	readyService(g, c, api)
	a := &ObjectStorageBucketAdapter{API: api, Client: c}
	b := newBucket("empty-bucket")
	ctx := context.Background()

	g.Expect(a.Create(ctx, b)).To(Succeed())
	// First delete succeeds and is pending until the metrics show it deleted.
	g.Expect(a.Delete(ctx, b)).To(MatchError(reconciler.ErrPending))
	// The fake marks the bucket deleted; the next pass finds it deleted/absent.
	obs, err := a.Observe(ctx, b)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(obs.Exists).To(BeFalse())
}
