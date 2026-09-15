package database

import (
	"context"
	"testing"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/polarsquad/upcloud-operator/api/common"
	databasev1alpha1 "github.com/polarsquad/upcloud-operator/api/database/v1alpha1"
	"github.com/polarsquad/upcloud-operator/internal/reconciler"
	"github.com/polarsquad/upcloud-operator/internal/upcloudapi/fake"
)

func newLogicalDB(name string) *databasev1alpha1.ManagedDatabaseLogicalDatabase {
	return &databasev1alpha1.ManagedDatabaseLogicalDatabase{
		ObjectMeta: v1.ObjectMeta{Name: name, Namespace: "ns", UID: "ldb-uid"},
		Spec: databasev1alpha1.ManagedDatabaseLogicalDatabaseSpec{
			ServiceRef: common.LocalObjectReference{Name: "parent"},
		},
	}
}

func TestLogicalDatabaseCreate(t *testing.T) {
	g := NewWithT(t)
	c := newUserFakeDBClient(t)
	api := fake.NewDatabaseAPI()
	readyParentMD(t, g, c, api)

	a := &LogicalDatabaseAdapter{API: api, Client: c}
	ld := newLogicalDB("analytics")
	g.Expect(a.Create(context.Background(), ld)).To(Succeed())
	g.Expect(ld.Status.ServiceUUID).To(Equal("mdb-parent"))
	g.Expect(ld.Status.Name).To(Equal("analytics"))
	g.Expect(api.Calls).To(ContainElement("CreateManagedDatabaseLogicalDatabase"))
	g.Expect(api.LogicalDBs["mdb-parent"]).To(HaveLen(1))
}

func TestLogicalDatabaseObservePresent(t *testing.T) {
	g := NewWithT(t)
	c := newUserFakeDBClient(t)
	api := fake.NewDatabaseAPI()
	readyParentMD(t, g, c, api)
	api.LogicalDBs["mdb-parent"] = []upcloud.ManagedDatabaseLogicalDatabase{
		{Name: "analytics", LCCollate: "C"},
	}

	a := &LogicalDatabaseAdapter{API: api, Client: c}
	ld := newLogicalDB("analytics")
	ld.Status.ServiceUUID = "mdb-parent"
	o, err := a.Observe(context.Background(), ld)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(o.Exists).To(BeTrue())
	g.Expect(o.UpToDate).To(BeTrue())
	g.Expect(o.Ready).To(BeTrue())
	g.Expect(ld.Status.Name).To(Equal("analytics"))
}

func TestLogicalDatabaseDeleteIdempotent(t *testing.T) {
	g := NewWithT(t)
	c := newUserFakeDBClient(t)
	api := fake.NewDatabaseAPI()
	readyParentMD(t, g, c, api)

	a := &LogicalDatabaseAdapter{API: api, Client: c}
	ld := newLogicalDB("analytics")
	ld.Status.ServiceUUID = "mdb-parent"

	// First delete removes it.
	g.Expect(a.Delete(context.Background(), ld)).To(Succeed())
	// Second delete is a no-op (already gone).
	g.Expect(a.Delete(context.Background(), ld)).To(Succeed())
	g.Expect(api.LogicalDBs["mdb-parent"]).To(BeEmpty())
}

func TestLogicalDatabaseObserveParentNotReady(t *testing.T) {
	g := NewWithT(t)
	c := newUserFakeDBClient(t)
	api := fake.NewDatabaseAPI()
	// No parent ManagedDatabase exists at all.
	a := &LogicalDatabaseAdapter{API: api, Client: c}
	ld := newLogicalDB("analytics")
	_, err := a.Observe(context.Background(), ld)
	g.Expect(err).To(MatchError(reconciler.ErrDependencyNotReady))
}
