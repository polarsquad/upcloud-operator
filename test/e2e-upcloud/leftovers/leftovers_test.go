package leftovers

import (
	"bytes"
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud"
	"github.com/stretchr/testify/assert"

	"github.com/polarsquad/upcloud-operator/internal/upcloudapi"
	"github.com/polarsquad/upcloud-operator/test/e2e-upcloud/leftovers/leftoverstest"
)

func TestSweepEmptyAccount(t *testing.T) {
	ctx := context.Background()
	account := leftoverstest.NewAccount()

	var out, errOut bytes.Buffer
	pass := Sweep(ctx, account, &out, &errOut)

	assert.Equal(t, 0, pass.Left(), "empty account should have nothing left")
	assert.Equal(t, 0, len(pass.Found))
	assert.Equal(t, 0, pass.Deleted)
	assert.Equal(t, 0, pass.ListFailures)
}

func TestSweepSelection(t *testing.T) {
	ctx := context.Background()
	account := leftoverstest.NewAccount()

	// Create a labelled database that should be deleted
	labelledDB := &upcloud.ManagedDatabase{
		UUID: "db-labelled",
		Labels: []upcloud.Label{
			{Key: upcloudapi.LabelManagedBy, Value: upcloudapi.ManagedByValue},
		},
	}
	account.DB.Databases[labelledDB.UUID] = labelledDB

	// Create an unlabelled database that should not be touched
	unlabelledDB := &upcloud.ManagedDatabase{
		UUID: "db-unlabelled",
	}
	account.DB.Databases[unlabelledDB.UUID] = unlabelledDB

	// Create a labelled object storage that should be deleted
	labelledOS := &upcloud.ManagedObjectStorage{
		UUID: "os-labelled",
		Labels: []upcloud.Label{
			{Key: upcloudapi.LabelManagedBy, Value: upcloudapi.ManagedByValue},
		},
	}
	account.OS.Services[labelledOS.UUID] = labelledOS

	// Create a labelled router that should be deleted
	labelledRouter := &upcloud.Router{
		UUID: "router-labelled",
		Labels: []upcloud.Label{
			{Key: upcloudapi.LabelManagedBy, Value: upcloudapi.ManagedByValue},
		},
	}
	account.Net.Routers[labelledRouter.UUID] = labelledRouter

	// Create a labelled network that should be deleted
	labelledNetwork := &upcloud.Network{
		UUID: "network-labelled",
		Labels: []upcloud.Label{
			{Key: upcloudapi.LabelManagedBy, Value: upcloudapi.ManagedByValue},
		},
	}
	account.Net.Networks[labelledNetwork.UUID] = labelledNetwork

	// Create detached floating IPs
	detachedFloatingIP := &upcloud.IPAddress{
		Address:    "1.2.3.4",
		Zone:       SampleZone,
		ServerUUID: "",
		Floating:   upcloud.True,
	}
	account.Net.IPs[detachedFloatingIP.Address] = detachedFloatingIP

	// Create attached floating IP (should not be deleted)
	attachedFloatingIP := &upcloud.IPAddress{
		Address:    "1.2.3.5",
		Zone:       SampleZone,
		ServerUUID: "server-uuid",
		Floating:   upcloud.True,
	}
	account.Net.IPs[attachedFloatingIP.Address] = attachedFloatingIP

	// Create detached floating IP in different zone (should not be deleted)
	otherZoneIP := &upcloud.IPAddress{
		Address:    "1.2.3.6",
		Zone:       "de-fra1",
		ServerUUID: "",
		Floating:   upcloud.True,
	}
	account.Net.IPs[otherZoneIP.Address] = otherZoneIP

	var out, errOut bytes.Buffer
	pass := Sweep(ctx, account, &out, &errOut)

	// Should find 5 resources: db, os, router, network, and the detached floating IP
	assert.Equal(t, 5, len(pass.Found))
	assert.Equal(t, 5, pass.Deleted)
	assert.Equal(t, 0, pass.ListFailures)

	// Verify all found resources are listed
	foundKinds := make(map[string]bool)
	foundIDs := make(map[string]bool)
	for _, r := range pass.Found {
		foundKinds[r.Kind] = true
		foundIDs[r.ID] = true
	}

	assert.True(t, foundKinds["managed database"])
	assert.True(t, foundKinds["managed object storage"])
	assert.True(t, foundKinds["router"])
	assert.True(t, foundKinds["network"])
	assert.True(t, foundKinds["floating IP"])

	assert.True(t, foundIDs["db-labelled"])
	assert.True(t, foundIDs["os-labelled"])
	assert.True(t, foundIDs["router-labelled"])
	assert.True(t, foundIDs["network-labelled"])
	assert.True(t, foundIDs["1.2.3.4"])

	// Verify that unlabelled resources were not deleted
	assert.NotNil(t, account.DB.Databases[unlabelledDB.UUID])
	assert.NotNil(t, account.Net.IPs[attachedFloatingIP.Address])
	assert.NotNil(t, account.Net.IPs[otherZoneIP.Address])
}

func TestSweepOrder(t *testing.T) {
	const testRouter = "router"

	account := leftoverstest.NewAccount()

	// Create resources in random order
	labelledDB := &upcloud.ManagedDatabase{
		UUID:   "db",
		Labels: []upcloud.Label{{Key: upcloudapi.LabelManagedBy}},
	}
	account.DB.Databases[labelledDB.UUID] = labelledDB

	labelledOS := &upcloud.ManagedObjectStorage{
		UUID:   "os",
		Labels: []upcloud.Label{{Key: upcloudapi.LabelManagedBy}},
	}
	account.OS.Services[labelledOS.UUID] = labelledOS

	labelledRouter := &upcloud.Router{
		UUID:   testRouter,
		Labels: []upcloud.Label{{Key: upcloudapi.LabelManagedBy}},
	}
	account.Net.Routers[labelledRouter.UUID] = labelledRouter

	labelledNetwork := &upcloud.Network{
		UUID:   "network",
		Labels: []upcloud.Label{{Key: upcloudapi.LabelManagedBy}},
	}
	account.Net.Networks[labelledNetwork.UUID] = labelledNetwork

	labelledIP := &upcloud.IPAddress{
		Address:    "1.2.3.4",
		Zone:       SampleZone,
		ServerUUID: "",
		Floating:   upcloud.True,
	}
	account.Net.IPs[labelledIP.Address] = labelledIP

	var out, errOut bytes.Buffer
	pass := Sweep(context.Background(), account, &out, &errOut)

	// Just verify all 5 resources were found and deleted
	assert.Equal(t, 5, len(pass.Found))
	assert.Equal(t, 5, pass.Deleted)
}

func TestSweepListFailure(t *testing.T) {
	const testRouter = "router"

	account := leftoverstest.NewAccount()

	// Inject a list failure
	testErr := fmt.Errorf("list routers failed")
	account.ListErr["GetRouters"] = testErr

	// Add a labelled router that won't be found due to list failure
	labelledRouter := &upcloud.Router{
		UUID:   testRouter,
		Labels: []upcloud.Label{{Key: upcloudapi.LabelManagedBy}},
	}
	account.Net.Routers[labelledRouter.UUID] = labelledRouter

	var out, errOut bytes.Buffer
	pass := Sweep(context.Background(), account, &out, &errOut)

	// Should count the list failure as a leftover
	assert.Equal(t, 1, pass.ListFailures)
	// Found should still be empty since we never got to enumerate
	assert.Equal(t, 0, len(pass.Found))
	assert.Equal(t, 1, pass.Left())

	// Error should be logged
	assert.Contains(t, errOut.String(), "list routers")
}

func TestDrainConverges(t *testing.T) {
	account := leftoverstest.NewAccount()

	// Create a resource that will be deleted on the first pass
	resource := &upcloud.Router{
		UUID:   "router",
		Labels: []upcloud.Label{{Key: upcloudapi.LabelManagedBy}},
	}
	account.Net.Routers[resource.UUID] = resource

	var out, errOut bytes.Buffer

	// Use a short deadline so we don't wait forever
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	first, last := Drain(ctx, account, &out, &errOut, 10*time.Millisecond)

	// First pass should find the resource
	assert.Equal(t, 1, len(first.Found))
	assert.Equal(t, 1, first.Deleted)

	// Drain should converge to zero resources
	assert.Equal(t, 0, last.Left(), "drain should converge to zero resources")
}

func TestDrainStopsAtDeadline(t *testing.T) {
	account := leftoverstest.NewAccount()

	// Create a resource
	resource := &upcloud.Router{
		UUID:   "router",
		Labels: []upcloud.Label{{Key: upcloudapi.LabelManagedBy}},
	}
	account.Net.Routers[resource.UUID] = resource

	// Inject a persistent list failure so resources are always "left"
	account.ListErr["GetRouters"] = fmt.Errorf("list failed")

	var out, errOut bytes.Buffer

	// Use a very short deadline
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	first, last := Drain(ctx, account, &out, &errOut, 10*time.Millisecond)

	// First pass should have a list failure
	assert.Equal(t, 1, first.ListFailures)
	// After deadline, we should still have leftovers due to list failure
	assert.Equal(t, 1, last.ListFailures)
}
