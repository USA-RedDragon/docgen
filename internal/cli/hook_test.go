package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStateHookAllocatesNothing(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	copyFixture(t, root)
	rc, log := docgen(t, root, "update", "-module", "v2")
	if rc != 0 || strings.Contains(log, "FAIL") {
		t.Fatalf("-update: rc %d, log:\n%s", rc, log)
	}
}

func TestStateHookTestFailureFails(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	copyFixture(t, root)
	if rc, log := docgen(t, root, "update", "-module", "v2"); rc != 0 {
		t.Fatalf("-update: rc %d, log:\n%s", rc, log)
	}
	f := filepath.Join(root, "v2/machine/machine_test.go")
	src, err := os.ReadFile(f)
	if err != nil {
		t.Fatal(err)
	}
	src = append(src, []byte("\nfunc TestBroken(t *testing.T) { t.Fatal(\"broken\") }\n")...)
	if err := os.WriteFile(f, src, 0o600); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"check", "update"} {
		if rc, log := docgen(t, root, mode, "-module", "v2"); rc != 1 || !strings.Contains(log, "TestBroken") {
			t.Errorf("%s with a failing test: rc %d, log:\n%s", mode, rc, log)
		}
	}
}
