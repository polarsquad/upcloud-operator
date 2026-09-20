package release

import (
	"strings"
	"testing"
)

func assertBuildxArguments(t *testing.T, log string) {
	t.Helper()
	first := strings.Fields(strings.Split(log, "\n")[0])
	builder := first[len(first)-1]
	if strings.Contains(strings.Join(first, " "), "--use") {
		t.Errorf("creation must not select the builder: %s", log)
	}
	for line := range strings.SplitSeq(log, "\n") {
		if strings.HasPrefix(line, "buildx build ") &&
			(!strings.Contains(line, "--tag registry.invalid/uck:test") || !strings.HasSuffix(line, " .")) {
			t.Errorf("image tag or context changed: %s", line)
		}
	}
	if !strings.Contains(log, "buildx rm "+builder+"\n") ||
		!strings.Contains(log, "--builder "+builder+" ") {
		t.Errorf("build and cleanup must target the created builder: %s", log)
	}
	if !strings.Contains(log, "--push") || !strings.Contains(log, "--platform=linux/amd64,linux/arm64") {
		t.Errorf("publish arguments lost: %s", log)
	}
}
