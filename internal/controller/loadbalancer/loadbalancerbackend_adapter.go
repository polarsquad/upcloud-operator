package loadbalancer

import (
	"context"
	"fmt"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud"
	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud/request"
	"sigs.k8s.io/controller-runtime/pkg/client"

	lb "github.com/polarsquad/upcloud-operator/api/loadbalancer/v1alpha1"
	"github.com/polarsquad/upcloud-operator/internal/reconciler"
	"github.com/polarsquad/upcloud-operator/internal/resolve"
	"github.com/polarsquad/upcloud-operator/internal/upcloudapi"
)

// LoadBalancerBackendAdapter maps LoadBalancerBackend onto the UpCloud load
// balancer backend API. Members are separate CRs, so the create request sends
// an explicit empty Members slice.
type LoadBalancerBackendAdapter struct {
	API    upcloudapi.LoadBalancerAPI
	Client client.Client
}

var _ reconciler.Adapter[*lb.LoadBalancerBackend] = (*LoadBalancerBackendAdapter)(nil)

// serviceUUID resolves the referenced LoadBalancer (Ready) on first use.
func (a *LoadBalancerBackendAdapter) serviceUUID(ctx context.Context, b *lb.LoadBalancerBackend) (string, error) {
	if b.Status.ServiceUUID != "" {
		return b.Status.ServiceUUID, nil
	}
	uuid, err := resolve.Ready(ctx, a.Client, b.Namespace, b.Spec.LoadBalancerRef.Name, &lb.LoadBalancer{})
	if err != nil {
		return "", err
	}
	b.Status.ServiceUUID = uuid
	return uuid, nil
}

// resolverName resolves the referenced LoadBalancerResolver (Ready) to its
// UpCloud name, or "" when no resolver is referenced.
func (a *LoadBalancerBackendAdapter) resolverName(ctx context.Context, b *lb.LoadBalancerBackend) (string, error) {
	if b.Spec.ResolverRef == nil {
		return "", nil
	}
	return resolve.Ready(ctx, a.Client, b.Namespace, b.Spec.ResolverRef.Name, &lb.LoadBalancerResolver{})
}

// desiredProperties maps the spec properties (nil when unset).
func desiredProperties(b *lb.LoadBalancerBackend) *upcloud.LoadBalancerBackendProperties {
	p := b.Spec.Properties
	if p == nil {
		return nil
	}
	out := &upcloud.LoadBalancerBackendProperties{
		TimeoutServer:             p.TimeoutServer,
		TimeoutTunnel:             p.TimeoutTunnel,
		HealthCheckType:           upcloud.LoadBalancerHealthCheckType(p.HealthCheckType),
		HealthCheckInterval:       p.HealthCheckInterval,
		HealthCheckFall:           p.HealthCheckFall,
		HealthCheckRise:           p.HealthCheckRise,
		HealthCheckURL:            p.HealthCheckURL,
		HealthCheckExpectedStatus: p.HealthCheckExpectedStatus,
		StickySessionCookieName:   p.StickySessionCookieName,
		OutboundProxyProtocol:     upcloud.LoadBalancerProxyProtocolVersion(p.OutboundProxyProtocol),
		HealthCheckTLSVerify:      p.HealthCheckTLSVerify,
		TLSEnabled:                p.TLSEnabled,
		TLSVerify:                 p.TLSVerify,
		TLSUseSystemCA:            p.TLSUseSystemCA,
		HTTP2Enabled:              p.HTTP2Enabled,
	}
	return out
}

// Observe implements reconciler.Adapter.
func (a *LoadBalancerBackendAdapter) Observe(ctx context.Context, b *lb.LoadBalancerBackend) (reconciler.Observation, error) {
	svcUUID, err := a.serviceUUID(ctx, b)
	if err != nil {
		return reconciler.Observation{}, err
	}
	be, err := a.API.GetLoadBalancerBackend(ctx, &request.GetLoadBalancerBackendRequest{ServiceUUID: svcUUID, Name: b.ExternalName()})
	if upcloudapi.IsNotFound(err) {
		return reconciler.Observation{}, nil
	}
	if err != nil {
		return reconciler.Observation{}, fmt.Errorf("get load balancer backend: %w", err)
	}
	b.Status.Name = be.Name
	resolver, err := a.resolverName(ctx, b)
	if err != nil {
		return reconciler.Observation{}, err
	}
	return reconciler.Observation{
		Exists:   true,
		UpToDate: backendMatches(be, b, resolver),
		Ready:    true,
		Message:  "backend " + be.Name,
	}, nil
}

// Create implements reconciler.Adapter. A Conflict is adoption.
func (a *LoadBalancerBackendAdapter) Create(ctx context.Context, b *lb.LoadBalancerBackend) error {
	svcUUID, err := a.serviceUUID(ctx, b)
	if err != nil {
		return err
	}
	resolver, err := a.resolverName(ctx, b)
	if err != nil {
		return err
	}
	created, err := a.API.CreateLoadBalancerBackend(ctx, &request.CreateLoadBalancerBackendRequest{
		ServiceUUID: svcUUID,
		Backend: request.LoadBalancerBackend{
			Name:       b.ExternalName(),
			Resolver:   resolver,
			Properties: desiredProperties(b),
			Members:    []request.LoadBalancerBackendMember{}, // members are separate CRs
		},
	})
	if err != nil {
		if upcloudapi.IsConflict(err) {
			return nil // adopted; the next Observe reconciles it
		}
		return fmt.Errorf("create load balancer backend: %w", err)
	}
	b.Status.Name = created.Name
	return nil
}

// Update implements reconciler.Adapter. Sends resolver and properties in one
// modify call.
func (a *LoadBalancerBackendAdapter) Update(ctx context.Context, b *lb.LoadBalancerBackend) error {
	svcUUID, err := a.serviceUUID(ctx, b)
	if err != nil {
		return err
	}
	resolver, err := a.resolverName(ctx, b)
	if err != nil {
		return err
	}
	if _, err := a.API.GetLoadBalancerBackend(ctx, &request.GetLoadBalancerBackendRequest{ServiceUUID: svcUUID, Name: b.ExternalName()}); upcloudapi.IsNotFound(err) {
		return reconciler.ErrPending
	} else if err != nil {
		return fmt.Errorf("get load balancer backend: %w", err)
	}
	_, err = a.API.ModifyLoadBalancerBackend(ctx, &request.ModifyLoadBalancerBackendRequest{
		ServiceUUID: svcUUID,
		Name:        b.ExternalName(),
		Backend: request.ModifyLoadBalancerBackend{
			Name:       b.ExternalName(),
			Resolver:   &resolver,
			Properties: desiredProperties(b),
		},
	})
	if err != nil {
		return fmt.Errorf("modify load balancer backend: %w", err)
	}
	return nil
}

// Delete implements reconciler.Adapter. A 404 counts as deleted.
func (a *LoadBalancerBackendAdapter) Delete(ctx context.Context, b *lb.LoadBalancerBackend) error {
	if b.Status.ServiceUUID == "" || b.Status.Name == "" {
		return nil
	}
	err := a.API.DeleteLoadBalancerBackend(ctx, &request.DeleteLoadBalancerBackendRequest{
		ServiceUUID: b.Status.ServiceUUID,
		Name:        b.Status.Name,
	})
	switch {
	case err == nil, upcloudapi.IsNotFound(err):
		return nil
	default:
		return fmt.Errorf("delete load balancer backend: %w", err)
	}
}

// backendMatches compares the resolver and every property the spec sets.
func backendMatches(be *upcloud.LoadBalancerBackend, b *lb.LoadBalancerBackend, resolver string) bool {
	if be.Resolver != resolver {
		return false
	}
	want := b.Spec.Properties
	if want == nil {
		return true
	}
	got := be.Properties
	if got == nil {
		return false
	}
	if got.TimeoutServer != want.TimeoutServer || got.TimeoutTunnel != want.TimeoutTunnel {
		return false
	}
	if string(got.HealthCheckType) != want.HealthCheckType ||
		got.HealthCheckInterval != want.HealthCheckInterval ||
		got.HealthCheckFall != want.HealthCheckFall ||
		got.HealthCheckRise != want.HealthCheckRise ||
		got.HealthCheckURL != want.HealthCheckURL ||
		got.HealthCheckExpectedStatus != want.HealthCheckExpectedStatus ||
		got.StickySessionCookieName != want.StickySessionCookieName ||
		string(got.OutboundProxyProtocol) != want.OutboundProxyProtocol {
		return false
	}
	if want.HealthCheckTLSVerify != nil && (got.HealthCheckTLSVerify == nil || *got.HealthCheckTLSVerify != *want.HealthCheckTLSVerify) {
		return false
	}
	if want.TLSEnabled != nil && (got.TLSEnabled == nil || *got.TLSEnabled != *want.TLSEnabled) {
		return false
	}
	if want.TLSVerify != nil && (got.TLSVerify == nil || *got.TLSVerify != *want.TLSVerify) {
		return false
	}
	if want.TLSUseSystemCA != nil && (got.TLSUseSystemCA == nil || *got.TLSUseSystemCA != *want.TLSUseSystemCA) {
		return false
	}
	if want.HTTP2Enabled != nil && (got.HTTP2Enabled == nil || *got.HTTP2Enabled != *want.HTTP2Enabled) {
		return false
	}
	return true
}
