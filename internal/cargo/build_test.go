package cargo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPackageName(t *testing.T) {
	t.Parallel()
	for id, want := range map[string]string{
		"path+file:///w/crates/tool#0.2.0":        "tool",
		"path+file:///w#tool@0.2.0":               "tool",
		"tool 0.2.0 (path+file:///w/crates/tool)": "tool",
		"registry+https://x/index#clap@4.5.0":     "clap",
	} {
		if got := packageName(id); got != want {
			t.Errorf("packageName(%q) = %q, want %q", id, got, want)
		}
	}
}

func TestExecutableOf(t *testing.T) {
	t.Parallel()
	out := strings.Join([]string{
		`{"reason":"compiler-artifact","package_id":"path+file:///w/crates/lib#0.1.0","target":{"name":"demo","kind":["lib"]},"executable":null}`,
		`{"reason":"compiler-artifact","package_id":"path+file:///w/crates/cli#0.1.0","target":{"name":"demo","kind":["bin"]},"executable":"/t/debug/demo"}`,
		`not json`,
		`{"reason":"build-finished","success":true}`,
	}, "\n")
	if got, err := executableOf(out, "cli", "bin", "demo"); err != nil || got != "/t/debug/demo" {
		t.Errorf("executableOf = %q, %v", got, err)
	}
	if _, err := executableOf(out, "lib", "bin", "demo"); err == nil {
		t.Error("a library has no executable")
	}
	if _, err := executableOf(out, "cli", "example", "demo"); err == nil {
		t.Error("a binary is not an example")
	}
}

func TestBuild(t *testing.T) {
	t.Parallel()
	needCargo(t)
	r := newRunner(t)
	ctx := t.Context()
	exe, err := r.Build(ctx, fixture, "tool", "bin", "tool")
	if err != nil {
		t.Fatal(err)
	}
	tmp, _ := r.Session.TempDir()
	if rel, err := filepath.Rel(tmp, exe); err != nil || !filepath.IsLocal(rel) {
		t.Errorf("executable %s is not in the session directory %s", exe, tmp)
	}
	if _, err := os.Stat(exe); err != nil {
		t.Error(err)
	}
	if _, err := os.Stat(filepath.Join(fixture, "target")); err == nil {
		t.Error("a target directory appeared in the fixture")
	}
	if _, err := r.Build(ctx, fixture, "nope", "bin", "tool"); err == nil {
		t.Error("an unknown package built")
	}
}
