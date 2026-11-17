package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRustStatesCheck(t *testing.T) {
	t.Parallel()
	needCargo(t)
	root := rustRepo(t, map[string]string{
		"docs/lamp.md": "<!-- docgen:states crate=states-lamp type=Lamp -->\n<!-- /docgen:states -->\n",
	})
	if rc, log := docgen(t, root, "update"); rc != 0 {
		t.Fatalf("update: rc %d, log:\n%s", rc, log)
	}
	svg := filepath.Join(root, "docs", "states", "states-lamp-lamp.svg")
	if _, err := os.Stat(svg); err != nil {
		t.Fatal(err)
	}
	if rc, log := docgen(t, root, "check"); rc != 0 {
		t.Fatalf("check: rc %d, log:\n%s", rc, log)
	}
	if err := os.WriteFile(svg, []byte("<svg/>\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if rc, log := docgen(t, root, "check"); rc != 1 || !strings.Contains(log, "docs/states/states-lamp-lamp.svg") {
		t.Errorf("check of a changed diagram: rc %d, log:\n%s", rc, log)
	}
	if err := os.WriteFile(filepath.Join(root, "docs", "lamp.md"), []byte("# Lamp\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if rc, log := docgen(t, root, "check"); rc != 1 || !strings.Contains(log, "unused: docs/states/states-lamp-lamp.svg: docgen:states diagram") {
		t.Errorf("check of an unused diagram: rc %d, log:\n%s", rc, log)
	}
}
