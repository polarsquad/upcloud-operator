//go:build e2e_upcloud
// +build e2e_upcloud

/*
Copyright 2026 Polar Squad.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package e2e_upcloud

import (
	"bytes"
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/polarsquad/upcloud-operator/internal/upcloudapi"
	"github.com/polarsquad/upcloud-operator/test/e2e-upcloud/leftovers/leftoverstest"
)

// TestPreflightCleanAccount tests preflight on an empty account.
func TestPreflightCleanAccount(t *testing.T) {
	ctx := context.Background()
	account := leftoverstest.NewAccount()

	var out bytes.Buffer
	found, err := preflightLeftovers(ctx, account, &out, 100*time.Millisecond, 10*time.Millisecond)

	assert.Nil(t, err)
	assert.Nil(t, found)
	assert.NotContains(t, out.String(), "ERROR")
}

// TestPreflightRemovableLeftovers tests preflight when leftovers exist but can be cleaned.
func TestPreflightRemovableLeftovers(t *testing.T) {
	ctx := context.Background()
	account := leftoverstest.NewAccount()

	// Create a labelled router and network
	router := &upcloud.Router{
		UUID:   "router-1",
		Labels: []upcloud.Label{{Key: upcloudapi.LabelManagedBy, Value: upcloudapi.ManagedByValue}},
	}
	account.Net.Routers[router.UUID] = router

	network := &upcloud.Network{
		UUID:   "network-1",
		Labels: []upcloud.Label{{Key: upcloudapi.LabelManagedBy, Value: upcloudapi.ManagedByValue}},
	}
	account.Net.Networks[network.UUID] = network

	var out bytes.Buffer
	found, err := preflightLeftovers(ctx, account, &out, 500*time.Millisecond, 10*time.Millisecond)

	// Since the fakes delete immediately, the preflight should succeed
	assert.Nil(t, err)
	require.NotNil(t, found)
	assert.Equal(t, 2, len(found))
	assert.Contains(t, out.String(), "ERROR")
	assert.Contains(t, out.String(), "removed; continuing")

	// Account should be clean
	assert.Equal(t, 0, len(account.Net.Routers))
	assert.Equal(t, 0, len(account.Net.Networks))
}

// TestPreflightIrremovableLeftover tests preflight when a delete fails permanently.
func TestPreflightIrremovableLeftover(t *testing.T) {
	ctx := context.Background()
	account := leftoverstest.NewAccount()

	// Create a labelled router so it will be enumerated into first.Found
	router := &upcloud.Router{
		UUID:   "router-1",
		Labels: []upcloud.Label{{Key: upcloudapi.LabelManagedBy, Value: upcloudapi.ManagedByValue}},
	}
	account.Net.Routers[router.UUID] = router

	// Inject a persistent delete failure for routers
	account.DeleteErr["DeleteRouter"] = fmt.Errorf("permanent delete error")

	var out bytes.Buffer
	// With delete failing, we should get an error and the router should remain
	found, err := preflightLeftovers(ctx, account, &out, 50*time.Millisecond, 10*time.Millisecond)

	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "preflight: previous run leaked")
	// found should contain the router we created
	require.NotNil(t, found)
	assert.Equal(t, 1, len(found))
	assert.Equal(t, "router", found[0].Kind)
	assert.Equal(t, "router-1", found[0].ID)
	// Router should still be in the fake (not deleted)
	assert.Contains(t, account.Net.Routers, router.UUID)
}

// TestPreflightListFailure tests preflight when a list operation fails.
func TestPreflightListFailure(t *testing.T) {
	ctx := context.Background()
	account := leftoverstest.NewAccount()

	// Inject a list failure
	testErr := fmt.Errorf("list routers failed")
	account.ListErr["GetRouters"] = testErr

	var out bytes.Buffer
	found, err := preflightLeftovers(ctx, account, &out, 100*time.Millisecond, 10*time.Millisecond)

	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "preflight: previous run leaked")
	// The error message should reference list failures
	assert.Contains(t, err.Error(), "list failures")
	// found will be empty (not nil) when there are list failures but no found resources
	require.NotNil(t, found)
	assert.Equal(t, 0, len(found))
}
