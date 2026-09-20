package loadbalancer

import (
	"testing"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud"
	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud/request"

	lb "github.com/polarsquad/upcloud-operator/api/loadbalancer/v1alpha1"
	"github.com/polarsquad/upcloud-operator/internal/upcloudapi"
	"github.com/polarsquad/upcloud-operator/internal/upcloudapi/fake"
)

func TestLoadBalancerFrontendRuleDeleteContract(t *testing.T) {
	checkDeleteContract(t, func(api *fake.LoadBalancerAPI) deleteContractFixture {
		api.LoadBalancers[parentUUID] = &upcloud.LoadBalancer{UUID: parentUUID, Frontends: []upcloud.LoadBalancerFrontend{{Name: feName, Rules: []upcloud.LoadBalancerFrontendRule{{Name: ruleName}}}}}
		obj := &lb.LoadBalancerFrontendRule{Status: lb.LoadBalancerFrontendRuleStatus{ServiceUUID: parentUUID, FrontendName: feName, Name: ruleName}}
		return deleteContractFixture{
			identities: map[string]*string{serviceUUIDField: &obj.Status.ServiceUUID, "FrontendName": &obj.Status.FrontendName, statusNameField: &obj.Status.Name},
			delete: func(api upcloudapi.LoadBalancerAPI) error {
				return (&LoadBalancerFrontendRuleAdapter{API: api}).Delete(gctx(), obj)
			},
			get: func(api *fake.LoadBalancerAPI) error {
				_, err := api.GetLoadBalancerFrontendRule(gctx(), &request.GetLoadBalancerFrontendRuleRequest{ServiceUUID: parentUUID, FrontendName: feName, Name: ruleName})
				return err
			},
			remove: func(api *fake.LoadBalancerAPI) error {
				return api.DeleteLoadBalancerFrontendRule(gctx(), &request.DeleteLoadBalancerFrontendRuleRequest{ServiceUUID: parentUUID, FrontendName: feName, Name: ruleName})
			},
			deleteCall: "DeleteLoadBalancerFrontendRule",
		}
	})
}
