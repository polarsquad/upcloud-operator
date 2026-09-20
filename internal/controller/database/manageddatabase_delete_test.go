package database

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud"
	. "github.com/onsi/gomega"

	"github.com/polarsquad/upcloud-operator/internal/reconciler"
	"github.com/polarsquad/upcloud-operator/internal/upcloudapi/fake"
)

func TestManagedDatabaseDeleteConflictReturnsPending(t *testing.T) {
	g := NewWithT(t)
	api := fake.NewDatabaseAPI()
	a := &ManagedDatabaseAdapter{API: api, Client: newFakeDBClient(t)}
	md := newManagedDatabase()
	ctx := context.Background()
	g.Expect(a.Create(ctx, md)).To(Succeed())
	api.Calls = nil
	api.FailNext = fmt.Errorf("dependency still attached: %w", fake.Conflict("database"))

	err := a.Delete(ctx, md)
	g.Expect(errors.Is(err, reconciler.ErrPending)).To(BeTrue(), "Delete returned %v", err)
	g.Expect(api.Calls).To(Equal([]string{getMDCall, deleteMDCall}))
	g.Expect(api.Databases).To(HaveKey(md.Status.UUID))

	// Once the conflict clears, deletion is still asynchronous.
	err = a.Delete(ctx, md)
	g.Expect(errors.Is(err, reconciler.ErrPending)).To(BeTrue(), "Delete returned %v", err)
	g.Expect(a.Delete(ctx, md)).To(Succeed())
	g.Expect(api.Databases).NotTo(HaveKey(md.Status.UUID))
}

func TestManagedDatabaseDeleteEmptyIdentityMakesNoAPICalls(t *testing.T) {
	g := NewWithT(t)
	api := fake.NewDatabaseAPI()
	a := &ManagedDatabaseAdapter{API: api, Client: newFakeDBClient(t)}
	md := newManagedDatabase()
	ctx := context.Background()
	g.Expect(a.Create(ctx, md)).To(Succeed())
	uuid := md.Status.UUID
	md.Status.UUID = ""
	api.Calls = nil

	g.Expect(a.Delete(ctx, md)).To(Succeed())
	g.Expect(api.Calls).To(BeEmpty())
	g.Expect(api.Databases).To(HaveKey(uuid))
}

func TestManagedDatabaseDeleteAbsentResourceSucceeds(t *testing.T) {
	g := NewWithT(t)
	api := fake.NewDatabaseAPI()
	a := &ManagedDatabaseAdapter{API: api, Client: newFakeDBClient(t)}
	md := newManagedDatabase()
	md.Status.UUID = mdbParent

	g.Expect(a.Delete(context.Background(), md)).To(Succeed())
	g.Expect(api.Calls).To(Equal([]string{getMDCall}))
	g.Expect(api.Databases).To(BeEmpty())
}

func TestManagedDatabaseDeleteTransitionalStateReturnsPending(t *testing.T) {
	g := NewWithT(t)
	api := fake.NewDatabaseAPI()
	a := &ManagedDatabaseAdapter{API: api, Client: newFakeDBClient(t)}
	md := newManagedDatabase()
	ctx := context.Background()
	g.Expect(a.Create(ctx, md)).To(Succeed())
	api.StateOverride = upcloud.ManagedDatabaseStateRebuilding
	api.Calls = nil

	err := a.Delete(ctx, md)
	g.Expect(errors.Is(err, reconciler.ErrPending)).To(BeTrue(), "Delete returned %v", err)
	g.Expect(api.Calls).To(Equal([]string{getMDCall}))
	g.Expect(api.Databases).To(HaveKey(md.Status.UUID))
}

func TestManagedDatabaseDeleteUnexpectedErrorRemainsError(t *testing.T) {
	for name, apiErr := range deleteErrorCases() {
		t.Run(name, func(t *testing.T) {
			g := NewWithT(t)
			api := fake.NewDatabaseAPI()
			a := &ManagedDatabaseAdapter{API: api, Client: newFakeDBClient(t)}
			md := newManagedDatabase()
			ctx := context.Background()
			g.Expect(a.Create(ctx, md)).To(Succeed())
			api.Calls = nil
			api.FailNext = apiErr

			err := a.Delete(ctx, md)
			g.Expect(errors.Is(err, apiErr)).To(BeTrue(), "Delete returned %v", err)
			g.Expect(errors.Is(err, reconciler.ErrPending)).To(BeFalse())
			g.Expect(api.Calls).To(Equal([]string{getMDCall, deleteMDCall}))
			g.Expect(api.Databases).To(HaveKey(md.Status.UUID))
			g.Expect(api.Databases[md.Status.UUID].State).To(Equal(upcloud.ManagedDatabaseStateRunning))
		})
	}
}
