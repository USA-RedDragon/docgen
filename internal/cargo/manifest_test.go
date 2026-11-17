package cargo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const wsText = `[workspace]
members = ["a"]

[workspace.package]
version = "1.2.3"
edition = "2021"
authors = ["A", "B"]
publish = false
license = "MIT"

[workspace.dependencies]
serde = "1"
clap = { version = "4", features = ["derive"], default-features = false }
`

const memberText = `[package]
name = "a"
version.workspace = true
edition.workspace = true
authors.workspace = true
publish.workspace = true
rust-version = "1.80"
build = 7
ratio = 1.5

[package.metadata.docs]
targets = ["x", "y"]

[dependencies]
serde = { workspace = true }
clap = { workspace = true, features = ["env"], optional = true }
other = "2"
`

func manifest(t *testing.T, text string) *Manifest {
	t.Helper()
	p := filepath.Join(t.TempDir(), "Cargo.toml")
	if err := os.WriteFile(p, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	m, err := ReadManifest(p, "crates/a/Cargo.toml")
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestField(t *testing.T) {
	t.Parallel()
	ws := manifest(t, wsText)
	ws.Name = "Cargo.toml"
	m := manifest(t, memberText)
	load := func() (*Manifest, error) { return ws, nil }
	for field, want := range map[string]string{
		"package.name":                       "a",
		"package.version":                    "1.2.3",
		"package.edition":                    "2021",
		"package.authors":                    "A, B",
		"package.publish":                    "false",
		"package.rust-version":               "1.80",
		"package.build":                      "7",
		"package.ratio":                      "1.5",
		"package.metadata.docs.targets":      "x, y",
		"dependencies.serde.version":         "1",
		"dependencies.clap.version":          "4",
		"dependencies.clap.features":         "derive, env",
		"dependencies.clap.optional":         "true",
		"dependencies.clap.default-features": "false",
		"dependencies.other":                 "2",
	} {
		got, err := m.Field(field, load)
		if err != nil || got != want {
			t.Errorf("%s = %q, %v, want %q", field, got, err, want)
		}
	}
	if got, err := ws.Field("workspace.package.version", nil); err != nil || got != "1.2.3" {
		t.Errorf("workspace.package.version = %q, %v", got, err)
	}
	if got, err := ws.Field("workspace.dependencies.serde", nil); err != nil || got != "1" {
		t.Errorf("workspace.dependencies.serde = %q, %v", got, err)
	}
}

func TestFieldErrors(t *testing.T) {
	t.Parallel()
	ws := manifest(t, wsText)
	ws.Name = "Cargo.toml"
	m := manifest(t, memberText)
	load := func() (*Manifest, error) { return ws, nil }
	empty := func() (*Manifest, error) { return manifest(t, "[workspace]\n"), nil }
	for name, c := range map[string]struct {
		m     *Manifest
		field string
		ws    func() (*Manifest, error)
		want  string
	}{
		"missing":          {m, "package.license", load, "crates/a/Cargo.toml has no package.license; the keys of package are: authors,"},
		"missing top":      {m, "profile", load, "has no profile; the keys are: dependencies, package"},
		"table":            {m, "package.metadata", load, "package.metadata is a table, not a value; select a key in it; the keys are: docs"},
		"through a string": {m, "package.name.x", load, "package.name is a string, so it has no key x"},
		"through a number": {m, "package.build.x", load, "package.build is a number, so it has no key x"},
		"no workspace":     {m, "package.version", nil, "package.version is set to workspace = true, but there is no workspace manifest"},
		"not inherited":    {m, "package.edition", empty, "has no workspace.package.edition"},
		"no dependency":    {m, "dependencies.serde.version", empty, "has no workspace.dependencies.serde"},
		"array of tables":  {manifest(t, "[[bin]]\nname = \"x\"\n"), "bin", nil, "is an array that holds a table"},
		"date":             {manifest(t, "when = 2024-01-02\n"), "when", nil, "is a date or time"},
	} {
		_, err := c.m.Field(c.field, c.ws)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: %s: error %v, want %q", name, c.field, err, c.want)
		}
	}
}

func TestReadManifestErrors(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if _, err := ReadManifest(filepath.Join(dir, "nope.toml"), "nope.toml"); err == nil || err.Error() != "no nope.toml" {
		t.Errorf("missing file: %v", err)
	}
	bad := filepath.Join(dir, "bad.toml")
	if err := os.WriteFile(bad, []byte("a = \n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadManifest(bad, "bad.toml"); err == nil || !strings.Contains(err.Error(), "bad.toml line 1") {
		t.Errorf("invalid TOML: %v", err)
	}
}
