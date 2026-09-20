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

func TestManagedDatabaseUserDeleteConflictReturnsPending(t *testing.T) {
	g := NewWithT(t)
	api := fake.NewDatabaseAPI()
	c := newUserFakeDBClient(t)
	readyParentMD(g, c, api)
	a := &ManagedDatabaseUserAdapter{API: api, Client: c}
	u := newUser(deleteUser)
	ctx := context.Background()
	g.Expect(a.Create(ctx, u)).To(Succeed())
	api.Calls = nil
	api.FailNext = fmt.Errorf("user in use: %w", fake.Conflict("database user"))

	err := a.Delete(ctx, u)
	g.Expect(errors.Is(err, reconciler.ErrPending)).To(BeTrue(), "Delete returned %v", err)
	g.Expect(api.Calls).To(Equal([]string{deleteUserCall}))
	g.Expect(api.Databases[mdbParent].Users).To(HaveLen(2))

	g.Expect(a.Delete(ctx, u)).To(Succeed())
	g.Expect(api.Databases[mdbParent].Users).To(HaveLen(1))
	g.Expect(api.Databases[mdbParent].Users[0].Username).To(Equal(upadminUser))
}

func TestManagedDatabaseUserDeleteEmptyIdentityMakesNoAPICalls(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status databasev1alpha1.ManagedDatabaseUserStatus
	}{
		{name: "empty_status"},
		{name: "empty_service_uuid", status: databasev1alpha1.ManagedDatabaseUserStatus{Username: deleteUser}},
		{name: "empty_username", status: databasev1alpha1.ManagedDatabaseUserStatus{ServiceUUID: mdbParent}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			api := fake.NewDatabaseAPI()
			c := newUserFakeDBClient(t)
			readyParentMD(g, c, api)
			a := &ManagedDatabaseUserAdapter{API: api, Client: c}
			u := newUser(deleteUser)
			ctx := context.Background()
			g.Expect(a.Create(ctx, u)).To(Succeed())
			u.Status = tc.status
			api.Calls = nil

			g.Expect(a.Delete(ctx, u)).To(Succeed())
			g.Expect(api.Calls).To(BeEmpty())
			g.Expect(api.Databases[mdbParent].Users).To(HaveLen(2))
		})
	}
}

func TestManagedDatabaseUserDeleteAbsentResourceSucceeds(t *testing.T) {
	for _, parentGone := range []bool{false, true} {
		t.Run(fmt.Sprintf("parent_gone_%t", parentGone), func(t *testing.T) {
			g := NewWithT(t)
			api := fake.NewDatabaseAPI()
			c := newUserFakeDBClient(t)
			readyParentMD(g, c, api)
			a := &ManagedDatabaseUserAdapter{API: api, Client: c}
			u := newUser(deleteUser)
			u.Status = databasev1alpha1.ManagedDatabaseUserStatus{ServiceUUID: mdbParent, Username: deleteUser}
			if parentGone {
				delete(api.Databases, mdbParent)
			}

			g.Expect(a.Delete(context.Background(), u)).To(Succeed())
			g.Expect(api.Calls).To(Equal([]string{deleteUserCall}))
			if !parentGone {
				g.Expect(api.Databases[mdbParent].Users).To(HaveLen(1))
				g.Expect(api.Databases[mdbParent].Users[0].Username).To(Equal(upadminUser))
			}
		})
	}
}

func TestManagedDatabaseUserDeleteUnexpectedErrorRemainsError(t *testing.T) {
	for name, apiErr := range deleteErrorCases() {
		t.Run(name, func(t *testing.T) {
			g := NewWithT(t)
			api := fake.NewDatabaseAPI()
			c := newUserFakeDBClient(t)
			readyParentMD(g, c, api)
			a := &ManagedDatabaseUserAdapter{API: api, Client: c}
			u := newUser(deleteUser)
			ctx := context.Background()
			g.Expect(a.Create(ctx, u)).To(Succeed())
			api.Calls = nil
			api.FailNext = apiErr

			err := a.Delete(ctx, u)
			g.Expect(errors.Is(err, apiErr)).To(BeTrue(), "Delete returned %v", err)
			g.Expect(errors.Is(err, reconciler.ErrPending)).To(BeFalse())
			g.Expect(api.Calls).To(Equal([]string{deleteUserCall}))
			g.Expect(api.Databases[mdbParent].Users).To(HaveLen(2))
		})
	}
}
