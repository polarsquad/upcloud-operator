package database

import (
	"context"
	"testing"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/polarsquad/upcloud-operator/api/common"
	databasev1alpha1 "github.com/polarsquad/upcloud-operator/api/database/v1alpha1"
	"github.com/polarsquad/upcloud-operator/internal/reconciler"
	"github.com/polarsquad/upcloud-operator/internal/upcloudapi/fake"
)

func newLogicalDB() *databasev1alpha1.ManagedDatabaseLogicalDatabase {
	return &databasev1alpha1.ManagedDatabaseLogicalDatabase{
		ObjectMeta: metav1.ObjectMeta{Name: testLDBName, Namespace: testNS, UID: "ldb-uid"},
		Spec: databasev1alpha1.ManagedDatabaseLogicalDatabaseSpec{
			ServiceRef: common.LocalObjectReference{Name: parentName},
		},
	}
}

func TestLogicalDatabaseCreate(t *testing.T) {
	g := NewWithT(t)
	c := newUserFakeDBClient(t)
	api := fake.NewDatabaseAPI()
	readyParentMD(g, c, api)

	a := &LogicalDatabaseAdapter{API: api, Client: c}
	ld := newLogicalDB()
	g.Expect(a.Create(context.Background(), ld)).To(Succeed())
	g.Expect(ld.Status.ServiceUUID).To(Equal(mdbParent))
	g.Expect(ld.Status.Name).To(Equal(testLDBName))
	g.Expect(api.Calls).To(ContainElement("CreateManagedDatabaseLogicalDatabase"))
	g.Expect(api.LogicalDBs[mdbParent]).To(HaveLen(1))
}

func TestLogicalDatabaseObservePresent(t *testing.T) {
	g := NewWithT(t)
	c := newUserFakeDBClient(t)
	api := fake.NewDatabaseAPI()
	readyParentMD(g, c, api)
	api.LogicalDBs[mdbParent] = []upcloud.ManagedDatabaseLogicalDatabase{
		{Name: testLDBName, LCCollate: "C"},
	}

	a := &LogicalDatabaseAdapter{API: api, Client: c}
	ld := newLogicalDB()
	ld.Status.ServiceUUID = mdbParent
	o, err := a.Observe(context.Background(), ld)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(o.Exists).To(BeTrue())
	g.Expect(o.UpToDate).To(BeTrue())
	g.Expect(o.Ready).To(BeTrue())
	g.Expect(ld.Status.Name).To(Equal(testLDBName))
}

func TestLogicalDatabaseDeleteIdempotent(t *testing.T) {
	g := NewWithT(t)
	c := newUserFakeDBClient(t)
	api := fake.NewDatabaseAPI()
	readyParentMD(g, c, api)

	a := &LogicalDatabaseAdapter{API: api, Client: c}
	ld := newLogicalDB()
	ld.Status.ServiceUUID = mdbParent

	// First delete removes it.
	g.Expect(a.Delete(context.Background(), ld)).To(Succeed())
	// Second delete is a no-op (already gone).
	g.Expect(a.Delete(context.Background(), ld)).To(Succeed())
	g.Expect(api.LogicalDBs[mdbParent]).To(BeEmpty())
}

func TestLogicalDatabaseObserveParentNotReady(t *testing.T) {
	g := NewWithT(t)
	c := newUserFakeDBClient(t)
	api := fake.NewDatabaseAPI()
	// No parent ManagedDatabase exists at all.
	a := &LogicalDatabaseAdapter{API: api, Client: c}
	ld := newLogicalDB()
	_, err := a.Observe(context.Background(), ld)
	g.Expect(err).To(MatchError(reconciler.ErrDependencyNotReady))
}
