// Package fake provides in-memory implementations of the upcloudapi interfaces.
package fake

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud"
	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud/request"

	"github.com/polarsquad/upcloud-operator/internal/upcloudapi"
)

var _ upcloudapi.NetworkAPI = (*NetworkAPI)(nil)

// NotFound is the error the real API returns for unknown identifiers.
func NotFound(what string) error {
	return &upcloud.Problem{Status: http.StatusNotFound, Title: what + " not found"}
}

// Conflict mimics a 409 from the API.
func Conflict(what string) error {
	return &upcloud.Problem{Status: http.StatusConflict, Title: what + " already exists"}
}

// Invalid mimics a 400 validation error from the API.
func Invalid(msg string) error {
	return &upcloud.Problem{Status: http.StatusBadRequest, Title: msg}
}

func hasAllLabels(labels []upcloud.Label, filters []request.QueryFilter) bool {
	for _, f := range filters {
		fl, ok := f.(request.FilterLabel)
		if !ok {
			continue
		}
		found := false
		for _, l := range labels {
			if l.Key == fl.Key && l.Value == fl.Value {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// NetworkAPI is an in-memory upcloudapi.NetworkAPI.
type NetworkAPI struct {
	mu       sync.Mutex
	seq      int
	Networks map[string]*upcloud.Network
	Routers  map[string]*upcloud.Router
	IPs      map[string]*upcloud.IPAddress // keyed by address
	Peerings map[string]*upcloud.NetworkPeering
	Calls    []string // method names in call order
	// FailNext, when set, is returned by the next mutating call and cleared.
	FailNext error
}

// NewNetworkAPI returns an empty fake.
func NewNetworkAPI() *NetworkAPI {
	return &NetworkAPI{
		Networks: map[string]*upcloud.Network{},
		Routers:  map[string]*upcloud.Router{},
		IPs:      map[string]*upcloud.IPAddress{},
		Peerings: map[string]*upcloud.NetworkPeering{},
	}
}

func (f *NetworkAPI) record(name string) error {
	f.Calls = append(f.Calls, name)
	if f.FailNext != nil {
		err := f.FailNext
		f.FailNext = nil
		return err
	}
	return nil
}

func (f *NetworkAPI) nextUUID(prefix string) string {
	f.seq++
	return fmt.Sprintf("%s-%04d", prefix, f.seq)
}

func (f *NetworkAPI) GetNetworks(_ context.Context, filters ...request.QueryFilter) (*upcloud.Networks, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Calls = append(f.Calls, "GetNetworks")
	out := &upcloud.Networks{}
	for _, n := range f.Networks {
		if hasAllLabels(n.Labels, filters) {
			out.Networks = append(out.Networks, *n)
		}
	}
	return out, nil
}

func (f *NetworkAPI) GetNetworkDetails(_ context.Context, r *request.GetNetworkDetailsRequest) (*upcloud.Network, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Calls = append(f.Calls, "GetNetworkDetails")
	n, ok := f.Networks[r.UUID]
	if !ok {
		return nil, NotFound("network")
	}
	cp := *n
	return &cp, nil
}

func (f *NetworkAPI) CreateNetwork(_ context.Context, r *request.CreateNetworkRequest) (*upcloud.Network, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.record("CreateNetwork"); err != nil {
		return nil, err
	}
	n := &upcloud.Network{
		UUID: f.nextUUID("net"), Name: r.Name, Zone: r.Zone, Type: upcloud.NetworkTypePrivate,
		Router: r.Router, IPNetworks: r.IPNetworks, Labels: r.Labels,
	}
	f.Networks[n.UUID] = n
	cp := *n
	return &cp, nil
}

func (f *NetworkAPI) ModifyNetwork(_ context.Context, r *request.ModifyNetworkRequest) (*upcloud.Network, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.record("ModifyNetwork"); err != nil {
		return nil, err
	}
	n, ok := f.Networks[r.UUID]
	if !ok {
		return nil, NotFound("network")
	}
	if r.Name != "" {
		n.Name = r.Name
	}
	if r.IPNetworks != nil {
		n.IPNetworks = r.IPNetworks
	}
	if r.Labels != nil {
		n.Labels = *r.Labels
	}
	cp := *n
	return &cp, nil
}

func (f *NetworkAPI) DeleteNetwork(_ context.Context, r *request.DeleteNetworkRequest) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.record("DeleteNetwork"); err != nil {
		return err
	}
	if _, ok := f.Networks[r.UUID]; !ok {
		return NotFound("network")
	}
	delete(f.Networks, r.UUID)
	return nil
}

func (f *NetworkAPI) AttachNetworkRouter(_ context.Context, r *request.AttachNetworkRouterRequest) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.record("AttachNetworkRouter"); err != nil {
		return err
	}
	n, ok := f.Networks[r.NetworkUUID]
	if !ok {
		return NotFound("network")
	}
	n.Router = r.RouterUUID
	return nil
}

func (f *NetworkAPI) DetachNetworkRouter(_ context.Context, r *request.DetachNetworkRouterRequest) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.record("DetachNetworkRouter"); err != nil {
		return err
	}
	n, ok := f.Networks[r.NetworkUUID]
	if !ok {
		return NotFound("network")
	}
	n.Router = ""
	return nil
}

func (f *NetworkAPI) GetRouters(_ context.Context, filters ...request.QueryFilter) (*upcloud.Routers, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Calls = append(f.Calls, "GetRouters")
	out := &upcloud.Routers{}
	for _, r := range f.Routers {
		if hasAllLabels(r.Labels, filters) {
			out.Routers = append(out.Routers, *r)
		}
	}
	return out, nil
}

func (f *NetworkAPI) GetRouterDetails(_ context.Context, r *request.GetRouterDetailsRequest) (*upcloud.Router, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Calls = append(f.Calls, "GetRouterDetails")
	rt, ok := f.Routers[r.UUID]
	if !ok {
		return nil, NotFound("router")
	}
	cp := *rt
	return &cp, nil
}

func (f *NetworkAPI) CreateRouter(_ context.Context, r *request.CreateRouterRequest) (*upcloud.Router, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.record("CreateRouter"); err != nil {
		return nil, err
	}
	rt := &upcloud.Router{UUID: f.nextUUID("rtr"), Name: r.Name, Type: "normal", Labels: r.Labels, StaticRoutes: r.StaticRoutes}
	f.Routers[rt.UUID] = rt
	cp := *rt
	return &cp, nil
}

func (f *NetworkAPI) ModifyRouter(_ context.Context, r *request.ModifyRouterRequest) (*upcloud.Router, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.record("ModifyRouter"); err != nil {
		return nil, err
	}
	rt, ok := f.Routers[r.UUID]
	if !ok {
		return nil, NotFound("router")
	}
	rt.Name = r.Name
	if r.Labels != nil {
		rt.Labels = *r.Labels
	}
	if r.StaticRoutes != nil {
		rt.StaticRoutes = *r.StaticRoutes
	}
	cp := *rt
	return &cp, nil
}

func (f *NetworkAPI) DeleteRouter(_ context.Context, r *request.DeleteRouterRequest) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.record("DeleteRouter"); err != nil {
		return err
	}
	if _, ok := f.Routers[r.UUID]; !ok {
		return NotFound("router")
	}
	for _, n := range f.Networks {
		if n.Router == r.UUID {
			return &upcloud.Problem{Status: http.StatusConflict, Title: "router has attached networks"}
		}
	}
	delete(f.Routers, r.UUID)
	return nil
}

func (f *NetworkAPI) GetIPAddresses(_ context.Context) (*upcloud.IPAddresses, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Calls = append(f.Calls, "GetIPAddresses")
	out := &upcloud.IPAddresses{}
	for _, ip := range f.IPs {
		out.IPAddresses = append(out.IPAddresses, *ip)
	}
	return out, nil
}

func (f *NetworkAPI) GetIPAddressDetails(_ context.Context, r *request.GetIPAddressDetailsRequest) (*upcloud.IPAddress, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Calls = append(f.Calls, "GetIPAddressDetails")
	ip, ok := f.IPs[r.Address]
	if !ok {
		return nil, NotFound("ip address")
	}
	cp := *ip
	return &cp, nil
}

func (f *NetworkAPI) AssignIPAddress(_ context.Context, r *request.AssignIPAddressRequest) (*upcloud.IPAddress, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.record("AssignIPAddress"); err != nil {
		return nil, err
	}
	var address string
	if r.Family == "IPv6" {
		f.seq++
		address = fmt.Sprintf("2001:db8:%x::%x", f.seq, f.seq)
	} else {
		f.seq++
		address = fmt.Sprintf("203.0.113.%d", 10+f.seq)
	}
	if _, exists := f.IPs[address]; exists {
		return nil, Conflict("ip address")
	}
	ip := &upcloud.IPAddress{
		Address:       address,
		Family:        r.Family,
		Access:        r.Access,
		Zone:          r.Zone,
		ReleasePolicy: r.ReleasePolicy,
		PTRRecord:     "",
		Floating:      upcloud.True,
	}
	f.IPs[address] = ip
	cp := *ip
	return &cp, nil
}

func (f *NetworkAPI) ModifyIPAddress(_ context.Context, r *request.ModifyIPAddressRequest) (*upcloud.IPAddress, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.record("ModifyIPAddress"); err != nil {
		return nil, err
	}
	ip, ok := f.IPs[r.IPAddress]
	if !ok {
		return nil, NotFound("ip address")
	}
	if r.PTRRecord != "" {
		ip.PTRRecord = r.PTRRecord
	}
	if r.ReleasePolicy != "" {
		ip.ReleasePolicy = r.ReleasePolicy
	}
	cp := *ip
	return &cp, nil
}

func (f *NetworkAPI) ReleaseIPAddress(_ context.Context, r *request.ReleaseIPAddressRequest) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.record("ReleaseIPAddress"); err != nil {
		return err
	}
	if _, ok := f.IPs[r.IPAddress]; !ok {
		return NotFound("ip address")
	}
	delete(f.IPs, r.IPAddress)
	return nil
}

func (f *NetworkAPI) GetNetworkPeerings(_ context.Context, filters ...request.QueryFilter) (upcloud.NetworkPeerings, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Calls = append(f.Calls, "GetNetworkPeerings")
	var out upcloud.NetworkPeerings
	for _, p := range f.Peerings {
		if hasAllLabels(p.Labels, filters) {
			out = append(out, *p)
		}
	}
	return out, nil
}

func (f *NetworkAPI) GetNetworkPeering(_ context.Context, r *request.GetNetworkPeeringRequest) (*upcloud.NetworkPeering, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Calls = append(f.Calls, "GetNetworkPeering")
	p, ok := f.Peerings[r.UUID]
	if !ok {
		return nil, NotFound("network peering")
	}
	cp := *p
	return &cp, nil
}

func (f *NetworkAPI) CreateNetworkPeering(_ context.Context, r *request.CreateNetworkPeeringRequest) (*upcloud.NetworkPeering, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.record("CreateNetworkPeering"); err != nil {
		return nil, err
	}
	if r.Network.UUID != "" {
		if _, ok := f.Networks[r.Network.UUID]; !ok {
			return nil, NotFound("network")
		}
	}
	p := &upcloud.NetworkPeering{
		UUID: f.nextUUID("np"),
		Name: r.Name,
		Network: upcloud.NetworkPeeringNetwork{
			UUID:       r.Network.UUID,
			IPNetworks: []upcloud.NetworkPeeringIPNetwork{},
		},
		PeerNetwork:      upcloud.NetworkPeeringNetwork{UUID: r.PeerNetwork.UUID},
		ConfiguredStatus: r.ConfiguredStatus,
		State:            upcloud.NetworkPeeringStateActive,
		Labels:           r.Labels,
	}
	f.Peerings[p.UUID] = p
	cp := *p
	return &cp, nil
}

func (f *NetworkAPI) ModifyNetworkPeering(_ context.Context, r *request.ModifyNetworkPeeringRequest) (*upcloud.NetworkPeering, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.record("ModifyNetworkPeering"); err != nil {
		return nil, err
	}
	p, ok := f.Peerings[r.UUID]
	if !ok {
		return nil, NotFound("network peering")
	}
	if r.NetworkPeering.Name != "" {
		p.Name = r.NetworkPeering.Name
	}
	if r.NetworkPeering.ConfiguredStatus != "" {
		p.ConfiguredStatus = r.NetworkPeering.ConfiguredStatus
	}
	if r.NetworkPeering.Labels != nil {
		p.Labels = *r.NetworkPeering.Labels
	}
	cp := *p
	return &cp, nil
}

func (f *NetworkAPI) DeleteNetworkPeering(_ context.Context, r *request.DeleteNetworkPeeringRequest) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.record("DeleteNetworkPeering"); err != nil {
		return err
	}
	if _, ok := f.Peerings[r.UUID]; !ok {
		return NotFound("network peering")
	}
	delete(f.Peerings, r.UUID)
	return nil
}
