package database

import (
	"context"
	"errors"
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
func readyParentMD(g *GomegaWithT, c client.Client, api *fake.DatabaseAPI) {
	md := &databasev1alpha1.ManagedDatabase{
		ObjectMeta: metav1.ObjectMeta{Name: parentName, Namespace: testNS, UID: "md-uid", Generation: 1},
		Spec:       databasev1alpha1.ManagedDatabaseSpec{Type: "pg", Plan: testPlan, Zone: testZone},
		Status: databasev1alpha1.ManagedDatabaseStatus{
			UUID: mdbParent,
			Conditions: []metav1.Condition{{
				Type: "Ready", Status: metav1.ConditionTrue, Reason: "Available",
				Message: "ok", ObservedGeneration: 1,
			}},
		},
	}
	g.Expect(c.Create(context.Background(), md)).To(Succeed())
	api.Databases[mdbParent] = &upcloud.ManagedDatabase{
		UUID:    mdbParent,
		Type:    upcloud.ManagedDatabaseServiceTypePostgreSQL,
		State:   upcloud.ManagedDatabaseStateRunning,
		Powered: true,
		ServiceURIParams: upcloud.ManagedDatabaseServiceURIParams{
			Host: mdbParent + ".db.upclouddatabases.com", Port: "11569",
			User: upadminUser, Password: "fake-pw", DatabaseName: "defaultdb", SSLMode: "require",
		},
		ServiceURI: "postgresql://upadmin:***@mdb-parent.db.upclouddatabases.com:11569/defaultdb?sslmode=require",
		Users:      []upcloud.ManagedDatabaseUser{{Username: upadminUser, Type: upcloud.ManagedDatabaseUserTypePrimary}},
	}
}

func newUser(name string) *databasev1alpha1.ManagedDatabaseUser {
	return &databasev1alpha1.ManagedDatabaseUser{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: testNS, UID: "user-uid"},
		Spec: databasev1alpha1.ManagedDatabaseUserSpec{
			ServiceRef: common.LocalObjectReference{Name: parentName},
		},
	}
}

func TestManagedDatabaseUserCreateGeneratedPasswordWritesSecret(t *testing.T) {
	g := NewWithT(t)
	api := fake.NewDatabaseAPI()
	c := newUserFakeDBClient(t)
	readyParentMD(g, c, api)
	a := &ManagedDatabaseUserAdapter{API: api, Client: c}
	u := newUser("alice")
	ctx := context.Background()

	g.Expect(a.Create(ctx, u)).To(Succeed())
	g.Expect(u.Status.ServiceUUID).To(Equal(mdbParent))
	g.Expect(u.Status.Username).To(Equal("alice"))
	g.Expect(u.Status.Type).To(Equal("normal"))

	created := api.Databases[mdbParent].Users
	g.Expect(created).To(HaveLen(2))
	g.Expect(created[1].Username).To(Equal("alice"))
	g.Expect(created[1].Password).ToNot(BeEmpty())

	var sec corev1.Secret
	g.Expect(c.Get(ctx, types.NamespacedName{Namespace: testNS, Name: "alice-credentials"}, &sec)).To(Succeed())
	for _, k := range []string{SecretKeyUsername, SecretKeyPassword, SecretKeyHost, SecretKeyPort, SecretKeyURI} {
		g.Expect(sec.Data).To(HaveKey(k), "secret missing key %s", k)
	}
	g.Expect(string(sec.Data[SecretKeyUsername])).To(Equal("alice"))
	g.Expect(string(sec.Data[SecretKeyHost])).To(Equal(mdbParent + ".db.upclouddatabases.com"))
	g.Expect(string(sec.Data[SecretKeyPort])).To(Equal("11569"))
	g.Expect(string(sec.Data[SecretKeyURI])).To(HavePrefix("postgresql://alice:"))
}

func TestManagedDatabaseUserCreateWithPasswordRef(t *testing.T) {
	g := NewWithT(t)
	api := fake.NewDatabaseAPI()
	c := newUserFakeDBClient(t)
	readyParentMD(g, c, api)
	a := &ManagedDatabaseUserAdapter{API: api, Client: c}
	u := newUser("bob")
	u.Spec.PasswordSecretRef = &common.SecretKeySelector{Name: "bob-pw", Key: pwValue}
	ctx := context.Background()

	sec := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "bob-pw", Namespace: testNS}}
	sec.Data = map[string][]byte{pwValue: []byte("s3cret")}
	g.Expect(c.Create(ctx, sec)).To(Succeed())

	g.Expect(a.Create(ctx, u)).To(Succeed())
	var found *upcloud.ManagedDatabaseUser
	for i := range api.Databases[mdbParent].Users {
		if api.Databases[mdbParent].Users[i].Username == "bob" {
			found = &api.Databases[mdbParent].Users[i]
		}
	}
	g.Expect(found).NotTo(BeNil())
	g.Expect(found.Password).To(Equal("s3cret"))

	var out corev1.Secret
	g.Expect(c.Get(ctx, types.NamespacedName{Namespace: testNS, Name: "bob-credentials"}, &out)).To(Succeed())
	g.Expect(string(out.Data[SecretKeyPassword])).To(Equal("s3cret"))
}

func TestManagedDatabaseUserCreateMissingSecretRef(t *testing.T) {
	g := NewWithT(t)
	api := fake.NewDatabaseAPI()
	c := newUserFakeDBClient(t)
	readyParentMD(g, c, api)
	a := &ManagedDatabaseUserAdapter{API: api, Client: c}
	u := newUser("carol")
	u.Spec.PasswordSecretRef = &common.SecretKeySelector{Name: "missing", Key: pwValue}
	g.Expect(a.Create(context.Background(), u)).To(MatchError(reconciler.ErrDependencyNotReady))
	g.Expect(api.Databases[mdbParent].Users).To(HaveLen(1))
}

func TestManagedDatabaseUserPasswordDriftModifies(t *testing.T) {
	g := NewWithT(t)
	api := fake.NewDatabaseAPI()
	c := newUserFakeDBClient(t)
	readyParentMD(g, c, api)
	a := &ManagedDatabaseUserAdapter{API: api, Client: c}
	u := newUser("dave")
	ctx := context.Background()
	g.Expect(a.Create(ctx, u)).To(Succeed())

	obs, err := a.Observe(ctx, u)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(obs.UpToDate).To(BeTrue())

	// Simulate the user rotating the password in the stored Secret.
	g.Expect(c.Get(ctx, types.NamespacedName{Namespace: testNS, Name: daveCreds}, &corev1.Secret{})).To(Succeed())
	sec := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: daveCreds, Namespace: testNS}}
	g.Expect(c.Get(ctx, types.NamespacedName{Namespace: testNS, Name: daveCreds}, sec)).To(Succeed())
	sec.Data[SecretKeyPassword] = []byte("rotated")
	g.Expect(c.Update(ctx, sec)).To(Succeed())

	obs, err = a.Observe(ctx, u)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(obs.UpToDate).To(BeFalse())
	g.Expect(a.Update(ctx, u)).To(Succeed())
	g.Expect(api.Calls).To(ContainElement("ModifyManagedDatabaseUser"))
	got := api.Databases[mdbParent].Users
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
	readyParentMD(g, c, api)
	a := &ManagedDatabaseUserAdapter{API: api, Client: c}
	u := newUser("erin")
	allow := true
	u.Spec.PGAccessControl = &databasev1alpha1.PGUserAccessControl{AllowReplication: &allow}
	ctx := context.Background()
	g.Expect(a.Create(ctx, u)).To(Succeed())

	// Simulate a drift: the stored user no longer allows replication.
	for i := range api.Databases[mdbParent].Users {
		if api.Databases[mdbParent].Users[i].Username == "erin" {
			api.Databases[mdbParent].Users[i].PGAccessControl = nil
		}
	}
	obs, err := a.Observe(ctx, u)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(obs.UpToDate).To(BeFalse())
	g.Expect(a.Update(ctx, u)).To(Succeed())
	g.Expect(api.Calls).To(ContainElement("ModifyManagedDatabaseUserAccessControl"))
	for _, usr := range api.Databases[mdbParent].Users {
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
	readyParentMD(g, c, api)
	a := &ManagedDatabaseUserAdapter{API: api, Client: c}
	admin := newUser(upadminUser)
	admin.Spec.Username = upadminUser
	admin.Status = databasev1alpha1.ManagedDatabaseUserStatus{ServiceUUID: mdbParent, Username: upadminUser, Type: "primary"}
	err := a.Delete(context.Background(), admin)
	g.Expect(err).To(HaveOccurred())
	g.Expect(errors.Is(err, reconciler.ErrPending)).To(BeFalse())
	g.Expect(err.Error()).To(ContainSubstring("primary"))
	g.Expect(api.Calls).NotTo(ContainElement("DeleteManagedDatabaseUser"))
}

func TestManagedDatabaseUserDeleteParentGone(t *testing.T) {
	g := NewWithT(t)
	api := fake.NewDatabaseAPI()
	c := newUserFakeDBClient(t)
	readyParentMD(g, c, api)
	a := &ManagedDatabaseUserAdapter{API: api, Client: c}
	u := newUser("frank")
	ctx := context.Background()
	g.Expect(a.Create(ctx, u)).To(Succeed())

	// Parent service is gone from both UpCloud and the cluster.
	delete(api.Databases, mdbParent)
	g.Expect(c.Delete(ctx, &databasev1alpha1.ManagedDatabase{ObjectMeta: metav1.ObjectMeta{Name: parentName, Namespace: testNS}})).To(Succeed())
	g.Expect(a.Delete(ctx, u)).To(Succeed())
}
