package network

import (
	"context"
	"errors"
	"testing"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud"
	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud/request"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/polarsquad/upcloud-operator/api/common"
	networkv1alpha1 "github.com/polarsquad/upcloud-operator/api/network/v1alpha1"
	"github.com/polarsquad/upcloud-operator/internal/reconciler"
	"github.com/polarsquad/upcloud-operator/internal/upcloudapi"
	"github.com/polarsquad/upcloud-operator/internal/upcloudapi/fake"
)

func newGateway() *networkv1alpha1.Gateway {
	return &networkv1alpha1.Gateway{
		ObjectMeta: metav1.ObjectMeta{Name: "gw1", Namespace: "ns", UID: "gw-uid", Generation: 1},
		Spec: networkv1alpha1.GatewaySpec{
			Zone:       testZone,
			Plan:       "small",
			Features:   []string{testNat},
			RouterUUID: "rtr-1",
			// Mirrors the +kubebuilder:default (not applied by the fake client).
			ConfiguredStatus: "started",
		},
	}
}

func obsUpToDate(a *GatewayAdapter, ctx context.Context, gw *networkv1alpha1.Gateway) bool {
	obs, err := a.Observe(ctx, gw)
	if err != nil {
		panic(err)
	}
	return obs.UpToDate
}

func TestGatewayCreateWithRouterRefWaitsThenCreates(t *testing.T) {
	g := NewWithT(t)
	api := fake.NewGatewayAPI()
	c := newFakeClient(t)
	a := &GatewayAdapter{API: api, Client: c}
	ctx := context.Background()

	gw := newGateway()
	gw.Spec.RouterUUID = ""
	gw.Spec.RouterRef = &common.LocalObjectReference{Name: "r1"}

	// No Router CR yet: Create must be a dependency error.
	g.Expect(a.Create(ctx, gw)).To(MatchError(reconciler.ErrDependencyNotReady))

	// Router ready: Create succeeds and records the UUID.
	g.Expect(c.Create(ctx, readyRouterCR("r1", "rtr-9"))).To(Succeed())
	g.Expect(a.Create(ctx, gw)).To(Succeed())
	g.Expect(gw.Status.UUID).NotTo(BeEmpty())
	created := api.Gateways[gw.Status.UUID]
	g.Expect(created.Routers).To(HaveLen(1))
	g.Expect(created.Routers[0].UUID).To(Equal("rtr-9"))
}

func TestGatewayPlanDriftUpdates(t *testing.T) {
	g := NewWithT(t)
	api := fake.NewGatewayAPI()
	a := &GatewayAdapter{API: api, Client: newFakeClient(t)}
	ctx := context.Background()

	gw := newGateway()
	g.Expect(a.Create(ctx, gw)).To(Succeed())
	created := api.Gateways[gw.Status.UUID]

	g.Expect(obsUpToDate(a, ctx, gw)).To(BeTrue())
	created.Plan = "medium" // drift in UpCloud
	g.Expect(obsUpToDate(a, ctx, gw)).To(BeFalse())
	g.Expect(a.Update(ctx, gw)).To(Succeed())
	g.Expect(api.Gateways[gw.Status.UUID].Plan).To(Equal("small"))
}

func TestGatewayConfiguredStatusFlip(t *testing.T) {
	g := NewWithT(t)
	api := fake.NewGatewayAPI()
	a := &GatewayAdapter{API: api, Client: newFakeClient(t)}
	ctx := context.Background()

	gw := newGateway()
	g.Expect(a.Create(ctx, gw)).To(Succeed())
	created := api.Gateways[gw.Status.UUID]

	g.Expect(created.ConfiguredStatus).To(Equal(upcloud.GatewayConfiguredStatusStarted))
	g.Expect(obsUpToDate(a, ctx, gw)).To(BeTrue())

	created.ConfiguredStatus = upcloud.GatewayConfiguredStatusStopped
	g.Expect(obsUpToDate(a, ctx, gw)).To(BeFalse())
	g.Expect(a.Update(ctx, gw)).To(Succeed())
	g.Expect(api.Gateways[gw.Status.UUID].ConfiguredStatus).To(Equal(upcloud.GatewayConfiguredStatusStarted))
}

func TestGatewayFeatureChangeFails(t *testing.T) {
	g := NewWithT(t)
	api := fake.NewGatewayAPI()
	a := &GatewayAdapter{API: api, Client: newFakeClient(t)}
	ctx := context.Background()

	gw := newGateway()
	g.Expect(a.Create(ctx, gw)).To(Succeed())
	api.Gateways[gw.Status.UUID].Features = []upcloud.GatewayFeature{testNat, testVPN} // API-side drift
	g.Expect(obsUpToDate(a, ctx, gw)).To(BeFalse())
	g.Expect(a.Update(ctx, gw)).To(MatchError(ContainSubstring("features are immutable")))
}

func TestGatewayAdoptedByLabel(t *testing.T) {
	g := NewWithT(t)
	api := fake.NewGatewayAPI()
	a := &GatewayAdapter{API: api, Client: newFakeClient(t)}
	ctx := context.Background()

	gw := newGateway()
	obs, err := a.Observe(ctx, gw)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(obs.Exists).To(BeFalse())

	// A gateway created by someone else, tagged with our UID label.
	created, err := api.CreateGateway(ctx, &request.CreateGatewayRequest{
		Name: "gw1", Zone: testZone, Plan: "small", Features: []upcloud.GatewayFeature{testNat},
		Labels: upcloudapi.DesiredLabels(gw, nil),
	})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(api.Gateways[created.UUID].OperationalState).To(Equal(upcloud.GatewayOperationalStateRunning))

	obs, err = a.Observe(ctx, gw)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(obs.Exists).To(BeTrue())
	g.Expect(obs.Ready).To(BeTrue())
	g.Expect(gw.Status.UUID).To(Equal(created.UUID))
}

func TestGatewayDeletePendingThenGone(t *testing.T) {
	g := NewWithT(t)
	api := fake.NewGatewayAPI()
	a := &GatewayAdapter{API: api, Client: newFakeClient(t)}
	ctx := context.Background()

	gw := newGateway()
	g.Expect(a.Create(ctx, gw)).To(Succeed())

	// A connection still present: delete is a pending conflict.
	_, err := api.CreateGatewayConnection(ctx, &request.CreateGatewayConnectionRequest{
		ServiceUUID: gw.Status.UUID,
		Connection:  request.GatewayConnection{Name: "c", Type: upcloud.GatewayConnectionTypeIPSec},
	})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(errors.Is(a.Delete(ctx, gw), reconciler.ErrPending)).To(BeTrue())
	g.Expect(api.Gateways).To(HaveKey(gw.Status.UUID))

	// Connection removed: delete now succeeds.
	api.Gateways[gw.Status.UUID].Connections = nil
	g.Expect(a.Delete(ctx, gw)).To(Succeed())
	g.Expect(api.Gateways).NotTo(HaveKey(gw.Status.UUID))
}
