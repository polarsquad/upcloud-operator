package loadbalancer

import (
	"errors"
	"testing"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	lb "github.com/polarsquad/upcloud-operator/api/loadbalancer/v1alpha1"
	"github.com/polarsquad/upcloud-operator/internal/reconciler"
	"github.com/polarsquad/upcloud-operator/internal/upcloudapi/fake"
)

func newResolver(name string) *lb.LoadBalancerResolver {
	return &lb.LoadBalancerResolver{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: testNS},
		Spec: lb.LoadBalancerResolverSpec{
			LoadBalancerRef: commonLocalRef(parentName),
			Nameservers:     []string{"1.1.1.1", "8.8.8.8"},
			Retries:         3,
			Timeout:         5,
			CacheValid:      60,
			CacheInvalid:    30,
		},
	}
}

func TestResolverCreateDriftDelete(t *testing.T) {
	g := NewWithT(t)
	api := fake.NewLoadBalancerAPI()
	c := newLBClient(t)
	a := &LoadBalancerResolverAdapter{API: api, Client: c}
	r := newResolver("res1")
	ctx := gctx()

	readyService(g, c, api)
	_, err := a.Observe(ctx, r)
	g.Expect(err).NotTo(HaveOccurred())

	g.Expect(a.Create(ctx, r)).To(Succeed())
	g.Expect(r.Status.Name).To(Equal("res1"))
	g.Expect(r.Status.ServiceUUID).To(Equal(parentUUID))

	obs, err := a.Observe(ctx, r)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(obs.Exists).To(BeTrue())
	g.Expect(obs.UpToDate).To(BeTrue())
	g.Expect(obs.Ready).To(BeTrue())

	// Drift: change nameservers.
	r.Spec.Nameservers = []string{"9.9.9.9"}
	obs, err = a.Observe(ctx, r)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(obs.UpToDate).To(BeFalse())

	g.Expect(a.Update(ctx, r)).To(Succeed())
	obs, err = a.Observe(ctx, r)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(obs.UpToDate).To(BeTrue())

	// Delete is idempotent.
	g.Expect(a.Delete(ctx, r)).To(Succeed())
	g.Expect(a.Delete(ctx, r)).To(Succeed())
}

func TestResolverParentGoneDeleteNil(t *testing.T) {
	g := NewWithT(t)
	api := fake.NewLoadBalancerAPI()
	c := newLBClient(t)
	a := &LoadBalancerResolverAdapter{API: api, Client: c}
	r := newResolver("res2")
	ctx := gctx()

	// No parent load balancer in the fake or client: create is a dependency
	// error.
	_, err := a.Observe(ctx, r)
	g.Expect(err).To(MatchError(ContainSubstring("not ready")))
	g.Expect(errors.Is(err, reconciler.ErrDependencyNotReady)).To(BeTrue())

	// Delete before the resolver ever existed is a no-op.
	g.Expect(a.Delete(ctx, r)).To(Succeed())
}

// TestResolverAdopt guards create-conflict adoption.
func TestResolverAdopt(t *testing.T) {
	g := NewWithT(t)
	api := fake.NewLoadBalancerAPI()
	c := newLBClient(t)
	a := &LoadBalancerResolverAdapter{API: api, Client: c}
	r := newResolver("res3")
	ctx := gctx()

	readyService(g, c, api)
	// Pre-existing resolver in the fake (simulates a prior create).
	api.LoadBalancers[parentUUID].Resolvers = append(api.LoadBalancers[parentUUID].Resolvers,
		upcloud.LoadBalancerResolver{Name: "res3", Nameservers: []string{"1.1.1.1"}})

	g.Expect(a.Create(ctx, r)).To(Succeed()) // conflict -> adoption
	obs, err := a.Observe(ctx, r)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(obs.Exists).To(BeTrue())
}
