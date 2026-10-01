//go:build e2e_upcloud

package e2e_upcloud

import (
	"context"
	"fmt"
	"os/exec"
	"reflect"
	"strings"
	"testing"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud"

	"github.com/polarsquad/upcloud-operator/internal/upcloudapi"
	"github.com/polarsquad/upcloud-operator/internal/upcloudapi/fake"
)

const ownershipNetworkKind = "network"

func TestCollectRunIdentities(t *testing.T) {
	savedProbes, savedRun := probes, runCmd
	t.Cleanup(func() { probes, runCmd = savedProbes, savedRun })
	const fixture = `{"apiVersion":"v1","kind":"List","items":[
		{
		 "apiVersion":"network.upcloud.polarsquad.com/v1alpha1","kind":"Network",
		 "metadata":{"name":"ready","uid":"cr-ready"},"status":{"uuid":"cloud-ready"}},
		{
		 "apiVersion":"network.upcloud.polarsquad.com/v1alpha1","kind":"Network",
		 "metadata":{"name":"creating-one","uid":"cr-creating-one"}},
		{
		 "apiVersion":"network.upcloud.polarsquad.com/v1alpha1","kind":"Network",
		 "metadata":{"name":"creating-two","uid":"cr-creating-two"},"status":{"uuid":""}}
	]}`
	runCmd = func(cmd *exec.Cmd) (string, error) {
		if len(cmd.Args) != 7 || cmd.Args[1] != "get" || cmd.Args[3] != "-n" ||
			cmd.Args[4] != namespace || cmd.Args[5] != "-o" || cmd.Args[6] != "json" {
			return "", fmt.Errorf("unexpected collection command: %v", cmd.Args)
		}
		if cmd.Args[2] == ownershipNetworkKind {
			return fixture, nil
		}
		return `{"apiVersion":"v1","kind":"List","items":[]}`, nil
	}
	for name, collect := range map[string]func(){"normal": collectProbes, "failure": collectLeakedCRs} {
		t.Run(name, func(t *testing.T) {
			probes = nil
			collect()
			collect() // repeated collection must retain identities without duplicates
			if len(probes) != 3 {
				t.Fatalf("got %d probes, want ready CR and both statusless CRs: %+v", len(probes), probes)
			}
			found := map[string]probe{}
			for _, p := range probes {
				found[p.crUID] = p
			}
			ready := found["cr-ready"]
			if ready.uuid != "cloud-ready" || !ready.verifiable {
				t.Fatalf("external identity was lost or confused with CR UID: %+v", ready)
			}
			for _, uid := range []string{"cr-creating-one", "cr-creating-two"} {
				p, ok := found[uid]
				if !ok || p.uuid != "" || p.verifiable {
					t.Fatalf("statusless CR %s must retain ownership without a verifiable identity: %+v", uid, p)
				}
				p.fetch = func(context.Context, string) error { t.Fatal("empty identity was probed"); return nil }
				probes = []probe{p}
				if gone, detail := allProbesGone(context.Background()); !gone {
					t.Fatalf("statusless probe blocks verification: %s", detail)
				}
			}
		})
	}
	t.Run("failed reads never invent ownership", func(t *testing.T) {
		for _, invalid := range []string{"not-json", ""} {
			probes = nil
			runCmd = func(*exec.Cmd) (string, error) { return invalid, nil }
			collectLeakedCRs()
			if len(probes) != 0 {
				t.Fatalf("invalid output created ownership: %+v", probes)
			}
		}
		probes = nil
		runCmd = func(*exec.Cmd) (string, error) { return fixture, fmt.Errorf("API unavailable") }
		collectProbes()
		if len(probes) != 0 {
			t.Fatalf("failed command created ownership: %+v", probes)
		}
	})
}

func TestSweepRunOwnership(t *testing.T) {
	saved := probes
	t.Cleanup(func() { probes = saved })
	const cloudID, ownerUID = "cloud-resource", "cr-owner"
	cases := []struct {
		name, knownUID, labelUID, manager string
		wantDelete                        bool
	}{
		{"this run", ownerUID, ownerUID, upcloudapi.ManagedByValue, true},
		{"another run", ownerUID, "other-owner", upcloudapi.ManagedByValue, false},
		{"external UUID is not a CR UID", ownerUID, cloudID, upcloudapi.ManagedByValue, false},
		{"missing owner", ownerUID, "", upcloudapi.ManagedByValue, false},
		{"empty allowlist UID", "", "", upcloudapi.ManagedByValue, false},
		{"wrong manager", ownerUID, ownerUID, "another-controller", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			probes = []probe{{kind: ownershipNetworkKind, uuid: cloudID, crUID: tc.knownUID, verifiable: true}}
			api := fake.NewNetworkAPI()
			labels := []upcloud.Label{
				{Key: upcloudapi.LabelManagedBy, Value: tc.manager},
				{Key: upcloudapi.LabelUID, Value: tc.labelUID},
			}
			api.Routers[cloudID] = &upcloud.Router{UUID: cloudID, Labels: labels}
			api.Networks[cloudID] = &upcloud.Network{UUID: cloudID, Labels: labels}
			sweepLabelled(api, func(string) {})
			if tc.wantDelete {
				if len(api.Routers) != 0 || len(api.Networks) != 0 {
					t.Fatalf("run-owned resources were not swept: routers=%v networks=%v", api.Routers, api.Networks)
				}
				want := []string{"GetRouters", "DeleteRouter", "GetNetworks", "DeleteNetwork"}
				if !reflect.DeepEqual(api.Calls, want) {
					t.Fatalf("want router-before-network cleanup: got %v, want %v", api.Calls, want)
				}
			} else if len(api.Routers) != 1 || len(api.Networks) != 1 {
				t.Fatalf("swept resources without proven run ownership: %v", api.Calls)
			}
		})
	}
}

func TestSweepReportsUnprovenOwnership(t *testing.T) {
	saved := probes
	t.Cleanup(func() { probes = saved })

	cases := []struct {
		name        string
		setup       func() ([]probe, *fake.NetworkAPI, *[]string)
		wantFailures int
		checkFn     func(t *testing.T, failures []string, api *fake.NetworkAPI)
	}{
		{
			name: "still present",
			setup: func() ([]probe, *fake.NetworkAPI, *[]string) {
				probes = []probe{
					{kind: "router", uuid: "rt-unproven-1", crUID: ""},
					{kind: ownershipNetworkKind, uuid: "net-unproven-1", crUID: ""},
				}
				api := fake.NewNetworkAPI()
				labels := []upcloud.Label{
					{Key: upcloudapi.LabelManagedBy, Value: upcloudapi.ManagedByValue},
					{Key: upcloudapi.LabelUID, Value: "some-other-uid"},
				}
				api.Routers["rt-unproven-1"] = &upcloud.Router{UUID: "rt-unproven-1", Labels: labels}
				api.Networks["net-unproven-1"] = &upcloud.Network{UUID: "net-unproven-1", Labels: labels}
				var failures []string
				return probes, api, &failures
			},
			wantFailures: 2,
			checkFn: func(t *testing.T, failures []string, api *fake.NetworkAPI) {
				if len(api.Routers) != 1 || len(api.Networks) != 1 {
					t.Fatalf("resources were deleted when they should not be: routers=%v networks=%v", api.Routers, api.Networks)
				}
				foundRouter := false
				foundNetwork := false
				for _, f := range failures {
					if strings.Contains(f, "rt-unproven-1") {
						foundRouter = true
					}
					if strings.Contains(f, "net-unproven-1") {
						foundNetwork = true
					}
				}
				if !foundRouter || !foundNetwork {
					t.Fatalf("expected failures containing resource UUIDs; got: %v", failures)
				}
			},
		},
		{
			name: "already deleted",
			setup: func() ([]probe, *fake.NetworkAPI, *[]string) {
				probes = []probe{
					{kind: "router", uuid: "rt-unproven-1", crUID: ""},
					{kind: ownershipNetworkKind, uuid: "net-unproven-1", crUID: ""},
				}
				api := fake.NewNetworkAPI()
				var failures []string
				return probes, api, &failures
			},
			wantFailures: 0,
		},
		{
			name: "owned by other run",
			setup: func() ([]probe, *fake.NetworkAPI, *[]string) {
				probes = []probe{}
				api := fake.NewNetworkAPI()
				labels := []upcloud.Label{
					{Key: upcloudapi.LabelManagedBy, Value: upcloudapi.ManagedByValue},
					{Key: upcloudapi.LabelUID, Value: "other-run-uid"},
				}
				api.Routers["rt-other-1"] = &upcloud.Router{UUID: "rt-other-1", Labels: labels}
				api.Networks["net-other-1"] = &upcloud.Network{UUID: "net-other-1", Labels: labels}
				var failures []string
				return probes, api, &failures
			},
			wantFailures: 0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, api, failuresPtr := tc.setup()
			sweepLabelled(api, func(msg string) { *failuresPtr = append(*failuresPtr, msg) })

			if len(*failuresPtr) != tc.wantFailures {
				t.Fatalf("got %d failures, want %d: %v", len(*failuresPtr), tc.wantFailures, *failuresPtr)
			}

			if tc.checkFn != nil {
				tc.checkFn(t, *failuresPtr, api)
			}
		})
	}
}
