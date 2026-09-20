package objectstorage

import (
	"context"
	"testing"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud"

	objectstoragev1alpha1 "github.com/polarsquad/upcloud-operator/api/objectstorage/v1alpha1"
	"github.com/polarsquad/upcloud-operator/internal/upcloudapi/fake"
)

func TestObjectStoragePolicyDeleteContract(t *testing.T) {
	checkDeleteContract(t, func() deleteContractFixture {
		api := fake.NewObjectStorageAPI()
		api.Services[parentUUID] = &upcloud.ManagedObjectStorage{UUID: parentUUID}
		api.Policies[parentUUID] = []upcloud.ManagedObjectStoragePolicy{{Name: testROPolicy}}
		p := newPolicy(testROPolicy)
		p.Status = objectstoragev1alpha1.ObjectStoragePolicyStatus{ServiceUUID: parentUUID, Name: testROPolicy}
		a := &ObjectStoragePolicyAdapter{API: api}
		return deleteContractFixture{
			api:            api,
			delete:         func() error { return a.Delete(context.Background(), p) },
			identity:       map[string]*string{serviceUUIDField: &p.Status.ServiceUUID, "Name": &p.Status.Name},
			removeResource: func() { delete(api.Policies, parentUUID) },
			absentCalls:    []string{"DeleteManagedObjectStoragePolicy"},
			deleteCalls:    []string{"DeleteManagedObjectStoragePolicy"},
		}
	})
}
