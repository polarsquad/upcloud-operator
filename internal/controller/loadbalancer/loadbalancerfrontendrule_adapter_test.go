package loadbalancer

import (
	"testing"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	lb "github.com/polarsquad/upcloud-operator/api/loadbalancer/v1alpha1"
	"github.com/polarsquad/upcloud-operator/internal/upcloudapi/fake"
)

func TestRuleCreateDriftDelete(t *testing.T) {
	g := NewWithT(t)
	api := fake.NewLoadBalancerAPI()
	c := newLBClient(t)
	a := &LoadBalancerFrontendRuleAdapter{API: api, Client: c}
	readyFrontend(g, c, api)
	readyBundle(g, c, api)

	r := &lb.LoadBalancerFrontendRule{
		ObjectMeta: metav1.ObjectMeta{Name: "r1", Namespace: testNS},
		Spec: lb.LoadBalancerFrontendRuleSpec{
			FrontendRef:       commonLocalRef(feName),
			Name:              "r1",
			Priority:          10,
			MatchingCondition: "and",
			Matchers: []lb.RuleMatcher{
				{Type: "path", Path: &lb.MatcherString{Method: "starts", Value: "/api"}},
			},
			Actions: []lb.RuleAction{
				{Type: "use_backend", UseBackend: &lb.ActionUseBackend{Backend: backendName}},
			},
		},
	}
	g.Expect(a.Create(gctx(), r)).To(Succeed())
	g.Expect(r.Status.ServiceUUID).To(Equal(parentUUID))
	g.Expect(r.Status.FrontendName).To(Equal(feName))
	g.Expect(r.Status.Name).To(Equal("r1"))

	obs, err := a.Observe(gctx(), r)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(obs.Exists).To(BeTrue())
	g.Expect(obs.Ready).To(BeTrue())
	g.Expect(obs.UpToDate).To(BeTrue())

	// Drift: change the action backend to a second backend.
	api.LoadBalancers[parentUUID].Backends = append(api.LoadBalancers[parentUUID].Backends,
		upcloud.LoadBalancerBackend{Name: "be2"})
	r.Spec.Actions = []lb.RuleAction{
		{Type: "use_backend", UseBackend: &lb.ActionUseBackend{Backend: "be2"}},
	}
	obs, err = a.Observe(gctx(), r)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(obs.UpToDate).To(BeFalse())
	g.Expect(a.Update(gctx(), r)).To(Succeed())
	obs, err = a.Observe(gctx(), r)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(obs.UpToDate).To(BeTrue())

	g.Expect(a.Delete(gctx(), r)).To(Succeed())
	g.Expect(a.Delete(gctx(), r)).To(Succeed())
}
