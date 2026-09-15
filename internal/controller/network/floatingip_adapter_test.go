package network

import (
	"context"
	"testing"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	networkv1alpha1 "github.com/polarsquad/upcloud-operator/api/network/v1alpha1"
	"github.com/polarsquad/upcloud-operator/internal/reconciler"
	"github.com/polarsquad/upcloud-operator/internal/upcloudapi/fake"
)

func newFloatingIP() *networkv1alpha1.FloatingIP {
	return &networkv1alpha1.FloatingIP{
		ObjectMeta: metav1.ObjectMeta{Name: "ip1", Namespace: "ns", UID: "fip-uid"},
		Spec: networkv1alpha1.FloatingIPSpec{
			Zone:          testZone,
			Family:        testIPFamily,
			Access:        testIPAccess,
			ReleasePolicy: "keep",
		},
	}
}

func TestFloatingIPObserveMissingThenCreate(t *testing.T) {
	g := NewWithT(t)
	api := fake.NewNetworkAPI()
	a := &FloatingIPAdapter{API: api}
	f := newFloatingIP()
	ctx := context.Background()

	obs, err := a.Observe(ctx, f)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(obs.Exists).To(BeFalse())

	g.Expect(a.Create(ctx, f)).To(Succeed())
	g.Expect(f.Status.Address).To(HavePrefix("203.0.113."))
	created := api.IPs[f.Status.Address]
	g.Expect(created.Zone).To(Equal(testZone))
	g.Expect(created.Family).To(Equal(testIPFamily))
	g.Expect(created.Access).To(Equal(testIPAccess))
	g.Expect(created.Floating).To(Equal(upcloud.True))

	obs, err = a.Observe(ctx, f)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(obs).To(Equal(reconciler.Observation{Exists: true, UpToDate: true, Ready: true}))
}

func TestFloatingIPObserveDetectsDriftAndUpdateFixesIt(t *testing.T) {
	g := NewWithT(t)
	api := fake.NewNetworkAPI()
	a := &FloatingIPAdapter{API: api}
	f := newFloatingIP()
	ctx := context.Background()
	g.Expect(a.Create(ctx, f)).To(Succeed())

	f.Spec.PTRRecord = "ip1.example.org."
	f.Spec.ReleasePolicy = "release"
	obs, err := a.Observe(ctx, f)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(obs.UpToDate).To(BeFalse())

	g.Expect(a.Update(ctx, f)).To(Succeed())
	g.Expect(api.IPs[f.Status.Address].PTRRecord).To(Equal("ip1.example.org."))
	g.Expect(api.IPs[f.Status.Address].ReleasePolicy).To(Equal(upcloud.IPAddressReleasePolicyRelease))

	obs, err = a.Observe(ctx, f)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(obs.UpToDate).To(BeTrue())
}

func TestFloatingIPObserveImmutablesAreOutOfScopeForUpdate(t *testing.T) {
	g := NewWithT(t)
	api := fake.NewNetworkAPI()
	a := &FloatingIPAdapter{API: api}
	f := newFloatingIP()
	ctx := context.Background()
	g.Expect(a.Create(ctx, f)).To(Succeed())

	// Zone is immutable (enforced at admission) but still compared, so a
	// mismatch in status flags drift.
	api.IPs[f.Status.Address].Zone = "de-fke1"
	obs, err := a.Observe(ctx, f)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(obs.UpToDate).To(BeFalse())
}

func TestFloatingIPDeleteIsIdempotent(t *testing.T) {
	g := NewWithT(t)
	api := fake.NewNetworkAPI()
	a := &FloatingIPAdapter{API: api}
	f := newFloatingIP()
	ctx := context.Background()
	g.Expect(a.Create(ctx, f)).To(Succeed())
	g.Expect(a.Delete(ctx, f)).To(Succeed())
	g.Expect(api.IPs).To(BeEmpty())
	g.Expect(a.Delete(ctx, f)).To(Succeed())

	g.Expect(a.Delete(ctx, &networkv1alpha1.FloatingIP{})).To(Succeed())
}

func TestFloatingIPObserveWithoutStatusIsAbsent(t *testing.T) {
	g := NewWithT(t)
	api := fake.NewNetworkAPI()
	a := &FloatingIPAdapter{API: api}
	f := newFloatingIP()
	ctx := context.Background()

	// Floating IPs carry no labels, so there is no adoption: with an empty
	// status.address the adapter cannot tell which of the account's addresses
	// belongs to this CR and reports absent.
	g.Expect(a.Create(ctx, f)).To(Succeed())
	got := api.IPs[f.Status.Address]
	g.Expect(got).NotTo(BeNil())
	f.Status.Address = ""

	obs, err := a.Observe(ctx, f)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(obs.Exists).To(BeFalse())
}
