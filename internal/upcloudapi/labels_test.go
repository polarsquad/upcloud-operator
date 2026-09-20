package upcloudapi_test

import (
	"testing"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/polarsquad/upcloud-operator/internal/upcloudapi"
)

const (
	testUID = "uid-123"
	onlyA   = "only-a"

	teamPlatform = "platform"
	teamKey      = "team"
)

func owner() *metav1.ObjectMeta {
	return &metav1.ObjectMeta{Name: "x", Namespace: "ns", UID: testUID}
}

func TestDesiredLabelsAddsDefaultLabelsAndSorts(t *testing.T) {
	g := NewWithT(t)
	got := upcloudapi.DesiredLabels(owner(), map[string]string{teamKey: teamPlatform, "env": "dev"})
	g.Expect(got).To(Equal([]upcloud.Label{
		{Key: "env", Value: "dev"},
		{Key: "services.k8s.upcloud/managed-by", Value: "upcloud-operator"},
		{Key: "services.k8s.upcloud/namespace", Value: "ns"},
		{Key: "services.k8s.upcloud/uid", Value: testUID},
		{Key: teamKey, Value: teamPlatform},
	}))
}

func TestDesiredLabelsUserOverridesDefaultsButNotUID(t *testing.T) {
	g := NewWithT(t)
	got := upcloudapi.DesiredLabels(owner(), map[string]string{
		upcloudapi.LabelUID:       "evil",
		upcloudapi.LabelManagedBy: "me",
		upcloudapi.LabelNamespace: "other",
	})
	g.Expect(got).To(Equal([]upcloud.Label{
		{Key: upcloudapi.LabelManagedBy, Value: "me"},
		{Key: upcloudapi.LabelNamespace, Value: "other"},
		{Key: upcloudapi.LabelUID, Value: testUID},
	}))
}

func TestDesiredLabelsOmitsEmptyNamespace(t *testing.T) {
	g := NewWithT(t)
	obj := &metav1.ObjectMeta{Name: "x", UID: testUID}
	g.Expect(upcloudapi.DesiredLabels(obj, nil)).To(Equal([]upcloud.Label{
		{Key: upcloudapi.LabelManagedBy, Value: "upcloud-operator"},
		{Key: upcloudapi.LabelUID, Value: testUID},
	}))
}

func TestMergeLabelsGivesPrecedenceToFirst(t *testing.T) {
	g := NewWithT(t)
	a := upcloudapi.Labels{"k": "a", onlyA: "1"}
	b := upcloudapi.Labels{"k": "b", "only-b": "2"}
	g.Expect(upcloudapi.MergeLabels(a, b)).To(Equal(upcloudapi.Labels{"k": "a", onlyA: "1", "only-b": "2"}))
	g.Expect(a).To(Equal(upcloudapi.Labels{"k": "a", onlyA: "1"}), "inputs are not mutated")
	g.Expect(upcloudapi.MergeLabels(nil, nil)).To(BeEmpty())
}

func TestDefaultLabelKeysFitUpCloudLimit(t *testing.T) {
	g := NewWithT(t)
	for k := range upcloudapi.DefaultLabels(owner()) {
		g.Expect(len(k)).To(BeNumerically("<=", 32), k)
	}
}

func TestLabelsEqualIgnoresOrder(t *testing.T) {
	g := NewWithT(t)
	a := []upcloud.Label{{Key: "a", Value: "1"}, {Key: "b", Value: "2"}}
	b := []upcloud.Label{{Key: "b", Value: "2"}, {Key: "a", Value: "1"}}
	g.Expect(upcloudapi.LabelsEqual(a, b)).To(BeTrue())
	g.Expect(upcloudapi.LabelsEqual(a, a[:1])).To(BeFalse())
}

func TestLabelsEqualDetectsChangedValue(t *testing.T) {
	g := NewWithT(t)
	a := []upcloud.Label{{Key: teamKey, Value: teamPlatform}}
	b := []upcloud.Label{{Key: teamKey, Value: "storage"}}
	g.Expect(upcloudapi.LabelsEqual(a, b)).To(BeFalse())
}

func TestHasUIDAndFilter(t *testing.T) {
	g := NewWithT(t)
	g.Expect(upcloudapi.HasUID(upcloudapi.DesiredLabels(owner(), nil), testUID)).To(BeTrue())
	g.Expect(upcloudapi.HasUID(nil, testUID)).To(BeFalse())
	g.Expect(upcloudapi.UIDFilter(owner()).ToQueryParam()).To(Equal("label=services.k8s.upcloud/uid=uid-123"))
}
