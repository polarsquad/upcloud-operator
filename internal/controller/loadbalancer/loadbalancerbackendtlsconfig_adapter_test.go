package loadbalancer

import (
	"errors"
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	lb "github.com/polarsquad/upcloud-operator/api/loadbalancer/v1alpha1"
	"github.com/polarsquad/upcloud-operator/internal/reconciler"
	"github.com/polarsquad/upcloud-operator/internal/upcloudapi/fake"
)

func TestBackendTLSConfigCreateDriftDelete(t *testing.T) {
	g := NewWithT(t)
	api := fake.NewLoadBalancerAPI()
	c := newLBClient(t)
	a := &LoadBalancerBackendTLSConfigAdapter{API: api, Client: c}
	cfg := &lb.LoadBalancerBackendTLSConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "tls1", Namespace: testNS},
		Spec: lb.LoadBalancerBackendTLSConfigSpec{
			BackendRef:           commonLocalRef(backendName),
			Name:                 "tls1",
			CertificateBundleRef: commonLocalRef(bundleName),
		},
	}
	ctx := gctx()

	readyBackend(g, c, api)
	readyBundle(g, c, api)
	g.Expect(a.Create(ctx, cfg)).To(Succeed())
	g.Expect(cfg.Status.CertificateBundleUUID).To(Equal(bundleUUID))

	obs, err := a.Observe(ctx, cfg)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(obs.Exists).To(BeTrue())
	g.Expect(obs.UpToDate).To(BeTrue())
	g.Expect(obs.Ready).To(BeTrue())

	g.Expect(a.Delete(ctx, cfg)).To(Succeed())
	g.Expect(a.Delete(ctx, cfg)).To(Succeed())
}

func TestBackendTLSConfigParentGoneDeleteNil(t *testing.T) {
	g := NewWithT(t)
	api := fake.NewLoadBalancerAPI()
	c := newLBClient(t)
	a := &LoadBalancerBackendTLSConfigAdapter{API: api, Client: c}
	cfg := &lb.LoadBalancerBackendTLSConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "tls2", Namespace: testNS},
		Spec: lb.LoadBalancerBackendTLSConfigSpec{
			BackendRef:           commonLocalRef(backendName),
			CertificateBundleRef: commonLocalRef(bundleName),
		},
	}
	ctx := gctx()

	// No parent backend: create is a dependency error.
	_, err := a.Observe(ctx, cfg)
	g.Expect(err).To(MatchError(ContainSubstring("not ready")))
	g.Expect(errors.Is(err, reconciler.ErrDependencyNotReady)).To(BeTrue())

	g.Expect(a.Delete(ctx, cfg)).To(Succeed())
}
