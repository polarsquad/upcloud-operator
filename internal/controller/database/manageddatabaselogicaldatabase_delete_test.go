package database

import (
	"context"
	"errors"
	"fmt"
	"testing"

	. "github.com/onsi/gomega"

	databasev1alpha1 "github.com/polarsquad/upcloud-operator/api/database/v1alpha1"
	"github.com/polarsquad/upcloud-operator/internal/reconciler"
	"github.com/polarsquad/upcloud-operator/internal/upcloudapi/fake"
)

func TestManagedDatabaseLogicalDatabaseDeleteConflictReturnsPending(t *testing.T) {
	g := NewWithT(t)
	api := fake.NewDatabaseAPI()
	c := newUserFakeDBClient(t)
	readyParentMD(g, c, api)
	a := &LogicalDatabaseAdapter{API: api, Client: c}
	ld := newLogicalDB()
	ctx := context.Background()
	g.Expect(a.Create(ctx, ld)).To(Succeed())
	api.Calls = nil
	api.FailNext = fmt.Errorf("database in use: %w", fake.Conflict("logical database"))

	err := a.Delete(ctx, ld)
	g.Expect(errors.Is(err, reconciler.ErrPending)).To(BeTrue(), "Delete returned %v", err)
	g.Expect(api.Calls).To(Equal([]string{deleteLDBCall}))
	g.Expect(api.LogicalDBs[mdbParent]).To(HaveLen(1))

	g.Expect(a.Delete(ctx, ld)).To(Succeed())
	g.Expect(api.LogicalDBs[mdbParent]).To(BeEmpty())
}

func TestManagedDatabaseLogicalDatabaseDeleteEmptyIdentityMakesNoAPICalls(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status databasev1alpha1.ManagedDatabaseLogicalDatabaseStatus
	}{
		{name: "empty_status"},
		{name: "empty_service_uuid", status: databasev1alpha1.ManagedDatabaseLogicalDatabaseStatus{Name: testLDBName}},
		{name: "empty_name", status: databasev1alpha1.ManagedDatabaseLogicalDatabaseStatus{ServiceUUID: mdbParent}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			api := fake.NewDatabaseAPI()
			c := newUserFakeDBClient(t)
			readyParentMD(g, c, api)
			a := &LogicalDatabaseAdapter{API: api, Client: c}
			ld := newLogicalDB()
			ctx := context.Background()
			g.Expect(a.Create(ctx, ld)).To(Succeed())
			ld.Status = tc.status
			api.Calls = nil

			g.Expect(a.Delete(ctx, ld)).To(Succeed())
			g.Expect(api.Calls).To(BeEmpty())
			g.Expect(api.LogicalDBs[mdbParent]).To(HaveLen(1))
		})
	}
}

func TestManagedDatabaseLogicalDatabaseDeleteAbsentResourceSucceeds(t *testing.T) {
	for _, parentGone := range []bool{false, true} {
		t.Run(fmt.Sprintf("parent_gone_%t", parentGone), func(t *testing.T) {
			g := NewWithT(t)
			api := fake.NewDatabaseAPI()
			c := newUserFakeDBClient(t)
			readyParentMD(g, c, api)
			a := &LogicalDatabaseAdapter{API: api, Client: c}
			ld := newLogicalDB()
			ld.Status = databasev1alpha1.ManagedDatabaseLogicalDatabaseStatus{ServiceUUID: mdbParent, Name: testLDBName}
			if parentGone {
				delete(api.Databases, mdbParent)
			}

			g.Expect(a.Delete(context.Background(), ld)).To(Succeed())
			g.Expect(api.Calls).To(Equal([]string{deleteLDBCall}))
			g.Expect(api.LogicalDBs[mdbParent]).To(BeEmpty())
		})
	}
}

func TestManagedDatabaseLogicalDatabaseDeleteUnexpectedErrorRemainsError(t *testing.T) {
	for name, apiErr := range deleteErrorCases() {
		t.Run(name, func(t *testing.T) {
			g := NewWithT(t)
			api := fake.NewDatabaseAPI()
			c := newUserFakeDBClient(t)
			readyParentMD(g, c, api)
			a := &LogicalDatabaseAdapter{API: api, Client: c}
			ld := newLogicalDB()
			ctx := context.Background()
			g.Expect(a.Create(ctx, ld)).To(Succeed())
			api.Calls = nil
			api.FailNext = apiErr

			err := a.Delete(ctx, ld)
			g.Expect(errors.Is(err, apiErr)).To(BeTrue(), "Delete returned %v", err)
			g.Expect(errors.Is(err, reconciler.ErrPending)).To(BeFalse())
			g.Expect(api.Calls).To(Equal([]string{deleteLDBCall}))
			g.Expect(api.LogicalDBs[mdbParent]).To(HaveLen(1))
		})
	}
}
