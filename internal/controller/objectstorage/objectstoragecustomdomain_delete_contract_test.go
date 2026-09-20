package objectstorage

import (
	"context"
	"testing"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud"
	. "github.com/onsi/gomega"

	objectstoragev1alpha1 "github.com/polarsquad/upcloud-operator/api/objectstorage/v1alpha1"
	"github.com/polarsquad/upcloud-operator/internal/upcloudapi/fake"
)

const testContractDomain = "contract.example.com"

func TestObjectStorageCustomDomainDeleteContract(t *testing.T) {
	checkDeleteContract(t, func() deleteContractFixture {
		api := fake.NewObjectStorageAPI()
		api.Services[parentUUID] = &upcloud.ManagedObjectStorage{
			UUID:          parentUUID,
			CustomDomains: []upcloud.ManagedObjectStorageCustomDomain{{DomainName: testContractDomain}},
		}
		d := newDomain("contract-domain", testContractDomain)
		d.Status = objectstoragev1alpha1.ObjectStorageCustomDomainStatus{ServiceUUID: parentUUID, DomainName: testContractDomain}
		a := &ObjectStorageCustomDomainAdapter{API: api}
		return deleteContractFixture{
			api:            api,
			delete:         func() error { return a.Delete(context.Background(), d) },
			identity:       map[string]*string{serviceUUIDField: &d.Status.ServiceUUID, "DomainName": &d.Status.DomainName},
			removeResource: func() { api.Services[parentUUID].CustomDomains = nil },
			absentCalls:    []string{"DeleteManagedObjectStorageCustomDomain"},
			deleteCalls:    []string{"DeleteManagedObjectStorageCustomDomain"},
		}
	})
}

func TestObjectStorageCustomDomainDeleteContractUsesStatusIdentity(t *testing.T) {
	g := NewWithT(t)
	api := fake.NewObjectStorageAPI()
	const changedDomain = "changed.example.com"
	api.Services[parentUUID] = &upcloud.ManagedObjectStorage{
		UUID: parentUUID,
		CustomDomains: []upcloud.ManagedObjectStorageCustomDomain{
			{DomainName: testContractDomain}, {DomainName: changedDomain},
		},
	}
	d := newDomain("changed-domain", changedDomain)
	d.Status = objectstoragev1alpha1.ObjectStorageCustomDomainStatus{ServiceUUID: parentUUID, DomainName: testContractDomain}
	a := &ObjectStorageCustomDomainAdapter{API: api}
	// DomainName is mutable: deletion must target the persisted identity,
	// never a newer spec value that may belong to a different resource.
	g.Expect(a.Delete(context.Background(), d)).To(Succeed())
	g.Expect(api.Services[parentUUID].CustomDomains).To(Equal([]upcloud.ManagedObjectStorageCustomDomain{{DomainName: changedDomain}}))
}
