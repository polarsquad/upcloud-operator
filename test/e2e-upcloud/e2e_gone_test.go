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
	"context"
	"net/http"
	"testing"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud"
)

// TestAllProbesGone covers the waitForGone poll predicate without the
// Ginkgo suite or an API client: probes are package state, fetches are
// stubs. Guards the run 35237159467 false positive, where a
// non-verifiable probe's nil fetch was read as "still exists" and the
// poll could never pass.
func TestAllProbesGone(t *testing.T) {
	saved := probes
	t.Cleanup(func() { probes = saved })

	nonVerifiable := probe{
		kind:       "objectstorageaccesskey",
		uuid:       "user-1",
		verifiable: false,
		fetch:      func(context.Context, string) error { return nil },
	}
	stillThere := probe{
		kind:       "router",
		uuid:       "uuid-1",
		verifiable: true,
		fetch:      func(context.Context, string) error { return nil },
	}
	gone := probe{
		kind:       "network",
		uuid:       "uuid-2",
		verifiable: true,
		fetch:      func(context.Context, string) error { return &upcloud.Problem{Status: http.StatusNotFound} },
	}

	t.Run("a non-verifiable probe does not block the poll", func(t *testing.T) {
		probes = []probe{nonVerifiable}
		ok, detail := allProbesGone(context.Background())
		if !ok {
			t.Fatalf("non-verifiable probe blocked the poll: %s", detail)
		}
	})

	t.Run("a verifiable probe with a live resource blocks and names it", func(t *testing.T) {
		probes = []probe{stillThere}
		ok, detail := allProbesGone(context.Background())
		if ok {
			t.Fatal("verifiable probe with a live resource passed the poll")
		}
		if detail == "" {
			t.Fatal("poll gave no detail for the resource still present")
		}
	})

	t.Run("a verifiable probe that 404s passes", func(t *testing.T) {
		probes = []probe{gone}
		ok, detail := allProbesGone(context.Background())
		if !ok {
			t.Fatalf("404 probe blocked the poll: %s", detail)
		}
	})

	t.Run("a leading non-verifiable probe does not skip later verifiable ones", func(t *testing.T) {
		probes = []probe{nonVerifiable, gone}
		ok, detail := allProbesGone(context.Background())
		if !ok {
			t.Fatalf("mixed probes blocked the poll: %s", detail)
		}
		probes = []probe{nonVerifiable, stillThere}
		ok, _ = allProbesGone(context.Background())
		if ok {
			t.Fatal("mixed probes passed while a verifiable resource is still present")
		}
	})
}
