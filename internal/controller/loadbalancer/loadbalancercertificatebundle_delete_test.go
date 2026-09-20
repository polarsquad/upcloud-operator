package loadbalancer

import (
	"testing"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud"
	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud/request"

	lb "github.com/polarsquad/upcloud-operator/api/loadbalancer/v1alpha1"
	"github.com/polarsquad/upcloud-operator/internal/upcloudapi"
	"github.com/polarsquad/upcloud-operator/internal/upcloudapi/fake"
)

func TestLoadBalancerCertificateBundleDeleteContract(t *testing.T) {
	checkDeleteContract(t, func(api *fake.LoadBalancerAPI) deleteContractFixture {
		api.CertificateBundles[bundleUUID] = &upcloud.LoadBalancerCertificateBundle{UUID: bundleUUID}
		obj := &lb.LoadBalancerCertificateBundle{Status: lb.LoadBalancerCertificateBundleStatus{UUID: bundleUUID}}
		return deleteContractFixture{
			identities: map[string]*string{"UUID": &obj.Status.UUID},
			delete: func(api upcloudapi.LoadBalancerAPI) error {
				return (&LoadBalancerCertificateBundleAdapter{API: api}).Delete(gctx(), obj)
			},
			get: func(api *fake.LoadBalancerAPI) error {
				_, err := api.GetLoadBalancerCertificateBundle(gctx(), &request.GetLoadBalancerCertificateBundleRequest{UUID: bundleUUID})
				return err
			},
			remove: func(api *fake.LoadBalancerAPI) error {
				return api.DeleteLoadBalancerCertificateBundle(gctx(), &request.DeleteLoadBalancerCertificateBundleRequest{UUID: bundleUUID})
			},
			deleteCall: "DeleteLoadBalancerCertificateBundle",
		}
	})
}
