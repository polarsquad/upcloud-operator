package upcloudapi

import (
	"maps"
	"slices"
	"strings"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud"
	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud/request"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	// LabelPrefix namespaces every label the operator sets on its own
	// behalf, mirroring the services.k8s.aws/ tags of the ACK controllers.
	// UpCloud label keys are limited to 32 characters, so the keys below
	// must stay short.
	LabelPrefix = "services.k8s.upcloud/"
	// LabelManagedBy marks resources created by this operator.
	LabelManagedBy = LabelPrefix + "managed-by"
	// LabelUID carries the owning CR's metadata.uid; used for adoption.
	LabelUID = LabelPrefix + "uid"
	// LabelNamespace carries the owning CR's namespace.
	LabelNamespace = LabelPrefix + "namespace"
	// ManagedByValue is the value of LabelManagedBy.
	ManagedByValue = "upcloud-operator"
)

// Labels is a set of UpCloud labels keyed by label key.
type Labels map[string]string

// NewLabels returns an empty Labels.
func NewLabels() Labels {
	return Labels{}
}

// MergeLabels combines two label sets. In case of collision precedence is
// given to the labels in a, as ACK's tags.Merge does.
func MergeLabels(a, b Labels) Labels {
	out := make(Labels, len(a)+len(b))
	maps.Copy(out, b)
	maps.Copy(out, a)
	return out
}

// DefaultLabels returns the labels the operator sets on every resource it
// manages. Users may override every key except LabelUID, which adoption
// depends on.
func DefaultLabels(obj metav1.Object) Labels {
	out := Labels{
		LabelUID:       string(obj.GetUID()),
		LabelManagedBy: ManagedByValue,
	}
	if ns := obj.GetNamespace(); ns != "" {
		out[LabelNamespace] = ns
	}
	return out
}

// DesiredLabels merges the default labels with user labels, sorted by key.
// As with ACK, user labels win over the defaults, except LabelUID, which is
// always the owner's UID.
func DesiredLabels(obj metav1.Object, user map[string]string) []upcloud.Label {
	merged := MergeLabels(Labels(user), DefaultLabels(obj))
	merged[LabelUID] = string(obj.GetUID())
	out := make([]upcloud.Label, 0, len(merged))
	for k, v := range merged {
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
