package loadbalancer

import (
	"testing"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud"
	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud/request"

	lb "github.com/polarsquad/upcloud-operator/api/loadbalancer/v1alpha1"
	"github.com/polarsquad/upcloud-operator/internal/upcloudapi"
	"github.com/polarsquad/upcloud-operator/internal/upcloudapi/fake"
)

func TestLoadBalancerFrontendTLSConfigDeleteContract(t *testing.T) {
	checkDeleteContract(t, func(api *fake.LoadBalancerAPI) deleteContractFixture {
		api.LoadBalancers[parentUUID] = &upcloud.LoadBalancer{UUID: parentUUID, Frontends: []upcloud.LoadBalancerFrontend{{Name: feName, TLSConfigs: []upcloud.LoadBalancerFrontendTLSConfig{{Name: ruleName}}}}}
		obj := &lb.LoadBalancerFrontendTLSConfig{Status: lb.LoadBalancerFrontendTLSConfigStatus{ServiceUUID: parentUUID, FrontendName: feName, Name: ruleName}}
		return deleteContractFixture{
			identities: map[string]*string{serviceUUIDField: &obj.Status.ServiceUUID, "FrontendName": &obj.Status.FrontendName, statusNameField: &obj.Status.Name},
			delete: func(api upcloudapi.LoadBalancerAPI) error {
				return (&LoadBalancerFrontendTLSConfigAdapter{API: api}).Delete(gctx(), obj)
			},
			get: func(api *fake.LoadBalancerAPI) error {
				_, err := api.GetLoadBalancerFrontendTLSConfig(gctx(), &request.GetLoadBalancerFrontendTLSConfigRequest{ServiceUUID: parentUUID, FrontendName: feName, Name: ruleName})
				return err
			},
			remove: func(api *fake.LoadBalancerAPI) error {
				return api.DeleteLoadBalancerFrontendTLSConfig(gctx(), &request.DeleteLoadBalancerFrontendTLSConfigRequest{ServiceUUID: parentUUID, FrontendName: feName, Name: ruleName})
			},
			deleteCall: "DeleteLoadBalancerFrontendTLSConfig",
		}
	})
}
