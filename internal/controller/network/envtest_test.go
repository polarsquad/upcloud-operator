package network

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/polarsquad/upcloud-operator/api/common"
	networkv1alpha1 "github.com/polarsquad/upcloud-operator/api/network/v1alpha1"
	"github.com/polarsquad/upcloud-operator/internal/reconciler"
)

var _ = Describe("Network group end to end against the fake API", func() {
	It("creates a router, then a network attached to it, then deletes both", func(ctx SpecContext) {
		router := &networkv1alpha1.Router{ObjectMeta: metav1.ObjectMeta{Name: "r1", Namespace: "default"}}
		Expect(k8sClient.Create(ctx, router)).To(Succeed())
		net := &networkv1alpha1.Network{
			ObjectMeta: metav1.ObjectMeta{Name: "n1", Namespace: "default"},
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

		Expect(k8sClient.Delete(ctx, net)).To(Succeed())
		Expect(k8sClient.Delete(ctx, router)).To(Succeed())
		Eventually(func() int { return len(fakeAPI.Networks) + len(fakeAPI.Routers) }, "20s", "250ms").Should(BeZero())
	})
})
