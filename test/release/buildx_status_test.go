package release

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildxHelperPreservesBuildStatus(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []int{0, 29} {
		t.Run(fmt.Sprintf("build=%d,cleanup=41", want), func(t *testing.T) {
			dir := t.TempDir()
			shim := filepath.Join(dir, "container-tool")
			logPath := filepath.Join(dir, "calls")
			const script = `#!/usr/bin/env bash
set -eu
printf '%s\n' "$*" >> "$CALL_LOG"
case "$2" in
  build) exit "$BUILD_STATUS" ;;
  rm) exit 41 ;;
esac
`
			if err := os.WriteFile(shim, []byte(script), 0o700); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command("bash", "hack/build-multiarch.sh")
			cmd.Dir = root
			cmd.Env = append(os.Environ(), "CONTAINER_TOOL="+shim, "CALL_LOG="+logPath,
				fmt.Sprintf("BUILD_STATUS=%d", want), "TMPDIR="+dir,
				"IMG=registry.invalid/uck:probe", "PLATFORMS=linux/amd64,linux/arm64")
			out, runErr := cmd.CombinedOutput()
			got := 0
			if runErr != nil {
				var exitErr *exec.ExitError
				if !errors.As(runErr, &exitErr) {
					t.Fatal(runErr)
				}
				got = exitErr.ExitCode()
			}
			if got != want {
				t.Fatalf("cleanup replaced build status: got %d, want %d\n%s", got, want, out)
			}
			calls, err := os.ReadFile(logPath)
			if err != nil || !strings.Contains(string(calls), "buildx rm") {
				t.Fatalf("cleanup failure path was not exercised: %v\n%s", err, calls)
			}
			entries, err := os.ReadDir(dir)
			if err != nil {
				t.Fatal(err)
			}
			for _, entry := range entries {
				if entry.IsDir() {
					t.Errorf("temporary directory survived failed builder cleanup: %s", entry.Name())
				}
			}
		})
	}
}
