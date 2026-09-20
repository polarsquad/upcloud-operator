package loadbalancer

import (
	"errors"
	"testing"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud"
	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud/request"
	. "github.com/onsi/gomega"

	lb "github.com/polarsquad/upcloud-operator/api/loadbalancer/v1alpha1"
	"github.com/polarsquad/upcloud-operator/internal/reconciler"
	"github.com/polarsquad/upcloud-operator/internal/upcloudapi"
	"github.com/polarsquad/upcloud-operator/internal/upcloudapi/fake"
)

func TestLoadBalancerDeleteContract(t *testing.T) {
	checkDeleteContract(t, func(api *fake.LoadBalancerAPI) deleteContractFixture {
		api.LoadBalancers[parentUUID] = &upcloud.LoadBalancer{
			UUID: parentUUID, OperationalState: upcloud.LoadBalancerOperationalStateRunning,
		}
		obj := &lb.LoadBalancer{Status: lb.LoadBalancerStatus{UUID: parentUUID}}
		return deleteContractFixture{
			identities: map[string]*string{"UUID": &obj.Status.UUID},
			delete: func(api upcloudapi.LoadBalancerAPI) error {
				return (&LoadBalancerAdapter{API: api}).Delete(gctx(), obj)
			},
			get: func(api *fake.LoadBalancerAPI) error {
				_, err := api.GetLoadBalancer(gctx(), &request.GetLoadBalancerRequest{UUID: parentUUID})
				return err
			},
			remove: func(api *fake.LoadBalancerAPI) error {
				return api.DeleteLoadBalancer(gctx(), &request.DeleteLoadBalancerRequest{UUID: parentUUID})
			},
			deleteCall: "DeleteLoadBalancer",
			async:      true,
		}
	})
}

func TestLoadBalancerDeleteBlockedByChildren(t *testing.T) {
	for _, child := range []string{"backend", "frontend", "resolver"} {
		t.Run(child, func(t *testing.T) {
			g := NewWithT(t)
			api := fake.NewLoadBalancerAPI()
			stored := &upcloud.LoadBalancer{UUID: parentUUID, OperationalState: upcloud.LoadBalancerOperationalStateRunning}
			api.LoadBalancers[parentUUID] = stored
			var removeChild func() error
			switch child {
			case "backend":
				stored.Backends = []upcloud.LoadBalancerBackend{{Name: backendName}}
				removeChild = func() error {
					return api.DeleteLoadBalancerBackend(gctx(), &request.DeleteLoadBalancerBackendRequest{ServiceUUID: parentUUID, Name: backendName})
				}
			case "frontend":
				stored.Frontends = []upcloud.LoadBalancerFrontend{{Name: feName}}
				removeChild = func() error {
					return api.DeleteLoadBalancerFrontend(gctx(), &request.DeleteLoadBalancerFrontendRequest{ServiceUUID: parentUUID, Name: feName})
				}
			case "resolver":
				stored.Resolvers = []upcloud.LoadBalancerResolver{{Name: ruleName}}
				removeChild = func() error {
					return api.DeleteLoadBalancerResolver(gctx(), &request.DeleteLoadBalancerResolverRequest{ServiceUUID: parentUUID, Name: ruleName})
				}
			}
			a := &LoadBalancerAdapter{API: api}
			obj := &lb.LoadBalancer{Status: lb.LoadBalancerStatus{UUID: parentUUID}}
			// No injected error: the fake rejects the actual dependent state.
			err := a.Delete(gctx(), obj)
			g.Expect(errors.Is(err, reconciler.ErrPending)).To(BeTrue(), "Delete returned %v", err)
			g.Expect(api.LoadBalancers).To(HaveKey(parentUUID))
			g.Expect(api.Calls).To(Equal([]string{"DeleteLoadBalancer"}))
			g.Expect(removeChild()).To(Succeed())
			g.Expect(errors.Is(a.Delete(gctx(), obj), reconciler.ErrPending)).To(BeTrue())
			g.Expect(a.Delete(gctx(), obj)).To(Succeed())
			g.Expect(api.LoadBalancers).NotTo(HaveKey(parentUUID))
		})
	}
}

func TestLoadBalancerDeleteInProgress(t *testing.T) {
	for _, state := range []upcloud.LoadBalancerOperationalState{
		upcloud.LoadBalancerOperationalStateDeleteDNS,
		upcloud.LoadBalancerOperationalStateDeleteNetwork,
		upcloud.LoadBalancerOperationalStateDeleteServer,
		upcloud.LoadBalancerOperationalStateDeleteService,
	} {
		t.Run(string(state), func(t *testing.T) {
			g := NewWithT(t)
			api := fake.NewLoadBalancerAPI()
			api.LoadBalancers[parentUUID] = &upcloud.LoadBalancer{UUID: parentUUID}
			api.StateOverride = state
			a := &LoadBalancerAdapter{API: api}
			obj := &lb.LoadBalancer{Status: lb.LoadBalancerStatus{UUID: parentUUID}}
			g.Expect(errors.Is(a.Delete(gctx(), obj), reconciler.ErrPending)).To(BeTrue())
			g.Expect(api.Calls).To(BeEmpty(), "must not repeat DELETE while deletion is already in progress")
			g.Expect(api.LoadBalancers).To(HaveKey(parentUUID))
			// External deletion completes between reconciliations.
			delete(api.LoadBalancers, parentUUID)
			g.Expect(a.Delete(gctx(), obj)).To(Succeed())
		})
	}
}
