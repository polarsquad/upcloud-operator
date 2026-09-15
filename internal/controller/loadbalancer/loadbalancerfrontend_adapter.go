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

// LoadBalancerFrontendAdapter maps LoadBalancerFrontend onto the UpCloud load
// balancer frontend API. Rules and TLS configs are separate CRs, so the create
// request sends explicit empty slices.
type LoadBalancerFrontendAdapter struct {
	API    upcloudapi.LoadBalancerAPI
	Client client.Client
}

var _ reconciler.Adapter[*lb.LoadBalancerFrontend] = (*LoadBalancerFrontendAdapter)(nil)

// serviceUUID resolves the referenced LoadBalancer (Ready) on first use.
func (a *LoadBalancerFrontendAdapter) serviceUUID(ctx context.Context, f *lb.LoadBalancerFrontend) (string, error) {
	if f.Status.ServiceUUID != "" {
		return f.Status.ServiceUUID, nil
	}
	uuid, err := resolve.Ready(ctx, a.Client, f.Namespace, f.Spec.LoadBalancerRef.Name, &lb.LoadBalancer{})
	if err != nil {
		return "", err
	}
	f.Status.ServiceUUID = uuid
	return uuid, nil
}

// defaultBackend resolves the referenced LoadBalancerBackend (Ready) to its
// UpCloud name.
func (a *LoadBalancerFrontendAdapter) defaultBackend(ctx context.Context, f *lb.LoadBalancerFrontend) (string, error) {
	return resolve.Ready(ctx, a.Client, f.Namespace, f.Spec.DefaultBackendRef.Name, &lb.LoadBalancerBackend{})
}

// desiredFrontendProperties maps the spec properties (nil when unset).
func desiredFrontendProperties(f *lb.LoadBalancerFrontend) *upcloud.LoadBalancerFrontendProperties {
	p := f.Spec.Properties
	if p == nil {
		return nil
	}
	return &upcloud.LoadBalancerFrontendProperties{
		TimeoutClient:        p.TimeoutClient,
		InboundProxyProtocol: p.InboundProxyProtocol,
		HTTP2Enabled:         p.HTTP2Enabled,
	}
}

// frontendNetworks maps the spec networks to the SDK shape (name only).
func frontendNetworks(f *lb.LoadBalancerFrontend) []upcloud.LoadBalancerFrontendNetwork {
	out := make([]upcloud.LoadBalancerFrontendNetwork, 0, len(f.Spec.Networks))
	for i := range f.Spec.Networks {
		out = append(out, upcloud.LoadBalancerFrontendNetwork{Name: f.Spec.Networks[i].Name})
	}
	return out
}

// Observe implements reconciler.Adapter.
func (a *LoadBalancerFrontendAdapter) Observe(ctx context.Context, f *lb.LoadBalancerFrontend) (reconciler.Observation, error) {
	svcUUID, err := a.serviceUUID(ctx, f)
	if err != nil {
		return reconciler.Observation{}, err
	}
	fe, err := a.API.GetLoadBalancerFrontend(ctx, &request.GetLoadBalancerFrontendRequest{ServiceUUID: svcUUID, Name: f.ExternalName()})
	if upcloudapi.IsNotFound(err) {
		return reconciler.Observation{}, nil
	}
	if err != nil {
		return reconciler.Observation{}, fmt.Errorf("get load balancer frontend: %w", err)
	}
	f.Status.Name = fe.Name
	db, err := a.defaultBackend(ctx, f)
	if err != nil {
		return reconciler.Observation{}, err
	}
	return reconciler.Observation{
		Exists:   true,
		UpToDate: frontendMatches(fe, f, db),
		Ready:    true,
		Message:  "frontend " + fe.Name,
	}, nil
}

// Create implements reconciler.Adapter. A Conflict is adoption.
func (a *LoadBalancerFrontendAdapter) Create(ctx context.Context, f *lb.LoadBalancerFrontend) error {
	svcUUID, err := a.serviceUUID(ctx, f)
	if err != nil {
		return err
	}
	db, err := a.defaultBackend(ctx, f)
	if err != nil {
		return err
	}
	created, err := a.API.CreateLoadBalancerFrontend(ctx, &request.CreateLoadBalancerFrontendRequest{
		ServiceUUID: svcUUID,
		Frontend: request.LoadBalancerFrontend{
			Name:           f.ExternalName(),
			Mode:           upcloud.LoadBalancerMode(f.Spec.Mode),
			Port:           f.Spec.Port,
			Networks:       frontendNetworks(f),
			DefaultBackend: db,
			Properties:     desiredFrontendProperties(f),
			Rules:          []request.LoadBalancerFrontendRule{},      // rules are separate CRs
			TLSConfigs:     []request.LoadBalancerFrontendTLSConfig{}, // TLS configs are separate CRs
		},
	})
	if err != nil {
		if upcloudapi.IsConflict(err) {
			return nil // adopted; the next Observe reconciles it
		}
		return fmt.Errorf("create load balancer frontend: %w", err)
	}
	f.Status.Name = created.Name
	return nil
}

// Update implements reconciler.Adapter. Sends mode, port, default backend and
// properties in one modify call. Networks are immutable.
func (a *LoadBalancerFrontendAdapter) Update(ctx context.Context, f *lb.LoadBalancerFrontend) error {
	svcUUID, err := a.serviceUUID(ctx, f)
	if err != nil {
		return err
	}
	db, err := a.defaultBackend(ctx, f)
	if err != nil {
		return err
	}
	if _, err := a.API.GetLoadBalancerFrontend(ctx, &request.GetLoadBalancerFrontendRequest{ServiceUUID: svcUUID, Name: f.ExternalName()}); upcloudapi.IsNotFound(err) {
		return reconciler.ErrPending
	} else if err != nil {
		return fmt.Errorf("get load balancer frontend: %w", err)
	}
	_, err = a.API.ModifyLoadBalancerFrontend(ctx, &request.ModifyLoadBalancerFrontendRequest{
		ServiceUUID: svcUUID,
		Name:        f.ExternalName(),
		Frontend: request.ModifyLoadBalancerFrontend{
			Name:           f.ExternalName(),
			Mode:           upcloud.LoadBalancerMode(f.Spec.Mode),
			Port:           f.Spec.Port,
			DefaultBackend: db,
			Properties:     desiredFrontendProperties(f),
		},
	})
	if err != nil {
		return fmt.Errorf("modify load balancer frontend: %w", err)
	}
	return nil
}

// Delete implements reconciler.Adapter. A 404 counts as deleted.
func (a *LoadBalancerFrontendAdapter) Delete(ctx context.Context, f *lb.LoadBalancerFrontend) error {
	if f.Status.ServiceUUID == "" || f.Status.Name == "" {
		return nil
	}
	err := a.API.DeleteLoadBalancerFrontend(ctx, &request.DeleteLoadBalancerFrontendRequest{
		ServiceUUID: f.Status.ServiceUUID,
		Name:        f.Status.Name,
	})
	switch {
	case err == nil, upcloudapi.IsNotFound(err):
		return nil
	default:
		return fmt.Errorf("delete load balancer frontend: %w", err)
	}
}

// frontendMatches compares the mutable fields against the observed frontend.
func frontendMatches(fe *upcloud.LoadBalancerFrontend, f *lb.LoadBalancerFrontend, defaultBackend string) bool {
	if string(fe.Mode) != f.Spec.Mode || fe.Port != f.Spec.Port || fe.DefaultBackend != defaultBackend {
		return false
	}
	want := f.Spec.Properties
	if want == nil {
		return fe.Properties == nil
	}
	got := fe.Properties
	if got == nil {
		return false
	}
	if got.TimeoutClient != want.TimeoutClient {
		return false
	}
	if want.InboundProxyProtocol != nil && (got.InboundProxyProtocol == nil || *got.InboundProxyProtocol != *want.InboundProxyProtocol) {
		return false
	}
	if want.HTTP2Enabled != nil && (got.HTTP2Enabled == nil || *got.HTTP2Enabled != *want.HTTP2Enabled) {
		return false
	}
	return true
}
