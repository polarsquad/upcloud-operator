package loadbalancer

import (
	"testing"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud"
	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud/request"

	lb "github.com/polarsquad/upcloud-operator/api/loadbalancer/v1alpha1"
	"github.com/polarsquad/upcloud-operator/internal/upcloudapi"
	"github.com/polarsquad/upcloud-operator/internal/upcloudapi/fake"
)

func TestLoadBalancerBackendMemberDeleteContract(t *testing.T) {
	checkDeleteContract(t, func(api *fake.LoadBalancerAPI) deleteContractFixture {
		api.LoadBalancers[parentUUID] = &upcloud.LoadBalancer{UUID: parentUUID, Backends: []upcloud.LoadBalancerBackend{{Name: backendName, Members: []upcloud.LoadBalancerBackendMember{{Name: ruleName}}}}}
		obj := &lb.LoadBalancerBackendMember{Status: lb.LoadBalancerBackendMemberStatus{ServiceUUID: parentUUID, BackendName: backendName, Name: ruleName}}
		return deleteContractFixture{
			identities: map[string]*string{serviceUUIDField: &obj.Status.ServiceUUID, "BackendName": &obj.Status.BackendName, statusNameField: &obj.Status.Name},
			delete: func(api upcloudapi.LoadBalancerAPI) error {
				return (&LoadBalancerBackendMemberAdapter{API: api}).Delete(gctx(), obj)
			},
			get: func(api *fake.LoadBalancerAPI) error {
				_, err := api.GetLoadBalancerBackendMember(gctx(), &request.GetLoadBalancerBackendMemberRequest{ServiceUUID: parentUUID, BackendName: backendName, Name: ruleName})
				return err
			},
			remove: func(api *fake.LoadBalancerAPI) error {
				return api.DeleteLoadBalancerBackendMember(gctx(), &request.DeleteLoadBalancerBackendMemberRequest{ServiceUUID: parentUUID, BackendName: backendName, Name: ruleName})
			},
			deleteCall: "DeleteLoadBalancerBackendMember",
		}
	})
}
