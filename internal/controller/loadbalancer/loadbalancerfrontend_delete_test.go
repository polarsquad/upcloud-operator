package loadbalancer

import (
	"testing"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud"
	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud/request"

	lb "github.com/polarsquad/upcloud-operator/api/loadbalancer/v1alpha1"
	"github.com/polarsquad/upcloud-operator/internal/upcloudapi"
	"github.com/polarsquad/upcloud-operator/internal/upcloudapi/fake"
)

func TestLoadBalancerFrontendDeleteContract(t *testing.T) {
	checkDeleteContract(t, func(api *fake.LoadBalancerAPI) deleteContractFixture {
		api.LoadBalancers[parentUUID] = &upcloud.LoadBalancer{UUID: parentUUID, Frontends: []upcloud.LoadBalancerFrontend{{Name: feName}}}
		obj := &lb.LoadBalancerFrontend{Status: lb.LoadBalancerFrontendStatus{ServiceUUID: parentUUID, Name: feName}}
		return deleteContractFixture{
			identities: map[string]*string{serviceUUIDField: &obj.Status.ServiceUUID, statusNameField: &obj.Status.Name},
			delete: func(api upcloudapi.LoadBalancerAPI) error {
				return (&LoadBalancerFrontendAdapter{API: api}).Delete(gctx(), obj)
			},
			get: func(api *fake.LoadBalancerAPI) error {
				_, err := api.GetLoadBalancerFrontend(gctx(), &request.GetLoadBalancerFrontendRequest{ServiceUUID: parentUUID, Name: feName})
				return err
			},
			remove: func(api *fake.LoadBalancerAPI) error {
				return api.DeleteLoadBalancerFrontend(gctx(), &request.DeleteLoadBalancerFrontendRequest{ServiceUUID: parentUUID, Name: feName})
			},
			deleteCall: "DeleteLoadBalancerFrontend",
		}
	})
}
