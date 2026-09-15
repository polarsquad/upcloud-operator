package objectstorage

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/polarsquad/upcloud-operator/api/common"
	objectstoragev1alpha1 "github.com/polarsquad/upcloud-operator/api/objectstorage/v1alpha1"
	"github.com/polarsquad/upcloud-operator/internal/upcloudapi/fake"
)

func newDomain(name, domain string) *objectstoragev1alpha1.ObjectStorageCustomDomain {
	return &objectstoragev1alpha1.ObjectStorageCustomDomain{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: testNS, UID: "domain-uid"},
		Spec: objectstoragev1alpha1.ObjectStorageCustomDomainSpec{
			ServiceRef: common.LocalObjectReference{Name: parentName},
			DomainName: domain,
		},
	}
}

func TestCustomDomainCreateObserve(t *testing.T) {
	g := NewWithT(t)
	api := fake.NewObjectStorageAPI()
	c := newMOSClient(t)
	readyService(g, c, api)
	a := &ObjectStorageCustomDomainAdapter{API: api, Client: c}
	d := newDomain("cdn", "objects.example.com")
	ctx := context.Background()

	obs, err := a.Observe(ctx, d)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(obs.Exists).To(BeFalse())

	g.Expect(a.Create(ctx, d)).To(Succeed())
	g.Expect(d.Status.ServiceUUID).To(Equal(parentUUID))
	g.Expect(d.Status.DomainName).To(Equal("objects.example.com"))
	g.Expect(d.Status.Type).To(Equal(EndpointTypePublic))

	obs, err = a.Observe(ctx, d)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(obs.Exists).To(BeTrue())
	g.Expect(obs.UpToDate).To(BeTrue())
	g.Expect(obs.Ready).To(BeTrue())
}

func TestCustomDomainTypeDrift(t *testing.T) {
	g := NewWithT(t)
	api := fake.NewObjectStorageAPI()
	c := newMOSClient(t)
	readyService(g, c, api)
	a := &ObjectStorageCustomDomainAdapter{API: api, Client: c}
	d := newDomain("cdn", "static.example.com")
	d.Spec.Type = EndpointTypePublic
	ctx := context.Background()

	g.Expect(a.Create(ctx, d)).To(Succeed())

	// Force the stored type to differ so Update is exercised.
	api.Services[parentUUID].CustomDomains[0].Type = "private"
	obs, err := a.Observe(ctx, d)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(obs.UpToDate).To(BeFalse())

	g.Expect(a.Update(ctx, d)).To(Succeed())
	g.Expect(api.Services[parentUUID].CustomDomains[0].Type).To(Equal(EndpointTypePublic))

	obs, err = a.Observe(ctx, d)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(obs.UpToDate).To(BeTrue())
}

func TestCustomDomainDeleteIdempotent(t *testing.T) {
	g := NewWithT(t)
	api := fake.NewObjectStorageAPI()
	c := newMOSClient(t)
	readyService(g, c, api)
	a := &ObjectStorageCustomDomainAdapter{API: api, Client: c}
	d := newDomain("gone", "old.example.com")
	ctx := context.Background()

	g.Expect(a.Create(ctx, d)).To(Succeed())
	g.Expect(a.Delete(ctx, d)).To(Succeed())
	g.Expect(api.Services[parentUUID].CustomDomains).To(BeEmpty())
	// Deleting again is a no-op (404 counts as deleted).
	g.Expect(a.Delete(ctx, d)).To(Succeed())
}
