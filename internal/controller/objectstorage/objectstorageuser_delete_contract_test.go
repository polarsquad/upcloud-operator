package objectstorage

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud"
	. "github.com/onsi/gomega"

	objectstoragev1alpha1 "github.com/polarsquad/upcloud-operator/api/objectstorage/v1alpha1"
	"github.com/polarsquad/upcloud-operator/internal/reconciler"
	"github.com/polarsquad/upcloud-operator/internal/upcloudapi/fake"
)

func TestObjectStorageUserDeleteContract(t *testing.T) {
	checkDeleteContract(t, func() deleteContractFixture {
		api := fake.NewObjectStorageAPI()
		api.Services[parentUUID] = &upcloud.ManagedObjectStorage{UUID: parentUUID}
		api.Users[parentUUID] = []upcloud.ManagedObjectStorageUser{{Username: testUserName}}
		u := newUserCR(testUserName, nil)
		u.Status = objectstoragev1alpha1.ObjectStorageUserStatus{ServiceUUID: parentUUID, Username: testUserName}
		a := &ObjectStorageUserAdapter{API: api}
		return deleteContractFixture{
			api:            api,
			delete:         func() error { return a.Delete(context.Background(), u) },
			identity:       map[string]*string{serviceUUIDField: &u.Status.ServiceUUID, "Username": &u.Status.Username},
			removeResource: func() { delete(api.Users, parentUUID) },
			absentCalls:    []string{getUserCall},
			deleteCalls:    []string{getUserCall, "DeleteManagedObjectStorageUser"},
		}
	})
}

func TestObjectStorageUserDeleteContractCleanupErrors(t *testing.T) {
	for _, step := range []struct {
		name   string
		user   upcloud.ManagedObjectStorageUser
		method string
	}{
		{
			name: "detach-policy",
			user: upcloud.ManagedObjectStorageUser{
				Username: testUserName,
				Policies: []upcloud.ManagedObjectStoragePolicy{{Name: testROPolicy}},
			},
			method: "DetachManagedObjectStorageUserPolicy",
		},
		{
			name: "delete-key",
			user: upcloud.ManagedObjectStorageUser{
				Username:   testUserName,
				AccessKeys: []upcloud.ManagedObjectStorageUserAccessKey{{AccessKeyID: "cleanup-key"}},
			},
			method: deleteAccessKeyCall,
		},
	} {
		t.Run(step.name, func(t *testing.T) {
			for _, tc := range []struct {
				name    string
				failure error
				pending bool
			}{
				{"Conflict", &upcloud.Problem{Status: http.StatusConflict, Title: "cleanup blocked"}, true},
				{"UnexpectedError", errors.New("cleanup transport failure"), false},
			} {
				t.Run(tc.name, func(t *testing.T) {
					g := NewWithT(t)
					api := fake.NewObjectStorageAPI()
					api.Services[parentUUID] = &upcloud.ManagedObjectStorage{UUID: parentUUID}
					api.Users[parentUUID] = []upcloud.ManagedObjectStorageUser{step.user}
					api.FailNext = tc.failure
					u := newUserCR(testUserName, nil)
					u.Status = objectstoragev1alpha1.ObjectStorageUserStatus{ServiceUUID: parentUUID, Username: testUserName}
					a := &ObjectStorageUserAdapter{API: api}
					err := a.Delete(context.Background(), u)
					g.Expect(errors.Is(err, reconciler.ErrPending)).To(Equal(tc.pending), "got %v", err)
					if !tc.pending {
						g.Expect(errors.Is(err, tc.failure)).To(BeTrue())
					}
					g.Expect(api.Calls).To(Equal([]string{getUserCall, step.method}))
					g.Expect(api.Users[parentUUID]).To(HaveLen(1), "failed cleanup must retain the user")
				})
			}
		})
	}
}
