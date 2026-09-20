package database

import (
	"context"
	"errors"
	"testing"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/polarsquad/upcloud-operator/api/common"
	databasev1alpha1 "github.com/polarsquad/upcloud-operator/api/database/v1alpha1"
	networkv1alpha1 "github.com/polarsquad/upcloud-operator/api/network/v1alpha1"
	"github.com/polarsquad/upcloud-operator/internal/reconciler"
	"github.com/polarsquad/upcloud-operator/internal/upcloudapi/fake"
)

func newFakeDBClient(t *testing.T) client.Client {
	g := NewWithT(t)
	s := runtime.NewScheme()
	g.Expect(corev1.AddToScheme(s)).To(Succeed())
	g.Expect(databasev1alpha1.AddToScheme(s)).To(Succeed())
	g.Expect(networkv1alpha1.AddToScheme(s)).To(Succeed())
	return fakeclient.NewClientBuilder().WithScheme(s).Build()
}

func newManagedDatabase() *databasev1alpha1.ManagedDatabase {
	return &databasev1alpha1.ManagedDatabase{
		ObjectMeta: metav1.ObjectMeta{Name: "md1", Namespace: "ns", UID: "md-uid"},
		Spec: databasev1alpha1.ManagedDatabaseSpec{
			Type:       "pg",
			Plan:       testPlan,
			Zone:       testZone,
			Properties: &apiextensionsv1.JSON{Raw: []byte(`{"version":"16","ip_filter":["0.0.0.0/0"]}`)},
		},
	}
}

func readyNetworkCR(name, uuid string) *networkv1alpha1.Network {
	return &networkv1alpha1.Network{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "ns", UID: "net-uid", Generation: 1},
		Spec:       networkv1alpha1.NetworkSpec{Zone: testZone},
		Status: networkv1alpha1.NetworkStatus{
			UUID: uuid,
			Conditions: []metav1.Condition{{
				Type: "Ready", Status: metav1.ConditionTrue, Reason: "Available",
				Message: "ok", ObservedGeneration: 1,
			}},
		},
	}
}

func TestManagedDatabaseCreateSetsUUIDAndUsesDefaults(t *testing.T) {
	g := NewWithT(t)
	api := fake.NewDatabaseAPI()
	a := &ManagedDatabaseAdapter{API: api, Client: newFakeDBClient(t)}
	md := newManagedDatabase()
	ctx := context.Background()

	obs, err := a.Observe(ctx, md)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(obs.Exists).To(BeFalse())

	g.Expect(a.Create(ctx, md)).To(Succeed())
	g.Expect(md.Status.UUID).To(HavePrefix("mdb-"))
	created := api.Databases[md.Status.UUID]
	g.Expect(created.Title).To(Equal("md1"))
	g.Expect(created.Name).To(Equal("md1"))
	g.Expect(string(created.Type)).To(Equal("pg"))
	g.Expect(created.Plan).To(Equal(testPlan))
	g.Expect(created.Zone).To(Equal(testZone))
	g.Expect(created.Properties["version"]).To(Equal("16"))
	g.Expect(created.Properties["ip_filter"]).To(Equal([]any{"0.0.0.0/0"}))
}

func TestManagedDatabaseObserveWritesConnectionSecret(t *testing.T) {
	g := NewWithT(t)
	api := fake.NewDatabaseAPI()
	c := newFakeDBClient(t)
	a := &ManagedDatabaseAdapter{API: api, Client: c}
	md := newManagedDatabase()
	ctx := context.Background()
	g.Expect(a.Create(ctx, md)).To(Succeed())

	obs, err := a.Observe(ctx, md)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(obs.Exists).To(BeTrue())
	g.Expect(obs.UpToDate).To(BeTrue())
	g.Expect(obs.Ready).To(BeTrue())

	var sec corev1.Secret
	g.Expect(c.Get(ctx, types.NamespacedName{Namespace: "ns", Name: md1ConnName}, &sec)).To(Succeed())
	for _, k := range []string{SecretKeyURI, SecretKeyHost, SecretKeyPort, SecretKeyUser, SecretKeyPassword, SecretKeyDBName, SecretKeySSLMode} {
		g.Expect(sec.Data).To(HaveKey(k), "secret missing key %s", k)
	}
	g.Expect(string(sec.Data[SecretKeyHost])).To(Equal(md.Status.UUID + ".db.upclouddatabases.com"))
	g.Expect(string(sec.Data["port"])).To(Equal("11569"))
	g.Expect(string(sec.Data["user"])).To(Equal("upadmin"))
	g.Expect(string(sec.Data["password"])).To(Equal("fake-pw"))
	g.Expect(string(sec.Data["dbname"])).To(Equal("defaultdb"))
	g.Expect(string(sec.Data["sslmode"])).To(Equal("require"))
	g.Expect(string(sec.Data["uri"])).To(HavePrefix("postgresql://upadmin:" + string(sec.Data["password"]) + "@"))
	g.Expect(md.Status.PrimaryHost).To(Equal(md.Status.UUID + ".db.upclouddatabases.com"))
	g.Expect(md.Status.PrimaryPort).To(Equal(11569))
}

func TestManagedDatabaseObserveNotReadyWhileRebuilding(t *testing.T) {
	g := NewWithT(t)
	api := fake.NewDatabaseAPI()
	c := newFakeDBClient(t)
	a := &ManagedDatabaseAdapter{API: api, Client: c}
	md := newManagedDatabase()
	ctx := context.Background()
	g.Expect(a.Create(ctx, md)).To(Succeed())

	api.StateOverride = upcloud.ManagedDatabaseStateRebuilding
	obs, err := a.Observe(ctx, md)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(obs.Exists).To(BeTrue())
	g.Expect(obs.Ready).To(BeFalse())
	g.Expect(obs.Message).To(Equal("state rebuilding"))

	var sec corev1.Secret
	err = c.Get(ctx, types.NamespacedName{Namespace: "ns", Name: md1ConnName}, &sec)
	g.Expect(apierrors.IsNotFound(err)).To(BeTrue())

	api.StateOverride = ""
	obs, err = a.Observe(ctx, md)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(obs.Ready).To(BeTrue())
	g.Expect(c.Get(ctx, types.NamespacedName{Namespace: "ns", Name: md1ConnName}, &sec)).To(Succeed())
}

func TestManagedDatabaseObserveDriftOnPlanAndUpdateModifies(t *testing.T) {
	g := NewWithT(t)
	api := fake.NewDatabaseAPI()
	a := &ManagedDatabaseAdapter{API: api, Client: newFakeDBClient(t)}
	md := newManagedDatabase()
	ctx := context.Background()
	g.Expect(a.Create(ctx, md)).To(Succeed())
	uuid := md.Status.UUID

	md.Spec.Plan = "4x50"
	obs, err := a.Observe(ctx, md)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(obs.UpToDate).To(BeFalse())

	g.Expect(a.Update(ctx, md)).To(Succeed())
	g.Expect(api.Calls).To(ContainElement("ModifyManagedDatabase"))
	g.Expect(api.Databases[uuid].Plan).To(Equal("4x50"))

	obs, err = a.Observe(ctx, md)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(obs.UpToDate).To(BeTrue())
}

func TestManagedDatabaseUpdateWhilePendingReturnsErrPending(t *testing.T) {
	g := NewWithT(t)
	api := fake.NewDatabaseAPI()
	a := &ManagedDatabaseAdapter{API: api, Client: newFakeDBClient(t)}
	md := newManagedDatabase()
	ctx := context.Background()
	g.Expect(a.Create(ctx, md)).To(Succeed())

	api.StateOverride = upcloud.ManagedDatabaseStateRebuilding
	defer func() { api.StateOverride = "" }()
	err := a.Update(ctx, md)
	g.Expect(err).To(MatchError(reconciler.ErrPending))
	g.Expect(api.Calls).NotTo(ContainElement("ModifyManagedDatabase"))
}

func TestManagedDatabasePoweredFalseShutsDown(t *testing.T) {
	g := NewWithT(t)
	api := fake.NewDatabaseAPI()
	a := &ManagedDatabaseAdapter{API: api, Client: newFakeDBClient(t)}
	md := newManagedDatabase()
	ctx := context.Background()
	g.Expect(a.Create(ctx, md)).To(Succeed())
	uuid := md.Status.UUID

	off := false
	md.Spec.Powered = &off
	obs, err := a.Observe(ctx, md)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(obs.UpToDate).To(BeFalse())
	g.Expect(obs.Ready).To(BeTrue())

	g.Expect(a.Update(ctx, md)).To(Succeed())
	g.Expect(api.Calls).To(ContainElement("ShutdownManagedDatabase"))
	g.Expect(api.Databases[uuid].Powered).To(BeFalse())
	g.Expect(api.Databases[uuid].State).To(Equal(upcloud.ManagedDatabaseStateStopped))

	obs, err = a.Observe(ctx, md)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(obs.UpToDate).To(BeTrue())
	g.Expect(obs.Ready).To(BeTrue())
}

func TestManagedDatabaseNetworkRefResolved(t *testing.T) {
	g := NewWithT(t)
	api := fake.NewDatabaseAPI()
	c := newFakeDBClient(t)
	a := &ManagedDatabaseAdapter{API: api, Client: c}
	md := newManagedDatabase()
	ctx := context.Background()
	g.Expect(c.Create(ctx, readyNetworkCR("net", "net-1"))).To(Succeed())
	md.Spec.Networks = []common.NetworkAttachment{
		{Name: "priv", Type: "private", Family: "IPv4", NetworkRef: &common.LocalObjectReference{Name: "net"}},
	}

	g.Expect(a.Create(ctx, md)).To(Succeed())
	created := api.Databases[md.Status.UUID]
	g.Expect(created.Networks).To(HaveLen(1))
	g.Expect(created.Networks[0].Name).To(Equal("priv"))
	g.Expect(created.Networks[0].Type).To(Equal("private"))
	g.Expect(created.Networks[0].Family).To(Equal("IPv4"))
	g.Expect(created.Networks[0].UUID).ToNot(BeNil())
	g.Expect(*created.Networks[0].UUID).To(Equal("net-1"))
}

func TestManagedDatabaseDeleteWithTerminationProtectionFails(t *testing.T) {
	g := NewWithT(t)
	api := fake.NewDatabaseAPI()
	a := &ManagedDatabaseAdapter{API: api, Client: newFakeDBClient(t)}
	md := newManagedDatabase()
	ctx := context.Background()
	g.Expect(a.Create(ctx, md)).To(Succeed())
	api.Databases[md.Status.UUID].TerminationProtection = true

	err := a.Delete(ctx, md)
	g.Expect(err).To(HaveOccurred())
	g.Expect(errors.Is(err, reconciler.ErrPending)).To(BeFalse())
	g.Expect(err.Error()).To(ContainSubstring("termination"))
}

func TestManagedDatabaseDeleteIsPendingUntilGone(t *testing.T) {
	g := NewWithT(t)
	api := fake.NewDatabaseAPI()
	a := &ManagedDatabaseAdapter{API: api, Client: newFakeDBClient(t)}
	md := newManagedDatabase()
	ctx := context.Background()
	g.Expect(a.Create(ctx, md)).To(Succeed())

	err := a.Delete(ctx, md)
	g.Expect(errors.Is(err, reconciler.ErrPending)).To(BeTrue(), "Delete returned %v", err)
	g.Expect(api.Databases).NotTo(BeEmpty())

	g.Expect(a.Delete(ctx, md)).To(Succeed())
	g.Expect(api.Databases).To(BeEmpty())
	g.Expect(a.Delete(ctx, md)).To(Succeed())
}

func TestManagedDatabaseAdoptsByLabel(t *testing.T) {
	g := NewWithT(t)
	api := fake.NewDatabaseAPI()
	a := &ManagedDatabaseAdapter{API: api, Client: newFakeDBClient(t)}
	md := newManagedDatabase()
	ctx := context.Background()
	g.Expect(a.Create(ctx, md)).To(Succeed())
	uuid := md.Status.UUID
	md.Status.UUID = ""

	obs, err := a.Observe(ctx, md)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(obs.Exists).To(BeTrue())
	g.Expect(md.Status.UUID).To(Equal(uuid))
}
