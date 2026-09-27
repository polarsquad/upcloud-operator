package upcloudapi

import (
	"slices"
	"strings"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud"
	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud/request"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	// LabelPrefix is the common prefix for operator-managed label keys.
	// We use "." instead of "/" because per-key label endpoints put the key
	// in the URL path unescaped, and a list filter would percent-encode "/".
	LabelPrefix = "services.k8s.upcloud."
	// LabelManagedBy marks resources created by this operator.
	LabelManagedBy = LabelPrefix + "managed-by"
	// LabelUID carries the owning CR's metadata.uid; used for adoption.
	LabelUID = LabelPrefix + "uid"
	// ManagedByValue is the value of LabelManagedBy.
	ManagedByValue = "uck"
)

// OwnerLabels returns the two labels every managed resource must carry.
func OwnerLabels(obj metav1.Object) []upcloud.Label {
	return []upcloud.Label{
		{Key: LabelUID, Value: string(obj.GetUID())},
		{Key: LabelManagedBy, Value: ManagedByValue},
	}
}

// DesiredLabels merges owner labels with user labels, sorted by key.
// User labels cannot override the owner keys.
func DesiredLabels(obj metav1.Object, user map[string]string) []upcloud.Label {
	out := OwnerLabels(obj)
	for k, v := range user {
		if k == LabelManagedBy || k == LabelUID ||
			k == "managed-by" || k == "k8s-uid" { // also strip the old key names so users cannot re-inject them via spec.labels
			continue
		}
		out = append(out, upcloud.Label{Key: k, Value: v})
	}
	sortLabels(out)
	return out
}

// LabelsEqual compares two label sets ignoring order.
func LabelsEqual(a, b []upcloud.Label) bool {
	if len(a) != len(b) {
		return false
	}
	as, bs := slices.Clone(a), slices.Clone(b)
	sortLabels(as)
	sortLabels(bs)
	return slices.Equal(as, bs)
}

// HasUID reports whether labels carry LabelUID equal to uid.
func HasUID(labels []upcloud.Label, uid types.UID) bool {
	for _, l := range labels {
		if l.Key == LabelUID && l.Value == string(uid) {
			return true
		}
	}
	return false
}

// UIDFilter is the list-endpoint filter selecting resources owned by obj.
func UIDFilter(obj metav1.Object) request.QueryFilter {
	return request.FilterLabel{Label: upcloud.Label{Key: LabelUID, Value: string(obj.GetUID())}}
}

func sortLabels(ls []upcloud.Label) {
	slices.SortFunc(ls, func(a, b upcloud.Label) int { return strings.Compare(a.Key, b.Key) })
}
