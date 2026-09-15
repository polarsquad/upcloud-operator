package upcloudapi

import (
	"context"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud"
	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud/request"
	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud/service"
)

// ObjectStorageAPI is the subset of service.Service used by the object
// storage adapters. GetManagedObjectStorages has no label filter (the API does
// not support one), so adoption for the top-level service is via list + HasUID
// on the response, not on the query.
type ObjectStorageAPI interface {
	// Service
	CreateManagedObjectStorage(ctx context.Context, r *request.CreateManagedObjectStorageRequest) (*upcloud.ManagedObjectStorage, error)
	GetManagedObjectStorages(ctx context.Context, r *request.GetManagedObjectStoragesRequest) ([]upcloud.ManagedObjectStorage, error)
	GetManagedObjectStorage(ctx context.Context, r *request.GetManagedObjectStorageRequest) (*upcloud.ManagedObjectStorage, error)
	ModifyManagedObjectStorage(ctx context.Context, r *request.ModifyManagedObjectStorageRequest) (*upcloud.ManagedObjectStorage, error)
	DeleteManagedObjectStorage(ctx context.Context, r *request.DeleteManagedObjectStorageRequest) error

	// Users
	CreateManagedObjectStorageUser(ctx context.Context, r *request.CreateManagedObjectStorageUserRequest) (*upcloud.ManagedObjectStorageUser, error)
	GetManagedObjectStorageUsers(ctx context.Context, r *request.GetManagedObjectStorageUsersRequest) ([]upcloud.ManagedObjectStorageUser, error)
	GetManagedObjectStorageUser(ctx context.Context, r *request.GetManagedObjectStorageUserRequest) (*upcloud.ManagedObjectStorageUser, error)
	DeleteManagedObjectStorageUser(ctx context.Context, r *request.DeleteManagedObjectStorageUserRequest) error

	// User policies
	AttachManagedObjectStorageUserPolicy(ctx context.Context, r *request.AttachManagedObjectStorageUserPolicyRequest) error
	DetachManagedObjectStorageUserPolicy(ctx context.Context, r *request.DetachManagedObjectStorageUserPolicyRequest) error
	GetManagedObjectStorageUserPolicies(ctx context.Context, r *request.GetManagedObjectStorageUserPoliciesRequest) ([]upcloud.ManagedObjectStorageUserPolicy, error)

	// Access keys
	CreateManagedObjectStorageUserAccessKey(ctx context.Context, r *request.CreateManagedObjectStorageUserAccessKeyRequest) (*upcloud.ManagedObjectStorageUserAccessKey, error)
	GetManagedObjectStorageUserAccessKeys(ctx context.Context, r *request.GetManagedObjectStorageUserAccessKeysRequest) ([]upcloud.ManagedObjectStorageUserAccessKey, error)
	GetManagedObjectStorageUserAccessKey(ctx context.Context, r *request.GetManagedObjectStorageUserAccessKeyRequest) (*upcloud.ManagedObjectStorageUserAccessKey, error)
	ModifyManagedObjectStorageUserAccessKey(ctx context.Context, r *request.ModifyManagedObjectStorageUserAccessKeyRequest) (*upcloud.ManagedObjectStorageUserAccessKey, error)
	DeleteManagedObjectStorageUserAccessKey(ctx context.Context, r *request.DeleteManagedObjectStorageUserAccessKeyRequest) error

	// Policies
	CreateManagedObjectStoragePolicy(ctx context.Context, r *request.CreateManagedObjectStoragePolicyRequest) (*upcloud.ManagedObjectStoragePolicy, error)
	GetManagedObjectStoragePolicies(ctx context.Context, r *request.GetManagedObjectStoragePoliciesRequest) ([]upcloud.ManagedObjectStoragePolicy, error)
	GetManagedObjectStoragePolicy(ctx context.Context, r *request.GetManagedObjectStoragePolicyRequest) (*upcloud.ManagedObjectStoragePolicy, error)
	DeleteManagedObjectStoragePolicy(ctx context.Context, r *request.DeleteManagedObjectStoragePolicyRequest) error

	// Buckets
	CreateManagedObjectStorageBucket(ctx context.Context, r *request.CreateManagedObjectStorageBucketRequest) (upcloud.ManagedObjectStorageBucketMetrics, error)
	DeleteManagedObjectStorageBucket(ctx context.Context, r *request.DeleteManagedObjectStorageBucketRequest) error
	GetManagedObjectStorageBucketMetrics(ctx context.Context, r *request.GetManagedObjectStorageBucketMetricsRequest) ([]upcloud.ManagedObjectStorageBucketMetrics, error)

	// Custom domains
	CreateManagedObjectStorageCustomDomain(ctx context.Context, r *request.CreateManagedObjectStorageCustomDomainRequest) error
	GetManagedObjectStorageCustomDomains(ctx context.Context, r *request.GetManagedObjectStorageCustomDomainsRequest) ([]upcloud.ManagedObjectStorageCustomDomain, error)
	GetManagedObjectStorageCustomDomain(ctx context.Context, r *request.GetManagedObjectStorageCustomDomainRequest) (*upcloud.ManagedObjectStorageCustomDomain, error)
	ModifyManagedObjectStorageCustomDomain(ctx context.Context, r *request.ModifyManagedObjectStorageCustomDomainRequest) (*upcloud.ManagedObjectStorageCustomDomain, error)
	DeleteManagedObjectStorageCustomDomain(ctx context.Context, r *request.DeleteManagedObjectStorageCustomDomainRequest) error
}

var _ ObjectStorageAPI = (*service.Service)(nil)
