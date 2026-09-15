package network

import (
	"context"
	"testing"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/polarsquad/upcloud-operator/api/common"
	networkv1alpha1 "github.com/polarsquad/upcloud-operator/api/network/v1alpha1"
	"github.com/polarsquad/upcloud-operator/internal/reconciler"
	"github.com/polarsquad/upcloud-operator/internal/upcloudapi/fake"
)

// putFakeTunnelChain seeds the fake API with a gateway + connection at known
// UUIDs so tunnel tests do not depend on sequence numbers.
func putFakeTunnelChain(t *testing.T, api *fake.GatewayAPI, gwUUID, connUUID string) {
	t.Helper()
	gw := &upcloud.Gateway{
		UUID: gwUUID, Name: testGWName, Zone: testZone,
		OperationalState: upcloud.GatewayOperationalStateRunning,
	}
	gw.Connections = []upcloud.GatewayConnection{{UUID: connUUID, Name: testConnName, Type: upcloud.GatewayConnectionTypeIPSec}}
	api.Gateways[gwUUID] = gw
}

func readyConnectionCR(name, uuid string) *networkv1alpha1.GatewayConnection {
	return &networkv1alpha1.GatewayConnection{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: testNS, UID: "conn-uid", Generation: 1},
		Status: networkv1alpha1.GatewayConnectionStatus{
			GatewayUUID: "gw-1",
			UUID:        uuid,
			Conditions: []metav1.Condition{{
				Type: testReadyType, Status: metav1.ConditionTrue, Reason: testReadyMsg,
				Message: "ok", ObservedGeneration: 1,
			}},
		},
	}
}

func pskSecret() *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "vpn-psk", Namespace: testNS},
		Data:       map[string][]byte{testPskKey: []byte("s3cr3t")},
	}
}

func newGatewayTunnel() *networkv1alpha1.GatewayTunnel {
	return &networkv1alpha1.GatewayTunnel{
		ObjectMeta: metav1.ObjectMeta{Name: "tun1", Namespace: testNS, UID: "tun-uid", Generation: 1},
		Spec: networkv1alpha1.GatewayTunnelSpec{
			ConnectionRef:    common.LocalObjectReference{Name: testConnName},
			LocalAddressName: testVPN,
			RemoteAddress:    "203.0.113.10",
			IPSec: &networkv1alpha1.GatewayTunnelIPSec{
				Authentication: networkv1alpha1.GatewayTunnelIPSecAuth{
					Type:         testPskKey,
					PskSecretRef: common.SecretKeySelector{Name: "vpn-psk", Key: testPskKey},
				},
			},
		},
	}
}

func TestGatewayTunnelCreateWithPSKFromSecret(t *testing.T) {
	g := NewWithT(t)
	api := fake.NewGatewayAPI()
	c := newFakeClient(t)
	a := &GatewayTunnelAdapter{API: api, Client: c}
	ctx := context.Background()

	g.Expect(c.Create(ctx, pskSecret())).To(Succeed())
	g.Expect(c.Create(ctx, readyConnectionCR("conn1", "conn-1"))).To(Succeed())
	putFakeTunnelChain(t, api, "gw-1", "conn-1")

	tun := newGatewayTunnel()
	g.Expect(a.Create(ctx, tun)).To(Succeed())
	g.Expect(tun.Status.GatewayUUID).To(Equal("gw-1"))
	g.Expect(tun.Status.ConnectionUUID).To(Equal("conn-1"))
	g.Expect(tun.Status.UUID).NotTo(BeEmpty())

	// The PSK from the Secret was sent to the API (present in the request).
	g.Expect(api.Calls).To(ContainElement("CreateGatewayConnectionTunnel"))
	// The fake never echoes the PSK back; the adapter does not store it.
	conn := api.Gateways["gw-1"].Connections[0]
	g.Expect(conn.Tunnels).To(HaveLen(1))
	g.Expect(conn.Tunnels[0].RemoteAddress.Address).To(Equal("203.0.113.10"))
}

func TestGatewayTunnelMissingSecretIsDependencyError(t *testing.T) {
	g := NewWithT(t)
	api := fake.NewGatewayAPI()
	c := newFakeClient(t)
	a := &GatewayTunnelAdapter{API: api, Client: c}
	ctx := context.Background()

	g.Expect(c.Create(ctx, readyConnectionCR("conn1", "conn-1"))).To(Succeed())
	putFakeTunnelChain(t, api, "gw-1", "conn-1")

	tun := newGatewayTunnel()
	// No "vpn-psk" secret: create must be a dependency error.
	g.Expect(a.Create(ctx, tun)).To(MatchError(reconciler.ErrDependencyNotReady))
}

func TestGatewayTunnelRemoteAddressDriftReplaces(t *testing.T) {
	g := NewWithT(t)
	api := fake.NewGatewayAPI()
	c := newFakeClient(t)
	a := &GatewayTunnelAdapter{API: api, Client: c}
	ctx := context.Background()

	g.Expect(c.Create(ctx, pskSecret())).To(Succeed())
	g.Expect(c.Create(ctx, readyConnectionCR("conn1", "conn-1"))).To(Succeed())
	putFakeTunnelChain(t, api, "gw-1", "conn-1")

	tun := newGatewayTunnel()
	g.Expect(a.Create(ctx, tun)).To(Succeed())
	g.Expect(obsTunnelUpToDate(a, ctx, tun)).To(BeTrue())

	// Remote address changed: drift, then update replaces (delete + create).
	tun.Spec.RemoteAddress = "203.0.113.99"
	g.Expect(obsTunnelUpToDate(a, ctx, tun)).To(BeFalse())
	g.Expect(a.Update(ctx, tun)).To(Succeed())

	// Delete must happen before re-create.
	delIdx, createIdx := -1, -1
	for i, call := range api.Calls {
		switch call {
		case "DeleteGatewayConnectionTunnel":
			delIdx = i
		case "CreateGatewayConnectionTunnel":
			createIdx = i
		}
	}
	g.Expect(delIdx).To(BeNumerically(">", 0))
	g.Expect(delIdx).To(BeNumerically("<", createIdx))

	// The tunnel exists again under the connection with the new address.
	conn := api.Gateways["gw-1"].Connections[0]
	g.Expect(conn.Tunnels).To(HaveLen(1))
	g.Expect(conn.Tunnels[0].RemoteAddress.Address).To(Equal("203.0.113.99"))
}

func TestGatewayTunnelDeleteIdempotent(t *testing.T) {
	g := NewWithT(t)
	api := fake.NewGatewayAPI()
	a := &GatewayTunnelAdapter{API: api, Client: newFakeClient(t)}
	ctx := context.Background()

	// No UUID yet: delete is a no-op.
	tun := newGatewayTunnel()
	g.Expect(a.Delete(ctx, tun)).To(Succeed())
}

func obsTunnelUpToDate(a *GatewayTunnelAdapter, ctx context.Context, t *networkv1alpha1.GatewayTunnel) bool {
	obs, err := a.Observe(ctx, t)
	if err != nil {
		panic(err)
	}
	return obs.UpToDate
}
