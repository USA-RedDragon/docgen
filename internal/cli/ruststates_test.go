package cli

import (
	"bytes"
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

// TestRustStatesFixtureImages checks the diagrams of the Rust fixture pages,
// which the site shows next to the guide, against an update.
func TestRustStatesFixtureImages(t *testing.T) {
	t.Parallel()
	needCargo(t)
	files := map[string]string{}
	for _, page := range []string{"docs/guide-states.md", "docs/guide-states-enum.md"} {
		b, err := os.ReadFile(filepath.Join("testdata/rust", page))
		if err != nil {
			t.Fatal(err)
		}
		files[page] = string(b)
	}
	root := rustRepo(t, files)
	if rc, log := docgen(t, root, "update"); rc != 0 {
		t.Fatalf("update: rc %d, log:\n%s", rc, log)
	}
	for _, f := range []string{"states-lamp-lamp.svg", "door-events.svg"} {
		got, err := os.ReadFile(filepath.Join(root, "docs/states", f))
		if err != nil {
			t.Fatal(err)
		}
		want, err := os.ReadFile(filepath.Join("testdata/rust/docs/states", f))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("testdata/rust/docs/states/%s differs from the diagram of an update; copy it from an update of the page", f)
		}
	}
}
