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

// NetworkAdapter maps Network onto the UpCloud SDN network API.
type NetworkAdapter struct {
	API    upcloudapi.NetworkAPI
	Client client.Client
}

var _ reconciler.Adapter[*networkv1alpha1.Network] = (*NetworkAdapter)(nil)

// desiredRouterUUID resolves the router the network should be attached to:
// an explicit RouterUUID wins, then a Router CR via RouterRef.
func (a *NetworkAdapter) desiredRouterUUID(ctx context.Context, n *networkv1alpha1.Network) (string, error) {
	if n.Spec.RouterUUID != "" {
		return n.Spec.RouterUUID, nil
	}
	if n.Spec.RouterRef != nil {
		return resolve.Ready(ctx, a.Client, n.Namespace, n.Spec.RouterRef.Name, &networkv1alpha1.Router{})
	}
	return "", nil
}

func (a *NetworkAdapter) lookup(ctx context.Context, n *networkv1alpha1.Network) (*upcloud.Network, error) {
	if n.Status.UUID != "" {
		net, err := a.API.GetNetworkDetails(ctx, &request.GetNetworkDetailsRequest{UUID: n.Status.UUID})
		if upcloudapi.IsNotFound(err) {
			return nil, nil
		}
		return net, err
	}
	list, err := a.API.GetNetworks(ctx, upcloudapi.UIDFilter(n))
	if err != nil {
		return nil, err
	}
	for i := range list.Networks {
		if upcloudapi.HasUID(list.Networks[i].Labels, n.UID) {
			return &list.Networks[i], nil
		}
	}
	return nil, nil
}

// Observe implements reconciler.Adapter.
func (a *NetworkAdapter) Observe(ctx context.Context, n *networkv1alpha1.Network) (reconciler.Observation, error) {
	desiredRouter, err := a.desiredRouterUUID(ctx, n)
	if err != nil {
		return reconciler.Observation{}, err
	}
	net, err := a.lookup(ctx, n)
	if err != nil {
		return reconciler.Observation{}, fmt.Errorf("get network: %w", err)
	}
	if net == nil {
		return reconciler.Observation{}, nil
	}
	n.Status.UUID = net.UUID
	n.Status.RouterUUID = net.Router
	upToDate := net.Name == n.ExternalName() &&
		ipNetworksEqual(net.IPNetworks, toIPNetworks(n.Spec.IPNetworks)) &&
		upcloudapi.LabelsEqual(net.Labels, upcloudapi.DesiredLabels(n, n.Spec.Labels)) &&
		net.Router == desiredRouter
	return reconciler.Observation{Exists: true, UpToDate: upToDate, Ready: true}, nil
}

// Create implements reconciler.Adapter.
func (a *NetworkAdapter) Create(ctx context.Context, n *networkv1alpha1.Network) error {
	desiredRouter, err := a.desiredRouterUUID(ctx, n)
	if err != nil {
		return err
	}
	net, err := a.API.CreateNetwork(ctx, &request.CreateNetworkRequest{
		Name:       n.ExternalName(),
		Zone:       n.Spec.Zone,
		Router:     desiredRouter,
		IPNetworks: toIPNetworks(n.Spec.IPNetworks),
		Labels:     upcloudapi.DesiredLabels(n, n.Spec.Labels),
	})
	if err != nil {
		return fmt.Errorf("create network: %w", err)
	}
	n.Status.UUID = net.UUID
	n.Status.RouterUUID = net.Router
	return nil
}

// Update implements reconciler.Adapter. The network is modified first and the
// router attachment changes last, in that order.
func (a *NetworkAdapter) Update(ctx context.Context, n *networkv1alpha1.Network) error {
	desiredRouter, err := a.desiredRouterUUID(ctx, n)
	if err != nil {
		return err
	}
	net, err := a.lookup(ctx, n)
	if err != nil {
		return fmt.Errorf("get network: %w", err)
	}
	if net == nil {
		return fmt.Errorf("update network: %w", reconciler.ErrPending)
	}
	labels := upcloudapi.DesiredLabels(n, n.Spec.Labels)
	ipChanged := !ipNetworksEqual(net.IPNetworks, toIPNetworks(n.Spec.IPNetworks))
	if net.Name != n.ExternalName() || ipChanged || !upcloudapi.LabelsEqual(net.Labels, labels) {
		modify := &request.ModifyNetworkRequest{UUID: net.UUID, Name: n.ExternalName(), Labels: &labels}
		if ipChanged {
			modify.IPNetworks = toIPNetworks(n.Spec.IPNetworks)
		}
		if _, err := a.API.ModifyNetwork(ctx, modify); err != nil {
			return fmt.Errorf("modify network: %w", err)
		}
	}
	switch {
	case net.Router != desiredRouter && desiredRouter != "":
		if err := a.API.AttachNetworkRouter(ctx, &request.AttachNetworkRouterRequest{NetworkUUID: net.UUID, RouterUUID: desiredRouter}); err != nil {
			return fmt.Errorf("attach router: %w", err)
		}
	case net.Router != "" && desiredRouter == "":
		if err := a.API.DetachNetworkRouter(ctx, &request.DetachNetworkRouterRequest{NetworkUUID: net.UUID}); err != nil {
			return fmt.Errorf("detach router: %w", err)
		}
	}
	return nil
}

// Delete implements reconciler.Adapter.
func (a *NetworkAdapter) Delete(ctx context.Context, n *networkv1alpha1.Network) error {
	if n.Status.UUID == "" {
		return nil
	}
	err := a.API.DeleteNetwork(ctx, &request.DeleteNetworkRequest{UUID: n.Status.UUID})
	switch {
	case err == nil, upcloudapi.IsNotFound(err):
		return nil
	case upcloudapi.IsConflict(err):
		return fmt.Errorf("%w: %v", reconciler.ErrPending, err)
	default:
		return fmt.Errorf("delete network: %w", err)
	}
}
