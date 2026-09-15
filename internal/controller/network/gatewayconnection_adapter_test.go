package network

import (
	"context"
	"testing"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud"
	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud/request"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/polarsquad/upcloud-operator/api/common"
	networkv1alpha1 "github.com/polarsquad/upcloud-operator/api/network/v1alpha1"
	"github.com/polarsquad/upcloud-operator/internal/reconciler"
	"github.com/polarsquad/upcloud-operator/internal/upcloudapi/fake"
)

const testNS = "ns"

// putFakeGateway seeds the fake API with a gateway at a known UUID (running).
func putFakeGateway(t *testing.T, api *fake.GatewayAPI, uuid string) {
	t.Helper()
	g := &upcloud.Gateway{
		UUID: uuid, Name: testGWName, Zone: testZone,
		OperationalState: upcloud.GatewayOperationalStateRunning,
	}
	api.Gateways[uuid] = g
}

func readyGatewayCR(name, uuid string) *networkv1alpha1.Gateway {
	return &networkv1alpha1.Gateway{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: testNS, UID: testGWUID, Generation: 1},
		Status: networkv1alpha1.GatewayStatus{
			UUID: uuid,
			Conditions: []metav1.Condition{{
				Type: testReadyType, Status: metav1.ConditionTrue, Reason: testReadyMsg,
				Message: "ok", ObservedGeneration: 1,
			}},
		},
	}
}

func newGatewayConnection() *networkv1alpha1.GatewayConnection {
	return &networkv1alpha1.GatewayConnection{
		ObjectMeta: metav1.ObjectMeta{Name: testConnName, Namespace: testNS, UID: "conn-uid", Generation: 1},
		Spec: networkv1alpha1.GatewayConnectionSpec{
			GatewayRef: common.LocalObjectReference{Name: testGWName},
			LocalRoutes: []networkv1alpha1.GatewayRoute{
				{Name: "r1", StaticNetwork: testCIDR, Type: testStaticRoute},
			},
		},
	}
}

func TestGatewayConnectionCreateWaitsForGateway(t *testing.T) {
	g := NewWithT(t)
	api := fake.NewGatewayAPI()
	c := newFakeClient(t)
	a := &GatewayConnectionAdapter{API: api, Client: c}
	ctx := context.Background()

	// Gateway exists in the cluster but has no UUID yet: create must wait.
	g.Expect(c.Create(ctx, &networkv1alpha1.Gateway{
		ObjectMeta: metav1.ObjectMeta{Name: testGWName, Namespace: testNS, UID: testGWUID, Generation: 1},
	})).To(Succeed())
	gw := newGatewayConnection()
	g.Expect(a.Create(ctx, gw)).To(MatchError(reconciler.ErrDependencyNotReady))

	// The gateway gets a UUID and is seeded in the fake API: create succeeds.
	gwCR := &networkv1alpha1.Gateway{}
	g.Expect(c.Get(ctx, client.ObjectKey{Namespace: testNS, Name: testGWName}, gwCR)).To(Succeed())
	gwCR.Status = readyGatewayCR("gw1", "gw-1").Status
	g.Expect(c.Update(ctx, gwCR)).To(Succeed())
	putFakeGateway(t, api, "gw-1")
	g.Expect(a.Create(ctx, gw)).To(Succeed())
	g.Expect(gw.Status.GatewayUUID).To(Equal("gw-1"))
	g.Expect(gw.Status.UUID).NotTo(BeEmpty())
}

func TestGatewayConnectionAdoptByName(t *testing.T) {
	g := NewWithT(t)
	api := fake.NewGatewayAPI()
	c := newFakeClient(t)
	a := &GatewayConnectionAdapter{API: api, Client: c}
	ctx := context.Background()

	g.Expect(c.Create(ctx, readyGatewayCR("gw1", "gw-1"))).To(Succeed())
	putFakeGateway(t, api, "gw-1")
	// Pre-existing connection in the fake, named to match.
	_, err := api.CreateGatewayConnection(ctx, &request.CreateGatewayConnectionRequest{
		ServiceUUID: "gw-1",
		Connection:  request.GatewayConnection{Name: testConnName, Type: upcloud.GatewayConnectionTypeIPSec},
	})
	g.Expect(err).NotTo(HaveOccurred())

	gw := newGatewayConnection()
	obs, err := a.Observe(ctx, gw)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(obs.Exists).To(BeTrue())
	g.Expect(obs.Ready).To(BeTrue())
	g.Expect(gw.Status.UUID).NotTo(BeEmpty())
}

func TestGatewayConnectionRouteDriftModifies(t *testing.T) {
	g := NewWithT(t)
	api := fake.NewGatewayAPI()
	c := newFakeClient(t)
	a := &GatewayConnectionAdapter{API: api, Client: c}
	ctx := context.Background()

	g.Expect(c.Create(ctx, readyGatewayCR("gw1", "gw-1"))).To(Succeed())
	putFakeGateway(t, api, "gw-1")
	gw := newGatewayConnection()
	g.Expect(a.Create(ctx, gw)).To(Succeed())
	g.Expect(obsConnUpToDate(a, ctx, gw)).To(BeTrue())

	// Desired gains a route: drift, then update sends both route sets.
	gw.Spec.LocalRoutes = append(gw.Spec.LocalRoutes, networkv1alpha1.GatewayRoute{
		Name: "r2", StaticNetwork: "10.0.1.0/24", Type: testStaticRoute,
	})
	g.Expect(obsConnUpToDate(a, ctx, gw)).To(BeFalse())
	g.Expect(a.Update(ctx, gw)).To(Succeed())
}

func TestGatewayConnectionDeleteIdempotent(t *testing.T) {
	g := NewWithT(t)
	api := fake.NewGatewayAPI()
	a := &GatewayConnectionAdapter{API: api, Client: newFakeClient(t)}
	ctx := context.Background()

	// No UUID yet: delete is a no-op.
	gw := newGatewayConnection()
	g.Expect(a.Delete(ctx, gw)).To(Succeed())
}

func obsConnUpToDate(a *GatewayConnectionAdapter, ctx context.Context, c *networkv1alpha1.GatewayConnection) bool {
	obs, err := a.Observe(ctx, c)
	if err != nil {
		panic(err)
	}
	return obs.UpToDate
}
