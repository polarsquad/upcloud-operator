package objectstorage

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud"
	. "github.com/onsi/gomega"

	objectstoragev1alpha1 "github.com/polarsquad/upcloud-operator/api/objectstorage/v1alpha1"
	"github.com/polarsquad/upcloud-operator/internal/reconciler"
	"github.com/polarsquad/upcloud-operator/internal/upcloudapi/fake"
)

// Each fixture uses persisted status identities without Kubernetes parents.
// Deletion must work even after the parent CR has already disappeared.
type deleteContractFixture struct {
	api            *fake.ObjectStorageAPI
	delete         func() error
	identity       map[string]*string
	removeResource func()
	absentCalls    []string
	deleteCalls    []string
}

func checkDeleteContract(t *testing.T, setup func() deleteContractFixture) {
	t.Helper()
	t.Run("EmptyIdentity", func(t *testing.T) {
		for field := range setup().identity {
			t.Run(field, func(t *testing.T) {
				g := NewWithT(t)
				f := setup()
				*f.identity[field] = ""
				g.Expect(f.delete()).To(Succeed())
				g.Expect(f.api.Calls).To(BeEmpty(), "empty %s must not call UpCloud", field)
			})
		}
	})
	t.Run("Absent", func(t *testing.T) {
		g := NewWithT(t)
		f := setup()
		f.removeResource()
		g.Expect(f.delete()).To(Succeed())
		g.Expect(f.api.Calls).To(Equal(f.absentCalls))
	})
	t.Run("AbsentParent", func(t *testing.T) {
		g := NewWithT(t)
		f := setup()
		delete(f.api.Services, parentUUID)
		g.Expect(f.delete()).To(Succeed())
		g.Expect(f.api.Calls).To(Equal(f.absentCalls))
	})
	t.Run("Conflict", func(t *testing.T) {
		g := NewWithT(t)
		f := setup()
		// Do not assume leaf endpoints can never return a retryable 409.
		f.api.FailNext = fmt.Errorf("API response: %w", &upcloud.Problem{
			Status: http.StatusConflict, Title: "deletion blocked",
		})
		err := f.delete()
		g.Expect(errors.Is(err, reconciler.ErrPending)).To(BeTrue(), "got %v", err)
		g.Expect(err).To(MatchError(ContainSubstring("deletion blocked")))
		g.Expect(f.api.Calls).To(Equal(f.deleteCalls))
	})
	t.Run("UnexpectedError", func(t *testing.T) {
		for name, failure := range map[string]error{
			"transport": errors.New("transport unavailable"),
			"forbidden": &upcloud.Problem{Status: http.StatusForbidden, Title: "permission denied"},
			"server":    &upcloud.Problem{Status: http.StatusInternalServerError, Title: "server failure"},
		} {
			t.Run(name, func(t *testing.T) {
				g := NewWithT(t)
				f := setup()
				f.api.FailNext = fmt.Errorf("API response: %w", failure)
				err := f.delete()
				g.Expect(errors.Is(err, failure)).To(BeTrue(), "got %v", err)
				g.Expect(errors.Is(err, reconciler.ErrPending)).To(BeFalse())
				g.Expect(f.api.Calls).To(Equal(f.deleteCalls))
			})
		}
	})
}

func TestManagedObjectStorageDeleteContract(t *testing.T) {
	checkDeleteContract(t, func() deleteContractFixture {
		api := fake.NewObjectStorageAPI()
		api.Services[parentUUID] = &upcloud.ManagedObjectStorage{UUID: parentUUID}
		s := newService(parentName)
		s.Status.UUID = parentUUID
		a := &ManagedObjectStorageAdapter{API: api}
		return deleteContractFixture{
			api:            api,
			delete:         func() error { return a.Delete(context.Background(), s) },
			identity:       map[string]*string{"UUID": &s.Status.UUID},
			removeResource: func() { delete(api.Services, parentUUID) },
			absentCalls:    []string{getServiceCall},
			deleteCalls:    []string{getServiceCall, "DeleteManagedObjectStorage"},
		}
	})
}

func TestManagedObjectStorageDeleteContractPendingStates(t *testing.T) {
	for _, state := range []upcloud.ManagedObjectStorageOperationalState{
		upcloud.ManagedObjectStorageOperationalStateDeleteService,
		upcloud.ManagedObjectStorageOperationalStateDeleteDNS,
		upcloud.ManagedObjectStorageOperationalStateDeleteNetwork,
	} {
		t.Run(string(state), func(t *testing.T) {
			g := NewWithT(t)
			api := fake.NewObjectStorageAPI()
			api.Services[parentUUID] = &upcloud.ManagedObjectStorage{UUID: parentUUID}
			api.StateOverride = state
			s := newService(parentName)
			s.Status.UUID = parentUUID
			a := &ManagedObjectStorageAdapter{API: api}
			g.Expect(errors.Is(a.Delete(context.Background(), s), reconciler.ErrPending)).To(BeTrue())
			g.Expect(api.Calls).To(Equal([]string{getServiceCall}))
		})
	}
}

func TestManagedObjectStorageDeleteContractBlockedByBucket(t *testing.T) {
	g := NewWithT(t)
	api := fake.NewObjectStorageAPI()
	api.Services[parentUUID] = &upcloud.ManagedObjectStorage{UUID: parentUUID}
	api.Buckets[parentUUID] = []upcloud.ManagedObjectStorageBucketMetrics{{Name: "blocking-bucket"}}
	s := &objectstoragev1alpha1.ManagedObjectStorage{
		Status: objectstoragev1alpha1.ManagedObjectStorageStatus{UUID: parentUUID},
	}
	a := &ManagedObjectStorageAdapter{API: api}
	g.Expect(errors.Is(a.Delete(context.Background(), s), reconciler.ErrPending)).To(BeTrue())
	g.Expect(api.Services).To(HaveKey(parentUUID))
	delete(api.Buckets, parentUUID)
	g.Expect(errors.Is(a.Delete(context.Background(), s), reconciler.ErrPending)).To(BeTrue())
	g.Expect(a.Delete(context.Background(), s)).To(Succeed())
}
