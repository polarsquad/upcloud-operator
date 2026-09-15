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

func TestFrontendCreateDriftDelete(t *testing.T) {
	g := NewWithT(t)
	api := fake.NewLoadBalancerAPI()
	c := newLBClient(t)
	a := &LoadBalancerFrontendAdapter{API: api, Client: c}
	readyBackend(g, c, api)
	readyBundle(g, c, api)

	fe := &lb.LoadBalancerFrontend{
		ObjectMeta: metav1.ObjectMeta{Name: "fe1", Namespace: testNS},
		Spec: lb.LoadBalancerFrontendSpec{
			LoadBalancerRef:   commonLocalRef(parentName),
			Name:              "fe1",
			Mode:              testModeHTTP,
			Port:              80,
			DefaultBackendRef: commonLocalRef(backendName),
			Networks:          []lb.FrontendNetwork{{Name: publicType}},
		},
	}
	g.Expect(a.Create(gctx(), fe)).To(Succeed())
	g.Expect(fe.Status.ServiceUUID).To(Equal(parentUUID))
	g.Expect(fe.Status.Name).To(Equal("fe1"))

	obs, err := a.Observe(gctx(), fe)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(obs.Exists).To(BeTrue())
	g.Expect(obs.Ready).To(BeTrue())
	g.Expect(obs.UpToDate).To(BeTrue())

	// Drift: change port.
	fe.Spec.Port = 8080
	obs, err = a.Observe(gctx(), fe)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(obs.UpToDate).To(BeFalse())
	g.Expect(a.Update(gctx(), fe)).To(Succeed())
	obs, err = a.Observe(gctx(), fe)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(obs.UpToDate).To(BeTrue())

	g.Expect(a.Delete(gctx(), fe)).To(Succeed())
	g.Expect(a.Delete(gctx(), fe)).To(Succeed())
}

// TestFrontendParentMissing asserts the dependency error when the service is
// gone, and that delete is a no-op.
func TestFrontendParentMissing(t *testing.T) {
	g := NewWithT(t)
	api := fake.NewLoadBalancerAPI()
	c := newLBClient(t)
	a := &LoadBalancerFrontendAdapter{API: api, Client: c}

	fe := &lb.LoadBalancerFrontend{
		ObjectMeta: metav1.ObjectMeta{Name: "fe2", Namespace: testNS},
		Spec:       lb.LoadBalancerFrontendSpec{LoadBalancerRef: commonLocalRef("nope"), Name: "fe2", Mode: testModeHTTP, Port: 80, DefaultBackendRef: commonLocalRef("be")},
	}
	_, err := a.Observe(gctx(), fe)
	g.Expect(err).To(MatchError(ContainSubstring("LoadBalancer")))
	g.Expect(errors.Is(err, reconciler.ErrDependencyNotReady)).To(BeTrue())
}
