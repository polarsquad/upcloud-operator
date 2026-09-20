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

// LoadBalancerBackendTLSConfigAdapter maps LoadBalancerBackendTLSConfig onto
// the UpCloud load balancer backend TLS config API. A TLS config is addressed
// by (serviceUUID, backendName, name) and is up to date when its attached
// certificate bundle UUID matches.
type LoadBalancerBackendTLSConfigAdapter struct {
	API    upcloudapi.LoadBalancerAPI
	Client client.Client
}

var _ reconciler.Adapter[*lb.LoadBalancerBackendTLSConfig] = (*LoadBalancerBackendTLSConfigAdapter)(nil)

// parent resolves the referenced LoadBalancerBackend (Ready) and stores its
// service UUID and backend name in status.
func (a *LoadBalancerBackendTLSConfigAdapter) parent(ctx context.Context, c *lb.LoadBalancerBackendTLSConfig) error {
	if c.Status.ServiceUUID != "" && c.Status.BackendName != "" {
		return nil
	}
	be := &lb.LoadBalancerBackend{}
	if err := a.Client.Get(ctx, client.ObjectKey{Namespace: c.Namespace, Name: c.Spec.BackendRef.Name}, be); err != nil {
		if apierrors.IsNotFound(err) {
			return fmt.Errorf("backend %q not ready: %w", c.Spec.BackendRef.Name, reconciler.ErrDependencyNotReady)
		}
		return fmt.Errorf("get backend: %w", err)
	}
	if !reconciler.IsReady(be) || be.Status.ServiceUUID == "" || be.Status.Name == "" {
		return fmt.Errorf("backend %q not ready: %w", c.Spec.BackendRef.Name, reconciler.ErrDependencyNotReady)
	}
	c.Status.ServiceUUID = be.Status.ServiceUUID
	c.Status.BackendName = be.Status.Name
	return nil
}

// bundleUUID resolves the referenced LoadBalancerCertificateBundle (Ready) to
// its UpCloud UUID.
func (a *LoadBalancerBackendTLSConfigAdapter) bundleUUID(ctx context.Context, c *lb.LoadBalancerBackendTLSConfig) (string, error) {
	return resolve.Ready(ctx, a.Client, c.Namespace, c.Spec.CertificateBundleRef.Name, &lb.LoadBalancerCertificateBundle{})
}

// Observe implements reconciler.Adapter.
func (a *LoadBalancerBackendTLSConfigAdapter) Observe(ctx context.Context, c *lb.LoadBalancerBackendTLSConfig) (reconciler.Observation, error) {
	if err := a.parent(ctx, c); err != nil {
		return reconciler.Observation{}, err
	}
	cfg, err := a.API.GetLoadBalancerBackendTLSConfig(ctx, &request.GetLoadBalancerBackendTLSConfigRequest{
		ServiceUUID: c.Status.ServiceUUID, BackendName: c.Status.BackendName, Name: c.ExternalName(),
	})
	if upcloudapi.IsNotFound(err) {
		return reconciler.Observation{}, nil
	}
	if err != nil {
		return reconciler.Observation{}, fmt.Errorf("get load balancer backend TLS config: %w", err)
	}
	c.Status.Name = cfg.Name
	c.Status.CertificateBundleUUID = cfg.CertificateBundleUUID
	bundleUUID, err := a.bundleUUID(ctx, c)
	if err != nil {
		return reconciler.Observation{}, err
	}
	return reconciler.Observation{
		Exists:   true,
		UpToDate: cfg.CertificateBundleUUID == bundleUUID,
		Ready:    true,
		Message:  "backend TLS config " + cfg.Name,
	}, nil
}

// Create implements reconciler.Adapter. A Conflict is adoption.
func (a *LoadBalancerBackendTLSConfigAdapter) Create(ctx context.Context, c *lb.LoadBalancerBackendTLSConfig) error {
	if err := a.parent(ctx, c); err != nil {
		return err
	}
	bundleUUID, err := a.bundleUUID(ctx, c)
	if err != nil {
		return err
	}
	created, err := a.API.CreateLoadBalancerBackendTLSConfig(ctx, &request.CreateLoadBalancerBackendTLSConfigRequest{
		ServiceUUID: c.Status.ServiceUUID,
		BackendName: c.Status.BackendName,
		Config:      request.LoadBalancerBackendTLSConfig{Name: c.ExternalName(), CertificateBundleUUID: bundleUUID},
	})
	if err != nil {
		if upcloudapi.IsConflict(err) {
			return nil // adopted; the next Observe reconciles it
		}
		return fmt.Errorf("create load balancer backend TLS config: %w", err)
	}
	c.Status.Name = created.Name
	c.Status.CertificateBundleUUID = created.CertificateBundleUUID
	return nil
}

// Update implements reconciler.Adapter. Re-sends the certificate bundle UUID.
func (a *LoadBalancerBackendTLSConfigAdapter) Update(ctx context.Context, c *lb.LoadBalancerBackendTLSConfig) error {
	if err := a.parent(ctx, c); err != nil {
		return err
	}
	bundleUUID, err := a.bundleUUID(ctx, c)
	if err != nil {
		return err
	}
	if _, err := a.API.GetLoadBalancerBackendTLSConfig(ctx, &request.GetLoadBalancerBackendTLSConfigRequest{
		ServiceUUID: c.Status.ServiceUUID, BackendName: c.Status.BackendName, Name: c.ExternalName(),
	}); upcloudapi.IsNotFound(err) {
		return reconciler.ErrPending
	} else if err != nil {
		return fmt.Errorf("get load balancer backend TLS config: %w", err)
	}
	_, err = a.API.ModifyLoadBalancerBackendTLSConfig(ctx, &request.ModifyLoadBalancerBackendTLSConfigRequest{
		ServiceUUID: c.Status.ServiceUUID,
		BackendName: c.Status.BackendName,
		Name:        c.ExternalName(),
		Config:      request.LoadBalancerBackendTLSConfig{Name: c.ExternalName(), CertificateBundleUUID: bundleUUID},
	})
	if err != nil {
		return fmt.Errorf("modify load balancer backend TLS config: %w", err)
	}
	return nil
}

// Delete implements reconciler.Adapter. A 404 counts as deleted.
func (a *LoadBalancerBackendTLSConfigAdapter) Delete(ctx context.Context, c *lb.LoadBalancerBackendTLSConfig) error {
	if c.Status.ServiceUUID == "" || c.Status.BackendName == "" || c.Status.Name == "" {
		return nil
	}
	err := a.API.DeleteLoadBalancerBackendTLSConfig(ctx, &request.DeleteLoadBalancerBackendTLSConfigRequest{
		ServiceUUID: c.Status.ServiceUUID, BackendName: c.Status.BackendName, Name: c.Status.Name,
	})
	switch {
	case err == nil, upcloudapi.IsNotFound(err):
		return nil
	case upcloudapi.IsConflict(err):
		return fmt.Errorf("%w: %s", reconciler.ErrPending, upcloudapi.TitleOf(err))
	default:
		return fmt.Errorf("delete load balancer backend TLS config: %w", err)
	}
}
