# Show Cargo values and crates

Put a version, a toolchain, the features of a crate or the crates of a workspace in a page, so that the page changes when `Cargo.toml` does. These markers need a Cargo workspace in the module directory, and `crate=` and the crate table need `cargo` on the `PATH`.

## A manifest value

<!-- site:editor -->
<!-- docgen:code file=internal/cli/testdata/rust/Cargo.toml symbol=workspace.package -->
```toml
[workspace.package]
version = "0.3.0"
edition = "2021"
rust-version = "1.85"
license = "MIT"
authors = ["Docgen Fixture"]
```
<!-- /docgen:code -->
<!-- docgen:code file=internal/cli/testdata/rust/crates/cargo-b-core/Cargo.toml symbol=package -->
```toml
[package]
name = "cargo-b-core"
version.workspace = true
edition.workspace = true
rust-version.workspace = true
license.workspace = true
description = "Core of the cargo marker fixture"
publish = false
```
<!-- /docgen:code -->
<!-- docgen:code file=internal/cli/testdata/rust/rust-toolchain.toml -->
```toml
[toolchain]
channel = "stable"
components = ["rustfmt", "clippy"]
profile = "minimal"
```
<!-- /docgen:code -->
<!-- docgen:docgen root=internal/cli/testdata/rust page=docs/guide-cargo.md -->
#### Before

```
- Version: <!-- docgen:cargo field=workspace.package.version --><!-- /docgen:cargo -->
- Minimum Rust: <!-- docgen:cargo field=package.rust-version crate=cargo-b-core --><!-- /docgen:cargo -->
- Version of the core crate: <!-- docgen:cargo field=package.version crate=cargo-b-core code=false --><!-- /docgen:cargo -->
- Channel: <!-- docgen:cargo field=toolchain.channel file=rust-toolchain.toml --><!-- /docgen:cargo -->
- Components: <!-- docgen:cargo field=toolchain.components file=rust-toolchain.toml --><!-- /docgen:cargo -->
```

#### After

```
- Version: <!-- docgen:cargo field=workspace.package.version -->`0.3.0`<!-- /docgen:cargo -->
- Minimum Rust: <!-- docgen:cargo field=package.rust-version crate=cargo-b-core -->`1.85`<!-- /docgen:cargo -->
- Version of the core crate: <!-- docgen:cargo field=package.version crate=cargo-b-core code=false -->0.3.0<!-- /docgen:cargo -->
- Channel: <!-- docgen:cargo field=toolchain.channel file=rust-toolchain.toml -->`stable`<!-- /docgen:cargo -->
- Components: <!-- docgen:cargo field=toolchain.components file=rust-toolchain.toml -->`rustfmt, clippy`<!-- /docgen:cargo -->
```
<!-- /docgen:docgen -->
<!-- /site:editor -->

`field=` is a dotted key path. Without `crate=` or `file=`, the marker reads the `Cargo.toml` in the module directory, so a virtual workspace uses `workspace.package.version`. `crate=` reads the `Cargo.toml` of a package of the workspace. `file=` reads any TOML file in the repository, such as `rust-toolchain.toml`.

A key that a crate sets to `workspace = true`, such as `version.workspace = true`, has the value from `[workspace.package]`. A dependency that a crate sets to `workspace = true` has the keys from `[workspace.dependencies]`. An array shows as its items with commas, a table is an error, and so are a missing file and a missing key. With `code=false`, the value has no backticks.

## The features of a crate

<!-- site:editor -->
<!-- docgen:docgen root=internal/cli/testdata/rust page=docs/guide-cargo-features.md -->
#### Before

```
<!-- docgen:cargo field=features crate=cargo-b-core -->
<!-- /docgen:cargo -->
```

#### After

```
<!-- docgen:cargo field=features crate=cargo-b-core -->
| Feature | Default | Enables |
|---|---|---|
| `checked` |  | `dep:limits`, `strict` |
| `fast` | yes |  |
| `strict` |  | `fast` |
<!-- /docgen:cargo -->
```
<!-- /docgen:docgen -->
<!-- /site:editor -->

`field=features` with `crate=` is a table of the features in name order. Default has `yes` for a feature that the `default` feature enables. A crate with no features is an error.

When the `features` settings or `features=` on the marker enable something, the table has an Enabled column with `yes` for each feature that a cargo command of the marker enables, with the features that those enable in the same crate:

<!-- site:editor -->
<!-- docgen:docgen root=internal/cli/testdata/rust page=docs/guide-features-table.md -->
#### Before

```
<!-- docgen:cargo field=features crate=feat-gate features=big nodefaultfeatures=true -->
<!-- /docgen:cargo -->
```

#### After

```
<!-- docgen:cargo field=features crate=feat-gate features=big nodefaultfeatures=true -->
| Feature | Default | Enabled | Enables |
|---|---|---|---|
| `big` |  | yes | `small` |
| `extra` |  |  |  |
| `small` | yes | yes |  |
<!-- /docgen:cargo -->
```
<!-- /docgen:docgen -->
<!-- /site:editor -->

## The crates of a workspace

<!-- site:editor -->
<!-- docgen:docgen root=internal/cli/testdata/rust page=docs/guide-crates.md -->
#### Before

```
<!-- docgen:crates -->
<!-- /docgen:crates -->
```

#### After

```
<!-- docgen:crates -->
| Crate | Path | Kind | Version | Depends on |
|---|---|---|---|---|
| `cargo-b-cli` | `crates/cargo-b-cli` | bin | 0.3.0 | `cargo-b-core` |
| `cargo-b-core` | `crates/cargo-b-core` | lib | 0.3.0 | `limits` |
| `core-a` | `crates/core-a` | lib | 0.1.0 |  |
| `core-a-bin` | `crates/core-a-bin` | bin | 0.1.0 |  |
| `feat-gate` | `crates/feat-gate` | lib | 0.1.0 |  |
| `limits` | `crates/limits` | lib | 0.1.0 |  |
| `run-c` | `crates/run-c` | lib, bin | 0.1.0 |  |
| `run-c-broken` | `crates/run-c-broken` | lib | 0.1.0 |  |
| `tool` | `crates/tool` | bin | 0.2.0 | `limits` |
<!-- /docgen:crates -->
```
<!-- /docgen:docgen -->
<!-- /site:editor -->

Each row is a crate with its path, kind, version and the crates of the workspace that it depends on, in name order. `kind=` keeps one kind of crate, `match=` keeps the names that a glob matches, and `deps=` picks the dependencies in the last column: `normal`, `dev`, `build` or `all`.

<!-- site:editor -->
<!-- docgen:docgen root=internal/cli/testdata/rust page=docs/guide-crates-filter.md -->
#### Before

```
<!-- docgen:crates kind=bin deps=all -->
<!-- /docgen:crates -->
```

#### After

```
<!-- docgen:crates kind=bin deps=all -->
| Crate | Path | Kind | Version | Depends on |
|---|---|---|---|---|
| `cargo-b-cli` | `crates/cargo-b-cli` | bin | 0.3.0 | `cargo-b-core`, `limits` (dev) |
| `core-a-bin` | `crates/core-a-bin` | bin | 0.1.0 |  |
| `run-c` | `crates/run-c` | lib, bin | 0.1.0 |  |
| `tool` | `crates/tool` | bin | 0.2.0 | `limits` |
<!-- /docgen:crates -->
```
<!-- /docgen:docgen -->
<!-- /site:editor -->

For details, see [`docgen:cargo`](../reference/markers.md#docgencargo) and [`docgen:crates`](../reference/markers.md#docgencrates).
