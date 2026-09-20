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

// LoadBalancerResolverAdapter maps LoadBalancerResolver onto the UpCloud load
// balancer resolver API. The modify call sends the full resolver, so any
// changed field is a modify (not a recreate).
type LoadBalancerResolverAdapter struct {
	API    upcloudapi.LoadBalancerAPI
	Client client.Client
}

var _ reconciler.Adapter[*lb.LoadBalancerResolver] = (*LoadBalancerResolverAdapter)(nil)

// serviceUUID resolves the referenced LoadBalancer (which must be Ready) on
// first use and stores the result in Status.
func (a *LoadBalancerResolverAdapter) serviceUUID(ctx context.Context, r *lb.LoadBalancerResolver) (string, error) {
	if r.Status.ServiceUUID != "" {
		return r.Status.ServiceUUID, nil
	}
	uuid, err := resolve.Ready(ctx, a.Client, r.Namespace, r.Spec.LoadBalancerRef.Name, &lb.LoadBalancer{})
	if err != nil {
		return "", err
	}
	r.Status.ServiceUUID = uuid
	return uuid, nil
}

// Observe implements reconciler.Adapter.
func (a *LoadBalancerResolverAdapter) Observe(ctx context.Context, r *lb.LoadBalancerResolver) (reconciler.Observation, error) {
	svcUUID, err := a.serviceUUID(ctx, r)
	if err != nil {
		return reconciler.Observation{}, err
	}
	res, err := a.API.GetLoadBalancerResolver(ctx, &request.GetLoadBalancerResolverRequest{ServiceUUID: svcUUID, Name: r.ExternalName()})
	if upcloudapi.IsNotFound(err) {
		return reconciler.Observation{}, nil
	}
	if err != nil {
		return reconciler.Observation{}, fmt.Errorf("get load balancer resolver: %w", err)
	}
	r.Status.Name = res.Name
	return reconciler.Observation{
		Exists:   true,
		UpToDate: resolverMatches(*res, r),
		Ready:    true,
		Message:  "resolver " + res.Name,
	}, nil
}

// Create implements reconciler.Adapter. A Conflict (resolver already exists in
// UpCloud) is treated as adoption: the next Observe reconciles it.
func (a *LoadBalancerResolverAdapter) Create(ctx context.Context, r *lb.LoadBalancerResolver) error {
	svcUUID, err := a.serviceUUID(ctx, r)
	if err != nil {
		return err
	}
	created, err := a.API.CreateLoadBalancerResolver(ctx, &request.CreateLoadBalancerResolverRequest{
		ServiceUUID: svcUUID,
		Resolver: request.LoadBalancerResolver{
			Name:         r.ExternalName(),
			Nameservers:  r.Spec.Nameservers,
			Retries:      r.Spec.Retries,
			Timeout:      r.Spec.Timeout,
			TimeoutRetry: r.Spec.TimeoutRetry,
			CacheValid:   r.Spec.CacheValid,
			CacheInvalid: r.Spec.CacheInvalid,
		},
	})
	if err != nil {
		if upcloudapi.IsConflict(err) {
			return nil // adopted; the next Observe reconciles it
		}
		return fmt.Errorf("create load balancer resolver: %w", err)
	}
	r.Status.Name = created.Name
	return nil
}

// Update implements reconciler.Adapter. The full resolver is sent.
func (a *LoadBalancerResolverAdapter) Update(ctx context.Context, r *lb.LoadBalancerResolver) error {
	svcUUID, err := a.serviceUUID(ctx, r)
	if err != nil {
		return err
	}
	if _, err := a.API.GetLoadBalancerResolver(ctx, &request.GetLoadBalancerResolverRequest{ServiceUUID: svcUUID, Name: r.ExternalName()}); upcloudapi.IsNotFound(err) {
		return reconciler.ErrPending
	} else if err != nil {
		return fmt.Errorf("get load balancer resolver: %w", err)
	}
	if _, err := a.API.ModifyLoadBalancerResolver(ctx, &request.ModifyLoadBalancerResolverRequest{
		ServiceUUID: svcUUID,
		Name:        r.ExternalName(),
		Resolver: request.LoadBalancerResolver{
			Name:         r.ExternalName(),
			Nameservers:  r.Spec.Nameservers,
			Retries:      r.Spec.Retries,
			Timeout:      r.Spec.Timeout,
			TimeoutRetry: r.Spec.TimeoutRetry,
			CacheValid:   r.Spec.CacheValid,
			CacheInvalid: r.Spec.CacheInvalid,
		},
	}); err != nil {
		return fmt.Errorf("modify load balancer resolver: %w", err)
	}
	return nil
}

// Delete implements reconciler.Adapter. A 404 counts as deleted.
func (a *LoadBalancerResolverAdapter) Delete(ctx context.Context, r *lb.LoadBalancerResolver) error {
	if r.Status.ServiceUUID == "" || r.Status.Name == "" {
		return nil
	}
	err := a.API.DeleteLoadBalancerResolver(ctx, &request.DeleteLoadBalancerResolverRequest{
		ServiceUUID: r.Status.ServiceUUID,
		Name:        r.Status.Name,
	})
	switch {
	case err == nil, upcloudapi.IsNotFound(err):
		return nil
	case upcloudapi.IsConflict(err):
		return fmt.Errorf("%w: %s", reconciler.ErrPending, upcloudapi.TitleOf(err))
	default:
		return fmt.Errorf("delete load balancer resolver: %w", err)
	}
}

// resolverMatches compares the mutable spec fields against the resolver.
func resolverMatches(res upcloud.LoadBalancerResolver, r *lb.LoadBalancerResolver) bool {
	if len(res.Nameservers) != len(r.Spec.Nameservers) {
		return false
	}
	for i, ns := range r.Spec.Nameservers {
		if i >= len(res.Nameservers) || res.Nameservers[i] != ns {
			return false
		}
	}
	if res.Retries != r.Spec.Retries || res.Timeout != r.Spec.Timeout ||
		res.TimeoutRetry != r.Spec.TimeoutRetry || res.CacheValid != r.Spec.CacheValid ||
		res.CacheInvalid != r.Spec.CacheInvalid {
		return false
	}
	return true
}
