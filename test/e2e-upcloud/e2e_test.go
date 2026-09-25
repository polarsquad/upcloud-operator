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
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud"
	upcloudrequest "github.com/UpCloudLtd/upcloud-go-api/v8/upcloud/request"
	upcloudsvc "github.com/UpCloudLtd/upcloud-go-api/v8/upcloud/service"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/polarsquad/upcloud-operator/internal/upcloudapi"
	"github.com/polarsquad/upcloud-operator/test/e2e-upcloud/leftovers"
)

// This suite runs against the REAL UpCloud API and costs money (it
// provisions a database for roughly 20 minutes). It is gated on
// UPCLOUD_TOKEN: when the variable is unset the whole suite is skipped.
// It is dispatched only from CI via the "e2e-upcloud" workflow
// (workflow_dispatch, environment: upcloud-e2e).

const (
	operatorNS = "uck-system"
	// sampleZone is the zone the samples allocate networks and floating
	// IPs in; the detached-floating-IP sweep only releases addresses in it.
	sampleZone = leftovers.SampleZone
)

// crsGoneBudget covers Managed Object Storage delete latency, which runs the
// same slow state machine as its create (over 20m observed).
const crsGoneBudget = 30 * time.Minute

// preflightBudget limits how long the preflight check waits for cleanup.
const preflightBudget = crsGoneBudget

// preflightPoll is the interval between preflight sweep passes.
const preflightPoll = 20 * time.Second

var (
	managerImage = "example.com/uck:e2e-upcloud"
	runID        = fmt.Sprintf("%d", time.Now().Unix())
	namespace    = fmt.Sprintf("upcloud-e2e-%s", runID)
	kindCluster  = fmt.Sprintf("upcloud-e2e-%s", runID)
	svc          *upcloudsvc.Service
	probes       []probe
	// crdsInstalled is set once BeforeSuite has installed the CRDs. Until
	// then there are no CRs to delete or wait for, and every kubectl get
	// of a managed kind fails.
	crdsInstalled bool
	// crsWaitSpent is set once the spec has run its own waitForCRsGone, so
	// AfterSuite does not spend the same budget a second time.
	crsWaitSpent bool
	repoRoot     string
	runCmd       = runCommand
	// diagDir collects diagnostics for upload on failure. Created lazily
	// by dumpDiagnostics; empty until then.
	diagDir string
)

// resolveRepoRoot walks up from the package directory to the repository
// root (the directory containing the Makefile). go test runs the suite in
// the package directory, but the Make targets and the kustomize bases /
// sample manifests use paths relative to the root, so every external
// command must be anchored there.
func resolveRepoRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		Fail(fmt.Sprintf("cannot resolve the working directory: %v", err))
	}
	for d := dir; d != string(os.PathSeparator); d = filepath.Dir(d) {
		if _, err := os.Stat(filepath.Join(d, "Makefile")); err == nil {
			return d
		}
	}
	Fail(fmt.Sprintf("no Makefile found above %s; cannot locate the repository root", dir))
	return ""
}

// probe records one UpCloud resource created in this run and how to check
// whether it still exists, so teardown can verify it is really gone.
// verifiable marks kinds the API can confirm individually; waitForGone
// skips the rest (their fetch is a no-op, and a nil error from it is not
// a 404).
type probe struct {
	kind       string
	uuid       string
	crUID      string // metadata.uid of the owning CR; the k8s-uid label value
	verifiable bool
	fetch      func(context.Context, string) error // returns the Get* call's error
}

// preflightLeftovers checks the UpCloud account for leftovers from a previous run
// and cleans them up if found. It returns the list of found resources and any
// error encountered. If resources are found but successfully cleaned up within
// the budget, it returns the found list with nil error and logs to w.
func preflightLeftovers(ctx context.Context, api leftovers.API, w io.Writer, budget, every time.Duration) ([]leftovers.Resource, error) {
	ctx, cancel := context.WithTimeout(ctx, budget)
	defer cancel()

	first, last := leftovers.Drain(ctx, api, w, w, every)

	if first.Left() == 0 {
		return nil, nil
	}

	// Resources were found; report and check if they were cleaned up.
	msg := fmt.Sprintf("ERROR: preflight: a previous run leaked %d UpCloud resource(s): ", len(first.Found))
	for i, r := range first.Found {
		if i > 0 {
			msg += ", "
		}
		msg += r.String()
	}
	if first.ListFailures > 0 {
		msg += fmt.Sprintf(" (%d list failure(s); see stderr for details)", first.ListFailures)
	}
	_, _ = fmt.Fprintln(w, msg)

	if last.Left() == 0 {
		_, _ = fmt.Fprintln(w, "preflight: leftovers removed; continuing")
		return first.Found, nil
	}

	return first.Found, fmt.Errorf("preflight: previous run leaked UpCloud resources that could not be removed within %v: %v (list failures: %d); run `go run ./test/e2e-upcloud/cleanup` and dispatch again", budget, last.Found, last.ListFailures)
}

func TestE2EUpCloud(t *testing.T) {
	RegisterFailHandler(Fail)
	_, _ = fmt.Fprintf(GinkgoWriter, "Starting uck real-API e2e suite (run %s)\n", runID)
	RunSpecs(t, "e2e-upcloud suite")
}

var _ = BeforeSuite(func() {
	repoRoot = resolveRepoRoot()

	if os.Getenv("UPCLOUD_TOKEN") == "" {
		Skip("UPCLOUD_TOKEN is not set; skipping the real-API e2e suite")
	}

	var err error
	client, err := upcloudapi.NewServiceFromEnv()
	Expect(err).NotTo(HaveOccurred(), "real UpCloud client should be built from UPCLOUD_TOKEN")

	By("checking the UpCloud account for leftovers from a previous run")
	found, err := preflightLeftovers(context.Background(), client, GinkgoWriter, preflightBudget, preflightPoll)
	if len(found) > 0 {
		AddReportEntry("preflight leftovers", found)
	}
	Expect(err).NotTo(HaveOccurred(), "preflight check")
	svc = client

	By("building the manager image")
	cmd := exec.Command("make", "docker-build", fmt.Sprintf("IMG=%s", managerImage))
	_, err = runCmd(cmd)
	Expect(err).NotTo(HaveOccurred(), "Failed to build the manager image")

	By("creating a dedicated kind cluster")
	cmd = exec.Command("kind", "create", "cluster", "--name", kindCluster)
	_, err = runCmd(cmd)
	Expect(err).NotTo(HaveOccurred(), "Failed to create kind cluster")

	By("loading the manager image into kind")
	cmd = exec.Command("kind", "load", "docker-image", managerImage, "--name", kindCluster)
	_, err = runCmd(cmd)
	Expect(err).NotTo(HaveOccurred(), "Failed to load image into kind")

	By("installing the CRDs")
	cmd = exec.Command("make", "install")
	_, err = runCmd(cmd)
	Expect(err).NotTo(HaveOccurred(), "Failed to install CRDs")
	crdsInstalled = true

	By("creating the operator namespace and the real credentials secret")
	cmd = exec.Command("kubectl", "apply", "-f", "-")
	cmd.Stdin = stdinReader(renderCredentialsSecret())
	_, err = runCmd(cmd)
	Expect(err).NotTo(HaveOccurred(), "Failed to create the operator namespace and credentials secret")

	By("deploying the operator")
	cmd = exec.Command("make", "deploy", fmt.Sprintf("IMG=%s", managerImage))
	_, err = runCmd(cmd)
	Expect(err).NotTo(HaveOccurred(), "Failed to deploy the operator")

	By("creating the run namespace")
	cmd = exec.Command("kubectl", "create", "namespace", namespace)
	_, err = runCmd(cmd)
	Expect(err).NotTo(HaveOccurred(), "Failed to create run namespace")
})

var _ = AfterSuite(func() {
	if svc == nil {
		return
	}
	// Best-effort teardown: always remove the real UpCloud resources and
	// the kind cluster, even if a spec failed halfway through. The
	// collection pass runs BEFORE the sample deletes: a spec that dies
	// before collectProbes (the normal failure path) leaves no probes, and
	// without this pass the sweeps and the gone-verification would iterate
	// an empty list while real resources from the failed spec are still
	// being provisioned. Sweeps report what they had to remove; the suite
	// is failed once, at the end.
	var failures []string
	recordFailure := func(message string) { failures = append(failures, message) }
	// A failed Eventually aborts this node, which would skip the sweeps and
	// the cluster teardown, so the waits record their timeout here instead.
	record := NewGomega(func(message string, _ ...int) { recordFailure(message) })
	By("collecting identities from every managed CR still in the run namespace")
	collectLeakedCRs()
	By("dumping diagnostics while the cluster is still up")
	dumpDiagnostics()
	if crdsInstalled {
		By("requesting non-blocking deletion of the applied samples")
		deleteAllSamples()
		if crsWaitSpent {
			By("skipping the CR wait: the spec already spent its budget")
		} else {
			By("waiting for the CRs to be removed (finalizers drive the real delete)")
			waitForCRsGone(record, crsGoneBudget)
		}
	} else {
		// Every get of a managed kind would fail and the wait would spend
		// its whole budget on nothing.
		By("skipping the CR deletes and wait: setup never installed the CRDs")
	}

	By("sweeping any leftover UpCloud resource labelled by this run")
	sweepLabelled(svc, recordFailure)
	By("sweeping any detached floating IP left in the sample zone")
	sweepDetachedFloatingIPs(recordFailure)
	By("verifying the UpCloud API confirms everything is gone")
	waitForGone(record, 5*time.Minute)
	By("deleting the kind cluster")
	_, _ = runCmd(exec.Command("kind", "delete", "cluster", "--name", kindCluster))

	if len(failures) > 0 {
		Fail("the operator left resources behind:\n" + strings.Join(failures, "\n"))
	}
})

var _ = Describe("real UpCloud API reconciliation", Ordered, func() {
	It("provisions a router, network, object storage chain and a database, then tears them all down", Label("real-api"), func() {
		By("waiting for the controller to be ready")
		waitForControllerReady()

		By("applying the samples (dependencies first)")
		samples := []string{
			"config/samples/network_v1alpha1_router.yaml",
			"config/samples/network_v1alpha1_network.yaml",
			"config/samples/network_v1alpha1_floatingip.yaml",
			"config/samples/objectstorage_v1alpha1_managedobjectstorage.yaml",
			"config/samples/objectstorage_v1alpha1_objectstoragepolicy.yaml",
			"config/samples/objectstorage_v1alpha1_objectstorageuser.yaml",
			"config/samples/objectstorage_v1alpha1_objectstorageaccesskey.yaml",
			"config/samples/database_v1alpha1_manageddatabase.yaml",
		}
		for _, f := range samples {
			cmd := exec.Command("kubectl", "apply", "-f", f, "-n", namespace)
			_, err := runCmd(cmd)
			Expect(err).NotTo(HaveOccurred(), "apply %s", f)
		}

		By("waiting for the network and object storage chains to be Ready")
		waitReady("router", "router-sample", 5*time.Minute)
		waitReady("network", "network-sample", 5*time.Minute)
		waitReady("floatingip", "floatingip-sample", 5*time.Minute)
		// setup-checkup takes ~13-15m; see docs/development.md.
		waitReady("managedobjectstorage", "managedobjectstorage-sample", 20*time.Minute)
		waitReady("objectstoragepolicy", "objectstoragepolicy-sample", 5*time.Minute)
		waitReady("objectstorageuser", "objectstorageuser-sample", 5*time.Minute)
		waitReady("objectstorageaccesskey", "objectstorageaccesskey-sample", 5*time.Minute)

		By("waiting for the database to be Ready (up to 15 minutes)")
		waitReady("manageddatabase", "manageddatabase-sample", 15*time.Minute)

		By("verifying the operator produced the access key S3 secret")
		out, err := runCmd(exec.Command("kubectl", "get", "secret", "objectstorageaccesskey-sample-s3",
			"-n", namespace, "-o", "jsonpath={.data.AWS_ACCESS_KEY_ID}"))
		Expect(err).NotTo(HaveOccurred(), "access key S3 secret should exist")
		Expect(out).NotTo(BeEmpty())

		By("verifying the operator produced the database connection secret")
		out, err = runCmd(exec.Command("kubectl", "get", "secret", "manageddatabase-sample-connection",
			"-n", namespace, "-o", "jsonpath={.data.uri}"))
		Expect(err).NotTo(HaveOccurred(), "database connection secret should exist")
		Expect(out).NotTo(BeEmpty())

		By("collecting the UpCloud identities created in this run")
		collectProbes()

		By("requesting non-blocking deletion of all samples")
		deleteAllSamples()

		By("waiting for the CRs to be removed (finalizers drive the real delete)")
		crsWaitSpent = true
		waitForCRsGone(Default, crsGoneBudget)

		By("verifying the UpCloud API confirms everything is gone")
		waitForGone(Default, 5*time.Minute)
	})
})

// deleteAllSamples requests deletion of every managed CR in the run namespace
// by kind. Kind-based rather than sample-file-based so CRs a failed spec
// created outside the sample set are still deleted. The request order is only
// a hint: non-blocking deletes overlap, and parents can start deleting while
// children still exist. Finalizer retries, not request order, drive convergence.
//
// The deletes are non-blocking (--wait=false): a blocking delete could park on
// a finalizer before the loop requests deletion of the resource blocking it.
// waitForCRsGone is the single place that waits for finalizer drain.
func deleteAllSamples() {
	for _, k := range managedKinds {
		logCmdErr(runCmd(exec.Command("kubectl", "delete", k.name, "-n", namespace,
			"--ignore-not-found=true", "--all=true", "--wait=false")))
	}
}

// logCmdErr writes a failed command's error to the Ginkgo log so a stuck
// teardown says why instead of only timing out.
func logCmdErr(_ string, err error) {
	if err != nil {
		_, _ = fmt.Fprintln(GinkgoWriter, err)
	}
}

// renderCredentialsSecret emits the operator namespace and the real
// credentials secret in a single multi-document YAML. The deployment reads
// the secret from operatorNS via envFrom, so it must live there, not in the
// per-run namespace.
func renderCredentialsSecret() string {
	return fmt.Sprintf(`apiVersion: v1
kind: Namespace
metadata:
  name: %s
---
apiVersion: v1
kind: Secret
metadata:
  name: upcloud-credentials
  namespace: %s
type: Opaque
stringData:
  UPCLOUD_TOKEN: %s
`, operatorNS, operatorNS, os.Getenv("UPCLOUD_TOKEN"))
}

// collectProbes uses the same run-scoped collection as failure teardown.
// This preserves CR UIDs separately from cloud identities in both paths.
func collectProbes() {
	collectLeakedCRs()
}

// waitForCRsGone blocks until every managed kind has zero CRs left in the
// run namespace (its finalizer ran, which is what performs the real
// UpCloud delete). Each poll re-issues the non-blocking deletes: re-deleting
// a CR that is already terminating is a no-op, but a CR the single delete
// pass missed (a late-failing spec, or a pass that died mid-loop) gets its
// deletion request (re)issued here instead of waiting out the clock.
// Presence is read via jsonpath: `kubectl get <kind>` exits 0 even for an
// empty list, and a kubectl failure (missing CRD, API blip) would
// otherwise be indistinguishable from "gone".
func waitForCRsGone(g Gomega, timeout time.Duration) {
	g.Eventually(func() bool {
		allGone := true
		for _, k := range managedKinds {
			logCmdErr(runCmd(exec.Command("kubectl", "delete", k.name, "-n", namespace,
				"--ignore-not-found=true", "--all=true", "--wait=false")))
			out, gerr := runCmd(exec.Command("kubectl", "get", k.name, "-n", namespace,
				"-o", "jsonpath={.items[*].metadata.name}"))
			if gerr != nil {
				_, _ = fmt.Fprintln(GinkgoWriter, gerr)
				allGone = false // CRD missing or API error: not provably gone
				continue
			}
			if trim(out) != "" {
				allGone = false // at least one CR is still present
			}
		}
		return allGone
	}, timeout, 10*time.Second).Should(BeTrue(), "all CRs should be removed from the cluster")
}

// waitForGone polls the real UpCloud API until every collected verifiable
// resource returns a 404.
func waitForGone(g Gomega, timeout time.Duration) {
	ctx := context.Background()
	g.Eventually(func() bool {
		gone, detail := allProbesGone(ctx)
		if !gone {
			_, _ = fmt.Fprintln(GinkgoWriter, detail)
		}
		return gone
	}, timeout, 15*time.Second).Should(BeTrue(), "every UpCloud resource from this run should be gone")
}

// allProbesGone is the waitForGone poll predicate, extracted so it is
// testable without the Ginkgo suite or an API client. Non-verifiable
// probes are skipped: their fetch is a no-op that returns nil, and nil is
// not a 404, so polling them could never pass (run 35237159467 spun the
// full budget on exactly that). The string names the first verifiable
// resource still present, for the Ginkgo writer.
func allProbesGone(ctx context.Context) (bool, string) {
	for _, p := range probes {
		if !p.verifiable {
			continue
		}
		err := p.fetch(ctx, p.uuid)
		if !upcloudapi.IsNotFound(err) {
			return false, fmt.Sprintf("%s %s still exists (err=%v)", p.kind, p.uuid, err)
		}
	}
	return true, ""
}

// sweepLabelled deletes any UpCloud network or router still carrying the
// operator's managed-by label with a uid label this run created (an orphan
// left behind, for example by a crash between create and finalizer delete).
//
// The sweep is a cost safety net, not a substitute for the operator's
// delete: every resource it has to remove is reported, with the state of
// its CR, so the failure points at the operator's Delete.
type networkSweeper interface {
	GetRouters(context.Context, ...upcloudrequest.QueryFilter) (*upcloud.Routers, error)
	DeleteRouter(context.Context, *upcloudrequest.DeleteRouterRequest) error
	GetNetworks(context.Context, ...upcloudrequest.QueryFilter) (*upcloud.Networks, error)
	DeleteNetwork(context.Context, *upcloudrequest.DeleteNetworkRequest) error
}

func sweepLabelled(api networkSweeper, report func(string)) {
	ctx := context.Background()
	known := map[string]bool{}
	for _, p := range probes {
		if p.crUID != "" {
			known[p.crUID] = true
		}
	}
	// UpCloud refuses network deletion while its router still exists.
	if rts, err := api.GetRouters(ctx); err == nil {
		for i := range rts.Routers {
			r := &rts.Routers[i]
			if hasManagedBy(r.Labels) && known[uidOf(r.Labels)] {
				state := crStateOf("router", uidOf(r.Labels))
				derr := api.DeleteRouter(ctx, &upcloudrequest.DeleteRouterRequest{UUID: r.UUID})
				report(leftoverMessage("router", r.UUID, state, derr))
			}
		}
	}
	if nets, err := api.GetNetworks(ctx); err == nil {
		for i := range nets.Networks {
			n := &nets.Networks[i]
			if hasManagedBy(n.Labels) && known[uidOf(n.Labels)] {
				state := crStateOf("network", uidOf(n.Labels))
				derr := api.DeleteNetwork(ctx, &upcloudrequest.DeleteNetworkRequest{UUID: n.UUID})
				report(leftoverMessage("network", n.UUID, state, derr))
			}
		}
	}
}

// crStateOf describes the CR that owns a leftover UpCloud resource, from
// the CR list of its kind (matched by metadata.uid, the k8s-uid label).
func crStateOf(kind, uid string) string {
	out, err := runCmd(exec.Command("kubectl", "get", kind, "-n", namespace, "-o",
		"jsonpath={range .items[*]}{.metadata.uid}{'\\t'}{.metadata.name}{'\\t'}"+
			"{.metadata.deletionTimestamp}{'\\t'}{.metadata.finalizers}{'\\t'}"+
			"{.status.conditions[?(@.type=='Ready')].reason}{'\\t'}"+
			"{.status.conditions[?(@.type=='Ready')].message}{'\\n'}{end}"))
	if err != nil {
		return fmt.Sprintf("its CR state could not be read (%v)", err)
	}
	return crStateFor(out, uid)
}

// crStateFor picks the tab-separated CR row with the given uid out of the
// crStateOf output and words what it says about why the delete did not
// happen.
func crStateFor(rows, uid string) string {
	for _, line := range strings.Split(rows, "\n") {
		f := strings.SplitN(line, "\t", 6)
		if len(f) < 6 || f[0] != uid {
			continue
		}
		return fmt.Sprintf("its CR %s still exists (deletionTimestamp=%q finalizers=%s Ready=%s: %s)",
			f[1], f[2], f[3], f[4], f[5])
	}
	return "its CR is already gone, so the finalizer was removed without the resource being deleted"
}

// leftoverMessage words one resource the operator should have deleted.
func leftoverMessage(kind, id, crState string, deleteErr error) string {
	msg := fmt.Sprintf("operator left %s %s on UpCloud: %s", kind, id, crState)
	if deleteErr != nil {
		msg += fmt.Sprintf("; the sweep's own delete also failed: %v", deleteErr)
	}
	return msg
}

func hasManagedBy(labels []upcloud.Label) bool {
	for _, l := range labels {
		if l.Key == upcloudapi.LabelManagedBy && l.Value == upcloudapi.ManagedByValue {
			return true
		}
	}
	return false
}

func uidOf(labels []upcloud.Label) string {
	for _, l := range labels {
		if l.Key == upcloudapi.LabelUID {
			return l.Value
		}
	}
	return ""
}

// managedKinds describes every kind the operator owns: the plural resource
// name, the status field holding its identity (empty means the kind has no
// per-instance identity worth probing), and whether the UpCloud API can
// confirm deletion for it. Children-first request order is a hint, not a
// completion guarantee; the non-blocking deletes converge through finalizers.
var managedKinds = []struct {
	name       string
	identField string // jsonpath segment under .status, e.g. "uuid" or "address"
	verifiable bool   // the API has a Get* whose 404 proves deletion
}{
	// database group
	{"manageddatabaselogicaldatabase", "name", false},
	{"manageddatabaseuser", "username", false},
	{"manageddatabase", "uuid", true},
	// object storage group
	{"objectstorageaccesskey", "username", false},
	{"objectstoragebucket", "name", false},
	{"objectstoragecustomdomain", "name", false},
	{"objectstorageuser", "username", false},
	{"objectstoragepolicy", "name", false},
	{"managedobjectstorage", "uuid", true},
	// load balancer group
	{"loadbalancercertificatebundle", "uuid", false},
	{"loadbalancerfrontendtlsconfig", "uuid", false},
	{"loadbalancerfrontendrule", "uuid", false},
	{"loadbalancerfrontend", "uuid", false},
	{"loadbalancerbackendtlsconfig", "uuid", false},
	{"loadbalancerbackendmember", "name", false},
	{"loadbalancerbackend", "uuid", false},
	{"loadbalancerresolver", "uuid", false},
	// gateway / network group
	{"gatewaytunnel", "uuid", false},
	{"gatewayconnection", "uuid", false},
	{"gateway", "uuid", false},
	{"networkpeering", "uuid", false},
	{"floatingip", "address", true},
	{"router", "uuid", true},
	{"network", "uuid", true},
}

// collectLeakedCRs retains run ownership even before status has been written.
// Only nonempty external identities become verifiable probes. JSON preserves
// the association between each CR UID and status, including missing status.
func collectLeakedCRs() {
	known := map[string]bool{}
	for _, p := range probes {
		known[p.kind+"/"+p.crUID+"/"+p.uuid] = true
	}
	for _, k := range managedKinds {
		out, err := runCmd(exec.Command("kubectl", "get", k.name, "-n", namespace, "-o", "json"))
		if err != nil {
			_, _ = fmt.Fprintf(GinkgoWriter, "collect %s: %v\n", k.name, err)
			continue
		}
		var list unstructured.UnstructuredList
		if err := json.Unmarshal([]byte(out), &list); err != nil {
			_, _ = fmt.Fprintf(GinkgoWriter, "decode %s: %v\n", k.name, err)
			continue
		}
		for _, obj := range list.Items {
			uid := string(obj.GetUID())
			ident, _, err := unstructured.NestedString(obj.Object, "status", k.identField)
			if err != nil {
				_, _ = fmt.Fprintf(GinkgoWriter, "read %s identity: %v\n", k.name, err)
				continue
			}
			key := k.name + "/" + uid + "/" + ident
			if (uid == "" && ident == "") || known[key] {
				continue
			}
			known[key] = true
			verifiable := k.verifiable && ident != ""
			probes = append(probes, probe{
				kind: k.name, uuid: ident, crUID: uid,
				verifiable: verifiable, fetch: fetchFor(k.name, verifiable),
			})
			_, _ = fmt.Fprintf(GinkgoWriter, "collected %s %s -> %s (CR UID %s)\n", k.name, obj.GetName(), ident, uid)
		}
	}
}

// fetchFor returns the Get* call whose 404 proves the resource is gone.
// For kinds the API cannot confirm individually the fetch is a no-op and
// waitForGone skips the probe entirely (see allProbesGone); the sweeps
// are the safety net for those kinds.
func fetchFor(kind string, verifiable bool) func(context.Context, string) error {
	if !verifiable {
		return func(context.Context, string) error { return nil }
	}
	switch kind {
	case "router":
		return func(ctx context.Context, u string) error {
			_, e := svc.GetRouterDetails(ctx, &upcloudrequest.GetRouterDetailsRequest{UUID: u})
			return e
		}
	case "network":
		return func(ctx context.Context, u string) error {
			_, e := svc.GetNetworkDetails(ctx, &upcloudrequest.GetNetworkDetailsRequest{UUID: u})
			return e
		}
	case "floatingip":
		return func(ctx context.Context, u string) error {
			_, e := svc.GetIPAddressDetails(ctx, &upcloudrequest.GetIPAddressDetailsRequest{Address: u})
			return e
		}
	case "managedobjectstorage":
		return func(ctx context.Context, u string) error {
			_, e := svc.GetManagedObjectStorage(ctx, &upcloudrequest.GetManagedObjectStorageRequest{UUID: u})
			return e
		}
	case "manageddatabase":
		return func(ctx context.Context, u string) error {
			_, e := svc.GetManagedDatabase(ctx, &upcloudrequest.GetManagedDatabaseRequest{UUID: u})
			return e
		}
	default:
		return func(context.Context, string) error { return nil }
	}
}

// sweepDetachedFloatingIPs releases floating IPs that escaped their CR: an
// address is swept only when it is unattached (no server), floating, in the
// sample zone, and not carried by any collected probe (a probed address is
// the CR/finalizer path's business and waitForGone verifies it). This
// cannot release attached or non-floating addresses.
func sweepDetachedFloatingIPs(report func(string)) {
	ctx := context.Background()
	ips, err := svc.GetIPAddresses(ctx)
	if err != nil {
		_, _ = fmt.Fprintf(GinkgoWriter, "floating IP sweep skipped: %v\n", err)
		return
	}
	probed := map[string]bool{}
	for _, p := range probes {
		if p.kind == "floatingip" {
			probed[p.uuid] = true
		}
	}
	for i := range ips.IPAddresses {
		ip := &ips.IPAddresses[i]
		if ip.Zone != sampleZone || ip.ServerUUID != "" || ip.Floating != upcloud.True || probed[ip.Address] {
			continue
		}
		derr := svc.ReleaseIPAddress(ctx, &upcloudrequest.ReleaseIPAddressRequest{IPAddress: ip.Address})
		report(leftoverMessage("floating IP", ip.Address, "no CR of this run tracks it", derr))
	}
}

// dumpDiagnostics writes everything needed to root-cause a Ready-wait
// timeout into a temp dir (path recorded in diagDir for the workflow to
// upload): CR table, full conditions with messages (the reconciler writes
// the failing API error there), manager logs, and namespace events. Runs
// in AfterSuite while the kind cluster is still up.
func dumpDiagnostics() {
	dir, err := os.MkdirTemp("", "e2e-diagnostics-")
	if err != nil {
		_, _ = fmt.Fprintf(GinkgoWriter, "diagnostics skipped: %v\n", err)
		return
	}
	diagDir = dir

	appendCmdToFile(filepath.Join(diagDir, "cr-table.txt"),
		exec.Command("kubectl", "get", managedKindsCSV(), "-n", namespace, "-o", "wide"))
	// Per-kind condition dump: every field of every condition of every CR,
	// so the recorded API error message is in the artifact.
	for _, k := range managedKinds {
		appendCmdToFile(filepath.Join(diagDir, "conditions-"+k.name+".txt"),
			exec.Command("kubectl", "get", k.name, "-n", namespace,
				"-o", "jsonpath={range .items[*]}{.metadata.name}{'\\t'}{.status.conditions}{'\\n'}{end}"))
	}
	appendCmdToFile(filepath.Join(diagDir, "manager-logs.txt"),
		exec.Command("kubectl", "logs", "-l", "control-plane=controller-manager",
			"-n", operatorNS, "--all-containers=true", "--tail=500"))
	appendCmdToFile(filepath.Join(diagDir, "events-run-ns.txt"),
		exec.Command("kubectl", "get", "events", "-n", namespace, "--sort-by=.lastTimestamp"))
	appendCmdToFile(filepath.Join(diagDir, "events-operator-ns.txt"),
		exec.Command("kubectl", "get", "events", "-n", operatorNS, "--sort-by=.lastTimestamp"))
}

// managedKindsCSV renders the kind list for a single kubectl get.
func managedKindsCSV() string {
	names := make([]string, len(managedKinds))
	for i, k := range managedKinds {
		names[i] = k.name
	}
	return strings.Join(names, ",")
}

// appendCmdToFile runs cmd from the repo root and writes its combined
// output to path. Diagnostics are best-effort: failures leave a note.
func appendCmdToFile(path string, cmd *exec.Cmd) {
	cmd.Dir = repoRoot
	out, err := cmd.CombinedOutput()
	if err != nil && len(out) == 0 {
		out = []byte(fmt.Sprintf("(command failed: %v)", err))
	}
	if writeErr := os.WriteFile(path, out, 0o644); writeErr != nil {
		_, _ = fmt.Fprintf(GinkgoWriter, "diagnostics write failed for %s: %v\n", path, writeErr)
	}
}

func waitForControllerReady() {
	Eventually(func() (string, error) {
		return runCmd(exec.Command("kubectl", "get", "pods",
			"-l", "control-plane=controller-manager",
			"-n", operatorNS,
			"-o", "jsonpath={.items[0].status.conditions[?(@.type=='Ready')].status}"))
	}, 3*time.Minute, time.Second).Should(Equal("True"), "controller-manager pod should be Ready")
}

// waitReady polls a CR until its Ready condition is True.
func waitReady(kind, name string, timeout time.Duration) {
	Eventually(func() (string, error) {
		return runCmd(exec.Command("kubectl", "get", kind, name, "-n", namespace,
			"-o", "jsonpath={.status.conditions[?(@.type=='Ready')].status}"))
	}, timeout, 5*time.Second).Should(Equal("True"), "%s/%s should be Ready", kind, name)
}

// runCmd runs an external command from the repository root and returns
// its combined output. Failures are wrapped with the output so the
// reason is visible in the suite log (a bare ExitError hides it).
func runCommand(cmd *exec.Cmd) (string, error) {
	cmd.Dir = repoRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("%s: %w (output: %s)", strings.Join(cmd.Args, " "), err, out)
	}
	return string(out), nil
}

func trim(s string) string {
	// kubectl jsonpath output is unquoted; trim any surrounding whitespace.
	for len(s) > 0 && (s[len(s)-1] == '\n' || s[len(s)-1] == ' ' || s[len(s)-1] == '\r') {
		s = s[:len(s)-1]
	}
	return s
}

func stdinReader(s string) *os.File {
	f, err := os.CreateTemp("", "e2e-upcloud-*")
	if err != nil {
		return nil
	}
	_, _ = f.WriteString(s)
	_, _ = f.Seek(0, 0)
	return f
}
