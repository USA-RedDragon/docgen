package cargo

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// writeTree writes each file of files, by slash path, below dir.
func writeTree(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, text := range files {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

// normalized returns a copy of w with the targets and the local dependencies
// of each package sorted, since cargo metadata keeps the order of the
// manifest.
func normalized(w *Workspace) []Package {
	out := make([]Package, len(w.Packages))
	for i, p := range w.Packages {
		out[i] = *p
		out[i].Targets = slices.Clone(p.Targets)
		slices.SortFunc(out[i].Targets, func(a, b Target) int {
			return strings.Compare(a.Kind+"\x00"+a.Name, b.Kind+"\x00"+b.Name)
		})
		out[i].LocalDeps = slices.Clone(p.LocalDeps)
		slices.SortFunc(out[i].LocalDeps, func(a, b Dependency) int {
			return strings.Compare(a.Name+"\x00"+a.Kind, b.Name+"\x00"+b.Kind)
		})
	}
	return out
}

func TestReadWorkspaceMatchesMetadata(t *testing.T) {
	t.Parallel()
	needCargo(t)
	dir := copyDir(t, fixture)
	want, err := newRunner(t).Open(t.Context(), dir)
	if err != nil {
		t.Fatal(err)
	}
	got, err := ReadWorkspace(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got.Root != want.Root {
		t.Errorf("Root = %s, want %s", got.Root, want.Root)
	}
	g, w := normalized(got), normalized(want)
	if len(g) != len(w) {
		t.Fatalf("%d packages, want %d", len(g), len(w))
	}
	for i := range w {
		if !reflect.DeepEqual(g[i], w[i]) {
			t.Errorf("package %s:\n got %+v\nwant %+v", w[i].Name, g[i], w[i])
		}
	}
}

func TestReadWorkspace(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeTree(t, dir, map[string]string{
		"Cargo.toml": `[package]
name = "top"
version = "0.9.0"

[workspace]
members = ["crates/*", "tools/gen"]
exclude = ["crates/skipped"]

[workspace.package]
version = "1.2.3"
edition = "2024"
rust-version = "1.85"
publish = ["crates-io", "inner"]
description = "The shared description"

[workspace.dependencies]
shared = { path = "crates/shared" }
`,
		"src/main.rs": "fn main() {}\n",
		"crates/app/Cargo.toml": `[package]
name = "app"
version.workspace = true
edition.workspace = true
rust-version.workspace = true
publish.workspace = true
description.workspace = true
autoexamples = false

[lib]
name = "app_core"
path = "src/core.rs"

[[bin]]
name = "app-cli"

[features]
fancy = []
both = ["fancy", "dep:extra"]

[dependencies]
shared = { workspace = true }
extra = { version = "1", optional = true }
lazy = { version = "1", optional = true }
renamed = { package = "helper", path = "../helper" }

[target.'cfg(unix)'.dev-dependencies]
top = { path = "../.." }

[build-dependencies]
helper = { path = "../helper" }
`,
		"crates/app/src/core.rs":           "",
		"crates/app/src/bin/app-cli.rs":    "fn main() {}\n",
		"crates/app/src/bin/other.rs":      "fn main() {}\n",
		"crates/app/src/bin/multi/main.rs": "fn main() {}\n",
		"crates/app/examples/hidden.rs":    "fn main() {}\n",
		"crates/app/build.rs":              "fn main() {}\n",
		"crates/app/tests/it.rs":           "",
		"crates/shared/Cargo.toml": `[package]
name = "shared"
edition = "2021"

[lib]
proc-macro = true
`,
		"crates/shared/src/lib.rs":            "",
		"crates/shared/examples/demo/main.rs": "fn main() {}\n",
		"crates/helper/Cargo.toml": `[package]
name = "helper"
build = false
publish = false
`,
		"crates/helper/src/lib.rs":  "",
		"crates/helper/build.rs":    "fn main() {}\n",
		"crates/skipped/Cargo.toml": "not toml [",
		"crates/notes.txt":          "",
		"tools/gen/Cargo.toml":      "[package]\nname = \"gen\"\nversion = \"0.1.0\"\nedition = \"2021\"\n",
		"tools/gen/src/main.rs":     "fn main() {}\n",
	})
	ws, err := ReadWorkspace(dir)
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(ws.Packages))
	for _, p := range ws.Packages {
		names = append(names, p.Name)
	}
	if want := []string{"app", "gen", "helper", "shared", "top"}; !slices.Equal(names, want) {
		t.Fatalf("packages %v, want %v", names, want)
	}
	app, err := ws.Package("app")
	if err != nil {
		t.Fatal(err)
	}
	if app.Version != "1.2.3" || app.Edition != "2024" || app.RustVersion != "1.85" {
		t.Errorf("app fields %s %s %s, want the workspace values", app.Version, app.Edition, app.RustVersion)
	}
	if app.Description != "The shared description" {
		t.Errorf("app description %q, want the workspace value", app.Description)
	}
	if shared, err := ws.Package("shared"); err != nil || shared.Description != "" {
		t.Errorf("shared description %q, %v, want none", shared.Description, err)
	}
	appDir := filepath.Join(dir, "crates", "app")
	wantTargets := []Target{
		{Name: "app_core", Kind: KindLib, Path: filepath.Join(appDir, "src", "core.rs"), Edition: "2024"},
		{Name: "app-cli", Kind: kindBin, Path: filepath.Join(appDir, "src", "bin", "app-cli.rs"), Edition: "2024"},
		{Name: "multi", Kind: kindBin, Path: filepath.Join(appDir, "src", "bin", "multi", "main.rs"), Edition: "2024"},
		{Name: "other", Kind: kindBin, Path: filepath.Join(appDir, "src", "bin", "other.rs"), Edition: "2024"},
		{Name: "it", Kind: kindTest, Path: filepath.Join(appDir, "tests", "it.rs"), Edition: "2024"},
		{Name: "build-script-build", Kind: "custom-build", Path: filepath.Join(appDir, "build.rs"), Edition: "2024"},
	}
	if !reflect.DeepEqual(app.Targets, wantTargets) {
		t.Errorf("app targets:\n got %+v\nwant %+v", app.Targets, wantTargets)
	}
	if want := []string{"crates-io", "inner"}; !reflect.DeepEqual(app.Registries, want) {
		t.Errorf("app registries %v, want %v", app.Registries, want)
	}
	wantFeatures := map[string][]string{"fancy": {}, "both": {"fancy", "dep:extra"}, "lazy": {"dep:lazy"}}
	if !reflect.DeepEqual(app.Features, wantFeatures) {
		t.Errorf("app features %v, want %v", app.Features, wantFeatures)
	}
	wantDeps := []Dependency{
		{Name: "helper", Path: filepath.Join(dir, "crates", "helper"), Kind: "normal"},
		{Name: "helper", Path: filepath.Join(dir, "crates", "helper"), Kind: "build"},
		{Name: "shared", Path: filepath.Join(dir, "crates", "shared"), Kind: "normal"},
		{Name: "top", Path: dir, Kind: "dev"},
	}
	if !reflect.DeepEqual(app.LocalDeps, wantDeps) {
		t.Errorf("app dependencies:\n got %+v\nwant %+v", app.LocalDeps, wantDeps)
	}
	shared, _ := ws.Package("shared")
	if shared.Version != "0.0.0" || shared.Lib() != nil || len(shared.Targets) != 2 || shared.Targets[0].Kind != "proc-macro" || shared.Targets[1].Name != "demo" {
		t.Errorf("shared: %+v", shared)
	}
	helper, _ := ws.Package("helper")
	if helper.Edition != "2015" || len(helper.Targets) != 1 || helper.Registries == nil || len(helper.Registries) != 0 {
		t.Errorf("helper: %+v", helper)
	}
	top, _ := ws.Package("top")
	if top.Version != "0.9.0" || top.Registries != nil || len(top.Targets) != 1 || top.Targets[0].Name != "top" || top.Targets[0].Kind != kindBin {
		t.Errorf("top: %+v", top)
	}
}

func TestReadWorkspaceErrors(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		files map[string]string
		want  string
	}{
		{"no manifest", map[string]string{"x.txt": ""}, "no Cargo.toml in"},
		{"bad toml", map[string]string{"Cargo.toml": "[package\n"}, "Cargo.toml line 1"},
		{"no member", map[string]string{"Cargo.toml": "[workspace]\nmembers = [\"a\"]\n"}, "workspace member a has no Cargo.toml"},
		{"empty", map[string]string{"Cargo.toml": "[workspace]\n"}, "has no package and no workspace members"},
		{"no name", map[string]string{"Cargo.toml": "[workspace]\nmembers = [\"a\"]\n", "a/Cargo.toml": "[package]\nversion = \"1.0.0\"\n"}, "a/Cargo.toml has no package.name"},
		{"no inherited dep", map[string]string{"Cargo.toml": "[package]\nname = \"a\"\n[dependencies]\nb = { workspace = true }\n"}, "no workspace.dependencies.b"},
		{"no bin file", map[string]string{"Cargo.toml": "[package]\nname = \"a\"\n[[bin]]\nname = \"x\"\n"}, "the bin target x has no path"},
		{"no lib file", map[string]string{"Cargo.toml": "[package]\nname = \"a\"\n[lib]\npath = \"lib.rs\"\n"}, "the library target a has no file"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			writeTree(t, dir, tc.files)
			_, err := ReadWorkspace(dir)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error %v, want one with %q", err, tc.want)
			}
		})
	}
}
