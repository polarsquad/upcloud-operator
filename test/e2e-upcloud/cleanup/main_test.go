package main

import (
	"context"
	"testing"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud"
	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud/request"

	"github.com/polarsquad/upcloud-operator/internal/upcloudapi"
)

func TestHasManagedByMatchesKeyAndValue(t *testing.T) {
	tests := []struct {
		name   string
		labels []upcloud.Label
		want   bool
	}{
		{
			name: "correct pair",
			labels: []upcloud.Label{
				{Key: upcloudapi.LabelManagedBy, Value: upcloudapi.ManagedByValue},
			},
			want: true,
		},
		{
			name: "wrong value",
			labels: []upcloud.Label{
				{Key: upcloudapi.LabelManagedBy, Value: "terraform"},
			},
			want: false,
		},
		{
			name: "legacy value",
			labels: []upcloud.Label{
				{Key: upcloudapi.LabelManagedBy, Value: "upcloud-operator"},
			},
			want: false,
		},
		{
			name: "empty value",
			labels: []upcloud.Label{
				{Key: upcloudapi.LabelManagedBy, Value: ""},
			},
			want: false,
		},
		{
			name: "misleading key",
			labels: []upcloud.Label{
				{Key: "uck", Value: upcloudapi.ManagedByValue},
			},
			want: false,
		},
		{
			name:   "nil labels",
			labels: nil,
			want:   false,
		},
		{
			name: "correct pair present among other labels",
			labels: []upcloud.Label{
				{Key: upcloudapi.LabelUID, Value: "some-uid"},
				{Key: upcloudapi.LabelManagedBy, Value: upcloudapi.ManagedByValue},
				{Key: "custom", Value: "value"},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasManagedBy(tt.labels)
			if got != tt.want {
				t.Errorf("hasManagedBy(%v) = %v, want %v", tt.labels, got, tt.want)
			}
		})
	}
}

type fakeSweeper struct {
	deletedNetworkUUIDs []string
}

func (f *fakeSweeper) GetManagedDatabases(
	context.Context, *request.GetManagedDatabasesRequest,
) ([]upcloud.ManagedDatabase, error) {
	return []upcloud.ManagedDatabase{}, nil
}

func (f *fakeSweeper) DeleteManagedDatabase(context.Context, *request.DeleteManagedDatabaseRequest) error {
	return nil
}

func (f *fakeSweeper) GetManagedObjectStorages(
	context.Context, *request.GetManagedObjectStoragesRequest,
) ([]upcloud.ManagedObjectStorage, error) {
	return []upcloud.ManagedObjectStorage{}, nil
}

func (f *fakeSweeper) DeleteManagedObjectStorage(context.Context, *request.DeleteManagedObjectStorageRequest) error {
	return nil
}

func (f *fakeSweeper) GetRouters(context.Context, ...request.QueryFilter) (*upcloud.Routers, error) {
	return &upcloud.Routers{Routers: []upcloud.Router{}}, nil
}

func (f *fakeSweeper) DeleteRouter(context.Context, *request.DeleteRouterRequest) error {
	return nil
}

func (f *fakeSweeper) GetNetworks(context.Context, ...request.QueryFilter) (*upcloud.Networks, error) {
	return &upcloud.Networks{
		Networks: []upcloud.Network{
			{
				UUID: "uck-network",
				Labels: []upcloud.Label{
					{Key: upcloudapi.LabelManagedBy, Value: upcloudapi.ManagedByValue},
				},
			},
			{
				UUID: "terraform-network",
				Labels: []upcloud.Label{
					{Key: upcloudapi.LabelManagedBy, Value: "terraform"},
				},
			},
		},
	}, nil
}

func (f *fakeSweeper) DeleteNetwork(ctx context.Context, req *request.DeleteNetworkRequest) error {
	f.deletedNetworkUUIDs = append(f.deletedNetworkUUIDs, req.UUID)
	return nil
}

func (f *fakeSweeper) GetIPAddresses(context.Context) (*upcloud.IPAddresses, error) {
	return &upcloud.IPAddresses{IPAddresses: []upcloud.IPAddress{}}, nil
}

func (f *fakeSweeper) ReleaseIPAddress(context.Context, *request.ReleaseIPAddressRequest) error {
	return nil
}

func TestSweepSkipsForeignManagedBy(t *testing.T) {
	fake := &fakeSweeper{}
	ctx := context.Background()

	left, deleted, listFailures := sweep(ctx, fake)

	// Verify that exactly one network was selected for deletion (the uck-network)
	if len(fake.deletedNetworkUUIDs) != 1 {
		t.Errorf("sweep deleted %d networks, want 1", len(fake.deletedNetworkUUIDs))
	}
	if fake.deletedNetworkUUIDs[0] != "uck-network" {
		t.Errorf("sweep deleted %v, want [uck-network]", fake.deletedNetworkUUIDs)
	}

	// sweep found exactly 1 matching resource and issued 1 accepted delete
	if left != 1 {
		t.Errorf("sweep returned left=%d, want 1", left)
	}
	if deleted != 1 {
		t.Errorf("sweep returned deleted=%d, want 1", deleted)
	}
	if listFailures != 0 {
		t.Errorf("sweep returned listFailures=%d, want 0", listFailures)
	}
}
