package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/USA-RedDragon/docgen/internal/cargo"
	"github.com/USA-RedDragon/docgen/internal/gocmd"
	"github.com/USA-RedDragon/docgen/internal/rustsrc"
)

// helperOnlyGuides returns the guides of the Rust fixture whose markers need
// the helper or the manifests and no cargo build.
func helperOnlyGuides() []string {
	return []string{
		"guide-cargo.md",
		"guide-cargo-features.md",
		"guide-code-symbol.md",
		"guide-crates.md",
		"guide-crates-filter.md",
		"guide-crates-columns.md",
		"guide-doc.md",
		"guide-doc-crate.md",
		"guide-errors.md",
		"guide-features-table.md",
	}
}

// cargoPage holds Rust markers that need a cargo build, a value test or a
// doctest, and one const that the helper reads.
const cargoPage = `- <!-- docgen:const crate=core-a name=limits::MAX_JOBS --><!-- /docgen:const -->
- <!-- docgen:const crate=core-a name=limits::LISTEN print=raw --><!-- /docgen:const -->

<!-- docgen:configulator crate=core-a type=config::AppConfig env=APP_ -->
<!-- /docgen:configulator -->

<!-- docgen:output crate=run-c example=greet -->
<!-- /docgen:output -->

<!-- docgen:output crate=run-c doctest=shapes::area -->
<!-- /docgen:output -->

<!-- docgen:usage crate=cargo-b-cli bin=demo -->
<!-- /docgen:usage -->

<!-- docgen:config crate=cfg-app type=guide::Listener -->
<!-- /docgen:config -->
`

// noCargoPath sets PATH to a new directory that holds only the go command,
// so that a run finds no cargo and no rustc.
func noCargoPath(t *testing.T) {
	t.Helper()
	goCmd, err := exec.LookPath("go")
	if err != nil {
		t.Skip("the go command is not on PATH")
	}
	if goCmd, err = filepath.EvalSymlinks(goCmd); err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	if err := os.Symlink(goCmd, filepath.Join(bin, filepath.Base(goCmd))); err != nil {
		t.Skip("no symlinks here:", err)
	}
	t.Setenv("PATH", bin)
}

// builtHelper builds the helper with cargo into the helper cache of the
// tests and returns its path.
func builtHelper(t *testing.T) string {
	t.Helper()
	s := &gocmd.Session{}
	t.Cleanup(func() { _ = s.Close() })
	h := &rustsrc.Helper{CacheDir: testOptions().rustHelperDir}
	bin, err := h.Path(t.Context(), &cargo.Runner{Session: s})
	if err != nil {
		t.Fatal(err)
	}
	return bin
}

// TestRustPrebuiltHelperNoCargo fills the guides whose markers need only the
// helper or the manifests with cargo, then checks them with no cargo and no
// rustc on PATH and a helper from DOCGEN_RUST_HELPER: they are current, so
// both runs write the same text. Each marker that needs cargo is an error
// that names it.
func TestRustPrebuiltHelperNoCargo(t *testing.T) {
	needCargo(t)
	bin := builtHelper(t)
	files := map[string]string{}
	for _, g := range helperOnlyGuides() {
		b, err := os.ReadFile(filepath.Join("testdata", "rust", "docs", g))
		if err != nil {
			t.Fatal(err)
		}
		files["docs/"+g] = string(b)
	}
	root := rustRepo(t, files)
	if rc, log := docgen(t, root, "update"); rc != 0 {
		t.Fatalf("update with cargo: rc %d, log:\n%s", rc, log)
	}
	noCargoPath(t)
	t.Setenv(rustsrc.BinEnv, bin)
	if rc, log := docgen(t, root, "check"); rc != 0 {
		t.Fatalf("check of the helper guides: rc %d, log:\n%s", rc, log)
	}

	writeTree(t, root, map[string]string{"docs/cargo.md": cargoPage})
	rc, log := docgen(t, root, "update")
	if rc != 1 {
		t.Fatalf("update: rc %d, want 1; log:\n%s", rc, log)
	}
	wantIn(t, log,
		"cargo is not on PATH, so docgen reads the Cargo workspace from its Cargo.toml files",
		"docs/cargo.md:1: docgen:const crate=core-a name=limits::MAX_JOBS: this marker needs cargo, which is not on PATH",
		"docs/cargo.md:4: docgen:configulator crate=core-a type=config::AppConfig env=APP_: this marker needs cargo",
		"docs/cargo.md:7: docgen:output crate=run-c example=greet: this marker needs cargo",
		"docs/cargo.md:10: docgen:output crate=run-c doctest=shapes::area: this marker needs cargo",
		"docs/cargo.md:13: docgen:usage crate=cargo-b-cli bin=demo: this marker needs cargo",
		"docs/cargo.md:16: docgen:config crate=cfg-app type=guide::Listener: this marker needs cargo",
	)
	wantNotIn(t, log, "docs/cargo.md:2:")
}

// TestRustHelperEnvMissing runs docgen with DOCGEN_RUST_HELPER set to a file
// that does not exist.
func TestRustHelperEnvMissing(t *testing.T) {
	root := rustRepo(t, map[string]string{"docs/doc.md": "<!-- docgen:doc crate=core-a name=net -->\n<!-- /docgen:doc -->\n"})
	t.Setenv(rustsrc.BinEnv, "no-such-helper")
	rc, log := docgen(t, root, "update")
	if rc != 1 {
		t.Fatalf("rc %d, want 1; log:\n%s", rc, log)
	}
	wantIn(t, log, "docs/doc.md:1: docgen:doc crate=core-a name=net: the Rust helper of DOCGEN_RUST_HELPER: stat "+filepath.Join(root, "no-such-helper"))
}
