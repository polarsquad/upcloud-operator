//go:build e2e_upcloud

package e2e_upcloud

import (
	"testing"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud"

	"github.com/polarsquad/upcloud-operator/internal/upcloudapi"
)

func TestHasManagedBy(t *testing.T) {
	cases := []struct {
		name   string
		labels []upcloud.Label
		want   bool
	}{
		{"correct pair", []upcloud.Label{{Key: upcloudapi.LabelManagedBy, Value: upcloudapi.ManagedByValue}}, true},
		{"wrong value", []upcloud.Label{{Key: upcloudapi.LabelManagedBy, Value: "another-controller"}}, false},
		{"empty value", []upcloud.Label{{Key: upcloudapi.LabelManagedBy}}, false},
		{"missing label", nil, false},
		{"misleading key", []upcloud.Label{{Key: upcloudapi.ManagedByValue, Value: "unrelated"}}, false},
		{"legacy value", []upcloud.Label{{Key: upcloudapi.LabelManagedBy, Value: "upcloud-operator"}}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := hasManagedBy(tc.labels); got != tc.want {
				t.Fatalf("hasManagedBy(%v) = %v, want %v", tc.labels, got, tc.want)
			}
		})
	}
}
