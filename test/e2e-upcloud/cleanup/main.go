// Command cleanup removes every UpCloud resource the operator labelled
// managed-by=upcloud-operator, plus detached floating IPs in the sample
// zone. The real-API e2e workflow runs it in an always() step after the
// suite, so a suite killed mid-teardown (timeout alarm, cancelled run)
// cannot leak paid resources. It keeps no state from the suite: it selects
// by label alone, which is safe because the test account is dedicated to
// this suite and the workflow serialises runs.
//
// It exits non-zero when anything is still there after the deadline.
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud"
	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud/request"

	"github.com/polarsquad/upcloud-operator/internal/upcloudapi"
)

const (
	sampleZone = "fi-hel1"
	deadline   = 15 * time.Minute
	pollEvery  = 20 * time.Second
)

func main() {
	svc, err := upcloudapi.NewServiceFromEnv()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), deadline)
	defer cancel()

	for {
		left := sweep(ctx, svc)
		if left == 0 {
			fmt.Println("cleanup: nothing left")
			return
		}
		select {
		case <-ctx.Done():
			fmt.Fprintf(os.Stderr, "cleanup: %d resource(s) still present after %s\n", left, deadline)
			os.Exit(1)
		case <-time.After(pollEvery):
		}
	}
}

type sweeper interface {
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

// sweep issues one delete per matching resource, dependants first (managed
// services, then routers, then networks: a network waits on its router), and
// returns how many matches it saw. Delete errors are logged and retried on
// the next pass, since the services delete asynchronously and a network
// delete 409s until they are gone.
func sweep(ctx context.Context, svc sweeper) int {
	seen := 0
	// A list that fails proves nothing is gone, so it counts as a leftover
	// and the next pass retries it.
	listFailed := func(kind string, err error) {
		seen++
		fmt.Fprintf(os.Stderr, "cleanup: list %s: %v\n", kind, err)
	}
	try := func(kind, id string, err error) {
		seen++
		if err != nil && !upcloudapi.IsNotFound(err) {
			fmt.Fprintf(os.Stderr, "cleanup: delete %s %s: %v\n", kind, id, err)
			return
		}
		fmt.Printf("cleanup: deleting %s %s\n", kind, id)
	}

	if dbs, err := svc.GetManagedDatabases(ctx, &request.GetManagedDatabasesRequest{}); err == nil {
		for i := range dbs {
			if hasManagedBy(dbs[i].Labels) {
				try("managed database", dbs[i].UUID,
					svc.DeleteManagedDatabase(ctx, &request.DeleteManagedDatabaseRequest{UUID: dbs[i].UUID}))
			}
		}
	} else {
		listFailed("managed databases", err)
	}
	if oss, err := svc.GetManagedObjectStorages(ctx, &request.GetManagedObjectStoragesRequest{}); err == nil {
		for i := range oss {
			if hasManagedBy(oss[i].Labels) {
				try("managed object storage", oss[i].UUID,
					svc.DeleteManagedObjectStorage(ctx, &request.DeleteManagedObjectStorageRequest{UUID: oss[i].UUID, Force: true}))
			}
		}
	} else {
		listFailed("managed object storages", err)
	}
	if rts, err := svc.GetRouters(ctx); err == nil {
		for i := range rts.Routers {
			if hasManagedBy(rts.Routers[i].Labels) {
				try("router", rts.Routers[i].UUID,
					svc.DeleteRouter(ctx, &request.DeleteRouterRequest{UUID: rts.Routers[i].UUID}))
			}
		}
	} else {
		listFailed("routers", err)
	}
	if nets, err := svc.GetNetworks(ctx); err == nil {
		for i := range nets.Networks {
			if hasManagedBy(nets.Networks[i].Labels) {
				try("network", nets.Networks[i].UUID,
					svc.DeleteNetwork(ctx, &request.DeleteNetworkRequest{UUID: nets.Networks[i].UUID}))
			}
		}
	} else {
		listFailed("networks", err)
	}
	// Same rule as the suite's sweep: only unattached floating IPs in the
	// sample zone, never an attached or non-floating address.
	if ips, err := svc.GetIPAddresses(ctx); err == nil {
		for i := range ips.IPAddresses {
			ip := &ips.IPAddresses[i]
			if ip.Zone == sampleZone && ip.ServerUUID == "" && ip.Floating == upcloud.True {
				try("floating IP", ip.Address,
					svc.ReleaseIPAddress(ctx, &request.ReleaseIPAddressRequest{IPAddress: ip.Address}))
			}
		}
	} else {
		listFailed("IP addresses", err)
	}
	return seen
}

func hasManagedBy(labels []upcloud.Label) bool {
	for _, l := range labels {
		if l.Key == upcloudapi.LabelManagedBy {
			return true
		}
	}
	return false
}
