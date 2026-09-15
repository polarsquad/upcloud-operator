package network

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/polarsquad/upcloud-operator/api/common"
	networkv1alpha1 "github.com/polarsquad/upcloud-operator/api/network/v1alpha1"
	"github.com/polarsquad/upcloud-operator/internal/reconciler"
)

var _ = Describe("Gateway group end to end against the fake API", func() {
	It("creates a router, gateway, connection and tunnel, then deletes all in reverse", func(ctx SpecContext) {
		// PSK secret for the tunnel.
		Expect(k8sClient.Create(ctx, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "gw-psk", Namespace: testDefaultNS},
			Data:       map[string][]byte{testPskKey: []byte("envtest-psk")},
		})).To(Succeed())

		router := &networkv1alpha1.Router{ObjectMeta: metav1.ObjectMeta{Name: "gw-r", Namespace: testDefaultNS}}
		Expect(k8sClient.Create(ctx, router)).To(Succeed())

		gw := &networkv1alpha1.Gateway{
			ObjectMeta: metav1.ObjectMeta{Name: "gw-gw", Namespace: testDefaultNS},
			Spec: networkv1alpha1.GatewaySpec{
				Zone: testZone, Plan: testSmallPlan, Features: []string{"nat", "vpn"},
				RouterRef:        &common.LocalObjectReference{Name: "gw-r"},
				ConfiguredStatus: "started",
				Addresses:        []networkv1alpha1.GatewayAddress{{Name: testLocalAddrName}},
			},
		}
		Expect(k8sClient.Create(ctx, gw)).To(Succeed())

		conn := &networkv1alpha1.GatewayConnection{
			ObjectMeta: metav1.ObjectMeta{Name: "gw-conn", Namespace: testDefaultNS},
			Spec: networkv1alpha1.GatewayConnectionSpec{
				GatewayRef:  common.LocalObjectReference{Name: "gw-gw"},
				LocalRoutes: []networkv1alpha1.GatewayRoute{{Name: "r1", StaticNetwork: testCIDR, Type: testStaticRoute}},
			},
		}
		Expect(k8sClient.Create(ctx, conn)).To(Succeed())

		tun := &networkv1alpha1.GatewayTunnel{
			ObjectMeta: metav1.ObjectMeta{Name: "gw-tun", Namespace: testDefaultNS},
			Spec: networkv1alpha1.GatewayTunnelSpec{
				ConnectionRef:    common.LocalObjectReference{Name: "gw-conn"},
				LocalAddressName: testVPN,
				RemoteAddress:    "203.0.113.10",
				IPSec: &networkv1alpha1.GatewayTunnelIPSec{
					Authentication: networkv1alpha1.GatewayTunnelIPSecAuth{
						Type:         testPskKey,
						PskSecretRef: common.SecretKeySelector{Name: "gw-psk", Key: testPskKey},
					},
				},
			},
		}
		Expect(k8sClient.Create(ctx, tun)).To(Succeed())

		// The whole chain reaches Ready.
		Eventually(func(g Gomega) {
			var got networkv1alpha1.Gateway
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(gw), &got)).To(Succeed())
			g.Expect(reconciler.IsReady(&got)).To(BeTrue())

			var gc networkv1alpha1.GatewayConnection
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(conn), &gc)).To(Succeed())
			g.Expect(reconciler.IsReady(&gc)).To(BeTrue())

			var gt networkv1alpha1.GatewayTunnel
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(tun), &gt)).To(Succeed())
			g.Expect(reconciler.IsReady(&gt)).To(BeTrue())
		}, "30s", "250ms").Should(Succeed())

		// Delete in reverse: tunnel, connection, gateway, then the router.
		Expect(k8sClient.Delete(ctx, tun)).To(Succeed())
		Expect(k8sClient.Delete(ctx, conn)).To(Succeed())
		Expect(k8sClient.Delete(ctx, gw)).To(Succeed())
		Eventually(func() int { return len(gwAPI.Gateways) }, "30s", "250ms").Should(BeZero())
		// The router is reconciled into the SAME fakeAPI the Network spec asserts
		// on, so it must be torn down too, or its router leaks into that spec's
		// final "fake is empty" check (Ginkgo runs the specs in random order).
		Expect(k8sClient.Delete(ctx, router)).To(Succeed())
		Eventually(func() error {
			return k8sClient.Get(ctx, client.ObjectKeyFromObject(router), &networkv1alpha1.Router{})
		}, "30s", "250ms").Should(MatchError(apierrors.IsNotFound, "router CR should be gone after its finalizer ran"))
		Eventually(func() int { return len(fakeAPI.Routers) }, "30s", "250ms").Should(BeZero())
	})
})
