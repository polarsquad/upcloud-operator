package leftovers

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud"
	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud/request"

	"github.com/polarsquad/upcloud-operator/internal/upcloudapi"
)

const SampleZone = "fi-hel1"

// API is a narrow interface for sweep operations.
type API interface {
	GetManagedDatabases(context.Context, *request.GetManagedDatabasesRequest) ([]upcloud.ManagedDatabase, error)
	DeleteManagedDatabase(context.Context, *request.DeleteManagedDatabaseRequest) error
	GetManagedObjectStorages(
		context.Context, *request.GetManagedObjectStoragesRequest,
	) ([]upcloud.ManagedObjectStorage, error)
	DeleteManagedObjectStorage(context.Context, *request.DeleteManagedObjectStorageRequest) error
	GetRouters(context.Context, ...request.QueryFilter) (*upcloud.Routers, error)
	DeleteRouter(context.Context, *request.DeleteRouterRequest) error
	GetNetworks(context.Context, ...request.QueryFilter) (*upcloud.Networks, error)
	DeleteNetwork(context.Context, *request.DeleteNetworkRequest) error
	GetIPAddresses(context.Context) (*upcloud.IPAddresses, error)
	ReleaseIPAddress(context.Context, *request.ReleaseIPAddressRequest) error
}

// Resource describes a leftover resource found during a sweep pass.
type Resource struct {
	Kind string
	ID   string
}

func (r Resource) String() string {
	return fmt.Sprintf("%s %s", r.Kind, r.ID)
}

// Pass describes the results of a single sweep pass.
type Pass struct {
	Found        []Resource
	Deleted      int
	ListFailures int
}

// Left returns the number of resources not yet confirmed gone.
// Includes both found resources and list failures.
func (p Pass) Left() int {
	return len(p.Found) + p.ListFailures
}

// Sweep issues one delete per matching resource, dependants first (managed
// services, then routers, then networks: a network waits on its router), and
// returns the results of the pass. Delete errors are logged and retried on
// the next pass, since the services delete asynchronously and a network
// delete 409s until they are gone. Resources are appended to Found for each
// matching resource, even if the delete is accepted.
func Sweep(ctx context.Context, api API, out, errOut io.Writer) Pass {
	pass := Pass{Found: []Resource{}}

	// A list that fails proves nothing is gone, so it counts as a leftover
	// and the next pass retries it.
	listFailed := func(kind string, err error) {
		pass.ListFailures++
		_, _ = fmt.Fprintf(errOut, "cleanup: list %s: %v\n", kind, err)
	}
	try := func(kind, id string, err error) {
		pass.Found = append(pass.Found, Resource{Kind: kind, ID: id})
		if err != nil && !upcloudapi.IsNotFound(err) {
			_, _ = fmt.Fprintf(errOut, "cleanup: delete %s %s: %v\n", kind, id, err)
			return
		}
		// An accepted delete (or a delete that found nothing) means the
		// resource is converging on the UpCloud side, not stuck.
		if err == nil {
			pass.Deleted++
		}
		_, _ = fmt.Fprintf(out, "cleanup: deleting %s %s\n", kind, id)
	}

	if dbs, err := api.GetManagedDatabases(ctx, &request.GetManagedDatabasesRequest{}); err == nil {
		for i := range dbs {
			if managedBy(dbs[i].Labels) {
				try("managed database", dbs[i].UUID,
					api.DeleteManagedDatabase(ctx, &request.DeleteManagedDatabaseRequest{UUID: dbs[i].UUID}))
			}
		}
	} else {
		listFailed("managed databases", err)
	}
	if oss, err := api.GetManagedObjectStorages(ctx, &request.GetManagedObjectStoragesRequest{}); err == nil {
		for i := range oss {
			if managedBy(oss[i].Labels) {
				try("managed object storage", oss[i].UUID,
					api.DeleteManagedObjectStorage(ctx, &request.DeleteManagedObjectStorageRequest{UUID: oss[i].UUID, Force: true}))
			}
		}
	} else {
		listFailed("managed object storages", err)
	}
	if rts, err := api.GetRouters(ctx); err == nil {
		for i := range rts.Routers {
			if managedBy(rts.Routers[i].Labels) {
				try("router", rts.Routers[i].UUID,
					api.DeleteRouter(ctx, &request.DeleteRouterRequest{UUID: rts.Routers[i].UUID}))
			}
		}
	} else {
		listFailed("routers", err)
	}
	if nets, err := api.GetNetworks(ctx); err == nil {
		for i := range nets.Networks {
			if managedBy(nets.Networks[i].Labels) {
				try("network", nets.Networks[i].UUID,
					api.DeleteNetwork(ctx, &request.DeleteNetworkRequest{UUID: nets.Networks[i].UUID}))
			}
		}
	} else {
		listFailed("networks", err)
	}
	// Same rule as the suite's sweep: only unattached floating IPs in the
	// sample zone, never an attached or non-floating address.
	if ips, err := api.GetIPAddresses(ctx); err == nil {
		for i := range ips.IPAddresses {
			ip := &ips.IPAddresses[i]
			if ip.Zone == SampleZone && ip.ServerUUID == "" && ip.Floating == upcloud.True {
				try("floating IP", ip.Address,
					api.ReleaseIPAddress(ctx, &request.ReleaseIPAddressRequest{IPAddress: ip.Address}))
			}
		}
	} else {
		listFailed("IP addresses", err)
	}
	return pass
}

// Drain polls the API until all resources are gone or the context expires,
// returning both the first pass (initial state) and the last pass (final state).
// The every parameter controls the polling interval between passes.
func Drain(ctx context.Context, api API, out, errOut io.Writer, every time.Duration) (first, last Pass) {
	first = Sweep(ctx, api, out, errOut)
	last = first
	if first.Left() == 0 {
		return first, last
	}

	for {
		select {
		case <-ctx.Done():
			return first, last
		case <-time.After(every):
			last = Sweep(ctx, api, out, errOut)
			if last.Left() == 0 {
				return first, last
			}
		}
	}
}

// managedBy checks if a resource carries the managed-by label (key-only match).
func managedBy(labels upcloud.LabelSlice) bool {
	for _, l := range labels {
		if l.Key == upcloudapi.LabelManagedBy {
			return true
		}
	}
	return false
}
