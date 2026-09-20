package loadbalancer

import (
	"context"
	"fmt"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud/request"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	lb "github.com/polarsquad/upcloud-operator/api/loadbalancer/v1alpha1"
	"github.com/polarsquad/upcloud-operator/internal/reconciler"
	"github.com/polarsquad/upcloud-operator/internal/resolve"
	"github.com/polarsquad/upcloud-operator/internal/upcloudapi"
)

// LoadBalancerFrontendTLSConfigAdapter maps LoadBalancerFrontendTLSConfig onto
// the UpCloud load balancer frontend TLS config API. A TLS config is addressed
// by (serviceUUID, frontendName, name) and is up to date when its attached
// certificate bundle UUID matches.
type LoadBalancerFrontendTLSConfigAdapter struct {
	API    upcloudapi.LoadBalancerAPI
	Client client.Client
}

var _ reconciler.Adapter[*lb.LoadBalancerFrontendTLSConfig] = (*LoadBalancerFrontendTLSConfigAdapter)(nil)

// parent resolves the referenced LoadBalancerFrontend (Ready) and stores its
// service UUID and frontend name in status.
func (a *LoadBalancerFrontendTLSConfigAdapter) parent(ctx context.Context, c *lb.LoadBalancerFrontendTLSConfig) error {
	if c.Status.ServiceUUID != "" && c.Status.FrontendName != "" {
		return nil
	}
	fe := &lb.LoadBalancerFrontend{}
	if err := a.Client.Get(ctx, client.ObjectKey{Namespace: c.Namespace, Name: c.Spec.FrontendRef.Name}, fe); err != nil {
		if apierrors.IsNotFound(err) {
			return fmt.Errorf("frontend %q not ready: %w", c.Spec.FrontendRef.Name, reconciler.ErrDependencyNotReady)
		}
		return fmt.Errorf("get frontend: %w", err)
	}
	if !reconciler.IsReady(fe) || fe.Status.ServiceUUID == "" || fe.Status.Name == "" {
		return fmt.Errorf("frontend %q not ready: %w", c.Spec.FrontendRef.Name, reconciler.ErrDependencyNotReady)
	}
	c.Status.ServiceUUID = fe.Status.ServiceUUID
	c.Status.FrontendName = fe.Status.Name
	return nil
}

// bundleUUID resolves the referenced LoadBalancerCertificateBundle (Ready) to
// its UpCloud UUID.
func (a *LoadBalancerFrontendTLSConfigAdapter) bundleUUID(ctx context.Context, c *lb.LoadBalancerFrontendTLSConfig) (string, error) {
	return resolve.Ready(ctx, a.Client, c.Namespace, c.Spec.CertificateBundleRef.Name, &lb.LoadBalancerCertificateBundle{})
}

// Observe implements reconciler.Adapter.
func (a *LoadBalancerFrontendTLSConfigAdapter) Observe(ctx context.Context, c *lb.LoadBalancerFrontendTLSConfig) (reconciler.Observation, error) {
	if err := a.parent(ctx, c); err != nil {
		return reconciler.Observation{}, err
	}
	cfg, err := a.API.GetLoadBalancerFrontendTLSConfig(ctx, &request.GetLoadBalancerFrontendTLSConfigRequest{
		ServiceUUID: c.Status.ServiceUUID, FrontendName: c.Status.FrontendName, Name: c.ExternalName(),
	})
	if upcloudapi.IsNotFound(err) {
		return reconciler.Observation{}, nil
	}
	if err != nil {
		return reconciler.Observation{}, fmt.Errorf("get load balancer frontend TLS config: %w", err)
	}
	c.Status.Name = cfg.Name
	bundleUUID, err := a.bundleUUID(ctx, c)
	if err != nil {
		return reconciler.Observation{}, err
	}
	return reconciler.Observation{
		Exists:   true,
		UpToDate: cfg.CertificateBundleUUID == bundleUUID,
		Ready:    true,
		Message:  "frontend TLS config " + cfg.Name,
	}, nil
}

// Create implements reconciler.Adapter. A Conflict is adoption.
func (a *LoadBalancerFrontendTLSConfigAdapter) Create(ctx context.Context, c *lb.LoadBalancerFrontendTLSConfig) error {
	if err := a.parent(ctx, c); err != nil {
		return err
	}
	bundleUUID, err := a.bundleUUID(ctx, c)
	if err != nil {
		return err
	}
	created, err := a.API.CreateLoadBalancerFrontendTLSConfig(ctx, &request.CreateLoadBalancerFrontendTLSConfigRequest{
		ServiceUUID:  c.Status.ServiceUUID,
		FrontendName: c.Status.FrontendName,
		Config:       request.LoadBalancerFrontendTLSConfig{Name: c.ExternalName(), CertificateBundleUUID: bundleUUID},
	})
	if err != nil {
		if upcloudapi.IsConflict(err) {
			return nil // adopted; the next Observe reconciles it
		}
		return fmt.Errorf("create load balancer frontend TLS config: %w", err)
	}
	c.Status.Name = created.Name
	return nil
}

// Update implements reconciler.Adapter. Re-sends the certificate bundle UUID.
func (a *LoadBalancerFrontendTLSConfigAdapter) Update(ctx context.Context, c *lb.LoadBalancerFrontendTLSConfig) error {
	if err := a.parent(ctx, c); err != nil {
		return err
	}
	bundleUUID, err := a.bundleUUID(ctx, c)
	if err != nil {
		return err
	}
	if _, err := a.API.GetLoadBalancerFrontendTLSConfig(ctx, &request.GetLoadBalancerFrontendTLSConfigRequest{
		ServiceUUID: c.Status.ServiceUUID, FrontendName: c.Status.FrontendName, Name: c.ExternalName(),
	}); upcloudapi.IsNotFound(err) {
		return reconciler.ErrPending
	} else if err != nil {
		return fmt.Errorf("get load balancer frontend TLS config: %w", err)
	}
	_, err = a.API.ModifyLoadBalancerFrontendTLSConfig(ctx, &request.ModifyLoadBalancerFrontendTLSConfigRequest{
		ServiceUUID:  c.Status.ServiceUUID,
		FrontendName: c.Status.FrontendName,
		Name:         c.ExternalName(),
		Config:       request.LoadBalancerFrontendTLSConfig{Name: c.ExternalName(), CertificateBundleUUID: bundleUUID},
	})
	if err != nil {
		return fmt.Errorf("modify load balancer frontend TLS config: %w", err)
	}
	return nil
}

// Delete implements reconciler.Adapter. A 404 counts as deleted.
func (a *LoadBalancerFrontendTLSConfigAdapter) Delete(ctx context.Context, c *lb.LoadBalancerFrontendTLSConfig) error {
	if c.Status.ServiceUUID == "" || c.Status.FrontendName == "" || c.Status.Name == "" {
		return nil
	}
	err := a.API.DeleteLoadBalancerFrontendTLSConfig(ctx, &request.DeleteLoadBalancerFrontendTLSConfigRequest{
		ServiceUUID: c.Status.ServiceUUID, FrontendName: c.Status.FrontendName, Name: c.Status.Name,
	})
	switch {
	case err == nil, upcloudapi.IsNotFound(err):
		return nil
	case upcloudapi.IsConflict(err):
		return fmt.Errorf("%w: %s", reconciler.ErrPending, upcloudapi.TitleOf(err))
	default:
		return fmt.Errorf("delete load balancer frontend TLS config: %w", err)
	}
}
