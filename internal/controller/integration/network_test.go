package integration_test

import (
	"context"
	"testing"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud/request"
	. "github.com/onsi/gomega"

	"github.com/polarsquad/upcloud-operator/api/common"
	networkv1 "github.com/polarsquad/upcloud-operator/api/network/v1alpha1"
	"github.com/polarsquad/upcloud-operator/internal/controller/network"
	"github.com/polarsquad/upcloud-operator/internal/reconciler"
	"github.com/polarsquad/upcloud-operator/internal/upcloudapi"
	"github.com/polarsquad/upcloud-operator/internal/upcloudapi/fake"
)

func TestParentFirstRouterNetworkRetriesAttachedNetwork(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()
	c, api := pairClient(t), fake.NewNetworkAPI()
	router := &networkv1.Router{ObjectMeta: pairMeta("parent-router")}
	net := &networkv1.Network{
		ObjectMeta: pairMeta("child-network"),
		Spec: networkv1.NetworkSpec{
			Zone: "fi-hel1", IPNetworks: []networkv1.IPNetwork{{Address: "10.53.0.0/24"}},
			RouterRef: &common.LocalObjectReference{Name: router.Name},
		},
	}
	r := &reconciler.Reconciler[*networkv1.Router]{
		Client: c, Adapter: &network.RouterAdapter{API: api},
		New:       func() *networkv1.Router { return &networkv1.Router{} },
		Finalizer: network.FinalizerRouter, PendingRequeue: retryDelay,
	}
	n := &reconciler.Reconciler[*networkv1.Network]{
		Client: c, Adapter: &network.NetworkAdapter{Client: c, API: api},
		New:       func() *networkv1.Network { return &networkv1.Network{} },
		Finalizer: network.FinalizerNetwork, PendingRequeue: retryDelay,
	}
	createReady(t, r, router)
	createReady(t, n, net)
	routerID, networkID := router.Status.UUID, net.Status.UUID
	g.Expect(net.Status.RouterUUID).To(Equal(routerID))

	requestDeletion(t, c, router)
	// The fake refuses an attached router with 409. The documented dependency
	// is router -> attached networks, not the reverse. Vendor docs spell the
	// ROUTER_ATTACHED status "400 Conflict"; this proves the fake's 409 retry
	// contract, not that vendor error's status mapping (see docs/development.md).
	for range 2 {
		requirePending(t, r, router)
		requireLive(t, c, net)
		cloudRouter, err := api.GetRouterDetails(ctx, &request.GetRouterDetailsRequest{UUID: routerID})
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(upcloudapi.HasUID(cloudRouter.Labels, router.UID)).To(BeTrue())
		cloudNetwork, err := api.GetNetworkDetails(ctx, &request.GetNetworkDetailsRequest{UUID: networkID})
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(cloudNetwork.Router).To(Equal(routerID))
		g.Expect(upcloudapi.HasUID(cloudNetwork.Labels, net.UID)).To(BeTrue())
	}

	requestDeletion(t, c, net)
	requireGone(t, n, net)
	requireGone(t, r, router)
	_, err := api.GetNetworkDetails(ctx, &request.GetNetworkDetailsRequest{UUID: networkID})
	g.Expect(upcloudapi.IsNotFound(err)).To(BeTrue())
	_, err = api.GetRouterDetails(ctx, &request.GetRouterDetailsRequest{UUID: routerID})
	g.Expect(upcloudapi.IsNotFound(err)).To(BeTrue())
	const deleteRouter = "DeleteRouter"
	g.Expect(deleteCalls(api.Calls)).To(Equal([]string{
		deleteRouter, deleteRouter, "DeleteNetwork", deleteRouter,
	}))
}
