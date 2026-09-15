// Package fake provides in-memory implementations of the upcloudapi interfaces.
package fake

import (
	"context"
	"fmt"
	"sync"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud"
	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud/request"

	"github.com/polarsquad/upcloud-operator/internal/upcloudapi"
)

var _ upcloudapi.GatewayAPI = (*GatewayAPI)(nil)

// GatewayAPI is an in-memory upcloudapi.GatewayAPI.
//
// Gateways are stored in Gateways keyed by UUID. Connections and tunnels are
// stored inside the owning Gateway struct (matching the real API, which
// nests them) and are also addressable by UUID.
type GatewayAPI struct {
	mu       sync.Mutex
	seq      int
	Gateways map[string]*upcloud.Gateway
	Calls    []string // method names in call order
	// StateOverride, when set, is the OperationalState reported for a created
	// gateway (to simulate transitional states such as setup-server).
	StateOverride upcloud.GatewayOperationalState
	// FailNext, when set, is returned by the next mutating call and cleared.
	FailNext error
}

// NewGatewayAPI returns an empty fake.
func NewGatewayAPI() *GatewayAPI {
	return &GatewayAPI{Gateways: map[string]*upcloud.Gateway{}}
}

func (f *GatewayAPI) record(name string) error {
	f.Calls = append(f.Calls, name)
	if f.FailNext != nil {
		err := f.FailNext
		f.FailNext = nil
		return err
	}
	return nil
}

func (f *GatewayAPI) nextUUID(prefix string) string {
	f.seq++
	return fmt.Sprintf("%s-%04d", prefix, f.seq)
}

// convertRouters maps request.GatewayRouter to upcloud.GatewayRouter.
func convertRouters(in []request.GatewayRouter) []upcloud.GatewayRouter {
	out := make([]upcloud.GatewayRouter, len(in))
	for i, r := range in {
		out[i] = upcloud.GatewayRouter{UUID: r.UUID}
	}
	return out
}

// convertConnections maps request.GatewayConnection (and its tunnels) to the
// response shapes, without UUIDs (assigned by the caller).
func convertConnections(in []request.GatewayConnection) []upcloud.GatewayConnection {
	out := make([]upcloud.GatewayConnection, len(in))
	for i, c := range in {
		tunnels := make([]upcloud.GatewayTunnel, len(c.Tunnels))
		for j, t := range c.Tunnels {
			tunnels[j] = upcloud.GatewayTunnel{
				Name:          t.Name,
				LocalAddress:  t.LocalAddress,
				RemoteAddress: t.RemoteAddress,
				IPSec:         t.IPSec,
			}
		}
		out[i] = upcloud.GatewayConnection{
			Name:         c.Name,
			Type:         c.Type,
			LocalRoutes:  c.LocalRoutes,
			RemoteRoutes: c.RemoteRoutes,
			Tunnels:      tunnels,
		}
	}
	return out
}

// findConnection locates a connection by UUID within a gateway.
func findConnection(g *upcloud.Gateway, uuid string) *upcloud.GatewayConnection {
	for i := range g.Connections {
		if g.Connections[i].UUID == uuid {
			return &g.Connections[i]
		}
	}
	return nil
}

func findTunnel(c *upcloud.GatewayConnection, uuid string) *upcloud.GatewayTunnel {
	for i := range c.Tunnels {
		if c.Tunnels[i].UUID == uuid {
			return &c.Tunnels[i]
		}
	}
	return nil
}

func (f *GatewayAPI) GetGateways(_ context.Context, filters ...request.QueryFilter) ([]upcloud.Gateway, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Calls = append(f.Calls, "GetGateways")
	var out []upcloud.Gateway
	for _, g := range f.Gateways {
		if hasAllLabels(g.Labels, filters) {
			cp := *g
			out = append(out, cp)
		}
	}
	return out, nil
}

func (f *GatewayAPI) GetGateway(_ context.Context, r *request.GetGatewayRequest) (*upcloud.Gateway, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Calls = append(f.Calls, "GetGateway")
	g, ok := f.Gateways[r.UUID]
	if !ok {
		return nil, NotFound("gateway")
	}
	cp := *g
	return &cp, nil
}

func (f *GatewayAPI) CreateGateway(_ context.Context, r *request.CreateGatewayRequest) (*upcloud.Gateway, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.record("CreateGateway"); err != nil {
		return nil, err
	}
	state := upcloud.GatewayOperationalStateRunning
	if f.StateOverride != "" {
		state = f.StateOverride
	}
	g := &upcloud.Gateway{
		UUID:             f.nextUUID("gw"),
		Name:             r.Name,
		Zone:             r.Zone,
		Plan:             r.Plan,
		Features:         r.Features,
		Routers:          convertRouters(r.Routers),
		Labels:           r.Labels,
		ConfiguredStatus: r.ConfiguredStatus,
		OperationalState: state,
		Addresses:        r.Addresses,
	}
	// Materialize any connections (and their tunnels) passed at create time.
	for _, rc := range convertConnections(r.Connections) {
		rc.UUID = f.nextUUID("conn")
		for ti := range rc.Tunnels {
			rc.Tunnels[ti].UUID = f.nextUUID("tun")
			rc.Tunnels[ti].OperationalState = upcloud.GatewayTunnelOperationalStateEstablished
		}
		g.Connections = append(g.Connections, rc)
	}
	f.Gateways[g.UUID] = g
	cp := *g
	return &cp, nil
}

func (f *GatewayAPI) ModifyGateway(_ context.Context, r *request.ModifyGatewayRequest) (*upcloud.Gateway, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.record("ModifyGateway"); err != nil {
		return nil, err
	}
	g, ok := f.Gateways[r.UUID]
	if !ok {
		return nil, NotFound("gateway")
	}
	if r.Name != "" {
		g.Name = r.Name
	}
	if r.Plan != "" {
		g.Plan = r.Plan
	}
	if r.ConfiguredStatus != "" {
		g.ConfiguredStatus = r.ConfiguredStatus
	}
	if r.Labels != nil {
		g.Labels = r.Labels
	}
	cp := *g
	return &cp, nil
}

func (f *GatewayAPI) DeleteGateway(_ context.Context, r *request.DeleteGatewayRequest) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.record("DeleteGateway"); err != nil {
		return err
	}
	g, ok := f.Gateways[r.UUID]
	if !ok {
		return NotFound("gateway")
	}
	// A gateway with connections cannot be deleted until they are removed.
	if len(g.Connections) > 0 {
		return Conflict("gateway has connections")
	}
	delete(f.Gateways, r.UUID)
	return nil
}

func (f *GatewayAPI) GetGatewayConnections(_ context.Context, r *request.GetGatewayConnectionsRequest) ([]upcloud.GatewayConnection, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Calls = append(f.Calls, "GetGatewayConnections")
	g, ok := f.Gateways[r.ServiceUUID]
	if !ok {
		return nil, NotFound("gateway")
	}
	out := make([]upcloud.GatewayConnection, len(g.Connections))
	copy(out, g.Connections)
	return out, nil
}

func (f *GatewayAPI) GetGatewayConnection(_ context.Context, r *request.GetGatewayConnectionRequest) (*upcloud.GatewayConnection, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Calls = append(f.Calls, "GetGatewayConnection")
	g, ok := f.Gateways[r.ServiceUUID]
	if !ok {
		return nil, NotFound("gateway")
	}
	c := findConnection(g, r.UUID)
	if c == nil {
		return nil, NotFound("gateway connection")
	}
	cp := *c
	return &cp, nil
}

func (f *GatewayAPI) CreateGatewayConnection(_ context.Context, r *request.CreateGatewayConnectionRequest) (*upcloud.GatewayConnection, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.record("CreateGatewayConnection"); err != nil {
		return nil, err
	}
	g, ok := f.Gateways[r.ServiceUUID]
	if !ok {
		return nil, NotFound("gateway")
	}
	c := upcloud.GatewayConnection{
		UUID:         f.nextUUID("conn"),
		Name:         r.Connection.Name,
		Type:         r.Connection.Type,
		LocalRoutes:  r.Connection.LocalRoutes,
		RemoteRoutes: r.Connection.RemoteRoutes,
		Tunnels:      make([]upcloud.GatewayTunnel, len(r.Connection.Tunnels)),
	}
	for j, t := range r.Connection.Tunnels {
		c.Tunnels[j] = upcloud.GatewayTunnel{
			UUID:             f.nextUUID("tun"),
			Name:             t.Name,
			LocalAddress:     t.LocalAddress,
			RemoteAddress:    t.RemoteAddress,
			IPSec:            t.IPSec,
			OperationalState: upcloud.GatewayTunnelOperationalStateEstablished,
		}
	}
	g.Connections = append(g.Connections, c)
	cp := c
	return &cp, nil
}

func (f *GatewayAPI) ModifyGatewayConnection(_ context.Context, r *request.ModifyGatewayConnectionRequest) (*upcloud.GatewayConnection, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.record("ModifyGatewayConnection"); err != nil {
		return nil, err
	}
	g, ok := f.Gateways[r.ServiceUUID]
	if !ok {
		return nil, NotFound("gateway")
	}
	c := findConnection(g, r.UUID)
	if c == nil {
		return nil, NotFound("gateway connection")
	}
	if r.Connection.LocalRoutes != nil {
		c.LocalRoutes = r.Connection.LocalRoutes
	}
	if r.Connection.RemoteRoutes != nil {
		c.RemoteRoutes = r.Connection.RemoteRoutes
	}
	cp := *c
	return &cp, nil
}

func (f *GatewayAPI) DeleteGatewayConnection(_ context.Context, r *request.DeleteGatewayConnectionRequest) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.record("DeleteGatewayConnection"); err != nil {
		return err
	}
	g, ok := f.Gateways[r.ServiceUUID]
	if !ok {
		return NotFound("gateway")
	}
	idx := -1
	for i := range g.Connections {
		if g.Connections[i].UUID == r.UUID {
			idx = i
			break
		}
	}
	if idx == -1 {
		return NotFound("gateway connection")
	}
	g.Connections = append(g.Connections[:idx], g.Connections[idx+1:]...)
	return nil
}

func (f *GatewayAPI) GetGatewayConnectionTunnels(_ context.Context, r *request.GetGatewayConnectionTunnelsRequest) ([]upcloud.GatewayTunnel, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Calls = append(f.Calls, "GetGatewayConnectionTunnels")
	g, ok := f.Gateways[r.ServiceUUID]
	if !ok {
		return nil, NotFound("gateway")
	}
	c := findConnection(g, r.ConnectionUUID)
	if c == nil {
		return nil, NotFound("gateway connection")
	}
	out := make([]upcloud.GatewayTunnel, len(c.Tunnels))
	copy(out, c.Tunnels)
	return out, nil
}

func (f *GatewayAPI) GetGatewayConnectionTunnel(_ context.Context, r *request.GetGatewayConnectionTunnelRequest) (*upcloud.GatewayTunnel, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Calls = append(f.Calls, "GetGatewayConnectionTunnel")
	g, ok := f.Gateways[r.ServiceUUID]
	if !ok {
		return nil, NotFound("gateway")
	}
	c := findConnection(g, r.ConnectionUUID)
	if c == nil {
		return nil, NotFound("gateway connection")
	}
	t := findTunnel(c, r.UUID)
	if t == nil {
		return nil, NotFound("gateway tunnel")
	}
	cp := *t
	return &cp, nil
}

func (f *GatewayAPI) CreateGatewayConnectionTunnel(_ context.Context, r *request.CreateGatewayConnectionTunnelRequest) (*upcloud.GatewayTunnel, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.record("CreateGatewayConnectionTunnel"); err != nil {
		return nil, err
	}
	g, ok := f.Gateways[r.ServiceUUID]
	if !ok {
		return nil, NotFound("gateway")
	}
	c := findConnection(g, r.ConnectionUUID)
	if c == nil {
		return nil, NotFound("gateway connection")
	}
	t := upcloud.GatewayTunnel{
		UUID:             f.nextUUID("tun"),
		Name:             r.Tunnel.Name,
		LocalAddress:     r.Tunnel.LocalAddress,
		RemoteAddress:    r.Tunnel.RemoteAddress,
		IPSec:            r.Tunnel.IPSec,
		OperationalState: upcloud.GatewayTunnelOperationalStateEstablished,
	}
	c.Tunnels = append(c.Tunnels, t)
	cp := t
	return &cp, nil
}

func (f *GatewayAPI) DeleteGatewayConnectionTunnel(_ context.Context, r *request.DeleteGatewayConnectionTunnelRequest) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.record("DeleteGatewayConnectionTunnel"); err != nil {
		return err
	}
	g, ok := f.Gateways[r.ServiceUUID]
	if !ok {
		return NotFound("gateway")
	}
	c := findConnection(g, r.ConnectionUUID)
	if c == nil {
		return NotFound("gateway connection")
	}
	idx := -1
	for i := range c.Tunnels {
		if c.Tunnels[i].UUID == r.UUID {
			idx = i
			break
		}
	}
	if idx == -1 {
		return NotFound("gateway tunnel")
	}
	c.Tunnels = append(c.Tunnels[:idx], c.Tunnels[idx+1:]...)
	return nil
}
