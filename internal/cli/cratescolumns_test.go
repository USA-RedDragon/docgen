package cli

import "testing"

const columnsManifest = `[workspace]
members = ["crates/*"]
resolver = "3"

[workspace.package]
version = "1.0.0"
edition = "2021"

[workspace.metadata.fixture]
owner = "workspace"
`

func columnsRepo(t *testing.T, marker string) string {
	t.Helper()
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		".docgen.yaml": "docs: docs\n",
		"Cargo.toml":   columnsManifest,
		"crates/a/Cargo.toml": `[package]
name = "a"
version.workspace = true
edition.workspace = true

[package.metadata.fixture]
owner = "team | one"
stable = true
level = 3
ratio = 1.5
tags = ["x", "y", 2]
created = 2026-11-17

[package.metadata.fixture.docs]
tier = "gold"

`,
		"crates/a/src/lib.rs": "",
		"crates/b/Cargo.toml": "[package]\nname = \"b\"\nversion.workspace = true\nedition.workspace = true\n\n[package.metadata.fixture]\nother = 1\n",
		"crates/b/src/lib.rs": "",
		"crates/c/Cargo.toml": "[package]\nname = \"c\"\nversion.workspace = true\nedition.workspace = true\n\n[package.metadata.fixture]\nowner = \"c\"\ntags = []\n",
		"crates/c/src/lib.rs": "",
		"docs/p.md":           "<!-- docgen:crates " + marker + " -->\n<!-- /docgen:crates -->\n",
	})
	return root
}

func checkColumns(t *testing.T, root string) {
	t.Helper()
	got := updateTwice(t, root, "docs/p.md")
	wantIn(t, got,
		"| Crate | Path | Kind | Version | Depends on | Description | owner | stable | level | ratio | tags | tier | created |",
		"|---|---|---|---|---|---|---|---|---|---|---|---|---|",
		"| `a` | `crates/a` | lib | 1.0.0 |  |  | team \\| one | true | 3 | 1.5 | x, y, 2 | gold | 2026-11-17 |",
		"| `b` | `crates/b` | lib | 1.0.0 |  |  |  |  |  |  |  |  |  |",
		"| `c` | `crates/c` | lib | 1.0.0 |  |  | c |  |  |  |  |  |  |",
	)
}

const columnsAll = `columns=metadata.fixture.owner,metadata.fixture.stable,metadata.fixture.level,metadata.fixture.ratio,metadata.fixture.tags,metadata.fixture.docs.tier,metadata.fixture.created`

func TestCratesColumns(t *testing.T) {
	t.Parallel()
	needCargo(t)
	checkColumns(t, columnsRepo(t, columnsAll))
}

func TestCratesColumnsNoCargo(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	checkColumns(t, columnsRepo(t, columnsAll))
}

func TestCratesColumnsNoWorkspaceInheritance(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	root := columnsRepo(t, "match=b columns=metadata.fixture.owner")
	writeTree(t, root, map[string]string{"crates/b/Cargo.toml": "[package]\nname = \"b\"\nversion.workspace = true\nedition.workspace = true\n\n[package.metadata.fixture]\nowner.workspace = true\n"})
	rc, log := docgen(t, root, "check")
	if rc != 1 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
	wantIn(t, log, "crate b: metadata.fixture.owner is a table, not a scalar or an array of scalars")
}

func TestCratesColumnsErrors(t *testing.T) {
	t.Parallel()
	for name, c := range map[string]struct{ marker, want string }{
		"table":        {"match=a columns=metadata.fixture.docs", "crate a: metadata.fixture.docs is a table, not a scalar or an array of scalars"},
		"namespace":    {"match=a columns=metadata.fixture", "crate a: metadata.fixture is a table, not a scalar or an array of scalars"},
		"empty":        {`columns=""`, "columns= is empty"},
		"no prefix":    {"columns=fixture.owner", `columns= key "fixture.owner" is not metadata.<key>[.<key>...]`},
		"full path":    {"columns=package.metadata.fixture.owner", `columns= key "package.metadata.fixture.owner" is not metadata.<key>[.<key>...]`},
		"only prefix":  {"columns=metadata", `columns= key "metadata" is not metadata.<key>[.<key>...]`},
		"empty key":    {"columns=metadata.fixture.owner,", `columns= key "" is not metadata.<key>[.<key>...]`},
		"empty seg":    {"columns=metadata..owner", `columns= key "metadata..owner" is not metadata.<key>[.<key>...]`},
		"slash":        {"columns=metadata.a/b", `columns= key "metadata.a/b" is not metadata.<key>[.<key>...]`},
		"space in key": {`columns="metadata.a b"`, `columns= key "metadata.a b" is not metadata.<key>[.<key>...]`},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := columnsRepo(t, c.marker)
			rc, log := docgen(t, root, "check")
			if rc != 1 {
				t.Fatalf("rc %d, log:\n%s", rc, log)
			}
			wantIn(t, log, c.want)
		})
	}
}

func TestCratesColumnsNestedArray(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	root := columnsRepo(t, "columns=metadata.fixture.owner")
	writeTree(t, root, map[string]string{"crates/c/Cargo.toml": "[package]\nname = \"c\"\nversion.workspace = true\nedition.workspace = true\n\n[package.metadata.fixture]\nowner = [[1], \"x\"]\n"})
	rc, log := docgen(t, root, "check")
	if rc != 1 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
	wantIn(t, log, "crate c: metadata.fixture.owner has an array as item 1, not a scalar")
}
