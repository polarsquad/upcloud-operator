package fake_test

import (
	"context"
	"sync"
	"testing"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud"
	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud/request"
	"github.com/polarsquad/upcloud-operator/internal/upcloudapi/fake"
)

func TestInspectIsSafeWithConcurrentMutations(t *testing.T) {
	api := fake.NewNetworkAPI()
	const key = "test-network"

	var wg sync.WaitGroup
	wg.Go(func() {
		for range 100 {
			ctx := context.Background()
			_, _ = api.CreateNetwork(ctx, &request.CreateNetworkRequest{
				Name: key,
				Zone: "fi-hel1",
				IPNetworks: []upcloud.IPNetwork{
					{Address: "10.0.0.0/24", DHCP: upcloud.True},
				},
			})
			// Safe here: the only goroutine that writes to Networks is this one; the concurrent Inspect call above only reads.
			nets := api.Networks
			for uuid := range nets {
				_ = api.DeleteNetwork(ctx, &request.DeleteNetworkRequest{UUID: uuid})
			}
		}
	})

	for range 50 {
		api.Inspect(func(a *fake.NetworkAPI) {
			_ = len(a.Networks)
		})
	}
	wg.Wait()
}
