//go:build e2e

package e2e

import (
	"errors"
	"os/exec"
	"reflect"
	"testing"
)

func TestCleanupMetricsBinding(t *testing.T) {
	want := []string{"kubectl", "delete", "clusterrolebinding", metricsRoleBindingName, "--ignore-not-found=true"}
	calls := 0
	run := func(cmd *exec.Cmd) (string, error) {
		calls++
		if !reflect.DeepEqual(cmd.Args, want) {
			t.Fatalf("cleanup must target only the metrics binding and tolerate absence: got %v, want %v", cmd.Args, want)
		}
		return "", nil
	}
	// Run the same cleanup for created and already-absent binding cases.
	for range 2 {
		if err := cleanupMetricsBinding(run); err != nil {
			t.Fatal(err)
		}
	}
	if calls != 2 {
		t.Fatalf("cleanup called kubectl %d times, want 2", calls)
	}
	failure := errors.New("API unavailable")
	if err := cleanupMetricsBinding(func(*exec.Cmd) (string, error) { return "", failure }); !errors.Is(err, failure) {
		t.Fatalf("cleanup concealed kubectl failure: %v", err)
	}
}
