package loadbalancer

import (
	"errors"
	"testing"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/polarsquad/upcloud-operator/api/common"
	lb "github.com/polarsquad/upcloud-operator/api/loadbalancer/v1alpha1"
	"github.com/polarsquad/upcloud-operator/internal/reconciler"
	"github.com/polarsquad/upcloud-operator/internal/upcloudapi/fake"
)

func TestBackendCreateDriftDelete(t *testing.T) {
	g := NewWithT(t)
	api := fake.NewLoadBalancerAPI()
	c := newLBClient(t)
	a := &LoadBalancerBackendAdapter{API: api, Client: c}
	be := &lb.LoadBalancerBackend{
		ObjectMeta: metav1.ObjectMeta{Name: "be1", Namespace: testNS},
		Spec: lb.LoadBalancerBackendSpec{
			LoadBalancerRef: commonLocalRef(parentName),
			Name:            "be1",
			Properties: &lb.LoadBalancerBackendProperties{
				HealthCheckType: "tcp",
			},
		},
	}
	ctx := gctx()

	readyService(g, c, api)
	g.Expect(a.Create(ctx, be)).To(Succeed())
	g.Expect(be.Status.Name).To(Equal("be1"))
	g.Expect(be.Status.ServiceUUID).To(Equal(parentUUID))

	obs, err := a.Observe(ctx, be)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(obs.Exists).To(BeTrue())
	g.Expect(obs.UpToDate).To(BeTrue())
	g.Expect(obs.Ready).To(BeTrue())

	// The create request sends an empty members slice (members are separate
	// CRs).
	stored := api.LoadBalancers[parentUUID].Backends[0]
	g.Expect(stored.Members).To(BeEmpty())

	// Drift: change health check type.
	be.Spec.Properties.HealthCheckType = "http"
	obs, err = a.Observe(ctx, be)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(obs.UpToDate).To(BeFalse())

	g.Expect(a.Update(ctx, be)).To(Succeed())
	obs, err = a.Observe(ctx, be)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(obs.UpToDate).To(BeTrue())

	g.Expect(a.Delete(ctx, be)).To(Succeed())
	g.Expect(a.Delete(ctx, be)).To(Succeed())
}

func TestBackendParentGoneDeleteNil(t *testing.T) {
	g := NewWithT(t)
	api := fake.NewLoadBalancerAPI()
	c := newLBClient(t)
	a := &LoadBalancerBackendAdapter{API: api, Client: c}
	be := &lb.LoadBalancerBackend{
		ObjectMeta: metav1.ObjectMeta{Name: "be2", Namespace: testNS},
		Spec:       lb.LoadBalancerBackendSpec{LoadBalancerRef: commonLocalRef(parentName)},
	}
	ctx := gctx()

	// No parent load balancer: create is a dependency error.
	_, err := a.Observe(ctx, be)
	g.Expect(err).To(MatchError(ContainSubstring("not ready")))
	g.Expect(errors.Is(err, reconciler.ErrDependencyNotReady)).To(BeTrue())

	g.Expect(a.Delete(ctx, be)).To(Succeed())
}

// TestBackendUsesResolverName guards resolver resolution.
func TestBackendUsesResolverName(t *testing.T) {
	g := NewWithT(t)
	api := fake.NewLoadBalancerAPI()
	c := newLBClient(t)
	a := &LoadBalancerBackendAdapter{API: api, Client: c}
	ctx := gctx()

	readyService(g, c, api)
	// A resolver CR in the client (Ready, external id "resname").
	res := &lb.LoadBalancerResolver{
		ObjectMeta: metav1.ObjectMeta{Name: "resx", Namespace: testNS, UID: "res-uid", Generation: 1},
		Spec:       lb.LoadBalancerResolverSpec{LoadBalancerRef: commonLocalRef(parentName)},
		Status: lb.LoadBalancerResolverStatus{
			ServiceUUID: parentUUID, Name: "resname",
			Conditions: []metav1.Condition{{Type: testReadyType, Status: metav1.ConditionTrue, Reason: readyReason, Message: "ok", ObservedGeneration: 1}},
		},
	}
	g.Expect(c.Create(ctx, res)).To(Succeed())
	// The matching fake resolver so the create succeeds.
	api.LoadBalancers[parentUUID].Resolvers = append(api.LoadBalancers[parentUUID].Resolvers,
		upcloud.LoadBalancerResolver{Name: "resname"})

	be := &lb.LoadBalancerBackend{
		ObjectMeta: metav1.ObjectMeta{Name: "be3", Namespace: testNS},
		Spec: lb.LoadBalancerBackendSpec{
			LoadBalancerRef: commonLocalRef(parentName),
			Name:            "be3",
		},
	}
	be.Spec.ResolverRef = &common.LocalObjectReference{Name: "resx"}
	g.Expect(a.Create(ctx, be)).To(Succeed())
	stored := api.LoadBalancers[parentUUID].Backends[0]
	g.Expect(stored.Resolver).To(Equal("resname"))
}
