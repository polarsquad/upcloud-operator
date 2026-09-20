package objectstorage

import (
	"context"
	"testing"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud"

	objectstoragev1alpha1 "github.com/polarsquad/upcloud-operator/api/objectstorage/v1alpha1"
	"github.com/polarsquad/upcloud-operator/internal/upcloudapi/fake"
)

func TestObjectStorageAccessKeyDeleteContract(t *testing.T) {
	checkDeleteContract(t, func() deleteContractFixture {
		api := fake.NewObjectStorageAPI()
		api.Services[parentUUID] = &upcloud.ManagedObjectStorage{UUID: parentUUID}
		api.Users[parentUUID] = []upcloud.ManagedObjectStorageUser{{
			Username:   testUserName,
			AccessKeys: []upcloud.ManagedObjectStorageUserAccessKey{{AccessKeyID: "contract-key"}},
		}}
		k := newAccessKey("contract-key")
		k.Status = objectstoragev1alpha1.ObjectStorageAccessKeyStatus{
			ServiceUUID: parentUUID, Username: testUserName, AccessKeyID: "contract-key",
		}
		a := &ObjectStorageAccessKeyAdapter{API: api}
		return deleteContractFixture{
			api:    api,
			delete: func() error { return a.Delete(context.Background(), k) },
			identity: map[string]*string{
				serviceUUIDField: &k.Status.ServiceUUID, "Username": &k.Status.Username, "AccessKeyID": &k.Status.AccessKeyID,
			},
			removeResource: func() { api.Users[parentUUID][0].AccessKeys = nil },
			absentCalls:    []string{deleteAccessKeyCall},
			deleteCalls:    []string{deleteAccessKeyCall},
		}
	})
}
