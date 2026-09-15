package objectstorage

import (
	"context"
	"testing"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/polarsquad/upcloud-operator/api/common"
	objectstoragev1alpha1 "github.com/polarsquad/upcloud-operator/api/objectstorage/v1alpha1"
	"github.com/polarsquad/upcloud-operator/internal/upcloudapi/fake"
)

func newAccessKey(name string) *objectstoragev1alpha1.ObjectStorageAccessKey {
	return &objectstoragev1alpha1.ObjectStorageAccessKey{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: testNS, UID: "key-uid"},
		Spec: objectstoragev1alpha1.ObjectStorageAccessKeySpec{
			UserRef: common.LocalObjectReference{Name: "alice"},
		},
	}
}

func TestAccessKeyCreateWritesSecret(t *testing.T) {
	g := NewWithT(t)
	api := fake.NewObjectStorageAPI()
	c := newMOSClient(t)
	readyService(g, c, api)
	readyUser(g, c, api)
	a := &ObjectStorageAccessKeyAdapter{API: api, Client: c}
	k := newAccessKey("alice-key")
	ctx := context.Background()

	obs, err := a.Observe(ctx, k)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(obs.Exists).To(BeFalse())

	g.Expect(a.Create(ctx, k)).To(Succeed())
	g.Expect(k.Status.ServiceUUID).To(Equal(parentUUID))
	g.Expect(k.Status.Username).To(Equal("alice"))
	g.Expect(k.Status.AccessKeyID).NotTo(BeEmpty())

	var sec corev1.Secret
	g.Expect(c.Get(ctx, types.NamespacedName{Namespace: testNS, Name: "alice-key-s3"}, &sec)).To(Succeed())
	for _, key := range []string{SecretKeyAccessKeyID, SecretKeySecretAccessKey, SecretKeyEndpointURL, SecretKeyRegion} {
		g.Expect(sec.Data).To(HaveKey(key), "secret missing key %s", key)
	}
	g.Expect(string(sec.Data[SecretKeyAccessKeyID])).To(Equal(k.Status.AccessKeyID))
	g.Expect(string(sec.Data[SecretKeySecretAccessKey])).NotTo(BeEmpty())
	g.Expect(string(sec.Data[SecretKeyEndpointURL])).To(HavePrefix("https://"))
	g.Expect(string(sec.Data[SecretKeyEndpointURL])).To(HaveSuffix(".upcloudobjects.com"))
	g.Expect(string(sec.Data[SecretKeyRegion])).To(Equal(regionFinland))

	obs, err = a.Observe(ctx, k)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(obs.Exists).To(BeTrue())
	g.Expect(obs.UpToDate).To(BeTrue())
}

// failingClient wraps a client whose Create/Update of Secrets fails, to
// simulate a failed Secret write during Create.
type failingClient struct {
	client.Client
}

func (f failingClient) Create(ctx context.Context, obj client.Object, _ ...client.CreateOption) error {
	if _, ok := obj.(*corev1.Secret); ok {
		return &failingError{msg: "cannot write secret"}
	}
	return f.Client.Create(ctx, obj)
}

type failingError struct{ msg string }

func (e *failingError) Error() string { return e.msg }

func TestAccessKeyCreateSecretFailureDeletesKey(t *testing.T) {
	g := NewWithT(t)
	api := fake.NewObjectStorageAPI()
	inner := newMOSClient(t)
	readyService(g, inner, api)
	readyUser(g, inner, api)
	c := failingClient{Client: inner}
	a := &ObjectStorageAccessKeyAdapter{API: api, Client: c}
	k := newAccessKey("fail-key")
	ctx := context.Background()

	// The Secret write fails, so the adapter must delete the key it created
	// and clear the AccessKeyID so the next pass recreates it.
	err := a.Create(ctx, k)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err).To(MatchError(ContainSubstring("secret")))

	// The user should have no access keys left in the fake.
	g.Expect(api.Users[parentUUID][0].AccessKeys).To(BeEmpty())
	g.Expect(k.Status.AccessKeyID).To(BeEmpty())
}

func TestAccessKeyStatusFlip(t *testing.T) {
	g := NewWithT(t)
	api := fake.NewObjectStorageAPI()
	c := newMOSClient(t)
	readyService(g, c, api)
	readyUser(g, c, api)
	a := &ObjectStorageAccessKeyAdapter{API: api, Client: c}
	k := newAccessKey("flip-key")
	ctx := context.Background()

	g.Expect(a.Create(ctx, k)).To(Succeed())

	// Drift: spec wants Inactive, the key is Active.
	k.Spec.Status = "Inactive"
	obs, err := a.Observe(ctx, k)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(obs.UpToDate).To(BeFalse())

	g.Expect(a.Update(ctx, k)).To(Succeed())
	g.Expect(api.Users[parentUUID][0].AccessKeys[0].Status).To(Equal(upcloud.ManagedObjectStorageUserAccessKeyStatusInactive))

	obs, err = a.Observe(ctx, k)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(obs.UpToDate).To(BeTrue())
}

func TestAccessKeyDeleteIdempotent(t *testing.T) {
	g := NewWithT(t)
	api := fake.NewObjectStorageAPI()
	c := newMOSClient(t)
	readyService(g, c, api)
	readyUser(g, c, api)
	a := &ObjectStorageAccessKeyAdapter{API: api, Client: c}
	k := newAccessKey("del-key")
	ctx := context.Background()

	g.Expect(a.Create(ctx, k)).To(Succeed())
	g.Expect(a.Delete(ctx, k)).To(Succeed())
	g.Expect(api.Users[parentUUID][0].AccessKeys).To(BeEmpty())

	// Deleting again is a no-op (404 counts as deleted).
	g.Expect(a.Delete(ctx, k)).To(Succeed())
}
