package gen

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestMissingFileIsStale(t *testing.T) {
	t.Parallel()
	root, g := setup(t, machineTree())
	if err := os.MkdirAll(filepath.Join(root, "docs/states"), 0o755); err != nil {
		t.Fatal(err)
	}
	stale, err := g.Run(t.Context(), filepath.Join(root, "docs"), false)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.ContainsFunc(stale, func(s string) bool { return strings.HasPrefix(s, "docs/states/p-lamp.svg (from docs/p.md:1:") }) {
		t.Errorf("stale = %q", stale)
	}
}

func TestUnreadableFileIsAnError(t *testing.T) {
	t.Parallel()
	text := machineTree()["docs/p.md"]
	root, g := setup(t, machineTree())
	if err := os.MkdirAll(filepath.Join(root, "docs/states/p-lamp.svg"), 0o755); err != nil {
		t.Fatal(err)
	}
	stale, err := g.Run(t.Context(), filepath.Join(root, "docs"), true)
	if err == nil || !strings.Contains(err.Error(), "docs/p.md:1: docgen:states pkg=p type=Lamp: ") || !strings.Contains(err.Error(), "is a directory") {
		t.Fatalf("err = %v", err)
	}
	if slices.ContainsFunc(stale, func(s string) bool { return strings.Contains(s, ".svg") }) {
		t.Errorf("a file that does not read is listed as stale: %q", stale)
	}
	if got := read(t, root, "docs/p.md"); got != text {
		t.Errorf("page changed: %q", got)
	}
}
