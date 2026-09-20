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

// GatewayTunnelAdapter maps GatewayTunnel onto the UpCloud gateway tunnel API.
// The SDK has no tunnel modify, so Update replaces: delete then create. A spec
// change therefore re-establishes the VPN. The PSK is read at create time only;
// the API never returns it, so spec.rotationToken is the operator-side marker
// that forces a PSK re-apply (a bump replaces the tunnel).
type GatewayTunnelAdapter struct {
	API    upcloudapi.GatewayAPI
	Client client.Client
}

var _ reconciler.Adapter[*networkv1alpha1.GatewayTunnel] = (*GatewayTunnelAdapter)(nil)

// resolveParents returns the parent gateway and connection UUIDs, requiring
// the connection to be Ready.
func (a *GatewayTunnelAdapter) resolveParents(ctx context.Context, t *networkv1alpha1.GatewayTunnel) (string, string, error) {
	conn := &networkv1alpha1.GatewayConnection{}
	connUUID, err := resolve.Ready(ctx, a.Client, t.Namespace, t.Spec.ConnectionRef.Name, conn)
	if err != nil {
		return "", "", err
	}
	gwUUID := conn.Status.GatewayUUID
	if gwUUID == "" {
		return "", "", fmt.Errorf("%w: GatewayConnection %q has no gateway uuid yet", reconciler.ErrDependencyNotReady, t.Spec.ConnectionRef.Name)
	}
	return gwUUID, connUUID, nil
}

// Observe implements reconciler.Adapter.
func (a *GatewayTunnelAdapter) Observe(ctx context.Context, t *networkv1alpha1.GatewayTunnel) (reconciler.Observation, error) {
	gwUUID, connUUID, err := a.resolveParents(ctx, t)
	if err != nil {
		return reconciler.Observation{}, err
	}
	t.Status.GatewayUUID = gwUUID
	t.Status.ConnectionUUID = connUUID

	if t.Status.UUID == "" {
		// No adoption possible: tunnels carry no labels. Absent.
		return reconciler.Observation{}, nil
	}
	tun, err := a.API.GetGatewayConnectionTunnel(ctx, &request.GetGatewayConnectionTunnelRequest{
		ServiceUUID: gwUUID, ConnectionUUID: connUUID, UUID: t.Status.UUID,
	})
	if upcloudapi.IsNotFound(err) {
		return reconciler.Observation{}, nil
	}
	if err != nil {
		return reconciler.Observation{}, fmt.Errorf("get gateway tunnel: %w", err)
	}

	t.Status.OperationalState = string(tun.OperationalState)
	// The SDK's established-state constants (both spellings) carry the string
	// "established", so compare against the literal to accept either.
	ready := string(tun.OperationalState) == "established"
	upToDate := tunnelMatches(tun, t.Spec)
	// The API never returns the PSK, so a token bump is the only signal that
	// the PSK in the Secret changed: force the delete+create path so the
	// current Secret value is re-sent.
	if upToDate && t.Status.RotationToken != t.Spec.RotationToken {
		upToDate = false
	}
	return reconciler.Observation{Exists: true, UpToDate: upToDate, Ready: ready}, nil
}

// Create implements reconciler.Adapter. The PSK is read from its Secret now;
// the API never returns it, so it cannot be recovered after create.
func (a *GatewayTunnelAdapter) Create(ctx context.Context, t *networkv1alpha1.GatewayTunnel) error {
	gwUUID, connUUID, err := a.resolveParents(ctx, t)
	if err != nil {
		return err
	}
	p, err := a.tunnelPayload(ctx, t)
	if err != nil {
		return err
	}
	created, err := a.API.CreateGatewayConnectionTunnel(ctx, &request.CreateGatewayConnectionTunnelRequest{
		ServiceUUID:    gwUUID,
		ConnectionUUID: connUUID,
		Tunnel:         p,
	})
	if err != nil {
		return fmt.Errorf("create gateway tunnel: %w", err)
	}
	t.Status.GatewayUUID = gwUUID
	t.Status.ConnectionUUID = connUUID
	t.Status.UUID = created.UUID
	t.Status.RotationToken = t.Spec.RotationToken
	return nil
}

// Update implements reconciler.Adapter. There is no tunnel modify in the API,
// so update deletes and recreates the tunnel (the VPN re-establishes).
func (a *GatewayTunnelAdapter) Update(ctx context.Context, t *networkv1alpha1.GatewayTunnel) error {
	if t.Status.UUID != "" {
		if err := a.API.DeleteGatewayConnectionTunnel(ctx, &request.DeleteGatewayConnectionTunnelRequest{
			ServiceUUID: t.Status.GatewayUUID, ConnectionUUID: t.Status.ConnectionUUID, UUID: t.Status.UUID,
		}); err != nil && !upcloudapi.IsNotFound(err) {
			return fmt.Errorf("delete gateway tunnel: %w", err)
		}
		t.Status.UUID = ""
	}
	return a.Create(ctx, t)
}

// Delete implements reconciler.Adapter.
func (a *GatewayTunnelAdapter) Delete(ctx context.Context, t *networkv1alpha1.GatewayTunnel) error {
	if t.Status.UUID == "" || t.Status.GatewayUUID == "" || t.Status.ConnectionUUID == "" {
		return nil
	}
	err := a.API.DeleteGatewayConnectionTunnel(ctx, &request.DeleteGatewayConnectionTunnelRequest{
		ServiceUUID: t.Status.GatewayUUID, ConnectionUUID: t.Status.ConnectionUUID, UUID: t.Status.UUID,
	})
	switch {
	case err == nil, upcloudapi.IsNotFound(err):
		return nil
	case upcloudapi.IsConflict(err):
		return fmt.Errorf("%w: %v", reconciler.ErrPending, err)
	default:
		return fmt.Errorf("delete gateway tunnel: %w", err)
	}
}

// tunnelPayload builds the request tunnel, reading the PSK from its Secret.
func (a *GatewayTunnelAdapter) tunnelPayload(ctx context.Context, t *networkv1alpha1.GatewayTunnel) (request.GatewayTunnel, error) {
	p := request.GatewayTunnel{
		Name:         t.ExternalName(),
		LocalAddress: upcloud.GatewayTunnelLocalAddress{Name: t.Spec.LocalAddressName},
		RemoteAddress: upcloud.GatewayTunnelRemoteAddress{
			Address: t.Spec.RemoteAddress,
		},
	}
	if t.Spec.IPSec != nil {
		i := t.Spec.IPSec
		psk, err := resolve.SecretKey(ctx, a.Client, t.Namespace, i.Authentication.PskSecretRef)
		if err != nil {
			return p, err
		}
		p.IPSec = upcloud.GatewayTunnelIPSec{
			Authentication: upcloud.GatewayTunnelIPSecAuth{
				Authentication: upcloud.GatewayIPSecAuthType(i.Authentication.Type),
				PSK:            psk,
			},
			RekeyTime:                 i.RekeyTime,
			ChildRekeyTime:            i.ChildRekeyTime,
			DPDDelay:                  i.DpdDelay,
			DPDTimeout:                i.DpdTimeout,
			IKELifetime:               i.IkeLifetime,
			Phase1Algorithms:          toIPSecAlgorithms(i.Phase1Algorithms),
			Phase1IntegrityAlgorithms: toIPSecIntegrityAlgorithms(i.Phase1IntegrityAlgorithms),
			Phase1DHGroupNumbers:      i.Phase1DhGroupNumbers,
			Phase2Algorithms:          toIPSecAlgorithms(i.Phase2Algorithms),
			Phase2IntegrityAlgorithms: toIPSecIntegrityAlgorithms(i.Phase2IntegrityAlgorithms),
			Phase2DHGroupNumbers:      i.Phase2DhGroupNumbers,
		}
	}
	return p, nil
}

// tunnelMatches compares the observed tunnel to the spec. The PSK is never
// echoed by the API, so it is not compared.
func tunnelMatches(tun *upcloud.GatewayTunnel, spec networkv1alpha1.GatewayTunnelSpec) bool {
	if tun.RemoteAddress.Address != spec.RemoteAddress {
		return false
	}
	if tun.LocalAddress.Name != spec.LocalAddressName {
		return false
	}
	if spec.IPSec != nil {
		i := spec.IPSec
		ips := tun.IPSec
		if ips.RekeyTime != i.RekeyTime || ips.ChildRekeyTime != i.ChildRekeyTime ||
			ips.DPDDelay != i.DpdDelay || ips.DPDTimeout != i.DpdTimeout || ips.IKELifetime != i.IkeLifetime {
			return false
		}
		if !stringSliceEqual(algoStrings(ips.Phase1Algorithms), i.Phase1Algorithms) {
			return false
		}
		if !stringSliceEqual(integrityStrings(ips.Phase1IntegrityAlgorithms), i.Phase1IntegrityAlgorithms) {
			return false
		}
		if !intSliceEqual(ips.Phase1DHGroupNumbers, i.Phase1DhGroupNumbers) {
			return false
		}
		if !stringSliceEqual(algoStrings(ips.Phase2Algorithms), i.Phase2Algorithms) {
			return false
		}
		if !stringSliceEqual(integrityStrings(ips.Phase2IntegrityAlgorithms), i.Phase2IntegrityAlgorithms) {
			return false
		}
		if !intSliceEqual(ips.Phase2DHGroupNumbers, i.Phase2DhGroupNumbers) {
			return false
		}
	}
	return true
}

func toIPSecAlgorithms(in []string) []upcloud.GatewayIPSecAlgorithm {
	out := make([]upcloud.GatewayIPSecAlgorithm, 0, len(in))
	for _, s := range in {
		out = append(out, upcloud.GatewayIPSecAlgorithm(s))
	}
	return out
}

func toIPSecIntegrityAlgorithms(in []string) []upcloud.GatewayIPSecIntegrityAlgorithm {
	out := make([]upcloud.GatewayIPSecIntegrityAlgorithm, 0, len(in))
	for _, s := range in {
		out = append(out, upcloud.GatewayIPSecIntegrityAlgorithm(s))
	}
	return out
}

func algoStrings(in []upcloud.GatewayIPSecAlgorithm) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		out = append(out, string(s))
	}
	return out
}

func integrityStrings(in []upcloud.GatewayIPSecIntegrityAlgorithm) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		out = append(out, string(s))
	}
	return out
}

func stringSliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func intSliceEqual(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
