package network

import (
	"context"
	"fmt"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud"
	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud/request"
	"sigs.k8s.io/controller-runtime/pkg/client"

	networkv1alpha1 "github.com/polarsquad/upcloud-operator/api/network/v1alpha1"
	"github.com/polarsquad/upcloud-operator/internal/reconciler"
	"github.com/polarsquad/upcloud-operator/internal/resolve"
	"github.com/polarsquad/upcloud-operator/internal/upcloudapi"
)

// GatewayAdapter maps Gateway onto the UpCloud network gateway API.
type GatewayAdapter struct {
	API    upcloudapi.GatewayAPI
	Client client.Client
}

var _ reconciler.Adapter[*networkv1alpha1.Gateway] = (*GatewayAdapter)(nil)

// desiredRouterUUID resolves the router: explicit RouterUUID wins, then a
// Router CR via RouterRef.
func (a *GatewayAdapter) desiredRouterUUID(ctx context.Context, g *networkv1alpha1.Gateway) (string, error) {
	if g.Spec.RouterUUID != "" {
		return g.Spec.RouterUUID, nil
	}
	if g.Spec.RouterRef != nil {
		return resolve.Ready(ctx, a.Client, g.Namespace, g.Spec.RouterRef.Name, &networkv1alpha1.Router{})
	}
	return "", nil
}

func (a *GatewayAdapter) lookup(ctx context.Context, g *networkv1alpha1.Gateway) (*upcloud.Gateway, error) {
	if g.Status.UUID != "" {
		gw, err := a.API.GetGateway(ctx, &request.GetGatewayRequest{UUID: g.Status.UUID})
		if upcloudapi.IsNotFound(err) {
			return nil, nil
		}
		return gw, err
	}
	list, err := a.API.GetGateways(ctx, upcloudapi.UIDFilter(g))
	if err != nil {
		return nil, err
	}
	for i := range list {
		if upcloudapi.HasUID(list[i].Labels, g.UID) {
			return &list[i], nil
		}
	}
	return nil, nil
}

// Observe implements reconciler.Adapter.
func (a *GatewayAdapter) Observe(ctx context.Context, g *networkv1alpha1.Gateway) (reconciler.Observation, error) {
	gw, err := a.lookup(ctx, g)
	if err != nil {
		return reconciler.Observation{}, fmt.Errorf("get gateway: %w", err)
	}
	if gw == nil {
		return reconciler.Observation{}, nil
	}

	g.Status.UUID = gw.UUID
	g.Status.OperationalState = string(gw.OperationalState)
	g.Status.Addresses = g.Status.Addresses[:0]
	for _, ad := range gw.Addresses {
		g.Status.Addresses = append(g.Status.Addresses, networkv1alpha1.GatewayAddress{Name: ad.Name})
	}
	if len(gw.Routers) > 0 {
		g.Status.RouterUUID = gw.Routers[0].UUID
	}

	upToDate := gw.Name == g.ExternalName() &&
		gw.Plan == g.Spec.Plan &&
		gw.ConfiguredStatus == upcloud.GatewayConfiguredStatus(g.Spec.ConfiguredStatus) &&
		featuresEqual(gw.Features, g.Spec.Features) &&
		upcloudapi.LabelsEqual(gw.Labels, upcloudapi.DesiredLabels(g, g.Spec.Labels))

	ready := gw.OperationalState == upcloud.GatewayOperationalStateRunning ||
		gw.ConfiguredStatus == upcloud.GatewayConfiguredStatusStopped

	return reconciler.Observation{Exists: true, UpToDate: upToDate, Ready: ready}, nil
}

// Create implements reconciler.Adapter.
func (a *GatewayAdapter) Create(ctx context.Context, g *networkv1alpha1.Gateway) error {
	router, err := a.desiredRouterUUID(ctx, g)
	if err != nil {
		return err
	}
	routers := make([]request.GatewayRouter, 0, 1)
	if router != "" {
		routers = append(routers, request.GatewayRouter{UUID: router})
	}
	addresses := make([]upcloud.GatewayAddress, 0, len(g.Spec.Addresses))
	for _, ad := range g.Spec.Addresses {
		addresses = append(addresses, upcloud.GatewayAddress{Name: ad.Name})
	}
	features := make([]upcloud.GatewayFeature, 0, len(g.Spec.Features))
	for _, f := range g.Spec.Features {
		features = append(features, upcloud.GatewayFeature(f))
	}
	created, err := a.API.CreateGateway(ctx, &request.CreateGatewayRequest{
		Name:             g.ExternalName(),
		Zone:             g.Spec.Zone,
		Plan:             g.Spec.Plan,
		Features:         features,
		Routers:          routers,
		ConfiguredStatus: upcloud.GatewayConfiguredStatus(g.Spec.ConfiguredStatus),
		Addresses:        addresses,
		Labels:           upcloudapi.DesiredLabels(g, g.Spec.Labels),
	})
	if err != nil {
		return fmt.Errorf("create gateway: %w", err)
	}
	g.Status.UUID = created.UUID
	return nil
}

// Update implements reconciler.Adapter. Features, zone and routers are
// immutable, so a change to features is a hard error.
func (a *GatewayAdapter) Update(ctx context.Context, g *networkv1alpha1.Gateway) error {
	gw, err := a.lookup(ctx, g)
	if err != nil {
		return fmt.Errorf("get gateway: %w", err)
	}
	if gw == nil {
		return fmt.Errorf("update gateway: %w", reconciler.ErrPending)
	}
	if !featuresEqual(gw.Features, g.Spec.Features) {
		return fmt.Errorf("features are immutable; recreate the Gateway")
	}
	_, err = a.API.ModifyGateway(ctx, &request.ModifyGatewayRequest{
		UUID:             g.Status.UUID,
		Name:             g.ExternalName(),
		Plan:             g.Spec.Plan,
		ConfiguredStatus: upcloud.GatewayConfiguredStatus(g.Spec.ConfiguredStatus),
		Labels:           upcloudapi.DesiredLabels(g, g.Spec.Labels),
	})
	if err != nil {
		return fmt.Errorf("modify gateway: %w", err)
	}
	return nil
}

// Delete implements reconciler.Adapter.
func (a *GatewayAdapter) Delete(ctx context.Context, g *networkv1alpha1.Gateway) error {
	if g.Status.UUID == "" {
		return nil
	}
	err := a.API.DeleteGateway(ctx, &request.DeleteGatewayRequest{UUID: g.Status.UUID})
	switch {
	case err == nil, upcloudapi.IsNotFound(err):
		return nil
	case upcloudapi.IsConflict(err):
		return fmt.Errorf("%w: %v", reconciler.ErrPending, err)
	default:
		return fmt.Errorf("delete gateway: %w", err)
	}
}

// featuresEqual compares UpCloud features to spec features.
func featuresEqual(got []upcloud.GatewayFeature, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	gotSet := make(map[upcloud.GatewayFeature]bool, len(got))
	for _, f := range got {
		gotSet[f] = true
	}
	for _, w := range want {
		if !gotSet[upcloud.GatewayFeature(w)] {
			return false
		}
	}
	return true
}
