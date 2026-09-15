package upcloudapi_test

import (
	"testing"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/polarsquad/upcloud-operator/internal/upcloudapi"
)

func owner() *metav1.ObjectMeta {
	return &metav1.ObjectMeta{Name: "x", Namespace: "ns", UID: "uid-123"}
}

func TestDesiredLabelsAddsOwnerLabelsAndSorts(t *testing.T) {
	g := NewWithT(t)
	got := upcloudapi.DesiredLabels(owner(), map[string]string{"team": "platform", "env": "dev"})
	g.Expect(got).To(Equal([]upcloud.Label{
		{Key: "env", Value: "dev"},
		{Key: "k8s-uid", Value: "uid-123"},
		{Key: "managed-by", Value: "upcloud-operator"},
		{Key: "team", Value: "platform"},
	}))
}

func TestDesiredLabelsUserCannotOverrideOwnerKeys(t *testing.T) {
	g := NewWithT(t)
	got := upcloudapi.DesiredLabels(owner(), map[string]string{"k8s-uid": "evil", "managed-by": "me"})
	g.Expect(got).To(Equal(upcloudapi.OwnerLabels(owner())))
}

func TestLabelsEqualIgnoresOrder(t *testing.T) {
	g := NewWithT(t)
	a := []upcloud.Label{{Key: "a", Value: "1"}, {Key: "b", Value: "2"}}
	b := []upcloud.Label{{Key: "b", Value: "2"}, {Key: "a", Value: "1"}}
	g.Expect(upcloudapi.LabelsEqual(a, b)).To(BeTrue())
	g.Expect(upcloudapi.LabelsEqual(a, a[:1])).To(BeFalse())
}

func TestHasUIDAndFilter(t *testing.T) {
	g := NewWithT(t)
	g.Expect(upcloudapi.HasUID(upcloudapi.OwnerLabels(owner()), "uid-123")).To(BeTrue())
	g.Expect(upcloudapi.HasUID(nil, "uid-123")).To(BeFalse())
	g.Expect(upcloudapi.UIDFilter(owner()).ToQueryParam()).To(Equal("label=k8s-uid=uid-123"))
}
