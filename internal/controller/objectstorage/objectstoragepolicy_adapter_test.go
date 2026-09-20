package objectstorage

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud/request"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	"github.com/polarsquad/upcloud-operator/api/common"
	objectstoragev1alpha1 "github.com/polarsquad/upcloud-operator/api/objectstorage/v1alpha1"
	"github.com/polarsquad/upcloud-operator/internal/reconciler"
	"github.com/polarsquad/upcloud-operator/internal/upcloudapi/fake"
)

const testPolicyDoc = `{"Version":"2012-10-17","Statement":[]}`

func newPolicy(name string) *objectstoragev1alpha1.ObjectStoragePolicy {
	return &objectstoragev1alpha1.ObjectStoragePolicy{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: testNS, UID: "pol-uid"},
		Spec: objectstoragev1alpha1.ObjectStoragePolicySpec{
			ServiceRef: common.LocalObjectReference{Name: parentName},
			Document:   testPolicyDoc,
		},
	}
}

func TestPolicyCreateRoundTripsDocument(t *testing.T) {
	g := NewWithT(t)
	api := fake.NewObjectStorageAPI()
	c := newMOSClient(t)
	readyService(g, c, api)
	a := &ObjectStoragePolicyAdapter{API: api, Client: c}
	p := newPolicy("admin")
	ctx := context.Background()

	obs, err := a.Observe(ctx, p)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(obs.Exists).To(BeFalse())

	g.Expect(a.Create(ctx, p)).To(Succeed())
	g.Expect(p.Status.ServiceUUID).To(Equal(parentUUID))
	g.Expect(p.Status.Name).To(Equal("admin"))
	g.Expect(p.Status.ARN).NotTo(BeEmpty())

	// The raw JSON document is stored unchanged (no encoding).
	pol := api.Policies[parentUUID][0]
	g.Expect(pol.Document).To(Equal(testPolicyDoc))

	obs, err = a.Observe(ctx, p)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(obs.Exists).To(BeTrue())
	g.Expect(obs.UpToDate).To(BeTrue())
	g.Expect(obs.Ready).To(BeTrue())
}

func TestPolicyAdoptOnConflict(t *testing.T) {
	g := NewWithT(t)
	api := fake.NewObjectStorageAPI()
	c := newMOSClient(t)
	readyService(g, c, api)
	a := &ObjectStoragePolicyAdapter{API: api, Client: c}
	p := newPolicy("existing")
	ctx := context.Background()

	// Pre-create the policy in the fake so Create hits a conflict (adoption).
	_, err := api.CreateManagedObjectStoragePolicy(ctx, &request.CreateManagedObjectStoragePolicyRequest{
		ServiceUUID: parentUUID,
		Name:        "existing",
		Document:    testPolicyDoc,
	})
	g.Expect(err).NotTo(HaveOccurred())

	// Create must succeed (adopt) and Observe must find it.
	g.Expect(a.Create(ctx, p)).To(Succeed())
	obs, err := a.Observe(ctx, p)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(obs.Exists).To(BeTrue())
	g.Expect(p.Status.Name).To(Equal("existing"))
}

func TestPolicyDeleteConflictsWhileAttached(t *testing.T) {
	g := NewWithT(t)
	api := fake.NewObjectStorageAPI()
	c := newMOSClient(t)
	readyService(g, c, api)
	a := &ObjectStoragePolicyAdapter{API: api, Client: c}
	p := newPolicy("inuse")
	ctx := context.Background()

	g.Expect(a.Create(ctx, p)).To(Succeed())

	// Attach the policy to a user in the fake, so deletion is refused.
	_, err := api.CreateManagedObjectStorageUser(ctx, &request.CreateManagedObjectStorageUserRequest{ServiceUUID: parentUUID, Username: "u1"})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(api.AttachManagedObjectStorageUserPolicy(ctx, &request.AttachManagedObjectStorageUserPolicyRequest{ServiceUUID: parentUUID, Username: "u1", Name: "inuse"})).To(Succeed())

	err = a.Delete(ctx, p)
	g.Expect(err).To(MatchError(ContainSubstring("attached")))
	g.Expect(errors.Is(err, reconciler.ErrPending)).To(BeTrue())
}

func TestPolicyCreateRejectsInventedResourceNamespace(t *testing.T) {
	g := NewWithT(t)
	api := fake.NewObjectStorageAPI()
	c := newMOSClient(t)
	readyService(g, c, api)
	a := &ObjectStoragePolicyAdapter{API: api, Client: c}
	p := newPolicy("bad")
	p.Spec.Document = `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":["s3:GetObject"],"Resource":["arn:upcloud:objectstorage::bucket/*"]}]}`

	err := a.Create(context.Background(), p)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("Policy has invalid resource"))
	g.Expect(api.Policies[parentUUID]).To(BeEmpty())
}

// The sample is applied by the real-API e2e; keep it acceptable to the fake's
// resource validation so a bad sample fails here, not on a dispatch.
func TestPolicySampleDocumentIsAccepted(t *testing.T) {
	g := NewWithT(t)
	raw, err := os.ReadFile("../../../config/samples/objectstorage_v1alpha1_objectstoragepolicy.yaml")
	g.Expect(err).NotTo(HaveOccurred())
	sample := &objectstoragev1alpha1.ObjectStoragePolicy{}
	g.Expect(yaml.Unmarshal(raw, sample)).To(Succeed())
	g.Expect(sample.Spec.Document).NotTo(BeEmpty())

	api := fake.NewObjectStorageAPI()
	c := newMOSClient(t)
	readyService(g, c, api)
	a := &ObjectStoragePolicyAdapter{API: api, Client: c}
	p := newPolicy("sample")
	p.Spec.Document = sample.Spec.Document

	g.Expect(a.Create(context.Background(), p)).To(Succeed())
}
