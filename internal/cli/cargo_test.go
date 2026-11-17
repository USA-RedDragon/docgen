package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func rustFixture(t *testing.T, page string) string {
	t.Helper()
	return rustRepo(t, map[string]string{".docgen.yaml": "docs: docs\n", "docs/page.md": page})
}

func TestCargoMarkers(t *testing.T) {
	t.Parallel()
	needCargo(t)
	root := rustFixture(t, strings.Join([]string{
		"<!-- docgen:cargo field=workspace.package.version --><!-- /docgen:cargo -->",
		"<!-- docgen:cargo field=package.version crate=cargo-b-core code=false --><!-- /docgen:cargo -->",
		"<!-- docgen:cargo field=package.rust-version crate=cargo-b-cli --><!-- /docgen:cargo -->",
		"<!-- docgen:cargo field=toolchain.components file=rust-toolchain.toml --><!-- /docgen:cargo -->",
		"<!-- docgen:cargo field=dependencies.clap.features crate=cargo-b-cli code=false --><!-- /docgen:cargo -->",
		"<!-- docgen:crates kind=lib --><!-- /docgen:crates -->",
		"<!-- docgen:crates kind=bin match=cargo-* deps=all --><!-- /docgen:crates -->",
		"<!-- docgen:usage crate=cargo-b-cli --><!-- /docgen:usage -->",
		"<!-- docgen:usage crate=cargo-b-cli bin=demo args=\"label --help\" --><!-- /docgen:usage -->",
		"",
	}, "\n"))
	if rc, log := docgen(t, root, "update", "-jobs", "4"); rc != 0 {
		t.Fatalf("update: rc %d, log:\n%s", rc, log)
	}
	got := read(t, root, "docs/page.md")
	for _, want := range []string{
		"-->`0.3.0`<!-- /docgen:cargo -->",
		"code=false -->0.3.0<!--",
		"crate=cargo-b-cli -->`1.88`<",
		"-->`rustfmt, clippy`<",
		"code=false -->derive<",
		"| Crate | Path | Kind | Version | Depends on | Description |",
		"| `cargo-b-core` | `crates/cargo-b-core` | lib | 0.3.0 | `limits` | Core of the cargo marker fixture |",
		"| `limits` | `crates/limits` | lib | 0.1.0 |  | Limits of the fixture. |",
		"| `cargo-b-cli` | `crates/cargo-b-cli` | bin | 0.3.0 | `cargo-b-core`, `limits` (dev) |",
		"Usage: demo [OPTIONS] <COMMAND>",
		"Usage: demo label [OPTIONS]",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("page has no %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "| `tool`") {
		t.Errorf("match=cargo-* kept tool:\n%s", got)
	}
	if rc, log := docgen(t, root, "check"); rc != 0 {
		t.Errorf("check after update: rc %d, log:\n%s", rc, log)
	}
	if _, err := os.Stat(filepath.Join(root, "target")); err == nil {
		t.Error("docgen made a target directory in the repository")
	}
	lock, _ := os.ReadFile("testdata/rust/Cargo.lock")
	if now := read(t, root, "Cargo.lock"); now != string(lock) {
		t.Error("docgen changed Cargo.lock")
	}
}

func TestCargoErrors(t *testing.T) {
	t.Parallel()
	needCargo(t)
	for name, c := range map[string]struct{ tag, want string }{
		"no key":            {"cargo field=package.rust-version", "Cargo.toml has no package; the keys are: workspace"},
		"no workspace key":  {"cargo field=workspace.package.nope", "Cargo.toml has no workspace.package.nope; the keys of workspace.package are:"},
		"table":             {"cargo field=workspace.package", "is a table, not a value"},
		"no file":           {"cargo field=toolchain.channel file=nope.toml", "no nope.toml"},
		"outside":           {"cargo field=a file=../x.toml", "file=../x.toml is outside the repository"},
		"unknown crate":     {"cargo field=package.name crate=nope", `crate "nope" is not in the Cargo workspace`},
		"missing in crate":  {"cargo field=package.license crate=cargo-b-cli", "crates/cargo-b-cli/Cargo.toml has no package.license"},
		"features, none":    {"cargo field=features crate=limits", "crate limits has no features"},
		"features, unknown": {"cargo field=features crate=nope", `crate "nope" is not in the Cargo workspace`},
		"crates, none":      {"crates match=zz*", "no crate of the Cargo workspace matches"},
		"no binary":         {"usage crate=limits", "crate limits has no binary"},
		"other binary":      {"usage crate=tool bin=nope", "crate tool has no binary nope; its binaries are: tool"},
		"usage, unknown":    {"usage crate=nope", `crate "nope" is not in the Cargo workspace`},
		"usage, no usage":   {"usage crate=tool args=--version", "printed no usage"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := rustFixture(t, "<!-- docgen:"+c.tag+" --><!-- /docgen:"+strings.Fields(c.tag)[0]+" -->\n")
			rc, log := docgen(t, root, "update", "-jobs", "4")
			if rc != 1 || !strings.Contains(log, c.want) {
				t.Errorf("rc %d, want 1 and %q, log:\n%s", rc, c.want, log)
			}
		})
	}
}

func TestCargoLockedOutOfDate(t *testing.T) {
	t.Parallel()
	needCargo(t)
	root := rustFixture(t, "<!-- docgen:usage crate=tool --><!-- /docgen:usage -->\n")
	writeTree(t, root, map[string]string{"crates/tool/Cargo.toml": strings.Replace(read(t, root, "crates/tool/Cargo.toml"), `version = "0.2.0"`, `version = "0.2.1"`, 1)})
	rc, log := docgen(t, root, "update", "-jobs", "4")
	if rc != 1 || !strings.Contains(log, "Cargo.lock is out of date") {
		t.Errorf("rc %d, log:\n%s", rc, log)
	}
}

func read(t *testing.T, root, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
