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
	Calls    []string // method names in call order
	// FailNext, when set, is returned by the next mutating call and cleared.
	FailNext error
}

// NewNetworkAPI returns an empty fake.
func NewNetworkAPI() *NetworkAPI {
	return &NetworkAPI{Networks: map[string]*upcloud.Network{}, Routers: map[string]*upcloud.Router{}}
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
