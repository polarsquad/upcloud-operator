package loadbalancer

import (
	"testing"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud"
	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud/request"

	lb "github.com/polarsquad/upcloud-operator/api/loadbalancer/v1alpha1"
	"github.com/polarsquad/upcloud-operator/internal/upcloudapi"
	"github.com/polarsquad/upcloud-operator/internal/upcloudapi/fake"
)

func TestLoadBalancerResolverDeleteContract(t *testing.T) {
	checkDeleteContract(t, func(api *fake.LoadBalancerAPI) deleteContractFixture {
		api.LoadBalancers[parentUUID] = &upcloud.LoadBalancer{UUID: parentUUID, Resolvers: []upcloud.LoadBalancerResolver{{Name: ruleName}}}
		obj := &lb.LoadBalancerResolver{Status: lb.LoadBalancerResolverStatus{ServiceUUID: parentUUID, Name: ruleName}}
		return deleteContractFixture{
			identities: map[string]*string{serviceUUIDField: &obj.Status.ServiceUUID, statusNameField: &obj.Status.Name},
			delete: func(api upcloudapi.LoadBalancerAPI) error {
				return (&LoadBalancerResolverAdapter{API: api}).Delete(gctx(), obj)
			},
			get: func(api *fake.LoadBalancerAPI) error {
				_, err := api.GetLoadBalancerResolver(gctx(), &request.GetLoadBalancerResolverRequest{ServiceUUID: parentUUID, Name: ruleName})
				return err
			},
			remove: func(api *fake.LoadBalancerAPI) error {
				return api.DeleteLoadBalancerResolver(gctx(), &request.DeleteLoadBalancerResolverRequest{ServiceUUID: parentUUID, Name: ruleName})
			},
			deleteCall: "DeleteLoadBalancerResolver",
		}
	})
}
