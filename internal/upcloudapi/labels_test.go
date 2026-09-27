package upcloudapi_test

import (
	"strings"
	"testing"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/polarsquad/upcloud-operator/internal/upcloudapi"
)

const testUID = "uid-123"

func owner() *metav1.ObjectMeta {
	return &metav1.ObjectMeta{Name: "x", Namespace: "ns", UID: testUID}
}

func TestDesiredLabelsAddsOwnerLabelsAndSorts(t *testing.T) {
	g := NewWithT(t)
	got := upcloudapi.DesiredLabels(owner(), map[string]string{"team": "platform", "env": "dev"})
	g.Expect(got).To(Equal([]upcloud.Label{
		{Key: "env", Value: "dev"},
		{Key: upcloudapi.LabelManagedBy, Value: "uck"},
		{Key: upcloudapi.LabelUID, Value: testUID},
		{Key: "team", Value: "platform"},
	})) // sorted by key: env < services.k8s.upcloud.managed-by < services.k8s.upcloud.uid < team
}

func TestDesiredLabelsUserCannotOverrideOwnerKeys(t *testing.T) {
	g := NewWithT(t)

	// Test old key names are filtered
	got := upcloudapi.DesiredLabels(owner(), map[string]string{"k8s-uid": "evil", "managed-by": "me"})
	g.Expect(got).To(Equal([]upcloud.Label{
		{Key: upcloudapi.LabelManagedBy, Value: "uck"},
		{Key: upcloudapi.LabelUID, Value: testUID},
	}))

	// Test new key names are also filtered; result is OwnerLabels, sorted
	got = upcloudapi.DesiredLabels(owner(), map[string]string{upcloudapi.LabelManagedBy: "evil", upcloudapi.LabelUID: "hijack"})
	g.Expect(got).To(Equal([]upcloud.Label{
		{Key: upcloudapi.LabelManagedBy, Value: "uck"},
		{Key: upcloudapi.LabelUID, Value: testUID},
	}))
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
	g.Expect(upcloudapi.HasUID(upcloudapi.OwnerLabels(owner()), testUID)).To(BeTrue())
	g.Expect(upcloudapi.HasUID(nil, testUID)).To(BeFalse())
	g.Expect(upcloudapi.UIDFilter(owner()).ToQueryParam()).To(Equal("label=" + upcloudapi.LabelUID + "=" + testUID))
}

func TestOperatorLabelKeysAreValidUpCloudKeys(t *testing.T) {
	g := NewWithT(t)
	for _, k := range []string{upcloudapi.LabelManagedBy, upcloudapi.LabelUID} {
		g.Expect(len(k)).To(BeNumerically(">=", 2), "key %q too short", k)
		g.Expect(len(k)).To(BeNumerically("<=", 32), "key %q too long", k)
		for i := 0; i < len(k); i++ {
			b := k[i]
			g.Expect(b).To(BeNumerically(">=", byte(0x20)), "key %q: byte %d (0x%x) not printable ASCII", k, i, b)
			g.Expect(b).To(BeNumerically("<=", byte(0x7E)), "key %q: byte %d (0x%x) not printable ASCII", k, i, b)
		}
		g.Expect(strings.HasPrefix(k, "_")).To(BeFalse(), "key %q starts with _", k)
		g.Expect(strings.Contains(k, "/")).To(BeFalse(), "key %q contains /", k)
	}
}
