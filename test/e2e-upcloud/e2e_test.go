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
	"fmt"
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

	"github.com/polarsquad/upcloud-operator/internal/upcloudapi"
)

// This suite runs against the REAL UpCloud API and costs money (it
// provisions a database for roughly 20 minutes). It is gated on
// UPCLOUD_TOKEN: when the variable is unset the whole suite is skipped.
// It is dispatched only from CI via the "e2e-upcloud" workflow
// (workflow_dispatch, environment: upcloud-e2e).

const (
	operatorNS = "upcloud-operator-system"
	managedBy  = "upcloud-operator"
	uidLabel   = "k8s-uid"
)

var (
	managerImage = "example.com/upcloud-operator:e2e-upcloud"
	runID        = fmt.Sprintf("%d", time.Now().Unix())
	namespace    = fmt.Sprintf("upcloud-e2e-%s", runID)
	kindCluster  = fmt.Sprintf("upcloud-e2e-%s", runID)
	svc          *upcloudsvc.Service
	probes       []probe
	repoRoot     string
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
type probe struct {
	kind  string
	uuid  string
	fetch func(context.Context, string) error // returns the Get* call's error
}

func TestE2EUpCloud(t *testing.T) {
	RegisterFailHandler(Fail)
	_, _ = fmt.Fprintf(GinkgoWriter, "Starting upcloud-operator real-API e2e suite (run %s)\n", runID)
	RunSpecs(t, "e2e-upcloud suite")
}

var _ = BeforeSuite(func() {
	repoRoot = resolveRepoRoot()

	if os.Getenv("UPCLOUD_TOKEN") == "" {
		Skip("UPCLOUD_TOKEN is not set; skipping the real-API e2e suite")
	}

	var err error
	svc, err = upcloudapi.NewServiceFromEnv()
	Expect(err).NotTo(HaveOccurred(), "real UpCloud client should be built from UPCLOUD_TOKEN")

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
	// Best-effort teardown that must not itself fail the suite: always
	// remove the real UpCloud resources and the kind cluster, even if a
	// spec failed halfway through.
	By("deleting the applied samples in reverse dependency order")
	deleteAllSamples()
	By("waiting for the CRs to be removed (finalizers drive the real delete)")
	waitForCRsGone(20 * time.Minute)
	By("sweeping any leftover UpCloud resource labelled by this run")
	sweepLabelled()
	By("verifying the UpCloud API confirms everything is gone")
	waitForGone(5 * time.Minute)
	By("undeploying the operator and uninstalling the CRDs")
	_, _ = runCmd(exec.Command("make", "undeploy"))
	_, _ = runCmd(exec.Command("make", "uninstall", "ignore-not-found=true"))
	_, _ = runCmd(exec.Command("kubectl", "delete", "ns", namespace, "--ignore-not-found=true"))
	_, _ = runCmd(exec.Command("kubectl", "delete", "ns", operatorNS, "--ignore-not-found=true"))
	By("deleting the kind cluster")
	_, _ = runCmd(exec.Command("kind", "delete", "cluster", "--name", kindCluster))
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
		waitReady("managedobjectstorage", "managedobjectstorage-sample", 5*time.Minute)
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

		By("deleting all samples in reverse dependency order")
		deleteAllSamples()

		By("waiting for the CRs to be removed (finalizers drive the real delete)")
		waitForCRsGone(20 * time.Minute)

		By("verifying the UpCloud API confirms everything is gone")
		waitForGone(5 * time.Minute)
	})
})

// deleteAllSamples removes the applied samples in reverse dependency order.
func deleteAllSamples() {
	order := []string{
		"config/samples/network_v1alpha1_floatingip.yaml",
		"config/samples/database_v1alpha1_manageddatabase.yaml",
		"config/samples/objectstorage_v1alpha1_objectstorageaccesskey.yaml",
		"config/samples/objectstorage_v1alpha1_objectstorageuser.yaml",
		"config/samples/objectstorage_v1alpha1_objectstoragepolicy.yaml",
		"config/samples/objectstorage_v1alpha1_managedobjectstorage.yaml",
		"config/samples/network_v1alpha1_network.yaml",
		"config/samples/network_v1alpha1_router.yaml",
	}
	for _, f := range order {
		_, _ = runCmd(exec.Command("kubectl", "delete", "-f", f, "-n", namespace, "--ignore-not-found=true"))
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

// collectProbes records each UpCloud identity created by this run, using the
// status UUIDs the operator wrote. Anything it cannot find is skipped (the
// corresponding resource may not have finished provisioning).
func collectProbes() {
	add := func(kind, name, jsonpath string, fetch func(context.Context, string) error) {
		out, err := runCmd(exec.Command("kubectl", "get", kind, name, "-n", namespace, "-o", jsonpath))
		out = trim(out)
		if err != nil || out == "" {
			return
		}
		probes = append(probes, probe{kind: kind, uuid: out, fetch: fetch})
		_, _ = fmt.Fprintf(GinkgoWriter, "collected %s %s -> %s\n", kind, name, out)
	}
	add("router", "router-sample", "{.status.uuid}",
		func(ctx context.Context, u string) error {
			_, e := svc.GetRouterDetails(ctx, &upcloudrequest.GetRouterDetailsRequest{UUID: u})
			return e
		})
	add("network", "network-sample", "{.status.uuid}",
		func(ctx context.Context, u string) error {
			_, e := svc.GetNetworkDetails(ctx, &upcloudrequest.GetNetworkDetailsRequest{UUID: u})
			return e
		})
	// A floating IP is addressed by its IP, not a UUID; the status.address
	// is the identity and the probe fetch must return 404 once released.
	add("floatingip", "floatingip-sample", "{.status.address}",
		func(ctx context.Context, u string) error {
			_, e := svc.GetIPAddressDetails(ctx, &upcloudrequest.GetIPAddressDetailsRequest{Address: u})
			return e
		})
	add("managedobjectstorage", "managedobjectstorage-sample", "{.status.uuid}",
		func(ctx context.Context, u string) error {
			_, e := svc.GetManagedObjectStorage(ctx, &upcloudrequest.GetManagedObjectStorageRequest{UUID: u})
			return e
		})
	add("manageddatabase", "manageddatabase-sample", "{.status.uuid}",
		func(ctx context.Context, u string) error {
			_, e := svc.GetManagedDatabase(ctx, &upcloudrequest.GetManagedDatabaseRequest{UUID: u})
			return e
		})
}

// waitForCRsGone blocks until every collected CR is removed from the cluster
// (its finalizer ran, which is what performs the real UpCloud delete).
func waitForCRsGone(timeout time.Duration) {
	Eventually(func() bool {
		allGone := true
		for _, p := range probes {
			_, gerr := runCmd(exec.Command("kubectl", "get", p.kind, crName(p.kind), "-n", namespace))
			if gerr == nil {
				allGone = false
			}
		}
		return allGone
	}, timeout, 10*time.Second).Should(BeTrue(), "all CRs should be removed from the cluster")
}

// crName maps a kind to the sample object name in the run namespace.
func crName(kind string) string {
	switch kind {
	case "router":
		return "router-sample"
	case "network":
		return "network-sample"
	case "managedobjectstorage":
		return "managedobjectstorage-sample"
	case "manageddatabase":
		return "manageddatabase-sample"
	default:
		return kind + "-sample"
	}
}

// waitForGone polls the real UpCloud API until every collected resource
// returns a 404.
func waitForGone(timeout time.Duration) {
	ctx := context.Background()
	Eventually(func() bool {
		for _, p := range probes {
			err := p.fetch(ctx, p.uuid)
			if !upcloudapi.IsNotFound(err) {
				_, _ = fmt.Fprintf(GinkgoWriter, "%s %s still exists (err=%v)\n", p.kind, p.uuid, err)
				return false
			}
		}
		return true
	}, timeout, 15*time.Second).Should(BeTrue(), "every UpCloud resource from this run should be gone")
}

// sweepLabelled deletes any UpCloud network or router still carrying the
// operator's managed-by label with a k8s-uid this run created (an orphan
// left behind, for example by a crash between create and finalizer delete).
func sweepLabelled() {
	ctx := context.Background()
	known := map[string]bool{}
	for _, p := range probes {
		known[p.uuid] = true
	}
	if nets, err := svc.GetNetworks(ctx); err == nil {
		for i := range nets.Networks {
			n := &nets.Networks[i]
			if hasManagedBy(n.Labels) && known[uidOf(n.Labels)] {
				_, _ = fmt.Fprintf(GinkgoWriter, "sweeping leftover network %s\n", n.UUID)
				_ = svc.DeleteNetwork(ctx, &upcloudrequest.DeleteNetworkRequest{UUID: n.UUID})
			}
		}
	}
	if rts, err := svc.GetRouters(ctx); err == nil {
		for i := range rts.Routers {
			r := &rts.Routers[i]
			if hasManagedBy(r.Labels) && known[uidOf(r.Labels)] {
				_, _ = fmt.Fprintf(GinkgoWriter, "sweeping leftover router %s\n", r.UUID)
				_ = svc.DeleteRouter(ctx, &upcloudrequest.DeleteRouterRequest{UUID: r.UUID})
			}
		}
	}
}

func hasManagedBy(labels []upcloud.Label) bool {
	for _, l := range labels {
		if l.Key == managedBy {
			return true
		}
	}
	return false
}

func uidOf(labels []upcloud.Label) string {
	for _, l := range labels {
		if l.Key == uidLabel {
			return l.Value
		}
	}
	return ""
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
func runCmd(cmd *exec.Cmd) (string, error) {
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
