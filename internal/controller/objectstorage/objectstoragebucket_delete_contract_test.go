package objectstorage

import (
	"context"
	"errors"
	"testing"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud"
	. "github.com/onsi/gomega"

	objectstoragev1alpha1 "github.com/polarsquad/upcloud-operator/api/objectstorage/v1alpha1"
	"github.com/polarsquad/upcloud-operator/internal/reconciler"
	"github.com/polarsquad/upcloud-operator/internal/upcloudapi/fake"
)

const testContractBucket = "contract-bucket"

func TestObjectStorageBucketDeleteContract(t *testing.T) {
	checkDeleteContract(t, func() deleteContractFixture {
		api := fake.NewObjectStorageAPI()
		api.Services[parentUUID] = &upcloud.ManagedObjectStorage{UUID: parentUUID}
		api.Buckets[parentUUID] = []upcloud.ManagedObjectStorageBucketMetrics{{Name: testContractBucket}}
		b := newBucket(testContractBucket)
		b.Status = objectstoragev1alpha1.ObjectStorageBucketStatus{ServiceUUID: parentUUID, Name: testContractBucket}
		a := &ObjectStorageBucketAdapter{API: api}
		return deleteContractFixture{
			api:            api,
			delete:         func() error { return a.Delete(context.Background(), b) },
			identity:       map[string]*string{serviceUUIDField: &b.Status.ServiceUUID, "Name": &b.Status.Name},
			removeResource: func() { delete(api.Buckets, parentUUID) },
			absentCalls:    []string{"DeleteManagedObjectStorageBucket"},
			deleteCalls:    []string{"DeleteManagedObjectStorageBucket"},
		}
	})
}

func TestObjectStorageBucketDeleteContractUsesStatusIdentity(t *testing.T) {
	g := NewWithT(t)
	api := fake.NewObjectStorageAPI()
	const changedBucket = "changed-bucket"
	api.Services[parentUUID] = &upcloud.ManagedObjectStorage{UUID: parentUUID}
	api.Buckets[parentUUID] = []upcloud.ManagedObjectStorageBucketMetrics{
		{Name: testContractBucket}, {Name: changedBucket},
	}
	b := newBucket(testContractBucket)
	// spec.name was added after creation; the CEL transition rule does not
	// fire when the old value is absent, so the spec can diverge from status.
	b.Spec.Name = changedBucket
	b.Status = objectstoragev1alpha1.ObjectStorageBucketStatus{ServiceUUID: parentUUID, Name: testContractBucket}
	a := &ObjectStorageBucketAdapter{API: api}
	// Deletion must target the persisted identity, never a newer spec value
	// that may belong to a different bucket.
	g.Expect(errors.Is(a.Delete(context.Background(), b), reconciler.ErrPending)).To(BeTrue())
	g.Expect(api.Buckets[parentUUID]).To(Equal([]upcloud.ManagedObjectStorageBucketMetrics{{Name: changedBucket}}))
}
