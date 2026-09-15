package loadbalancer

import (
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	lb "github.com/polarsquad/upcloud-operator/api/loadbalancer/v1alpha1"
	"github.com/polarsquad/upcloud-operator/internal/upcloudapi/fake"
)

func TestFrontendTLSConfigCreateDriftDelete(t *testing.T) {
	g := NewWithT(t)
	api := fake.NewLoadBalancerAPI()
	c := newLBClient(t)
	a := &LoadBalancerFrontendTLSConfigAdapter{API: api, Client: c}
	readyFrontend(g, c, api)
	readyBundle(g, c, api)

	c2 := &lb.LoadBalancerFrontendTLSConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "t1", Namespace: testNS},
		Spec: lb.LoadBalancerFrontendTLSConfigSpec{
			FrontendRef:          commonLocalRef(feName),
			Name:                 "t1",
			CertificateBundleRef: commonLocalRef(bundleName),
		},
	}
	g.Expect(a.Create(gctx(), c2)).To(Succeed())
	g.Expect(c2.Status.ServiceUUID).To(Equal(parentUUID))
	g.Expect(c2.Status.FrontendName).To(Equal(feName))
	g.Expect(c2.Status.Name).To(Equal("t1"))

	obs, err := a.Observe(gctx(), c2)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(obs.Exists).To(BeTrue())
	g.Expect(obs.Ready).To(BeTrue())
	g.Expect(obs.UpToDate).To(BeTrue())

	g.Expect(a.Delete(gctx(), c2)).To(Succeed())
	g.Expect(a.Delete(gctx(), c2)).To(Succeed())
}
