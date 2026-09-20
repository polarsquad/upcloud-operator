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

// GatewayConnectionAdapter maps GatewayConnection onto the UpCloud gateway
// connection API.
type GatewayConnectionAdapter struct {
	API    upcloudapi.GatewayAPI
	Client client.Client
}

var _ reconciler.Adapter[*networkv1alpha1.GatewayConnection] = (*GatewayConnectionAdapter)(nil)

// parentGatewayUUID resolves the parent gateway's UUID. In Observe the
// gateway may be mid-update, so the external id is used without requiring
// Ready; in Create the gateway must be Ready.
func (a *GatewayConnectionAdapter) parentGatewayUUID(ctx context.Context, c *networkv1alpha1.GatewayConnection, requireReady bool) (string, error) {
	if requireReady {
		return resolve.Ready(ctx, a.Client, c.Namespace, c.Spec.GatewayRef.Name, &networkv1alpha1.Gateway{})
	}
	return resolve.ExternalID(ctx, a.Client, c.Namespace, c.Spec.GatewayRef.Name, &networkv1alpha1.Gateway{})
}

func (a *GatewayConnectionAdapter) lookup(ctx context.Context, c *networkv1alpha1.GatewayConnection) (*upcloud.GatewayConnection, error) {
	if c.Status.UUID != "" {
		gw, err := a.parentGatewayUUID(ctx, c, false)
		if err != nil {
			return nil, err
		}
		conn, err := a.API.GetGatewayConnection(ctx, &request.GetGatewayConnectionRequest{ServiceUUID: gw, UUID: c.Status.UUID})
		if upcloudapi.IsNotFound(err) {
			return nil, nil
		}
		return conn, err
	}
	// Adoption: list the parent's connections and match by name.
	gw, err := a.parentGatewayUUID(ctx, c, false)
	if err != nil {
		return nil, err
	}
	conns, err := a.API.GetGatewayConnections(ctx, &request.GetGatewayConnectionsRequest{ServiceUUID: gw})
	if err != nil {
		return nil, err
	}
	for i := range conns {
		if conns[i].Name == c.ExternalName() {
			return &conns[i], nil
		}
	}
	return nil, nil
}

// Observe implements reconciler.Adapter.
func (a *GatewayConnectionAdapter) Observe(ctx context.Context, c *networkv1alpha1.GatewayConnection) (reconciler.Observation, error) {
	gw, err := a.parentGatewayUUID(ctx, c, false)
	if err != nil {
		return reconciler.Observation{}, err
	}
	c.Status.GatewayUUID = gw

	conn, err := a.lookup(ctx, c)
	if err != nil {
		return reconciler.Observation{}, fmt.Errorf("get gateway connection: %w", err)
	}
	if conn == nil {
		return reconciler.Observation{}, nil
	}

	c.Status.UUID = conn.UUID
	c.Status.Name = conn.Name
	upToDate := gatewayRoutesEqual(conn.LocalRoutes, toGatewayRoutes(c.Spec.LocalRoutes)) &&
		gatewayRoutesEqual(conn.RemoteRoutes, toGatewayRoutes(c.Spec.RemoteRoutes))
	return reconciler.Observation{Exists: true, UpToDate: upToDate, Ready: true}, nil
}

// Create implements reconciler.Adapter.
func (a *GatewayConnectionAdapter) Create(ctx context.Context, c *networkv1alpha1.GatewayConnection) error {
	gw, err := a.parentGatewayUUID(ctx, c, true)
	if err != nil {
		return err
	}
	created, err := a.API.CreateGatewayConnection(ctx, &request.CreateGatewayConnectionRequest{
		ServiceUUID: gw,
		Connection: request.GatewayConnection{
			Name:         c.ExternalName(),
			Type:         upcloud.GatewayConnectionType(c.Spec.Type),
			LocalRoutes:  toGatewayRoutes(c.Spec.LocalRoutes),
			RemoteRoutes: toGatewayRoutes(c.Spec.RemoteRoutes),
		},
	})
	if err != nil {
		return fmt.Errorf("create gateway connection: %w", err)
	}
	c.Status.GatewayUUID = gw
	c.Status.UUID = created.UUID
	c.Status.Name = created.Name
	return nil
}

// Update implements reconciler.Adapter. Only the two route sets are sent;
// tunnels are owned by GatewayTunnel CRs.
func (a *GatewayConnectionAdapter) Update(ctx context.Context, c *networkv1alpha1.GatewayConnection) error {
	gw, err := a.parentGatewayUUID(ctx, c, false)
	if err != nil {
		return err
	}
	_, err = a.API.ModifyGatewayConnection(ctx, &request.ModifyGatewayConnectionRequest{
		ServiceUUID: gw,
		UUID:        c.Status.UUID,
		Connection: request.ModifyGatewayConnection{
			LocalRoutes:  toGatewayRoutes(c.Spec.LocalRoutes),
			RemoteRoutes: toGatewayRoutes(c.Spec.RemoteRoutes),
		},
	})
	if err != nil {
		return fmt.Errorf("modify gateway connection: %w", err)
	}
	return nil
}

// Delete implements reconciler.Adapter.
func (a *GatewayConnectionAdapter) Delete(ctx context.Context, c *networkv1alpha1.GatewayConnection) error {
	if c.Status.UUID == "" || c.Status.GatewayUUID == "" {
		return nil
	}
	err := a.API.DeleteGatewayConnection(ctx, &request.DeleteGatewayConnectionRequest{
		ServiceUUID: c.Status.GatewayUUID,
		UUID:        c.Status.UUID,
	})
	switch {
	case err == nil, upcloudapi.IsNotFound(err):
		return nil
	case upcloudapi.IsConflict(err):
		return fmt.Errorf("%w: %v", reconciler.ErrPending, err)
	default:
		return fmt.Errorf("delete gateway connection: %w", err)
	}
}

// toGatewayRoutes converts spec routes to UpCloud routes.
func toGatewayRoutes(in []networkv1alpha1.GatewayRoute) []upcloud.GatewayRoute {
	out := make([]upcloud.GatewayRoute, 0, len(in))
	for _, r := range in {
		out = append(out, upcloud.GatewayRoute{
			Name:          r.Name,
			StaticNetwork: r.StaticNetwork,
			Type:          upcloud.GatewayRouteType(r.Type),
		})
	}
	return out
}

// routesEqual compares two route sets by name+staticNetwork.
func gatewayRoutesEqual(got []upcloud.GatewayRoute, want []upcloud.GatewayRoute) bool {
	if len(got) != len(want) {
		return false
	}
	gotSet := make(map[string]bool, len(got))
	for _, r := range got {
		gotSet[r.Name+"|"+r.StaticNetwork] = true
	}
	for _, w := range want {
		if !gotSet[w.Name+"|"+w.StaticNetwork] {
			return false
		}
	}
	return true
}
