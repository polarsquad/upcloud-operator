package upcloudapi

import (
	"context"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud"
	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud/request"
	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud/service"
)

// NetworkAPI is the subset of service.Service used by the network adapters.
type NetworkAPI interface {
	GetNetworks(ctx context.Context, f ...request.QueryFilter) (*upcloud.Networks, error)
	GetNetworkDetails(ctx context.Context, r *request.GetNetworkDetailsRequest) (*upcloud.Network, error)
	CreateNetwork(ctx context.Context, r *request.CreateNetworkRequest) (*upcloud.Network, error)
	ModifyNetwork(ctx context.Context, r *request.ModifyNetworkRequest) (*upcloud.Network, error)
	DeleteNetwork(ctx context.Context, r *request.DeleteNetworkRequest) error
	AttachNetworkRouter(ctx context.Context, r *request.AttachNetworkRouterRequest) error
	DetachNetworkRouter(ctx context.Context, r *request.DetachNetworkRouterRequest) error
	GetRouters(ctx context.Context, f ...request.QueryFilter) (*upcloud.Routers, error)
	GetRouterDetails(ctx context.Context, r *request.GetRouterDetailsRequest) (*upcloud.Router, error)
	CreateRouter(ctx context.Context, r *request.CreateRouterRequest) (*upcloud.Router, error)
	ModifyRouter(ctx context.Context, r *request.ModifyRouterRequest) (*upcloud.Router, error)
	DeleteRouter(ctx context.Context, r *request.DeleteRouterRequest) error
	GetIPAddresses(ctx context.Context) (*upcloud.IPAddresses, error)
	GetIPAddressDetails(ctx context.Context, r *request.GetIPAddressDetailsRequest) (*upcloud.IPAddress, error)
	AssignIPAddress(ctx context.Context, r *request.AssignIPAddressRequest) (*upcloud.IPAddress, error)
	ModifyIPAddress(ctx context.Context, r *request.ModifyIPAddressRequest) (*upcloud.IPAddress, error)
	ReleaseIPAddress(ctx context.Context, r *request.ReleaseIPAddressRequest) error
	GetNetworkPeerings(ctx context.Context, f ...request.QueryFilter) (upcloud.NetworkPeerings, error)
	GetNetworkPeering(ctx context.Context, r *request.GetNetworkPeeringRequest) (*upcloud.NetworkPeering, error)
	CreateNetworkPeering(ctx context.Context, r *request.CreateNetworkPeeringRequest) (*upcloud.NetworkPeering, error)
	ModifyNetworkPeering(ctx context.Context, r *request.ModifyNetworkPeeringRequest) (*upcloud.NetworkPeering, error)
	DeleteNetworkPeering(ctx context.Context, r *request.DeleteNetworkPeeringRequest) error
}

var _ NetworkAPI = (*service.Service)(nil)
