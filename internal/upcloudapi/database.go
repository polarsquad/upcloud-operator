package upcloudapi

import (
	"context"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud"
	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud/request"
	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud/service"
)

// DatabaseAPI is the subset of service.Service used by the database adapters.
type DatabaseAPI interface {
	CreateManagedDatabase(ctx context.Context, r *request.CreateManagedDatabaseRequest) (*upcloud.ManagedDatabase, error)
	GetManagedDatabase(ctx context.Context, r *request.GetManagedDatabaseRequest) (*upcloud.ManagedDatabase, error)
	GetAllManagedDatabases(ctx context.Context) ([]upcloud.ManagedDatabase, error)
	ModifyManagedDatabase(ctx context.Context, r *request.ModifyManagedDatabaseRequest) (*upcloud.ManagedDatabase, error)
	DeleteManagedDatabase(ctx context.Context, r *request.DeleteManagedDatabaseRequest) error
	StartManagedDatabase(ctx context.Context, r *request.StartManagedDatabaseRequest) (*upcloud.ManagedDatabase, error)
	ShutdownManagedDatabase(ctx context.Context, r *request.ShutdownManagedDatabaseRequest) (*upcloud.ManagedDatabase, error)
	CreateManagedDatabaseUser(ctx context.Context, r *request.CreateManagedDatabaseUserRequest) (*upcloud.ManagedDatabaseUser, error)
	GetManagedDatabaseUser(ctx context.Context, r *request.GetManagedDatabaseUserRequest) (*upcloud.ManagedDatabaseUser, error)
	ModifyManagedDatabaseUser(ctx context.Context, r *request.ModifyManagedDatabaseUserRequest) (*upcloud.ManagedDatabaseUser, error)
	ModifyManagedDatabaseUserAccessControl(ctx context.Context, r *request.ModifyManagedDatabaseUserAccessControlRequest) (*upcloud.ManagedDatabaseUser, error)
	DeleteManagedDatabaseUser(ctx context.Context, r *request.DeleteManagedDatabaseUserRequest) error
	CreateManagedDatabaseLogicalDatabase(ctx context.Context, r *request.CreateManagedDatabaseLogicalDatabaseRequest) (*upcloud.ManagedDatabaseLogicalDatabase, error)
	GetManagedDatabaseLogicalDatabases(ctx context.Context, r *request.GetManagedDatabaseLogicalDatabasesRequest) ([]upcloud.ManagedDatabaseLogicalDatabase, error)
	DeleteManagedDatabaseLogicalDatabase(ctx context.Context, r *request.DeleteManagedDatabaseLogicalDatabaseRequest) error
}

var _ DatabaseAPI = (*service.Service)(nil)
