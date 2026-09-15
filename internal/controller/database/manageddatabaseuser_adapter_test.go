package database

import (
	"context"
	"testing"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	client "sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/polarsquad/upcloud-operator/api/common"
	databasev1alpha1 "github.com/polarsquad/upcloud-operator/api/database/v1alpha1"
	"github.com/polarsquad/upcloud-operator/internal/reconciler"
	"github.com/polarsquad/upcloud-operator/internal/upcloudapi/fake"
)

func newUserFakeDBClient(t *testing.T) client.Client {
	g := NewWithT(t)
	s := runtime.NewScheme()
	g.Expect(corev1.AddToScheme(s)).To(Succeed())
	g.Expect(databasev1alpha1.AddToScheme(s)).To(Succeed())
	return fakeclient.NewClientBuilder().WithScheme(s).Build()
}

// readyParentMD creates a Ready ManagedDatabase CR and matching fake service.
func readyParentMD(t *testing.T, g *GomegaWithT, c client.Client, api *fake.DatabaseAPI) (uuid string) {
	md := &databasev1alpha1.ManagedDatabase{
		ObjectMeta: metav1.ObjectMeta{Name: "parent", Namespace: "ns", UID: "md-uid", Generation: 1},
		Spec:       databasev1alpha1.ManagedDatabaseSpec{Type: "pg", Plan: "3x25", Zone: "fi-hel1"},
		Status: databasev1alpha1.ManagedDatabaseStatus{
			UUID: "mdb-parent",
			Conditions: []metav1.Condition{{
				Type: "Ready", Status: metav1.ConditionTrue, Reason: "Available",
				Message: "ok", ObservedGeneration: 1,
			}},
		},
	}
	g.Expect(c.Create(context.Background(), md)).To(Succeed())
	api.Databases["mdb-parent"] = &upcloud.ManagedDatabase{
		UUID:    "mdb-parent",
		Type:    upcloud.ManagedDatabaseServiceTypePostgreSQL,
		State:   upcloud.ManagedDatabaseStateRunning,
		Powered: true,
		ServiceURIParams: upcloud.ManagedDatabaseServiceURIParams{
			Host: "mdb-parent.db.upclouddatabases.com", Port: "11569",
			User: "upadmin", Password: "fake-pw", DatabaseName: "defaultdb", SSLMode: "require",
		},
		ServiceURI: "postgresql://upadmin:fake-pw@mdb-parent.db.upclouddatabases.com:11569/defaultdb?sslmode=require",
		Users:      []upcloud.ManagedDatabaseUser{{Username: "upadmin", Type: upcloud.ManagedDatabaseUserTypePrimary}},
	}
	return "mdb-parent"
}

func newUser(name string) *databasev1alpha1.ManagedDatabaseUser {
	return &databasev1alpha1.ManagedDatabaseUser{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "ns", UID: "user-uid"},
		Spec: databasev1alpha1.ManagedDatabaseUserSpec{
			ServiceRef: common.LocalObjectReference{Name: "parent"},
		},
	}
}

func TestManagedDatabaseUserCreateGeneratedPasswordWritesSecret(t *testing.T) {
	g := NewWithT(t)
	api := fake.NewDatabaseAPI()
	c := newUserFakeDBClient(t)
	readyParentMD(t, g, c, api)
	a := &ManagedDatabaseUserAdapter{API: api, Client: c}
	u := newUser("alice")
	ctx := context.Background()

	g.Expect(a.Create(ctx, u)).To(Succeed())
	g.Expect(u.Status.ServiceUUID).To(Equal("mdb-parent"))
	g.Expect(u.Status.Username).To(Equal("alice"))
	g.Expect(u.Status.Type).To(Equal("normal"))

	created := api.Databases["mdb-parent"].Users
	g.Expect(created).To(HaveLen(2))
	g.Expect(created[1].Username).To(Equal("alice"))
	g.Expect(created[1].Password).ToNot(BeEmpty())

	var sec corev1.Secret
	g.Expect(c.Get(ctx, types.NamespacedName{Namespace: "ns", Name: "alice-credentials"}, &sec)).To(Succeed())
	for _, k := range []string{"username", "password", "host", "port", "uri"} {
		g.Expect(sec.Data).To(HaveKey(k), "secret missing key %s", k)
	}
	g.Expect(string(sec.Data["username"])).To(Equal("alice"))
	g.Expect(string(sec.Data["host"])).To(Equal("mdb-parent.db.upclouddatabases.com"))
	g.Expect(string(sec.Data["port"])).To(Equal("11569"))
	g.Expect(string(sec.Data["uri"])).To(HavePrefix("postgresql://alice:"))
}

func TestManagedDatabaseUserCreateWithPasswordRef(t *testing.T) {
	g := NewWithT(t)
	api := fake.NewDatabaseAPI()
	c := newUserFakeDBClient(t)
	readyParentMD(t, g, c, api)
	a := &ManagedDatabaseUserAdapter{API: api, Client: c}
	u := newUser("bob")
	u.Spec.PasswordSecretRef = &common.SecretKeySelector{Name: "bob-pw", Key: "value"}
	ctx := context.Background()

	sec := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "bob-pw", Namespace: "ns"}}
	sec.Data = map[string][]byte{"value": []byte("s3cret")}
	g.Expect(c.Create(ctx, sec)).To(Succeed())

	g.Expect(a.Create(ctx, u)).To(Succeed())
	var found *upcloud.ManagedDatabaseUser
	for i := range api.Databases["mdb-parent"].Users {
		if api.Databases["mdb-parent"].Users[i].Username == "bob" {
			found = &api.Databases["mdb-parent"].Users[i]
		}
	}
	g.Expect(found).NotTo(BeNil())
	g.Expect(found.Password).To(Equal("s3cret"))

	var out corev1.Secret
	g.Expect(c.Get(ctx, types.NamespacedName{Namespace: "ns", Name: "bob-credentials"}, &out)).To(Succeed())
	g.Expect(string(out.Data["password"])).To(Equal("s3cret"))
}

func TestManagedDatabaseUserCreateMissingSecretRef(t *testing.T) {
	g := NewWithT(t)
	api := fake.NewDatabaseAPI()
	c := newUserFakeDBClient(t)
	readyParentMD(t, g, c, api)
	a := &ManagedDatabaseUserAdapter{API: api, Client: c}
	u := newUser("carol")
	u.Spec.PasswordSecretRef = &common.SecretKeySelector{Name: "missing", Key: "value"}
	g.Expect(a.Create(context.Background(), u)).To(MatchError(reconciler.ErrDependencyNotReady))
	g.Expect(api.Databases["mdb-parent"].Users).To(HaveLen(1))
}

func TestManagedDatabaseUserPasswordDriftModifies(t *testing.T) {
	g := NewWithT(t)
	api := fake.NewDatabaseAPI()
	c := newUserFakeDBClient(t)
	readyParentMD(t, g, c, api)
	a := &ManagedDatabaseUserAdapter{API: api, Client: c}
	u := newUser("dave")
	ctx := context.Background()
	g.Expect(a.Create(ctx, u)).To(Succeed())

	obs, err := a.Observe(ctx, u)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(obs.UpToDate).To(BeTrue())

	// Simulate the user rotating the password in the stored Secret.
	g.Expect(c.Get(ctx, types.NamespacedName{Namespace: "ns", Name: "dave-credentials"}, &corev1.Secret{})).To(Succeed())
	sec := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "dave-credentials", Namespace: "ns"}}
	g.Expect(c.Get(ctx, types.NamespacedName{Namespace: "ns", Name: "dave-credentials"}, sec)).To(Succeed())
	sec.Data["password"] = []byte("rotated")
	g.Expect(c.Update(ctx, sec)).To(Succeed())

	obs, err = a.Observe(ctx, u)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(obs.UpToDate).To(BeFalse())
	g.Expect(a.Update(ctx, u)).To(Succeed())
	g.Expect(api.Calls).To(ContainElement("ModifyManagedDatabaseUser"))
	got := api.Databases["mdb-parent"].Users
	for _, usr := range got {
		if usr.Username == "dave" {
			g.Expect(usr.Password).To(Equal("rotated"))
		}
	}
}

func TestManagedDatabaseUserAccessControlDrift(t *testing.T) {
	g := NewWithT(t)
	api := fake.NewDatabaseAPI()
	c := newUserFakeDBClient(t)
	readyParentMD(t, g, c, api)
	a := &ManagedDatabaseUserAdapter{API: api, Client: c}
	u := newUser("erin")
	allow := true
	u.Spec.PGAccessControl = &databasev1alpha1.PGUserAccessControl{AllowReplication: &allow}
	ctx := context.Background()
	g.Expect(a.Create(ctx, u)).To(Succeed())

	// Simulate a drift: the stored user no longer allows replication.
	for i := range api.Databases["mdb-parent"].Users {
		if api.Databases["mdb-parent"].Users[i].Username == "erin" {
			api.Databases["mdb-parent"].Users[i].PGAccessControl = nil
		}
	}
	obs, err := a.Observe(ctx, u)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(obs.UpToDate).To(BeFalse())
	g.Expect(a.Update(ctx, u)).To(Succeed())
	g.Expect(api.Calls).To(ContainElement("ModifyManagedDatabaseUserAccessControl"))
	for _, usr := range api.Databases["mdb-parent"].Users {
		if usr.Username == "erin" {
			g.Expect(usr.PGAccessControl).NotTo(BeNil())
			g.Expect(*usr.PGAccessControl.AllowReplication).To(BeTrue())
		}
	}
}

func TestManagedDatabaseUserDeletePrimaryRefused(t *testing.T) {
	g := NewWithT(t)
	api := fake.NewDatabaseAPI()
	c := newUserFakeDBClient(t)
	readyParentMD(t, g, c, api)
	a := &ManagedDatabaseUserAdapter{API: api, Client: c}
	admin := newUser("upadmin")
	admin.Spec.Username = "upadmin"
	admin.Status = databasev1alpha1.ManagedDatabaseUserStatus{ServiceUUID: "mdb-parent", Username: "upadmin", Type: "primary"}
	err := a.Delete(context.Background(), admin)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("primary"))
	g.Expect(api.Calls).NotTo(ContainElement("DeleteManagedDatabaseUser"))
}

func TestManagedDatabaseUserDeleteParentGone(t *testing.T) {
	g := NewWithT(t)
	api := fake.NewDatabaseAPI()
	c := newUserFakeDBClient(t)
	readyParentMD(t, g, c, api)
	a := &ManagedDatabaseUserAdapter{API: api, Client: c}
	u := newUser("frank")
	ctx := context.Background()
	g.Expect(a.Create(ctx, u)).To(Succeed())

	// Parent service is gone from both UpCloud and the cluster.
	delete(api.Databases, "mdb-parent")
	g.Expect(c.Delete(ctx, &databasev1alpha1.ManagedDatabase{ObjectMeta: metav1.ObjectMeta{Name: "parent", Namespace: "ns"}})).To(Succeed())
	g.Expect(a.Delete(ctx, u)).To(Succeed())
}
