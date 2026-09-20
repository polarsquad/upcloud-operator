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

// NetworkPeeringAdapter maps NetworkPeering onto the UpCloud network peering
// API. The local network is resolved from a Network CR (which must be Ready);
// the peer network is referenced by UpCloud UUID and may be in another
// account.
type NetworkPeeringAdapter struct {
	API    upcloudapi.NetworkAPI
	Client client.Client
}

var _ reconciler.Adapter[*networkv1alpha1.NetworkPeering] = (*NetworkPeeringAdapter)(nil)

// localNetworkUUID resolves the local Network CR to its UpCloud UUID,
// requiring it to be Ready.
func (a *NetworkPeeringAdapter) localNetworkUUID(ctx context.Context, p *networkv1alpha1.NetworkPeering) (string, error) {
	net := &networkv1alpha1.Network{}
	return resolve.Ready(ctx, a.Client, p.Namespace, p.Spec.NetworkRef.Name, net)
}

// lookup finds the peering by status UUID, or by the uid label across the
// list (peerings carry labels).
func (a *NetworkPeeringAdapter) lookup(ctx context.Context, p *networkv1alpha1.NetworkPeering) (*upcloud.NetworkPeering, error) {
	if p.Status.UUID != "" {
		got, err := a.API.GetNetworkPeering(ctx, &request.GetNetworkPeeringRequest{UUID: p.Status.UUID})
		if upcloudapi.IsNotFound(err) {
			return nil, nil
		}
		return got, err
	}
	list, err := a.API.GetNetworkPeerings(ctx, upcloudapi.UIDFilter(p))
	if err != nil {
		return nil, err
	}
	for i := range list {
		if upcloudapi.HasUID(list[i].Labels, p.UID) {
			return &list[i], nil
		}
	}
	return nil, nil
}

// Observe implements reconciler.Adapter.
func (a *NetworkPeeringAdapter) Observe(ctx context.Context, p *networkv1alpha1.NetworkPeering) (reconciler.Observation, error) {
	localUUID, err := a.localNetworkUUID(ctx, p)
	if err != nil {
		return reconciler.Observation{}, err
	}

	got, err := a.lookup(ctx, p)
	if err != nil {
		return reconciler.Observation{}, fmt.Errorf("get network peering: %w", err)
	}
	if got == nil {
		return reconciler.Observation{}, nil
	}
	p.Status.UUID = got.UUID
	p.Status.State = string(got.State)

	upToDate := got.Name == p.ExternalName() &&
		got.Network.UUID == localUUID &&
		got.PeerNetwork.UUID == p.Spec.PeerNetworkUUID &&
		string(got.ConfiguredStatus) == p.Spec.ConfiguredStatus &&
		upcloudapi.LabelsEqual(got.Labels, upcloudapi.DesiredLabels(p, p.Spec.Labels))
	// active is the steady state; everything else (pending-peer,
	// provisioning, conflict-subnet, missing routers) is transitional.
	ready := got.State == upcloud.NetworkPeeringStateActive
	message := string(got.State)
	return reconciler.Observation{Exists: true, UpToDate: upToDate, Ready: ready, Message: message}, nil
}

// Create implements reconciler.Adapter.
func (a *NetworkPeeringAdapter) Create(ctx context.Context, p *networkv1alpha1.NetworkPeering) error {
	localUUID, err := a.localNetworkUUID(ctx, p)
	if err != nil {
		return err
	}
	created, err := a.API.CreateNetworkPeering(ctx, &request.CreateNetworkPeeringRequest{
		Name:             p.ExternalName(),
		ConfiguredStatus: upcloud.NetworkPeeringConfiguredStatus(p.Spec.ConfiguredStatus),
		Network:          request.NetworkPeeringNetwork{UUID: localUUID},
		PeerNetwork:      request.NetworkPeeringNetwork{UUID: p.Spec.PeerNetworkUUID},
		Labels:           upcloudapi.DesiredLabels(p, p.Spec.Labels),
	})
	if err != nil {
		return fmt.Errorf("create network peering: %w", err)
	}
	p.Status.UUID = created.UUID
	p.Status.State = string(created.State)
	return nil
}

// Update implements reconciler.Adapter. Only the name, configured status and
// labels are mutable; the networks are fixed at create.
func (a *NetworkPeeringAdapter) Update(ctx context.Context, p *networkv1alpha1.NetworkPeering) error {
	labels := upcloudapi.DesiredLabels(p, p.Spec.Labels)
	_, err := a.API.ModifyNetworkPeering(ctx, &request.ModifyNetworkPeeringRequest{
		UUID: p.Status.UUID,
		NetworkPeering: request.ModifyNetworkPeering{
			Name:             p.ExternalName(),
			ConfiguredStatus: upcloud.NetworkPeeringConfiguredStatus(p.Spec.ConfiguredStatus),
			Labels:           &labels,
		},
	})
	if err != nil {
		return fmt.Errorf("modify network peering: %w", err)
	}
	return nil
}

// Delete implements reconciler.Adapter. A 404 counts as deleted.
func (a *NetworkPeeringAdapter) Delete(ctx context.Context, p *networkv1alpha1.NetworkPeering) error {
	if p.Status.UUID == "" {
		return nil
	}
	err := a.API.DeleteNetworkPeering(ctx, &request.DeleteNetworkPeeringRequest{UUID: p.Status.UUID})
	switch {
	case err == nil, upcloudapi.IsNotFound(err):
		return nil
	case upcloudapi.IsConflict(err):
		return fmt.Errorf("%w: %v", reconciler.ErrPending, err)
	default:
		return fmt.Errorf("delete network peering: %w", err)
	}
}
