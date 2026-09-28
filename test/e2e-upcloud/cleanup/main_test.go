package main

import (
	"testing"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud"

	"github.com/polarsquad/upcloud-operator/internal/upcloudapi"
)

func TestHasManagedByMatchesKeyAndValue(t *testing.T) {
	tests := []struct {
		name   string
		labels []upcloud.Label
		want   bool
	}{
		{
			name: "operator label",
			labels: []upcloud.Label{{
				Key: upcloudapi.LabelManagedBy, Value: upcloudapi.ManagedByValue,
			}},
			want: true,
		},
		{
			name: "wrong value",
			labels: []upcloud.Label{{
				Key: upcloudapi.LabelManagedBy, Value: "terraform",
			}},
		},
		{
			name: "wrong key",
			labels: []upcloud.Label{{
				Key: "managed-by", Value: upcloudapi.ManagedByValue,
			}},
		},
		{name: "missing label"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hasManagedBy(tt.labels); got != tt.want {
				t.Fatalf("hasManagedBy() = %t, want %t", got, tt.want)
			}
		})
	}
}
