package network

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"testing"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud"

	networkv1alpha1 "github.com/polarsquad/upcloud-operator/api/network/v1alpha1"
	"github.com/polarsquad/upcloud-operator/internal/reconciler"
	"github.com/polarsquad/upcloud-operator/internal/upcloudapi/fake"
)

// deleteFixture exercises a real adapter against the shared in-memory API.
// Each factory supplies a status identity but leaves the external target absent.
// Parent identities are cached in status; Delete must not resolve Kubernetes refs.
type deleteFixture struct {
	delete        func(context.Context) error
	clearIdentity func()
	seed          func()
	exists        func() bool
	calls         *[]string
	failNext      *error
	method        string
}

func checkDeleteCalls(t *testing.T, f deleteFixture, want ...string) {
	t.Helper()
	if !slices.Equal(*f.calls, want) {
		t.Fatalf("API calls = %v, want %v", *f.calls, want)
	}
}

func testDeleteContract(t *testing.T, newFixture func() deleteFixture) {
	t.Helper()
	t.Run("EmptyIdentityNoAPICalls", func(t *testing.T) {
		f := newFixture()
		f.seed()
		f.clearIdentity()
		wantErr := errors.New("API must not be called")
		*f.failNext = wantErr

		err := f.delete(t.Context())
		checkDeleteCalls(t, f)
		if err != nil {
			t.Fatalf("Delete without an identity = %v, want nil", err)
		}
		if *f.failNext != wantErr || !f.exists() {
			t.Fatal("Delete without an identity changed external state")
		}
	})

	t.Run("AlreadyAbsent", func(t *testing.T) {
		f := newFixture()
		if err := f.delete(t.Context()); err != nil {
			t.Fatalf("Delete absent target = %v, want nil", err)
		}
		checkDeleteCalls(t, f, f.method)
		if f.exists() {
			t.Fatal("absent target exists after Delete")
		}
	})

	t.Run("ConflictPending", func(t *testing.T) {
		f := newFixture()
		f.seed()
		// Exercise the vendor's HTTP 409 mapping without inventing dependency
		// relationships that this fake does not model for every kind.
		*f.failNext = fake.Conflict("delete blocked")
		err := f.delete(t.Context())
		if !errors.Is(err, reconciler.ErrPending) {
			t.Fatalf("Delete conflict = %v, want errors.Is(ErrPending)", err)
		}
		checkDeleteCalls(t, f, f.method)
		if !f.exists() {
			t.Fatal("conflicting Delete removed the target")
		}
		if *f.failNext != nil {
			t.Fatal("Delete did not consume the injected API conflict")
		}
		// Once the API stops rejecting deletion, a retry must remove the target.
		if err := f.delete(t.Context()); err != nil {
			t.Fatalf("Delete retry = %v, want nil", err)
		}
		checkDeleteCalls(t, f, f.method, f.method)
		if f.exists() {
			t.Fatal("successful Delete retry left the target present")
		}
	})

	t.Run("UnexpectedError", func(t *testing.T) {
		for _, tc := range []struct {
			name string
			err  error
		}{
			{name: "Transport", err: errors.New("connection lost")},
			{name: "Forbidden", err: &upcloud.Problem{Status: http.StatusForbidden, Title: "access denied"}},
			{name: "Server", err: &upcloud.Problem{Status: http.StatusInternalServerError, Title: "service failed"}},
		} {
			t.Run(tc.name, func(t *testing.T) {
				f := newFixture()
				f.seed()
				*f.failNext = tc.err
				err := f.delete(t.Context())
				if !errors.Is(err, tc.err) {
					t.Fatalf("Delete error = %v, want wrapped %v", err, tc.err)
				}
				if errors.Is(err, reconciler.ErrPending) {
					t.Fatalf("unexpected API error classified as pending: %v", err)
				}
				checkDeleteCalls(t, f, f.method)
				if !f.exists() {
					t.Fatal("failed Delete removed the target")
				}
			})
		}
	})

	t.Run("DeletesExistingThenAlreadyAbsent", func(t *testing.T) {
		f := newFixture()
		f.seed()
		if err := f.delete(t.Context()); err != nil {
			t.Fatalf("Delete existing target = %v, want nil", err)
		}
		if f.exists() {
			t.Fatal("successful Delete left the target present")
		}
		if err := f.delete(t.Context()); err != nil {
			t.Fatalf("repeated Delete = %v, want nil", err)
		}
		checkDeleteCalls(t, f, f.method, f.method)
	})
}

func TestRouterDeleteContract(t *testing.T) {
	testDeleteContract(t, func() deleteFixture {
		api := fake.NewNetworkAPI()
		a := &RouterAdapter{API: api}
		r := &networkv1alpha1.Router{}
		uuid := "delete-router"
		r.Status.UUID = uuid
		return deleteFixture{
			delete:        func(ctx context.Context) error { return a.Delete(ctx, r) },
			clearIdentity: func() { r.Status.UUID = "" },
			seed:          func() { api.Routers[uuid] = &upcloud.Router{UUID: uuid} },
			exists:        func() bool { return api.Routers[uuid] != nil },
			calls:         &api.Calls, failNext: &api.FailNext, method: "DeleteRouter",
		}
	})
}

func TestGatewayDeleteContract(t *testing.T) {
	testDeleteContract(t, func() deleteFixture {
		api := fake.NewGatewayAPI()
		a := &GatewayAdapter{API: api}
		g := &networkv1alpha1.Gateway{}
		g.Status.UUID = testGatewayUUID
		return deleteFixture{
			delete:        func(ctx context.Context) error { return a.Delete(ctx, g) },
			clearIdentity: func() { g.Status.UUID = "" },
			seed:          func() { api.Gateways[testGatewayUUID] = &upcloud.Gateway{UUID: testGatewayUUID} },
			exists:        func() bool { return api.Gateways[testGatewayUUID] != nil },
			calls:         &api.Calls, failNext: &api.FailNext, method: "DeleteGateway",
		}
	})
}

func TestGatewayConnectionDeleteContract(t *testing.T) {
	testDeleteContract(t, func() deleteFixture {
		api := fake.NewGatewayAPI()
		a := &GatewayConnectionAdapter{API: api}
		c := &networkv1alpha1.GatewayConnection{}
		c.Status.GatewayUUID = testGatewayUUID
		c.Status.UUID = testConnectionUUID
		gw := &upcloud.Gateway{UUID: testGatewayUUID}
		api.Gateways[testGatewayUUID] = gw
		return deleteFixture{
			delete:        func(ctx context.Context) error { return a.Delete(ctx, c) },
			clearIdentity: func() { c.Status.UUID = "" },
			seed:          func() { gw.Connections = []upcloud.GatewayConnection{{UUID: testConnectionUUID}} },
			exists:        func() bool { return len(gw.Connections) != 0 },
			calls:         &api.Calls, failNext: &api.FailNext, method: "DeleteGatewayConnection",
		}
	})
}

// A populated child UUID with a missing parent identity must not reach the
// API: the request coordinates would be incomplete.
func TestGatewayConnectionDeleteEmptyParentIdentity(t *testing.T) {
	api := fake.NewGatewayAPI()
	a := &GatewayConnectionAdapter{API: api}
	c := &networkv1alpha1.GatewayConnection{}
	c.Status.UUID = testConnectionUUID
	if err := a.Delete(context.Background(), c); err != nil {
		t.Fatalf("empty gateway identity must delete without API calls: %v", err)
	}
	if len(api.Calls) != 0 {
		t.Fatalf("API was called with incomplete coordinates: %v", api.Calls)
	}
}

func TestGatewayTunnelDeleteContract(t *testing.T) {
	testDeleteContract(t, func() deleteFixture {
		api := fake.NewGatewayAPI()
		a := &GatewayTunnelAdapter{API: api}
		tun := &networkv1alpha1.GatewayTunnel{}
		uuid := "delete-tunnel"
		tun.Status.GatewayUUID = testGatewayUUID
		tun.Status.ConnectionUUID = testConnectionUUID
		tun.Status.UUID = uuid
		gw := &upcloud.Gateway{
			UUID:        testGatewayUUID,
			Connections: []upcloud.GatewayConnection{{UUID: testConnectionUUID}},
		}
		api.Gateways[testGatewayUUID] = gw
		return deleteFixture{
			delete:        func(ctx context.Context) error { return a.Delete(ctx, tun) },
			clearIdentity: func() { tun.Status.UUID = "" },
			seed:          func() { gw.Connections[0].Tunnels = []upcloud.GatewayTunnel{{UUID: uuid}} },
			exists:        func() bool { return len(gw.Connections[0].Tunnels) != 0 },
			calls:         &api.Calls, failNext: &api.FailNext, method: "DeleteGatewayConnectionTunnel",
		}
	})
}

func TestGatewayTunnelDeleteEmptyParentIdentity(t *testing.T) {
	api := fake.NewGatewayAPI()
	a := &GatewayTunnelAdapter{API: api}
	for name, clear := range map[string]func(t *networkv1alpha1.GatewayTunnel){
		"gateway":    func(t *networkv1alpha1.GatewayTunnel) { t.Status.GatewayUUID = "" },
		"connection": func(t *networkv1alpha1.GatewayTunnel) { t.Status.ConnectionUUID = "" },
	} {
		tun := &networkv1alpha1.GatewayTunnel{}
		tun.Status.GatewayUUID = testGatewayUUID
		tun.Status.ConnectionUUID = testConnectionUUID
		tun.Status.UUID = "delete-tunnel"
		clear(tun)
		t.Run(name, func(t *testing.T) {
			if err := a.Delete(context.Background(), tun); err != nil {
				t.Fatalf("empty parent identity must delete without API calls: %v", err)
			}
			if len(api.Calls) != 0 {
				t.Fatalf("API was called with incomplete coordinates: %v", api.Calls)
			}
		})
	}
}

func TestNetworkPeeringDeleteContract(t *testing.T) {
	testDeleteContract(t, func() deleteFixture {
		api := fake.NewNetworkAPI()
		a := &NetworkPeeringAdapter{API: api}
		p := &networkv1alpha1.NetworkPeering{}
		uuid := "delete-peering"
		p.Status.UUID = uuid
		return deleteFixture{
			delete:        func(ctx context.Context) error { return a.Delete(ctx, p) },
			clearIdentity: func() { p.Status.UUID = "" },
			seed:          func() { api.Peerings[uuid] = &upcloud.NetworkPeering{UUID: uuid} },
			exists:        func() bool { return api.Peerings[uuid] != nil },
			calls:         &api.Calls, failNext: &api.FailNext, method: "DeleteNetworkPeering",
		}
	})
}

func TestFloatingIPDeleteContract(t *testing.T) {
	testDeleteContract(t, func() deleteFixture {
		api := fake.NewNetworkAPI()
		a := &FloatingIPAdapter{API: api}
		f := &networkv1alpha1.FloatingIP{}
		f.Status.Address = testRemoteAddr
		return deleteFixture{
			delete:        func(ctx context.Context) error { return a.Delete(ctx, f) },
			clearIdentity: func() { f.Status.Address = "" },
			seed:          func() { api.IPs[testRemoteAddr] = &upcloud.IPAddress{Address: testRemoteAddr} },
			exists:        func() bool { return api.IPs[testRemoteAddr] != nil },
			calls:         &api.Calls, failNext: &api.FailNext, method: "ReleaseIPAddress",
		}
	})
}

func TestNetworkDeleteContract(t *testing.T) {
	testDeleteContract(t, func() deleteFixture {
		api := fake.NewNetworkAPI()
		a := &NetworkAdapter{API: api}
		n := &networkv1alpha1.Network{}
		uuid := "delete-network"
		n.Status.UUID = uuid
		return deleteFixture{
			delete:        func(ctx context.Context) error { return a.Delete(ctx, n) },
			clearIdentity: func() { n.Status.UUID = "" },
			seed:          func() { api.Networks[uuid] = &upcloud.Network{UUID: uuid} },
			exists:        func() bool { return api.Networks[uuid] != nil },
			calls:         &api.Calls, failNext: &api.FailNext, method: "DeleteNetwork",
		}
	})
}
