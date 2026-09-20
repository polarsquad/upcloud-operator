package network

import (
	"context"
	"testing"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud"
	. "github.com/onsi/gomega"

	"github.com/polarsquad/upcloud-operator/internal/upcloudapi"
	"github.com/polarsquad/upcloud-operator/internal/upcloudapi/fake"
)

// Existing CRs keep their UID, even when the controller moves namespaces.
// Adoption must not depend on the legacy managed-by value.
func TestNetworkMigratesLegacyOwnerLabelWithoutRecreation(t *testing.T) {
	for name, clearStatus := range map[string]bool{
		"existing status identity": false,
		"adopt by preserved UID":   true,
	} {
		t.Run(name, func(t *testing.T) {
			g := NewWithT(t)
			api := fake.NewNetworkAPI()
			a := &NetworkAdapter{API: api, Client: newFakeClient(t)}
			n := newNetwork()
			teamLabel := upcloud.Label{Key: "team", Value: "platform"}
			n.Spec.Labels = map[string]string{teamLabel.Key: teamLabel.Value}
			ctx := context.Background()
			g.Expect(a.Create(ctx, n)).To(Succeed())
			uuid := n.Status.UUID
			net := api.Networks[uuid]
			net.Labels = []upcloud.Label{
				{Key: upcloudapi.LabelUID, Value: string(n.UID)},
				{Key: upcloudapi.LabelManagedBy, Value: "upcloud-operator"},
				teamLabel,
			}
			api.Networks[uuid] = net
			if clearStatus {
				n.Status.UUID = ""
			}
			api.Calls = nil

			obs, err := a.Observe(ctx, n)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(obs.Exists).To(BeTrue())
			g.Expect(obs.UpToDate).To(BeFalse())
			g.Expect(n.Status.UUID).To(Equal(uuid))
			g.Expect(a.Update(ctx, n)).To(Succeed())
			g.Expect(api.Networks[uuid].Labels).To(ContainElement(upcloud.Label{
				Key: upcloudapi.LabelManagedBy, Value: "uck",
			}))
			g.Expect(upcloudapi.HasUID(api.Networks[uuid].Labels, n.UID)).To(BeTrue())
			g.Expect(api.Networks[uuid].Labels).To(ContainElement(teamLabel))

			obs, err = a.Observe(ctx, n)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(obs.UpToDate).To(BeTrue())
			g.Expect(n.Status.UUID).To(Equal(uuid))
			g.Expect(api.Networks).To(HaveLen(1))
			g.Expect(api.Calls).To(ContainElement("ModifyNetwork"))
			g.Expect(api.Calls).NotTo(ContainElement("CreateNetwork"))
			g.Expect(api.Calls).NotTo(ContainElement("DeleteNetwork"))
		})
	}
}
