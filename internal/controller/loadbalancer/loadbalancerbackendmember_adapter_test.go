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

func TestMemberCreateDriftDelete(t *testing.T) {
	g := NewWithT(t)
	api := fake.NewLoadBalancerAPI()
	c := newLBClient(t)
	a := &LoadBalancerBackendMemberAdapter{API: api, Client: c}
	m := &lb.LoadBalancerBackendMember{
		ObjectMeta: metav1.ObjectMeta{Name: "m1", Namespace: testNS},
		Spec: lb.LoadBalancerBackendMemberSpec{
			BackendRef: commonLocalRef(backendName),
			Name:       "m1",
			Type:       "static",
			IP:         "10.0.0.5",
			Port:       8080,
			Weight:     100,
		},
	}
	ctx := gctx()

	readyBackend(g, c, api)
	g.Expect(a.Create(ctx, m)).To(Succeed())
	g.Expect(m.Status.ServiceUUID).To(Equal(parentUUID))
	g.Expect(m.Status.BackendName).To(Equal(backendName))
	g.Expect(m.Status.Name).To(Equal("m1"))

	obs, err := a.Observe(ctx, m)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(obs.Exists).To(BeTrue())
	g.Expect(obs.UpToDate).To(BeTrue())
	g.Expect(obs.Ready).To(BeTrue())

	// An ip change is a modify, not a recreate.
	m.Spec.IP = "10.0.0.6"
	obs, err = a.Observe(ctx, m)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(obs.UpToDate).To(BeFalse())

	g.Expect(a.Update(ctx, m)).To(Succeed())
	obs, err = a.Observe(ctx, m)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(obs.UpToDate).To(BeTrue())
	stored := api.LoadBalancers[parentUUID].Backends[0].Members[0]
	g.Expect(stored.IP).To(Equal("10.0.0.6"))

	g.Expect(a.Delete(ctx, m)).To(Succeed())
	g.Expect(a.Delete(ctx, m)).To(Succeed())
}

func TestMemberParentGoneDeleteNil(t *testing.T) {
	g := NewWithT(t)
	api := fake.NewLoadBalancerAPI()
	c := newLBClient(t)
	a := &LoadBalancerBackendMemberAdapter{API: api, Client: c}
	m := &lb.LoadBalancerBackendMember{
		ObjectMeta: metav1.ObjectMeta{Name: "m2", Namespace: testNS},
		Spec:       lb.LoadBalancerBackendMemberSpec{BackendRef: commonLocalRef(backendName)},
	}
	ctx := gctx()

	// No parent backend: create is a dependency error.
	_, err := a.Observe(ctx, m)
	g.Expect(err).To(MatchError(ContainSubstring("not ready")))
	g.Expect(errors.Is(err, reconciler.ErrDependencyNotReady)).To(BeTrue())

	g.Expect(a.Delete(ctx, m)).To(Succeed())
}
