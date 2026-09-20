package release

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Run the real Make target with every container operation intercepted. No
// registry, daemon, builder or image is touched by these regression tests.
func TestBuildxExitAndCleanup(t *testing.T) {
	for _, step := range []string{"", "create", "build"} {
		t.Run("failure="+step, func(t *testing.T) {
			dir := t.TempDir()
			for _, name := range []string{"Makefile", "Dockerfile", "hack/build-multiarch.sh"} {
				data, err := os.ReadFile(filepath.Join("..", "..", name))
				if os.IsNotExist(err) && strings.HasPrefix(name, "hack/") {
					continue // The pre-fix Makefile has no helper script.
				}
				if err != nil {
					t.Fatal(err)
				}
				path := filepath.Join(dir, name)
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, data, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			shim := filepath.Join(dir, "container-tool")
			const script = `#!/usr/bin/env bash
set -eu
printf '%s\n' "$*" >> "$CALL_LOG"
if [ "$1/$2" = "buildx/build" ]; then
  previous=
  for arg in "$@"; do
    if [ "$previous" = "-f" ]; then
      cp "$arg" "$DOCKERFILE_SNAPSHOT"
      printf '%s' "$arg" > "$DOCKERFILE_LOCATION"
    fi
    previous="$arg"
  done
fi
if [ "$2" = "$FAIL_STEP" ]; then exit 29; fi
`
			if err := os.WriteFile(shim, []byte(script), 0o700); err != nil {
				t.Fatal(err)
			}
			logPath := filepath.Join(dir, "calls")
			snapshot := filepath.Join(dir, "dockerfile.snapshot")
			location := filepath.Join(dir, "dockerfile.location")
			cmd := exec.Command("make", "docker-buildx", "CONTAINER_TOOL="+shim,
				"IMG=registry.invalid/uck:test", "PLATFORMS=linux/amd64,linux/arm64")
			cmd.Dir = dir
			cmd.Env = append(os.Environ(), "CALL_LOG="+logPath, "DOCKERFILE_SNAPSHOT="+snapshot,
				"DOCKERFILE_LOCATION="+location, "FAIL_STEP="+step, "TMPDIR="+dir)
			out, runErr := cmd.CombinedOutput()
			calls, err := os.ReadFile(logPath)
			if err != nil {
				t.Fatalf("container calls were not intercepted: %v\n%s", err, out)
			}
			if (runErr != nil) != (step != "") {
				t.Errorf("failed %q: exit=%v, expected failure=%v\n%s", step, runErr, step != "", out)
			}
			log := string(calls)
			if strings.Contains(log, "buildx use") {
				t.Errorf("target changed the caller's default builder: %s", log)
			}
			entries, err := os.ReadDir(dir)
			if err != nil {
				t.Fatal(err)
			}
			for _, entry := range entries {
				if entry.IsDir() && entry.Name() != "hack" {
					t.Errorf("temporary build directory remains: %s", entry.Name())
				}
			}
			if step == "create" {
				if strings.Contains(log, "buildx build") || strings.Contains(log, "buildx rm") {
					t.Errorf("failed builder creation must not build or remove an unowned builder: %s", log)
				}
				return
			}
			assertBuildxArguments(t, log)
			used, err := os.ReadFile(location)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(filepath.Dir(string(used))); !os.IsNotExist(err) {
				t.Errorf("actual temporary Dockerfile directory was not removed: %q (%v)", used, err)
			}
			data, err := os.ReadFile(snapshot)
			if err != nil || !strings.Contains(string(data), "FROM --platform=${BUILDPLATFORM} golang:") {
				t.Errorf("native build-stage platform was not injected: %v\n%s", err, data)
			}
			if !strings.Contains(string(data), "FROM gcr.io/distroless/static:nonroot") ||
				!strings.Contains(string(data), "GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH}") {
				t.Errorf("runtime stage or cross-compilation arguments changed: %s", data)
			}
		})
	}
}
