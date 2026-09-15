package network

import (
	"context"
	"testing"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/polarsquad/upcloud-operator/api/common"
	networkv1alpha1 "github.com/polarsquad/upcloud-operator/api/network/v1alpha1"
	"github.com/polarsquad/upcloud-operator/internal/reconciler"
	"github.com/polarsquad/upcloud-operator/internal/upcloudapi"
	"github.com/polarsquad/upcloud-operator/internal/upcloudapi/fake"
)

func boolPtr(v bool) *bool { return &v }

func newFakeClient(t *testing.T) client.Client {
	g := NewWithT(t)
	s := runtime.NewScheme()
	g.Expect(networkv1alpha1.AddToScheme(s)).To(Succeed())
	return fakeclient.NewClientBuilder().WithScheme(s).Build()
}

func newNetwork() *networkv1alpha1.Network {
	return &networkv1alpha1.Network{
		ObjectMeta: metav1.ObjectMeta{Name: "n1", Namespace: "ns", UID: "net-uid"},
		Spec: networkv1alpha1.NetworkSpec{
			Zone:       "fi-hel1",
			IPNetworks: []networkv1alpha1.IPNetwork{{Address: "10.0.1.0/24", DHCP: boolPtr(true)}},
		},
	}
}

func readyRouterCR(name, uuid string) *networkv1alpha1.Router {
	return &networkv1alpha1.Router{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "ns", UID: "router-uid", Generation: 1},
		Status: networkv1alpha1.RouterStatus{
			UUID: uuid,
			Conditions: []metav1.Condition{{
				Type: "Ready", Status: metav1.ConditionTrue, Reason: "Available",
				Message: "ok", ObservedGeneration: 1,
			}},
		},
	}
}

func TestNetworkCreateWithoutRouter(t *testing.T) {
	g := NewWithT(t)
	api := fake.NewNetworkAPI()
	a := &NetworkAdapter{API: api, Client: newFakeClient(t)}
	n := newNetwork()
	ctx := context.Background()

	obs, err := a.Observe(ctx, n)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(obs.Exists).To(BeFalse())

	g.Expect(a.Create(ctx, n)).To(Succeed())
	created := api.Networks[n.Status.UUID]
	g.Expect(created.Zone).To(Equal("fi-hel1"))
	g.Expect(created.Name).To(Equal("n1"))
	g.Expect(created.IPNetworks[0].Address).To(Equal("10.0.1.0/24"))
	g.Expect(created.IPNetworks[0].DHCP).To(Equal(upcloud.True))
	g.Expect(created.Router).To(BeEmpty())
	g.Expect(upcloudapi.HasUID(created.Labels, "net-uid")).To(BeTrue())
}

func TestNetworkCreateWithRouterRefWaitsForRouter(t *testing.T) {
	g := NewWithT(t)
	api := fake.NewNetworkAPI()
	c := newFakeClient(t)
	a := &NetworkAdapter{API: api, Client: c}
	n := newNetwork()
	n.Spec.RouterRef = &common.LocalObjectReference{Name: "r1"}
	ctx := context.Background()

	obs, err := a.Observe(ctx, n)
	g.Expect(obs.Exists).To(BeFalse())
	g.Expect(err).To(MatchError(reconciler.ErrDependencyNotReady))

	g.Expect(c.Create(ctx, readyRouterCR("r1", "rtr-1"))).To(Succeed())

	obs, err = a.Observe(ctx, n)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(obs.Exists).To(BeFalse())

	g.Expect(a.Create(ctx, n)).To(Succeed())
	g.Expect(api.Networks[n.Status.UUID].Router).To(Equal("rtr-1"))
	g.Expect(n.Status.RouterUUID).To(Equal("rtr-1"))
}

func TestNetworkObserveDetectsRouterDriftAndUpdateReattaches(t *testing.T) {
	g := NewWithT(t)
	api := fake.NewNetworkAPI()
	a := &NetworkAdapter{API: api, Client: newFakeClient(t)}
	n := newNetwork()
	ctx := context.Background()
	g.Expect(a.Create(ctx, n)).To(Succeed())
	uuid := n.Status.UUID

	n.Spec.RouterUUID = "rtr-9"
	obs, err := a.Observe(ctx, n)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(obs.UpToDate).To(BeFalse())

	g.Expect(a.Update(ctx, n)).To(Succeed())
	g.Expect(api.Calls).To(ContainElement("AttachNetworkRouter"))
	g.Expect(api.Networks[uuid].Router).To(Equal("rtr-9"))

	n.Spec.RouterUUID = ""
	obs, err = a.Observe(ctx, n)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(obs.UpToDate).To(BeFalse())

	g.Expect(a.Update(ctx, n)).To(Succeed())
	g.Expect(api.Calls).To(ContainElement("DetachNetworkRouter"))
	g.Expect(api.Networks[uuid].Router).To(BeEmpty())
}

func TestNetworkObserveDetectsIPNetworkDrift(t *testing.T) {
	g := NewWithT(t)
	api := fake.NewNetworkAPI()
	a := &NetworkAdapter{API: api, Client: newFakeClient(t)}
	n := newNetwork()
	ctx := context.Background()
	g.Expect(a.Create(ctx, n)).To(Succeed())
	uuid := n.Status.UUID

	n.Spec.IPNetworks[0].DHCPDns = []string{"1.1.1.1"}
	obs, err := a.Observe(ctx, n)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(obs.UpToDate).To(BeFalse())

	g.Expect(a.Update(ctx, n)).To(Succeed())
	g.Expect(api.Calls).To(ContainElement("ModifyNetwork"))
	g.Expect(api.Networks[uuid].IPNetworks[0].DHCPDns).To(Equal([]string{"1.1.1.1"}))
}

func TestNetworkAdoptsByLabel(t *testing.T) {
	g := NewWithT(t)
	api := fake.NewNetworkAPI()
	a := &NetworkAdapter{API: api, Client: newFakeClient(t)}
	n := newNetwork()
	ctx := context.Background()
	g.Expect(a.Create(ctx, n)).To(Succeed())
	uuid := n.Status.UUID
	n.Status.UUID = ""

	obs, err := a.Observe(ctx, n)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(obs.Exists).To(BeTrue())
	g.Expect(n.Status.UUID).To(Equal(uuid))
}

func TestNetworkDeleteIdempotent(t *testing.T) {
	g := NewWithT(t)
	api := fake.NewNetworkAPI()
	a := &NetworkAdapter{API: api, Client: newFakeClient(t)}
	n := newNetwork()
	ctx := context.Background()
	g.Expect(a.Create(ctx, n)).To(Succeed())
	g.Expect(a.Delete(ctx, n)).To(Succeed())
	g.Expect(api.Networks).To(BeEmpty())
	g.Expect(a.Delete(ctx, n)).To(Succeed())

	g.Expect(a.Delete(ctx, &networkv1alpha1.Network{})).To(Succeed())
}
