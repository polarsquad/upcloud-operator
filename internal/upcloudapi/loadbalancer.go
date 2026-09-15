package upcloudapi

import (
	"context"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud"
	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud/request"
	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud/service"
)

// LoadBalancerAPI is the subset of service.Service used by the load balancer
// adapters. The DNS-challenge, plans, and IP-address attach/remove methods are
// deliberately excluded (out of scope for v0.1).
type LoadBalancerAPI interface {
	// Service
	CreateLoadBalancer(ctx context.Context, r *request.CreateLoadBalancerRequest) (*upcloud.LoadBalancer, error)
	GetLoadBalancers(ctx context.Context, r *request.GetLoadBalancersRequest) ([]upcloud.LoadBalancer, error)
	GetLoadBalancer(ctx context.Context, r *request.GetLoadBalancerRequest) (*upcloud.LoadBalancer, error)
	ModifyLoadBalancer(ctx context.Context, r *request.ModifyLoadBalancerRequest) (*upcloud.LoadBalancer, error)
	DeleteLoadBalancer(ctx context.Context, r *request.DeleteLoadBalancerRequest) error

	// Backends
	CreateLoadBalancerBackend(ctx context.Context, r *request.CreateLoadBalancerBackendRequest) (*upcloud.LoadBalancerBackend, error)
	GetLoadBalancerBackends(ctx context.Context, r *request.GetLoadBalancerBackendsRequest) ([]upcloud.LoadBalancerBackend, error)
	GetLoadBalancerBackend(ctx context.Context, r *request.GetLoadBalancerBackendRequest) (*upcloud.LoadBalancerBackend, error)
	ModifyLoadBalancerBackend(ctx context.Context, r *request.ModifyLoadBalancerBackendRequest) (*upcloud.LoadBalancerBackend, error)
	DeleteLoadBalancerBackend(ctx context.Context, r *request.DeleteLoadBalancerBackendRequest) error

	// Backend members
	CreateLoadBalancerBackendMember(ctx context.Context, r *request.CreateLoadBalancerBackendMemberRequest) (*upcloud.LoadBalancerBackendMember, error)
	GetLoadBalancerBackendMembers(ctx context.Context, r *request.GetLoadBalancerBackendMembersRequest) ([]upcloud.LoadBalancerBackendMember, error)
	GetLoadBalancerBackendMember(ctx context.Context, r *request.GetLoadBalancerBackendMemberRequest) (*upcloud.LoadBalancerBackendMember, error)
	ModifyLoadBalancerBackendMember(ctx context.Context, r *request.ModifyLoadBalancerBackendMemberRequest) (*upcloud.LoadBalancerBackendMember, error)
	DeleteLoadBalancerBackendMember(ctx context.Context, r *request.DeleteLoadBalancerBackendMemberRequest) error

	// Backend TLS configs
	CreateLoadBalancerBackendTLSConfig(ctx context.Context, r *request.CreateLoadBalancerBackendTLSConfigRequest) (*upcloud.LoadBalancerBackendTLSConfig, error)
	GetLoadBalancerBackendTLSConfigs(ctx context.Context, r *request.GetLoadBalancerBackendTLSConfigsRequest) ([]upcloud.LoadBalancerBackendTLSConfig, error)
	GetLoadBalancerBackendTLSConfig(ctx context.Context, r *request.GetLoadBalancerBackendTLSConfigRequest) (*upcloud.LoadBalancerBackendTLSConfig, error)
	ModifyLoadBalancerBackendTLSConfig(ctx context.Context, r *request.ModifyLoadBalancerBackendTLSConfigRequest) (*upcloud.LoadBalancerBackendTLSConfig, error)
	DeleteLoadBalancerBackendTLSConfig(ctx context.Context, r *request.DeleteLoadBalancerBackendTLSConfigRequest) error

	// Resolvers
	CreateLoadBalancerResolver(ctx context.Context, r *request.CreateLoadBalancerResolverRequest) (*upcloud.LoadBalancerResolver, error)
	GetLoadBalancerResolvers(ctx context.Context, r *request.GetLoadBalancerResolversRequest) ([]upcloud.LoadBalancerResolver, error)
	GetLoadBalancerResolver(ctx context.Context, r *request.GetLoadBalancerResolverRequest) (*upcloud.LoadBalancerResolver, error)
	ModifyLoadBalancerResolver(ctx context.Context, r *request.ModifyLoadBalancerResolverRequest) (*upcloud.LoadBalancerResolver, error)
	DeleteLoadBalancerResolver(ctx context.Context, r *request.DeleteLoadBalancerResolverRequest) error

	// Frontends
	CreateLoadBalancerFrontend(ctx context.Context, r *request.CreateLoadBalancerFrontendRequest) (*upcloud.LoadBalancerFrontend, error)
	GetLoadBalancerFrontends(ctx context.Context, r *request.GetLoadBalancerFrontendsRequest) ([]upcloud.LoadBalancerFrontend, error)
	GetLoadBalancerFrontend(ctx context.Context, r *request.GetLoadBalancerFrontendRequest) (*upcloud.LoadBalancerFrontend, error)
	ModifyLoadBalancerFrontend(ctx context.Context, r *request.ModifyLoadBalancerFrontendRequest) (*upcloud.LoadBalancerFrontend, error)
	DeleteLoadBalancerFrontend(ctx context.Context, r *request.DeleteLoadBalancerFrontendRequest) error

	// Frontend rules
	CreateLoadBalancerFrontendRule(ctx context.Context, r *request.CreateLoadBalancerFrontendRuleRequest) (*upcloud.LoadBalancerFrontendRule, error)
	GetLoadBalancerFrontendRules(ctx context.Context, r *request.GetLoadBalancerFrontendRulesRequest) ([]upcloud.LoadBalancerFrontendRule, error)
	GetLoadBalancerFrontendRule(ctx context.Context, r *request.GetLoadBalancerFrontendRuleRequest) (*upcloud.LoadBalancerFrontendRule, error)
	ReplaceLoadBalancerFrontendRule(ctx context.Context, r *request.ReplaceLoadBalancerFrontendRuleRequest) (*upcloud.LoadBalancerFrontendRule, error)
	DeleteLoadBalancerFrontendRule(ctx context.Context, r *request.DeleteLoadBalancerFrontendRuleRequest) error

	// Frontend TLS configs
	CreateLoadBalancerFrontendTLSConfig(ctx context.Context, r *request.CreateLoadBalancerFrontendTLSConfigRequest) (*upcloud.LoadBalancerFrontendTLSConfig, error)
	GetLoadBalancerFrontendTLSConfigs(ctx context.Context, r *request.GetLoadBalancerFrontendTLSConfigsRequest) ([]upcloud.LoadBalancerFrontendTLSConfig, error)
	GetLoadBalancerFrontendTLSConfig(ctx context.Context, r *request.GetLoadBalancerFrontendTLSConfigRequest) (*upcloud.LoadBalancerFrontendTLSConfig, error)
	ModifyLoadBalancerFrontendTLSConfig(ctx context.Context, r *request.ModifyLoadBalancerFrontendTLSConfigRequest) (*upcloud.LoadBalancerFrontendTLSConfig, error)
	DeleteLoadBalancerFrontendTLSConfig(ctx context.Context, r *request.DeleteLoadBalancerFrontendTLSConfigRequest) error

	// Certificate bundles (account-level, not scoped to a load balancer)
	CreateLoadBalancerCertificateBundle(ctx context.Context, r *request.CreateLoadBalancerCertificateBundleRequest) (*upcloud.LoadBalancerCertificateBundle, error)
	GetLoadBalancerCertificateBundles(ctx context.Context, r *request.GetLoadBalancerCertificateBundlesRequest) ([]upcloud.LoadBalancerCertificateBundle, error)
	GetLoadBalancerCertificateBundle(ctx context.Context, r *request.GetLoadBalancerCertificateBundleRequest) (*upcloud.LoadBalancerCertificateBundle, error)
	ModifyLoadBalancerCertificateBundle(ctx context.Context, r *request.ModifyLoadBalancerCertificateBundleRequest) (*upcloud.LoadBalancerCertificateBundle, error)
	DeleteLoadBalancerCertificateBundle(ctx context.Context, r *request.DeleteLoadBalancerCertificateBundleRequest) error
}

var _ LoadBalancerAPI = (*service.Service)(nil)
