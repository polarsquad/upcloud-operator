package loadbalancer

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud"
	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud/request"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/polarsquad/upcloud-operator/api/common"
	lb "github.com/polarsquad/upcloud-operator/api/loadbalancer/v1alpha1"
	"github.com/polarsquad/upcloud-operator/internal/reconciler"
	"github.com/polarsquad/upcloud-operator/internal/upcloudapi/fake"
)

func newService(name string) *lb.LoadBalancer {
	return &lb.LoadBalancer{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: testNS, UID: "svc-uid"},
		Spec: lb.LoadBalancerSpec{
			Plan: testPlan,
			Zone: testZone,
			Networks: []lb.LoadBalancerNetworkAttachment{
				{Name: publicType, Type: publicType},
			},
		},
	}
}

func TestServiceCreateDefaults(t *testing.T) {
	g := NewWithT(t)
	api := fake.NewLoadBalancerAPI()
	c := newLBClient(t)
	a := &LoadBalancerAdapter{API: api, Client: c}
	s := newService("lb-svc")
	ctx := gctx()

	obs, err := a.Observe(ctx, s)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(obs.Exists).To(BeFalse())

	g.Expect(a.Create(ctx, s)).To(Succeed())
	g.Expect(s.Status.UUID).NotTo(BeEmpty())

	svc := api.LoadBalancers[s.Status.UUID]
	g.Expect(svc).NotTo(BeNil())
	g.Expect(svc.OperationalState).To(Equal(upcloud.LoadBalancerOperationalStateRunning))
	g.Expect(svc.Name).To(Equal("lb-svc"))
	g.Expect(svc.Networks).To(HaveLen(1))
	g.Expect(svc.Networks[0].DNSName).To(HaveSuffix(".lb.upcloud.com"))
	g.Expect(svc.Networks[0].IPAddresses).To(HaveLen(1))
	// Frontends, backends and resolvers are separate CRs; the create request
	// sends explicit empty slices.
	g.Expect(svc.Frontends).To(BeEmpty())
	g.Expect(svc.Backends).To(BeEmpty())
	g.Expect(svc.Resolvers).To(BeEmpty())

	g.Expect(svc.Labels).To(ContainElement(upcloud.Label{Key: "managed-by", Value: "upcloud-operator"}))

	obs, err = a.Observe(ctx, s)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(obs.Exists).To(BeTrue())
	g.Expect(obs.Ready).To(BeTrue())
	g.Expect(obs.UpToDate).To(BeTrue())
	g.Expect(s.Status.OperationalState).To(Equal("running"))
	g.Expect(s.Status.Networks).To(HaveLen(1))
}

func TestServiceNetworkRefResolution(t *testing.T) {
	g := NewWithT(t)
	api := fake.NewLoadBalancerAPI()
	c := newLBClient(t)
	a := &LoadBalancerAdapter{API: api, Client: c}
	s := newService("net-lb")
	ctx := gctx()

	g.Expect(a.Create(ctx, s)).To(Succeed())

	s.Spec.Networks = []lb.LoadBalancerNetworkAttachment{
		{Name: "priv", Type: "private", NetworkRef: &common.LocalObjectReference{Name: "nonexistent"}},
	}
	_, err := a.Observe(ctx, s)
	g.Expect(err).To(MatchError(ContainSubstring("Network")))
	g.Expect(errors.Is(err, reconciler.ErrDependencyNotReady)).To(BeTrue())
}

func TestServiceDriftOnLabelsTriggersUpdate(t *testing.T) {
	g := NewWithT(t)
	api := fake.NewLoadBalancerAPI()
	c := newLBClient(t)
	a := &LoadBalancerAdapter{API: api, Client: c}
	s := newService("label-lb")
	ctx := gctx()

	g.Expect(a.Create(ctx, s)).To(Succeed())

	s.Spec.Labels = common.UpCloudLabels{"team": "platform"}
	obs, err := a.Observe(ctx, s)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(obs.UpToDate).To(BeFalse())

	g.Expect(a.Update(ctx, s)).To(Succeed())
	got := api.LoadBalancers[s.Status.UUID].Labels
	g.Expect(got).To(ContainElement(upcloud.Label{Key: "team", Value: "platform"}))

	obs, err = a.Observe(ctx, s)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(obs.UpToDate).To(BeTrue())
}

func TestServiceDeletePendingUntilGone(t *testing.T) {
	g := NewWithT(t)
	api := fake.NewLoadBalancerAPI()
	c := newLBClient(t)
	a := &LoadBalancerAdapter{API: api, Client: c}
	s := newService("del-lb")
	ctx := gctx()

	g.Expect(a.Create(ctx, s)).To(Succeed())
	uuid := s.Status.UUID

	g.Expect(a.Delete(ctx, s)).To(MatchError(reconciler.ErrPending))
	g.Expect(a.Delete(ctx, s)).To(Succeed())
	g.Expect(api.LoadBalancers[uuid]).To(BeNil())
}

func TestServiceAdoptByUID(t *testing.T) {
	g := NewWithT(t)
	api := fake.NewLoadBalancerAPI()
	c := newLBClient(t)
	a := &LoadBalancerAdapter{API: api, Client: c}
	s := newService("adopt-lb")
	ctx := gctx()

	adopted := &upcloud.LoadBalancer{
		UUID:             "lb-adopt",
		Name:             "adopt-lb",
		Zone:             testZone,
		Plan:             testPlan,
		OperationalState: upcloud.LoadBalancerOperationalStateRunning,
		Labels:           []upcloud.Label{{Key: "k8s-uid", Value: "svc-uid"}},
	}
	api.LoadBalancers[adopted.UUID] = adopted

	obs, err := a.Observe(ctx, s)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(obs.Exists).To(BeTrue())
	g.Expect(s.Status.UUID).To(Equal("lb-adopt"))
}

// TestCreateRequestMarshalsEmptyChildren guards the non-omitempty Frontends /
// Backends / Resolvers fields: an explicit empty slice must marshal to "[]"
// (so UpCloud sees "no children"), not be omitted or marshal to "null".
func TestCreateRequestMarshalsEmptyChildren(t *testing.T) {
	g := NewWithT(t)
	req := &request.CreateLoadBalancerRequest{
		Name:      "x",
		Frontends: []request.LoadBalancerFrontend{},
		Backends:  []request.LoadBalancerBackend{},
		Resolvers: []request.LoadBalancerResolver{},
	}
	b, err := json.Marshal(req)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(string(b)).To(ContainSubstring(`"frontends":[]`))
	g.Expect(string(b)).To(ContainSubstring(`"backends":[]`))
	g.Expect(string(b)).To(ContainSubstring(`"resolvers":[]`))
}
