package cli

import (
	"testing"
)

func descriptionRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		".docgen.yaml": "docs: docs\n",
		"Cargo.toml": `[workspace]
members = ["crates/*"]
resolver = "3"

[workspace.package]
version = "1.0.0"
edition = "2021"
description = "Shared words"
`,
		"crates/manifest/Cargo.toml": "[package]\nname = \"manifest\"\nversion.workspace = true\nedition.workspace = true\ndescription = \"From the manifest | with a pipe\"\n",
		"crates/manifest/src/lib.rs": "//! Not used.\n",
		"crates/inherit/Cargo.toml":  "[package]\nname = \"inherit\"\nversion.workspace = true\nedition.workspace = true\ndescription.workspace = true\n",
		"crates/inherit/src/lib.rs":  "//! Not used.\n",
		"crates/doclib/Cargo.toml":   "[package]\nname = \"doclib\"\nversion.workspace = true\nedition.workspace = true\n",
		"crates/doclib/src/lib.rs":   "// plain comment\n#![forbid(unsafe_code)]\n//! # Doc title\n//!\n//! Body.\n",
		"crates/docbin/Cargo.toml":   "[package]\nname = \"docbin\"\nversion.workspace = true\nedition.workspace = true\n",
		"crates/docbin/src/main.rs":  "//! A binary. It has two sentences.\nfn main() {}\n",
		"crates/nodoc/Cargo.toml":    "[package]\nname = \"nodoc\"\nversion.workspace = true\nedition.workspace = true\n",
		"crates/nodoc/src/lib.rs":    "/// An item.\npub fn f() {}\n",
		"crates/blockdoc/Cargo.toml": "[package]\nname = \"blockdoc\"\nversion.workspace = true\nedition.workspace = true\n",
		"crates/blockdoc/src/lib.rs": "/*! Block docs. */\n",
		"crates/both/Cargo.toml":     "[package]\nname = \"both\"\nversion.workspace = true\nedition.workspace = true\n",
		"crates/both/src/lib.rs":     "//! The library.\n",
		"crates/both/src/main.rs":    "//! The binary.\nfn main() {}\n",
		"docs/p.md":                  "<!-- docgen:crates -->\n<!-- /docgen:crates -->\n",
	})
	return root
}

func TestCratesDescription(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	root := descriptionRepo(t)
	got := updateTwice(t, root, "docs/p.md")
	wantIn(t, got,
		"| Crate | Path | Kind | Version | Depends on | Description |",
		"| `manifest` | `crates/manifest` | lib | 1.0.0 |  | From the manifest \\| with a pipe |",
		"| `inherit` | `crates/inherit` | lib | 1.0.0 |  | Shared words |",
		"| `doclib` | `crates/doclib` | lib | 1.0.0 |  | Doc title |",
		"| `docbin` | `crates/docbin` | bin | 1.0.0 |  | A binary. It has two sentences. |",
		"| `nodoc` | `crates/nodoc` | lib | 1.0.0 |  |  |",
		"| `blockdoc` | `crates/blockdoc` | lib | 1.0.0 |  |  |",
		"| `both` | `crates/both` | lib, bin | 1.0.0 |  | The library. |",
	)
}

func TestCratesDescriptionWithCargo(t *testing.T) {
	t.Parallel()
	needCargo(t)
	root := descriptionRepo(t)
	got := updateTwice(t, root, "docs/p.md")
	wantIn(t, got,
		"| `manifest` | `crates/manifest` | lib | 1.0.0 |  | From the manifest \\| with a pipe |",
		"| `inherit` | `crates/inherit` | lib | 1.0.0 |  | Shared words |",
		"| `docbin` | `crates/docbin` | bin | 1.0.0 |  | A binary. It has two sentences. |",
		"| `nodoc` | `crates/nodoc` | lib | 1.0.0 |  |  |",
	)
}
