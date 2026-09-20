package loadbalancer

import (
	"errors"
	"fmt"
	"testing"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud"
	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud/request"
	. "github.com/onsi/gomega"

	lb "github.com/polarsquad/upcloud-operator/api/loadbalancer/v1alpha1"
	"github.com/polarsquad/upcloud-operator/internal/reconciler"
	"github.com/polarsquad/upcloud-operator/internal/upcloudapi"
	"github.com/polarsquad/upcloud-operator/internal/upcloudapi/fake"
)

// Each fixture keeps status identity separate from the fake's stored object.
// Delete must use status without resolving a Kubernetes parent during teardown.
type deleteContractFixture struct {
	identities map[string]*string
	delete     func(upcloudapi.LoadBalancerAPI) error
	get        func(*fake.LoadBalancerAPI) error
	remove     func(*fake.LoadBalancerAPI) error
	deleteCall string
	async      bool
}

func checkDeleteContract(t *testing.T, setup func(*fake.LoadBalancerAPI) deleteContractFixture) {
	t.Helper()
	t.Run("EmptyIdentity", func(t *testing.T) {
		for field := range setup(fake.NewLoadBalancerAPI()).identities {
			t.Run(field, func(t *testing.T) {
				g := NewWithT(t)
				api := fake.NewLoadBalancerAPI()
				f := setup(api)
				*f.identities[field] = ""
				// The fake's Calls tracks mutations only. A nil interface also
				// makes ANY read or write fail, proving the stronger no-API contract.
				g.Expect(f.delete(nil)).To(Succeed())
				g.Expect(f.delete(api)).To(Succeed())
				g.Expect(api.Calls).To(BeEmpty())
				g.Expect(f.get(api)).To(Succeed())
			})
		}
	})
	t.Run("AlreadyAbsent", func(t *testing.T) {
		g := NewWithT(t)
		api := fake.NewLoadBalancerAPI()
		f := setup(api)
		g.Expect(f.remove(api)).To(Succeed())
		g.Expect(upcloudapi.IsNotFound(f.get(api))).To(BeTrue())
		api.Calls = nil
		g.Expect(f.delete(api)).To(Succeed())
		g.Expect(f.delete(api)).To(Succeed())
	})
	t.Run("ConflictPending", func(t *testing.T) {
		g := NewWithT(t)
		api := fake.NewLoadBalancerAPI()
		f := setup(api)
		// No leaf exemption: a 409 reported by the API must be retryable.
		// Exercise the fake's error-injection path, not an adapter stub.
		api.FailNext = fmt.Errorf("API response: %w", fake.Conflict("service is busy"))
		err := f.delete(api)
		g.Expect(errors.Is(err, reconciler.ErrPending)).To(BeTrue(), "Delete returned %v", err)
		g.Expect(api.Calls).To(Equal([]string{f.deleteCall}))
		g.Expect(f.get(api)).To(Succeed())
		g.Expect(api.FailNext).NotTo(HaveOccurred())
		// The conflict must not remove the object; retry after it clears.
		err = f.delete(api)
		if f.async {
			g.Expect(errors.Is(err, reconciler.ErrPending)).To(BeTrue())
		} else {
			g.Expect(err).NotTo(HaveOccurred())
		}
		g.Expect(upcloudapi.IsNotFound(f.get(api))).To(BeTrue())
		g.Expect(f.delete(api)).To(Succeed())
	})
	t.Run("UnexpectedError", func(t *testing.T) {
		g := NewWithT(t)
		api := fake.NewLoadBalancerAPI()
		f := setup(api)
		unexpected := errors.New("delete transport failed")
		api.FailNext = fmt.Errorf("transport: %w", unexpected)
		err := f.delete(api)
		g.Expect(errors.Is(err, unexpected)).To(BeTrue(), "Delete returned %v", err)
		g.Expect(errors.Is(err, reconciler.ErrPending)).To(BeFalse())
		g.Expect(api.Calls).To(Equal([]string{f.deleteCall}))
		g.Expect(f.get(api)).To(Succeed())
	})
}

func TestLoadBalancerBackendDeleteContract(t *testing.T) {
	checkDeleteContract(t, func(api *fake.LoadBalancerAPI) deleteContractFixture {
		api.LoadBalancers[parentUUID] = &upcloud.LoadBalancer{
			UUID: parentUUID, Backends: []upcloud.LoadBalancerBackend{{Name: backendName}},
		}
		obj := &lb.LoadBalancerBackend{Status: lb.LoadBalancerBackendStatus{ServiceUUID: parentUUID, Name: backendName}}
		return deleteContractFixture{
			identities: map[string]*string{serviceUUIDField: &obj.Status.ServiceUUID, statusNameField: &obj.Status.Name},
			delete: func(api upcloudapi.LoadBalancerAPI) error {
				return (&LoadBalancerBackendAdapter{API: api}).Delete(gctx(), obj)
			},
			get: func(api *fake.LoadBalancerAPI) error {
				_, err := api.GetLoadBalancerBackend(gctx(), &request.GetLoadBalancerBackendRequest{ServiceUUID: parentUUID, Name: backendName})
				return err
			},
			remove: func(api *fake.LoadBalancerAPI) error {
				return api.DeleteLoadBalancerBackend(gctx(), &request.DeleteLoadBalancerBackendRequest{ServiceUUID: parentUUID, Name: backendName})
			},
			deleteCall: "DeleteLoadBalancerBackend",
		}
	})
}
