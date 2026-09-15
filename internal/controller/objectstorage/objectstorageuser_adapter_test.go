package objectstorage

import (
	"context"
	"testing"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud/request"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/polarsquad/upcloud-operator/api/common"
	objectstoragev1alpha1 "github.com/polarsquad/upcloud-operator/api/objectstorage/v1alpha1"
	"github.com/polarsquad/upcloud-operator/internal/upcloudapi/fake"
)

func newUserCR(name string, policies []string) *objectstoragev1alpha1.ObjectStorageUser {
	return &objectstoragev1alpha1.ObjectStorageUser{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: testNS, UID: "user-uid"},
		Spec: objectstoragev1alpha1.ObjectStorageUserSpec{
			ServiceRef: common.LocalObjectReference{Name: parentName},
			Policies:   policies,
		},
	}
}

func TestUserCreateWithPoliciesAttaches(t *testing.T) {
	g := NewWithT(t)
	api := fake.NewObjectStorageAPI()
	c := newMOSClient(t)
	readyService(g, c, api)
	a := &ObjectStorageUserAdapter{API: api, Client: c}
	u := newUserCR("alice", []string{testROPolicy})
	ctx := context.Background()

	// Create the referenced policy in the fake first.
	_, err := api.CreateManagedObjectStoragePolicy(ctx, &request.CreateManagedObjectStoragePolicyRequest{ServiceUUID: parentUUID, Name: testROPolicy, Document: testPolicyDoc})
	g.Expect(err).NotTo(HaveOccurred())

	obs, err := a.Observe(ctx, u)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(obs.Exists).To(BeFalse())

	g.Expect(a.Create(ctx, u)).To(Succeed())
	g.Expect(u.Status.ServiceUUID).To(Equal(parentUUID))
	g.Expect(u.Status.Username).To(Equal("alice"))
	g.Expect(u.Status.ARN).NotTo(BeEmpty())

	// The policy is attached in the fake.
	fakeUser := api.Users[parentUUID][0]
	g.Expect(fakeUser.Policies).To(HaveLen(1))
	g.Expect(fakeUser.Policies[0].Name).To(Equal(testROPolicy))

	obs, err = a.Observe(ctx, u)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(obs.Exists).To(BeTrue())
	g.Expect(obs.UpToDate).To(BeTrue())
	g.Expect(u.Status.AttachedPolicies).To(ConsistOf(testROPolicy))
}

func TestUserUpdateDetachesExtraAndAttachesMissing(t *testing.T) {
	g := NewWithT(t)
	api := fake.NewObjectStorageAPI()
	c := newMOSClient(t)
	readyService(g, c, api)
	a := &ObjectStorageUserAdapter{API: api, Client: c}
	u := newUserCR("bob", []string{testROPolicy, "admin"})
	ctx := context.Background()

	for _, name := range []string{testROPolicy, "admin"} {
		_, err := api.CreateManagedObjectStoragePolicy(ctx, &request.CreateManagedObjectStoragePolicyRequest{ServiceUUID: parentUUID, Name: name, Document: testPolicyDoc})
		g.Expect(err).NotTo(HaveOccurred())
	}

	g.Expect(a.Create(ctx, u)).To(Succeed())

	// Drift: drop admin, add write.
	_, err := api.CreateManagedObjectStoragePolicy(ctx, &request.CreateManagedObjectStoragePolicyRequest{ServiceUUID: parentUUID, Name: "write", Document: testPolicyDoc})
	g.Expect(err).NotTo(HaveOccurred())
	u.Spec.Policies = []string{testROPolicy, "write"}

	obs, err := a.Observe(ctx, u)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(obs.UpToDate).To(BeFalse())

	g.Expect(a.Update(ctx, u)).To(Succeed())
	fakeUser := api.Users[parentUUID][0]
	g.Expect(fakeUser.Policies).To(HaveLen(2))
	names := []string{fakeUser.Policies[0].Name, fakeUser.Policies[1].Name}
	g.Expect(names).To(ConsistOf(testROPolicy, "write"))

	obs, err = a.Observe(ctx, u)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(obs.UpToDate).To(BeTrue())
}

func TestUserDeleteDetachesAndDeletesKeys(t *testing.T) {
	g := NewWithT(t)
	api := fake.NewObjectStorageAPI()
	c := newMOSClient(t)
	readyService(g, c, api)
	a := &ObjectStorageUserAdapter{API: api, Client: c}
	u := newUserCR("carol", []string{testROPolicy})
	ctx := context.Background()

	_, err := api.CreateManagedObjectStoragePolicy(ctx, &request.CreateManagedObjectStoragePolicyRequest{ServiceUUID: parentUUID, Name: testROPolicy, Document: testPolicyDoc})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(a.Create(ctx, u)).To(Succeed())

	// Add an access key to the user in the fake, then delete the user.
	_, err = api.CreateManagedObjectStorageUserAccessKey(ctx, &request.CreateManagedObjectStorageUserAccessKeyRequest{ServiceUUID: parentUUID, Username: "carol"})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(api.Users[parentUUID][0].AccessKeys).To(HaveLen(1))

	g.Expect(a.Delete(ctx, u)).To(Succeed())
	g.Expect(api.Users[parentUUID]).To(BeEmpty())
}
