package network

import (
	"context"
	"fmt"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud"
	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud/request"

	networkv1alpha1 "github.com/polarsquad/upcloud-operator/api/network/v1alpha1"
	"github.com/polarsquad/upcloud-operator/internal/reconciler"
	"github.com/polarsquad/upcloud-operator/internal/upcloudapi"
)

// RouterAdapter maps Router onto the UpCloud router API.
type RouterAdapter struct {
	API upcloudapi.NetworkAPI
}

var _ reconciler.Adapter[*networkv1alpha1.Router] = (*RouterAdapter)(nil)

func (a *RouterAdapter) lookup(ctx context.Context, r *networkv1alpha1.Router) (*upcloud.Router, error) {
	if r.Status.UUID != "" {
		rt, err := a.API.GetRouterDetails(ctx, &request.GetRouterDetailsRequest{UUID: r.Status.UUID})
		if upcloudapi.IsNotFound(err) {
			return nil, nil
		}
		return rt, err
	}
	list, err := a.API.GetRouters(ctx, upcloudapi.UIDFilter(r))
	if err != nil {
		return nil, err
	}
	for i := range list.Routers {
		if upcloudapi.HasUID(list.Routers[i].Labels, r.UID) {
			return &list.Routers[i], nil
		}
	}
	return nil, nil
}

// Observe implements reconciler.Adapter.
func (a *RouterAdapter) Observe(ctx context.Context, r *networkv1alpha1.Router) (reconciler.Observation, error) {
	rt, err := a.lookup(ctx, r)
	if err != nil {
		return reconciler.Observation{}, fmt.Errorf("get router: %w", err)
	}
	if rt == nil {
		return reconciler.Observation{}, nil
	}
	r.Status.UUID = rt.UUID
	r.Status.AttachedNetworks = r.Status.AttachedNetworks[:0]
	for _, n := range rt.AttachedNetworks {
		r.Status.AttachedNetworks = append(r.Status.AttachedNetworks, n.NetworkUUID)
	}
	upToDate := rt.Name == r.ExternalName() &&
		routesEqual(userRoutes(rt.StaticRoutes), toStaticRoutes(r.Spec.StaticRoutes)) &&
		upcloudapi.LabelsEqual(rt.Labels, upcloudapi.DesiredLabels(r, r.Spec.Labels))
	return reconciler.Observation{Exists: true, UpToDate: upToDate, Ready: true}, nil
}

// Create implements reconciler.Adapter.
func (a *RouterAdapter) Create(ctx context.Context, r *networkv1alpha1.Router) error {
	rt, err := a.API.CreateRouter(ctx, &request.CreateRouterRequest{
		Name:         r.ExternalName(),
		Labels:       upcloudapi.DesiredLabels(r, r.Spec.Labels),
		StaticRoutes: toStaticRoutes(r.Spec.StaticRoutes),
	})
	if err != nil {
		return fmt.Errorf("create router: %w", err)
	}
	r.Status.UUID = rt.UUID
	return nil
}

// Update implements reconciler.Adapter.
func (a *RouterAdapter) Update(ctx context.Context, r *networkv1alpha1.Router) error {
	labels := upcloudapi.DesiredLabels(r, r.Spec.Labels)
	routes := toStaticRoutes(r.Spec.StaticRoutes)
	_, err := a.API.ModifyRouter(ctx, &request.ModifyRouterRequest{
		UUID: r.Status.UUID, Name: r.ExternalName(), Labels: &labels, StaticRoutes: &routes,
	})
	if err != nil {
		return fmt.Errorf("modify router: %w", err)
	}
	return nil
}

// Delete implements reconciler.Adapter.
func (a *RouterAdapter) Delete(ctx context.Context, r *networkv1alpha1.Router) error {
	if r.Status.UUID == "" {
		return nil
	}
	err := a.API.DeleteRouter(ctx, &request.DeleteRouterRequest{UUID: r.Status.UUID})
	switch {
	case err == nil, upcloudapi.IsNotFound(err):
		return nil
	case upcloudapi.IsConflict(err):
		return fmt.Errorf("%w: %v", reconciler.ErrPending, err)
	default:
		return fmt.Errorf("delete router: %w", err)
	}
}
