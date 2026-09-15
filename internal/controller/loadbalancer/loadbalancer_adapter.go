package loadbalancer

import (
	"context"
	"fmt"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud"
	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud/request"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/polarsquad/upcloud-operator/api/common"
	lb "github.com/polarsquad/upcloud-operator/api/loadbalancer/v1alpha1"
	"github.com/polarsquad/upcloud-operator/internal/reconciler"
	"github.com/polarsquad/upcloud-operator/internal/resolve"
	"github.com/polarsquad/upcloud-operator/internal/upcloudapi"
)

// LoadBalancerAdapter maps LoadBalancer onto the UpCloud Load Balancer API.
//
// Frontends, backends and resolvers are separate CRs and are never sent from
// this adapter. The Create request carries empty slices for them (the SDK
// fields are non-omitempty, so nil would marshal to their zero value rather
// than an empty list).
type LoadBalancerAdapter struct {
	API    upcloudapi.LoadBalancerAPI
	Client client.Client
}

var _ reconciler.Adapter[*lb.LoadBalancer] = (*LoadBalancerAdapter)(nil)

// lookup finds the service by status UUID, or by owner label across the list
// (the list endpoint has no label filter, so the filter is applied in Go).
func (a *LoadBalancerAdapter) lookup(ctx context.Context, s *lb.LoadBalancer) (*upcloud.LoadBalancer, error) {
	if s.Status.UUID != "" {
		lbw, err := a.API.GetLoadBalancer(ctx, &request.GetLoadBalancerRequest{UUID: s.Status.UUID})
		if upcloudapi.IsNotFound(err) {
			return nil, nil
		}
		return lbw, err
	}
	list, err := a.API.GetLoadBalancers(ctx, &request.GetLoadBalancersRequest{})
	if err != nil {
		return nil, err
	}
	for i := range list {
		if upcloudapi.HasUID(list[i].Labels, s.UID) {
			return &list[i], nil
		}
	}
	return nil, nil
}

// desiredNetworks resolves the spec network attachments, turning networkRef
// entries into the referenced Network CR's UUID.
func (a *LoadBalancerAdapter) desiredNetworks(ctx context.Context, s *lb.LoadBalancer) ([]request.LoadBalancerNetwork, error) {
	out := make([]request.LoadBalancerNetwork, 0, len(s.Spec.Networks))
	for _, att := range s.Spec.Networks {
		uuid, err := resolve.NetworkUUID(ctx, a.Client, s.Namespace, toCommonAtt(att))
		if err != nil {
			return nil, err
		}
		n := request.LoadBalancerNetwork{
			Name:   att.Name,
			Type:   upcloud.LoadBalancerNetworkType(att.Type),
			Family: upcloud.LoadBalancerAddressFamily(att.Family),
		}
		if uuid != "" {
			n.UUID = uuid
		}
		out = append(out, n)
	}
	return out, nil
}

func toCommonAtt(a lb.LoadBalancerNetworkAttachment) common.NetworkAttachment {
	return common.NetworkAttachment{Name: a.Name, Type: a.Type, Family: a.Family, UUID: a.UUID, NetworkRef: a.NetworkRef}
}

// Observe implements reconciler.Adapter.
func (a *LoadBalancerAdapter) Observe(ctx context.Context, s *lb.LoadBalancer) (reconciler.Observation, error) {
	lbw, err := a.lookup(ctx, s)
	if err != nil {
		return reconciler.Observation{}, fmt.Errorf("get load balancer: %w", err)
	}
	if lbw == nil {
		return reconciler.Observation{}, nil
	}
	s.Status.UUID = lbw.UUID
	a.syncStatus(s, lbw)

	networks, err := a.desiredNetworks(ctx, s)
	if err != nil {
		return reconciler.Observation{}, err
	}
	return reconciler.Observation{
		Exists:   true,
		UpToDate: specMatches(lbw, s, networks),
		Ready:    lbw.OperationalState == upcloud.LoadBalancerOperationalStateRunning,
		Message:  "state " + string(lbw.OperationalState),
	}, nil
}

// Create implements reconciler.Adapter.
func (a *LoadBalancerAdapter) Create(ctx context.Context, s *lb.LoadBalancer) error {
	networks, err := a.desiredNetworks(ctx, s)
	if err != nil {
		return err
	}
	req := &request.CreateLoadBalancerRequest{
		Name:             s.ExternalName(),
		Plan:             s.Spec.Plan,
		Zone:             s.Spec.Zone,
		Networks:         networks,
		ConfiguredStatus: upcloud.LoadBalancerConfiguredStatus(s.DesiredConfiguredStatus()),
		// Frontends, backends and resolvers are separate CRs; send explicit
		// empty slices (the SDK fields are non-omitempty).
		Frontends: []request.LoadBalancerFrontend{},
		Backends:  []request.LoadBalancerBackend{},
		Resolvers: []request.LoadBalancerResolver{},
		Labels:    upcloudapi.DesiredLabels(s, s.Spec.Labels),
	}
	if s.Spec.Maintenance.DOW != "" {
		req.MaintenanceDOW = upcloud.LoadBalancerMaintenanceDOW(s.Spec.Maintenance.DOW)
	}
	if s.Spec.Maintenance.Time != "" {
		req.MaintenanceTime = s.Spec.Maintenance.Time
	}
	created, err := a.API.CreateLoadBalancer(ctx, req)
	if err != nil {
		return fmt.Errorf("create load balancer: %w", err)
	}
	s.Status.UUID = created.UUID
	return nil
}

// Update implements reconciler.Adapter. Each drifting field is sent in a
// single modify call. Networks are immutable, so they are never sent.
func (a *LoadBalancerAdapter) Update(ctx context.Context, s *lb.LoadBalancer) error {
	lbw, err := a.lookup(ctx, s)
	if err != nil {
		return fmt.Errorf("get load balancer: %w", err)
	}
	if lbw == nil {
		return fmt.Errorf("update load balancer: %w", reconciler.ErrPending)
	}

	name := s.ExternalName()
	desired := s.DesiredConfiguredStatus()
	labels := upcloudapi.DesiredLabels(s, s.Spec.Labels)
	dow := s.Spec.Maintenance.DOW
	mtime := s.Spec.Maintenance.Time

	modify := &request.ModifyLoadBalancerRequest{UUID: lbw.UUID}
	changed := false
	if lbw.Name != name {
		modify.Name = name
		changed = true
	}
	if string(lbw.ConfiguredStatus) != desired {
		modify.ConfiguredStatus = desired
		changed = true
	}
	if !upcloudapi.LabelsEqual(lbw.Labels, labels) {
		modify.Labels = &labels
		changed = true
	}
	if string(lbw.MaintenanceDOW) != dow {
		modify.MaintenanceDOW = upcloud.LoadBalancerMaintenanceDOW(dow)
		changed = true
	}
	if lbw.MaintenanceTime != mtime {
		modify.MaintenanceTime = mtime
		changed = true
	}
	if !changed {
		return nil
	}
	if _, err := a.API.ModifyLoadBalancer(ctx, modify); err != nil {
		return fmt.Errorf("modify load balancer: %w", err)
	}
	return nil
}

// Delete implements reconciler.Adapter. A service with frontends, backends, or
// resolvers is refused (409) and reported as pending with the API title; a 404
// counts as deleted; a successful delete request is pending until it is gone.
func (a *LoadBalancerAdapter) Delete(ctx context.Context, s *lb.LoadBalancer) error {
	if s.Status.UUID == "" {
		return nil
	}
	lbw, err := a.API.GetLoadBalancer(ctx, &request.GetLoadBalancerRequest{UUID: s.Status.UUID})
	if upcloudapi.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("get load balancer: %w", err)
	}
	switch lbw.OperationalState {
	case upcloud.LoadBalancerOperationalStateDeleteDNS,
		upcloud.LoadBalancerOperationalStateDeleteNetwork,
		upcloud.LoadBalancerOperationalStateDeleteServer,
		upcloud.LoadBalancerOperationalStateDeleteService:
		return reconciler.ErrPending
	default:
	}
	err = a.API.DeleteLoadBalancer(ctx, &request.DeleteLoadBalancerRequest{UUID: s.Status.UUID})
	switch {
	case err == nil:
		return reconciler.ErrPending
	case upcloudapi.IsNotFound(err):
		return nil
	case upcloudapi.IsConflict(err):
		return fmt.Errorf("%w: %s", reconciler.ErrPending, upcloudapi.TitleOf(err))
	default:
		return fmt.Errorf("delete load balancer: %w", err)
	}
}

// syncStatus copies the observed state and networks into status.
func (a *LoadBalancerAdapter) syncStatus(s *lb.LoadBalancer, lbw *upcloud.LoadBalancer) {
	s.Status.OperationalState = string(lbw.OperationalState)
	networks := make([]lb.LBStatusNetwork, 0, len(lbw.Networks))
	for _, n := range lbw.Networks {
		status := lb.LBStatusNetwork{
			Name:    n.Name,
			Type:    string(n.Type),
			Family:  string(n.Family),
			DNSName: n.DNSName,
		}
		for _, ip := range n.IPAddresses {
			status.IPAddresses = append(status.IPAddresses, ip.Address)
		}
		networks = append(networks, status)
	}
	s.Status.Networks = networks
	s.Status.Nodes = len(lbw.Nodes)
}

// specMatches compares the mutable spec fields against the service.
func specMatches(lbw *upcloud.LoadBalancer, s *lb.LoadBalancer, networks []request.LoadBalancerNetwork) bool {
	if lbw.Name != s.ExternalName() {
		return false
	}
	if lbw.Plan != s.Spec.Plan {
		return false
	}
	if string(lbw.ConfiguredStatus) != s.DesiredConfiguredStatus() {
		return false
	}
	if !upcloudapi.LabelsEqual(lbw.Labels, upcloudapi.DesiredLabels(s, s.Spec.Labels)) {
		return false
	}
	if string(lbw.MaintenanceDOW) != s.Spec.Maintenance.DOW || lbw.MaintenanceTime != s.Spec.Maintenance.Time {
		return false
	}
	return lbNetworksEqual(lbw.Networks, networks)
}

func lbNetworkKey(n upcloud.LoadBalancerNetwork) string {
	return n.Name + "|" + string(n.Type) + "|" + string(n.Family) + "|" + n.UUID
}

// lbNetworksEqual compares the observed networks to the desired set. When the
// spec pinned a UUID the match is exact; otherwise (public networks get an
// API-generated UUID) the match is on name|type|family.
func lbNetworksEqual(observed []upcloud.LoadBalancerNetwork, desired []request.LoadBalancerNetwork) bool {
	if len(observed) != len(desired) {
		return false
	}
	base := make(map[string]bool, len(observed))
	full := make(map[string]bool, len(observed))
	for _, n := range observed {
		full[lbNetworkKey(n)] = true
		base[n.Name+"|"+string(n.Type)+"|"+string(n.Family)] = true
	}
	for _, n := range desired {
		if n.UUID != "" {
			if !full[n.Name+"|"+string(n.Type)+"|"+string(n.Family)+"|"+n.UUID] {
				return false
			}
			continue
		}
		if !base[n.Name+"|"+string(n.Type)+"|"+string(n.Family)] {
			return false
		}
	}
	return true
}
