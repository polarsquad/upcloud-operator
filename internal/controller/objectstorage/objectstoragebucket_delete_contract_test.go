package objectstorage

import (
	"context"
	"testing"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud"

	objectstoragev1alpha1 "github.com/polarsquad/upcloud-operator/api/objectstorage/v1alpha1"
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
