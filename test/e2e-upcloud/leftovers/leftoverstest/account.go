package leftoverstest

import (
	"context"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud"
	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud/request"

	"github.com/polarsquad/upcloud-operator/internal/upcloudapi/fake"
)

// Account is a shared fake account that satisfies leftovers.API, backed by
// the fakes from internal/upcloudapi/fake.
type Account struct {
	Net *fake.NetworkAPI
	OS  *fake.ObjectStorageAPI
	DB  *fake.DatabaseAPI

	// ListErr maps method names to errors to inject; when set, the error is
	// returned instead of calling the fake.
	ListErr map[string]error
	// DeleteErr maps method names to errors to inject; when set, the error is
	// returned instead of calling the fake.
	DeleteErr map[string]error
}

// NewAccount creates a new fake account with all APIs initialized.
func NewAccount() *Account {
	return &Account{
		Net:       fake.NewNetworkAPI(),
		OS:        fake.NewObjectStorageAPI(),
		DB:        fake.NewDatabaseAPI(),
		ListErr:   map[string]error{},
		DeleteErr: map[string]error{},
	}
}

// GetManagedDatabases implements leftovers.API.
func (a *Account) GetManagedDatabases(
	ctx context.Context, r *request.GetManagedDatabasesRequest,
) ([]upcloud.ManagedDatabase, error) {
	if err := a.ListErr["GetManagedDatabases"]; err != nil {
		return nil, err
	}
	return a.DB.GetAllManagedDatabases(ctx)
}

// DeleteManagedDatabase implements leftovers.API.
func (a *Account) DeleteManagedDatabase(ctx context.Context, r *request.DeleteManagedDatabaseRequest) error {
	if err := a.DeleteErr["DeleteManagedDatabase"]; err != nil {
		return err
	}
	return a.DB.DeleteManagedDatabase(ctx, r)
}

// GetManagedObjectStorages implements leftovers.API.
func (a *Account) GetManagedObjectStorages(
	ctx context.Context, r *request.GetManagedObjectStoragesRequest,
) ([]upcloud.ManagedObjectStorage, error) {
	if err := a.ListErr["GetManagedObjectStorages"]; err != nil {
		return nil, err
	}
	return a.OS.GetManagedObjectStorages(ctx, r)
}

// DeleteManagedObjectStorage implements leftovers.API.
func (a *Account) DeleteManagedObjectStorage(ctx context.Context, r *request.DeleteManagedObjectStorageRequest) error {
	if err := a.DeleteErr["DeleteManagedObjectStorage"]; err != nil {
		return err
	}
	return a.OS.DeleteManagedObjectStorage(ctx, r)
}

// GetRouters implements leftovers.API.
func (a *Account) GetRouters(ctx context.Context, filters ...request.QueryFilter) (*upcloud.Routers, error) {
	if err := a.ListErr["GetRouters"]; err != nil {
		return nil, err
	}
	return a.Net.GetRouters(ctx, filters...)
}

// DeleteRouter implements leftovers.API.
func (a *Account) DeleteRouter(ctx context.Context, r *request.DeleteRouterRequest) error {
	if err := a.DeleteErr["DeleteRouter"]; err != nil {
		return err
	}
	return a.Net.DeleteRouter(ctx, r)
}

// GetNetworks implements leftovers.API.
func (a *Account) GetNetworks(ctx context.Context, filters ...request.QueryFilter) (*upcloud.Networks, error) {
	if err := a.ListErr["GetNetworks"]; err != nil {
		return nil, err
	}
	return a.Net.GetNetworks(ctx, filters...)
}

// DeleteNetwork implements leftovers.API.
func (a *Account) DeleteNetwork(ctx context.Context, r *request.DeleteNetworkRequest) error {
	if err := a.DeleteErr["DeleteNetwork"]; err != nil {
		return err
	}
	return a.Net.DeleteNetwork(ctx, r)
}

// GetIPAddresses implements leftovers.API.
func (a *Account) GetIPAddresses(ctx context.Context) (*upcloud.IPAddresses, error) {
	if err := a.ListErr["GetIPAddresses"]; err != nil {
		return nil, err
	}
	return a.Net.GetIPAddresses(ctx)
}

// ReleaseIPAddress implements leftovers.API.
func (a *Account) ReleaseIPAddress(ctx context.Context, r *request.ReleaseIPAddressRequest) error {
	if err := a.DeleteErr["ReleaseIPAddress"]; err != nil {
		return err
	}
	return a.Net.ReleaseIPAddress(ctx, r)
}
