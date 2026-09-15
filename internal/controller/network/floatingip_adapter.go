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

// FloatingIPAdapter maps FloatingIP onto the UpCloud IP address API. The
// address is account-level and unbound (no server); the IP string is its
// identity. Floating IPs carry no labels, so there is no adoption: if
// status.address is empty and no address is pinned, the operator cannot tell
// which of the account's addresses belongs to this CR.
type FloatingIPAdapter struct {
	API upcloudapi.NetworkAPI
}

var _ reconciler.Adapter[*networkv1alpha1.FloatingIP] = (*FloatingIPAdapter)(nil)

// lookup returns the address by its status identity.
func (a *FloatingIPAdapter) lookup(ctx context.Context, f *networkv1alpha1.FloatingIP) (*upcloud.IPAddress, error) {
	if f.Status.Address == "" {
		return nil, nil
	}
	ip, err := a.API.GetIPAddressDetails(ctx, &request.GetIPAddressDetailsRequest{Address: f.Status.Address})
	if upcloudapi.IsNotFound(err) {
		return nil, nil
	}
	return ip, err
}

// Observe implements reconciler.Adapter.
func (a *FloatingIPAdapter) Observe(ctx context.Context, f *networkv1alpha1.FloatingIP) (reconciler.Observation, error) {
	ip, err := a.lookup(ctx, f)
	if err != nil {
		return reconciler.Observation{}, fmt.Errorf("get ip address: %w", err)
	}
	if ip == nil {
		return reconciler.Observation{}, nil
	}
	f.Status.Address = ip.Address
	f.Status.MAC = ip.MAC
	f.Status.PartOfPlan = ip.PartOfPlan == upcloud.True
	upToDate := ip.Zone == f.Spec.Zone &&
		ip.Family == f.Spec.Family &&
		ip.Access == f.Spec.Access &&
		string(ip.ReleasePolicy) == f.Spec.ReleasePolicy &&
		ip.PTRRecord == f.Spec.PTRRecord
	return reconciler.Observation{Exists: true, UpToDate: upToDate, Ready: true}, nil
}

// Create implements reconciler.Adapter. The IP is allocated in the spec zone;
// the allocated address is written to status.
func (a *FloatingIPAdapter) Create(ctx context.Context, f *networkv1alpha1.FloatingIP) error {
	ip, err := a.API.AssignIPAddress(ctx, &request.AssignIPAddressRequest{
		Zone:          f.Spec.Zone,
		Family:        f.Spec.Family,
		Access:        f.Spec.Access,
		ReleasePolicy: upcloud.IPAddressReleasePolicy(f.Spec.ReleasePolicy),
		Floating:      upcloud.True,
	})
	if err != nil {
		return fmt.Errorf("assign ip address: %w", err)
	}
	f.Status.Address = ip.Address
	f.Status.MAC = ip.MAC
	return nil
}

// Update implements reconciler.Adapter.
func (a *FloatingIPAdapter) Update(ctx context.Context, f *networkv1alpha1.FloatingIP) error {
	_, err := a.API.ModifyIPAddress(ctx, &request.ModifyIPAddressRequest{
		IPAddress:     f.Status.Address,
		PTRRecord:     f.Spec.PTRRecord,
		ReleasePolicy: upcloud.IPAddressReleasePolicy(f.Spec.ReleasePolicy),
	})
	if err != nil {
		return fmt.Errorf("modify ip address: %w", err)
	}
	return nil
}

// Delete implements reconciler.Adapter. A 404 counts as released.
func (a *FloatingIPAdapter) Delete(ctx context.Context, f *networkv1alpha1.FloatingIP) error {
	if f.Status.Address == "" {
		return nil
	}
	err := a.API.ReleaseIPAddress(ctx, &request.ReleaseIPAddressRequest{IPAddress: f.Status.Address})
	switch {
	case err == nil, upcloudapi.IsNotFound(err):
		return nil
	case upcloudapi.IsConflict(err):
		return fmt.Errorf("%w: %v", reconciler.ErrPending, err)
	default:
		return fmt.Errorf("release ip address: %w", err)
	}
}
