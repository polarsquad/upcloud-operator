package upcloudapi

import (
	"context"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud"
	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud/request"
	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud/service"
)

// GatewayAPI is the subset of service.Service used by the gateway adapters.
type GatewayAPI interface {
	GetGateways(ctx context.Context, f ...request.QueryFilter) ([]upcloud.Gateway, error)
	GetGateway(ctx context.Context, r *request.GetGatewayRequest) (*upcloud.Gateway, error)
	CreateGateway(ctx context.Context, r *request.CreateGatewayRequest) (*upcloud.Gateway, error)
	ModifyGateway(ctx context.Context, r *request.ModifyGatewayRequest) (*upcloud.Gateway, error)
	DeleteGateway(ctx context.Context, r *request.DeleteGatewayRequest) error

	GetGatewayConnections(ctx context.Context, r *request.GetGatewayConnectionsRequest) ([]upcloud.GatewayConnection, error)
	GetGatewayConnection(ctx context.Context, r *request.GetGatewayConnectionRequest) (*upcloud.GatewayConnection, error)
	CreateGatewayConnection(ctx context.Context, r *request.CreateGatewayConnectionRequest) (*upcloud.GatewayConnection, error)
	ModifyGatewayConnection(ctx context.Context, r *request.ModifyGatewayConnectionRequest) (*upcloud.GatewayConnection, error)
	DeleteGatewayConnection(ctx context.Context, r *request.DeleteGatewayConnectionRequest) error

	GetGatewayConnectionTunnels(ctx context.Context, r *request.GetGatewayConnectionTunnelsRequest) ([]upcloud.GatewayTunnel, error)
	GetGatewayConnectionTunnel(ctx context.Context, r *request.GetGatewayConnectionTunnelRequest) (*upcloud.GatewayTunnel, error)
	CreateGatewayConnectionTunnel(ctx context.Context, r *request.CreateGatewayConnectionTunnelRequest) (*upcloud.GatewayTunnel, error)
	DeleteGatewayConnectionTunnel(ctx context.Context, r *request.DeleteGatewayConnectionTunnelRequest) error
}

var _ GatewayAPI = (*service.Service)(nil)
