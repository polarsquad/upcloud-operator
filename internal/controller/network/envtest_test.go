package network

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/polarsquad/upcloud-operator/api/common"
	networkv1alpha1 "github.com/polarsquad/upcloud-operator/api/network/v1alpha1"
	"github.com/polarsquad/upcloud-operator/internal/reconciler"
)

var _ = Describe("Network group end to end against the fake API", func() {
	It("creates a router, then a network attached to it, then deletes both", func(ctx SpecContext) {
		router := &networkv1alpha1.Router{ObjectMeta: metav1.ObjectMeta{Name: "r1", Namespace: testDefaultNS}}
		Expect(k8sClient.Create(ctx, router)).To(Succeed())
		net := &networkv1alpha1.Network{
			ObjectMeta: metav1.ObjectMeta{Name: "n1", Namespace: testDefaultNS},
			Spec: networkv1alpha1.NetworkSpec{Zone: testZone,
				IPNetworks: []networkv1alpha1.IPNetwork{{Address: "10.1.0.0/24"}},
				RouterRef:  &common.LocalObjectReference{Name: "r1"}},
		}
		Expect(k8sClient.Create(ctx, net)).To(Succeed())

		Eventually(func(g Gomega) {
			var n networkv1alpha1.Network
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(net), &n)).To(Succeed())
			g.Expect(reconciler.IsReady(&n)).To(BeTrue())
			g.Expect(n.Status.RouterUUID).To(HavePrefix("rtr-"))
		}, "20s", "250ms").Should(Succeed())

		// Delete the network first and wait for it to leave the fake before
		// deleting the router; the router's DeleteRouter 409s (and requeues)
		// while a network is still attached, so an unordered delete is flaky.
		Expect(k8sClient.Delete(ctx, net)).To(Succeed())
		// The Network CR is gone only after its finalizer has run, which happens
		// after DeleteNetwork has removed it from the fake. IgnoreNotFound returns
		// nil for BOTH a found and a not-found object, so assert the error directly.
		Eventually(func() error {
			return k8sClient.Get(ctx, client.ObjectKeyFromObject(net), &networkv1alpha1.Network{})
		}, "20s", "250ms").Should(MatchError(apierrors.IsNotFound, "network CR should be gone after its finalizer ran"))
		Expect(k8sClient.Delete(ctx, router)).To(Succeed())
		Eventually(func() int { return len(fakeAPI.Networks) + len(fakeAPI.Routers) }, "20s", "250ms").Should(BeZero())
	})

	It("allocates a floating IP, then releases it", func(ctx SpecContext) {
		fip := &networkv1alpha1.FloatingIP{
			ObjectMeta: metav1.ObjectMeta{Name: "ip1", Namespace: testDefaultNS},
			Spec:       networkv1alpha1.FloatingIPSpec{Zone: testZone, Family: testIPFamily, Access: testIPAccess},
		}
		Expect(k8sClient.Create(ctx, fip)).To(Succeed())

		Eventually(func(g Gomega) {
			var f networkv1alpha1.FloatingIP
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(fip), &f)).To(Succeed())
			g.Expect(reconciler.IsReady(&f)).To(BeTrue())
			g.Expect(f.Status.Address).To(HavePrefix("203.0.113."))
		}, "20s", "250ms").Should(Succeed())
		// The IP is allocated in the fake with the floating flag set.
		var f networkv1alpha1.FloatingIP
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(fip), &f)).To(Succeed())
		Expect(fakeAPI.IPs[f.Status.Address]).NotTo(BeNil())

		Expect(k8sClient.Delete(ctx, fip)).To(Succeed())
		// The CR is gone only after its finalizer has run, which is after
		// ReleaseIPAddress removed the address from the fake.
		Eventually(func() int { return len(fakeAPI.IPs) }, "20s", "250ms").Should(BeZero())
	})

	It("creates a network peering between a local network and a peer, then tears it down", func(ctx SpecContext) {
		net := &networkv1alpha1.Network{
			ObjectMeta: metav1.ObjectMeta{Name: "pn1", Namespace: testDefaultNS},
			Spec:       networkv1alpha1.NetworkSpec{Zone: testZone, IPNetworks: []networkv1alpha1.IPNetwork{{Address: "10.2.0.0/24"}}},
		}
		Expect(k8sClient.Create(ctx, net)).To(Succeed())
		Eventually(func(g Gomega) {
			var n networkv1alpha1.Network
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(net), &n)).To(Succeed())
			g.Expect(reconciler.IsReady(&n)).To(BeTrue())
		}, "20s", "250ms").Should(Succeed())

		peering := &networkv1alpha1.NetworkPeering{
			ObjectMeta: metav1.ObjectMeta{Name: "np1", Namespace: testDefaultNS},
			Spec: networkv1alpha1.NetworkPeeringSpec{
				NetworkRef:       common.LocalObjectReference{Name: "pn1"},
				PeerNetworkUUID:  "peer-net-1",
				ConfiguredStatus: testPeeringState,
			},
		}
		Expect(k8sClient.Create(ctx, peering)).To(Succeed())

		var localUUID string
		Eventually(func(g Gomega) {
			var n networkv1alpha1.Network
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(net), &n)).To(Succeed())
			localUUID = n.Status.UUID
			var p networkv1alpha1.NetworkPeering
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(peering), &p)).To(Succeed())
			g.Expect(reconciler.IsReady(&p)).To(BeTrue())
			g.Expect(p.Status.UUID).To(HavePrefix("np-"))
		}, "20s", "250ms").Should(Succeed())

		// The peering references the local network's UpCloud UUID and the peer.
		var p networkv1alpha1.NetworkPeering
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(peering), &p)).To(Succeed())
		Expect(fakeAPI.Peerings[p.Status.UUID].Network.UUID).To(Equal(localUUID))
		Expect(fakeAPI.Peerings[p.Status.UUID].PeerNetwork.UUID).To(Equal("peer-net-1"))

		// Delete the peering first (the network must not be deleted while a
		// peering references it), then the network.
		Expect(k8sClient.Delete(ctx, peering)).To(Succeed())
		Eventually(func() int { return len(fakeAPI.Peerings) }, "20s", "250ms").Should(BeZero())
		Expect(k8sClient.Delete(ctx, net)).To(Succeed())
		Eventually(func() bool { _, ok := fakeAPI.Networks[localUUID]; return !ok }, "20s", "250ms").Should(BeTrue())
	})
})
