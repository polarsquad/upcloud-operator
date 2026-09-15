package network

import (
	"context"
	"testing"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/polarsquad/upcloud-operator/api/common"
	networkv1alpha1 "github.com/polarsquad/upcloud-operator/api/network/v1alpha1"
	"github.com/polarsquad/upcloud-operator/internal/reconciler"
	"github.com/polarsquad/upcloud-operator/internal/upcloudapi"
	"github.com/polarsquad/upcloud-operator/internal/upcloudapi/fake"
)

// readyNetworkCR returns a Ready Network CR with the given name and UUID, so
// peering tests do not depend on the Network adapter's sequence numbers.
func readyNetworkCR(name, uuid string) *networkv1alpha1.Network {
	return &networkv1alpha1.Network{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "ns", UID: testNetUID, Generation: 1},
		Status: networkv1alpha1.NetworkStatus{
			UUID: uuid,
			Conditions: []metav1.Condition{{
				Type: testReadyType, Status: metav1.ConditionTrue, Reason: testReadyMsg,
				Message: "ok", ObservedGeneration: 1,
			}},
		},
	}
}

func newNetworkPeering() *networkv1alpha1.NetworkPeering {
	return &networkv1alpha1.NetworkPeering{
		ObjectMeta: metav1.ObjectMeta{Name: "peering1", Namespace: "ns", UID: "peering-uid"},
		Spec: networkv1alpha1.NetworkPeeringSpec{
			NetworkRef:       common.LocalObjectReference{Name: "n1"},
			PeerNetworkUUID:  "peer-net-1",
			ConfiguredStatus: testPeeringState,
			Labels:           map[string]string{"env": "test"},
		},
	}
}

func newPeeringAdapter(t *testing.T) (*NetworkPeeringAdapter, *fake.NetworkAPI) {
	g := NewWithT(t)
	api := fake.NewNetworkAPI()
	// The peering's local network must exist both as a Ready CR (for the
	// adapter's parent lookup) and in the fake API (the create endpoint
	// validates the local network UUID).
	api.Networks["net-1"] = &upcloud.Network{UUID: "net-1", Name: "n1"}
	c := newFakeClient(t)
	g.Expect(c.Create(context.Background(), readyNetworkCR("n1", "net-1"))).To(Succeed())
	return &NetworkPeeringAdapter{API: api, Client: c}, api
}

func TestNetworkPeeringObserveMissingThenCreate(t *testing.T) {
	g := NewWithT(t)
	a, api := newPeeringAdapter(t)
	p := newNetworkPeering()
	ctx := context.Background()

	obs, err := a.Observe(ctx, p)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(obs.Exists).To(BeFalse())

	g.Expect(a.Create(ctx, p)).To(Succeed())
	g.Expect(p.Status.UUID).To(HavePrefix("np-"))
	created := api.Peerings[p.Status.UUID]
	g.Expect(created.Name).To(Equal("peering1"))
	g.Expect(created.Network.UUID).To(Equal("net-1"))
	g.Expect(created.PeerNetwork.UUID).To(Equal("peer-net-1"))
	g.Expect(created.ConfiguredStatus).To(Equal(upcloud.NetworkPeeringConfiguredStatusActive))
	g.Expect(upcloudapi.HasUID(created.Labels, "peering-uid")).To(BeTrue())

	obs, err = a.Observe(ctx, p)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(obs).To(Equal(reconciler.Observation{Exists: true, UpToDate: true, Ready: true, Message: testPeeringState}))
}

func TestNetworkPeeringObserveAdoptsByLabelWhenStatusEmpty(t *testing.T) {
	g := NewWithT(t)
	a, _ := newPeeringAdapter(t)
	p := newNetworkPeering()
	ctx := context.Background()
	g.Expect(a.Create(ctx, p)).To(Succeed())
	uuid := p.Status.UUID
	p.Status.UUID = ""

	obs, err := a.Observe(ctx, p)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(obs.Exists).To(BeTrue())
	g.Expect(p.Status.UUID).To(Equal(uuid))
}

func TestNetworkPeeringObserveWaitsForNetworkReady(t *testing.T) {
	g := NewWithT(t)
	api := fake.NewNetworkAPI()
	c := newFakeClient(t)
	// A Network CR without a Ready condition.
	notReady := &networkv1alpha1.Network{ObjectMeta: metav1.ObjectMeta{Name: "n1", Namespace: "ns", UID: testNetUID}}
	g.Expect(c.Create(context.Background(), notReady)).To(Succeed())
	a := &NetworkPeeringAdapter{API: api, Client: c}
	p := newNetworkPeering()

	_, err := a.Observe(context.Background(), p)
	g.Expect(err).To(MatchError(reconciler.ErrDependencyNotReady))
}

func TestNetworkPeeringObserveDetectsDriftAndUpdateFixesIt(t *testing.T) {
	g := NewWithT(t)
	a, api := newPeeringAdapter(t)
	p := newNetworkPeering()
	ctx := context.Background()
	g.Expect(a.Create(ctx, p)).To(Succeed())

	p.Spec.Name = "renamed"
	p.Spec.ConfiguredStatus = "disabled"
	obs, err := a.Observe(ctx, p)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(obs.UpToDate).To(BeFalse())

	g.Expect(a.Update(ctx, p)).To(Succeed())
	g.Expect(api.Peerings[p.Status.UUID].Name).To(Equal("renamed"))
	g.Expect(api.Peerings[p.Status.UUID].ConfiguredStatus).To(Equal(upcloud.NetworkPeeringConfiguredStatusDisabled))

	obs, err = a.Observe(ctx, p)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(obs.UpToDate).To(BeTrue())
}

func TestNetworkPeeringTransitionalStateIsNotReady(t *testing.T) {
	g := NewWithT(t)
	a, api := newPeeringAdapter(t)
	p := newNetworkPeering()
	ctx := context.Background()
	g.Expect(a.Create(ctx, p)).To(Succeed())

	// The peer has not accepted yet: pending-peer is not Ready.
	api.Peerings[p.Status.UUID].State = upcloud.NetworkPeeringStatePendingPeer
	obs, err := a.Observe(ctx, p)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(obs.Exists).To(BeTrue())
	g.Expect(obs.UpToDate).To(BeTrue())
	g.Expect(obs.Ready).To(BeFalse())
	g.Expect(obs.Message).To(Equal("pending-peer"))
}

func TestNetworkPeeringDeleteIsIdempotent(t *testing.T) {
	g := NewWithT(t)
	a, api := newPeeringAdapter(t)
	p := newNetworkPeering()
	ctx := context.Background()
	g.Expect(a.Create(ctx, p)).To(Succeed())
	g.Expect(a.Delete(ctx, p)).To(Succeed())
	g.Expect(api.Peerings).To(BeEmpty())
	g.Expect(a.Delete(ctx, p)).To(Succeed())

	g.Expect(a.Delete(ctx, &networkv1alpha1.NetworkPeering{})).To(Succeed())
}
