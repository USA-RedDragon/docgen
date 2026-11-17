# docgen

[![Go Reference](https://pkg.go.dev/badge/github.com/USA-RedDragon/docgen.svg)](https://pkg.go.dev/github.com/USA-RedDragon/docgen) [![go.mod version](https://img.shields.io/github/go-mod/go-version/USA-RedDragon/docgen.svg?filename=go.mod)](https://github.com/USA-RedDragon/docgen) [![coverage](https://raw.githubusercontent.com/USA-RedDragon/docgen/main/.github/badges/coverage.svg)](https://github.com/USA-RedDragon/docgen/actions/workflows/test.yaml) [![License](https://badgen.net/github/license/USA-RedDragon/docgen)](https://github.com/USA-RedDragon/docgen/blob/main/LICENSE)

docgen fills the facts in your Markdown docs from your Go and Rust code. You write the prose and put a marker where a fact from the code goes. `docgen update` writes the fact into the marker, and `docgen check` fails in CI when a page no longer matches the code.

## Installation

Download the archive of a [release](https://github.com/USA-RedDragon/docgen/releases) for Linux, macOS or Windows (amd64 or arm64), check it against `checksums.txt`, and put `docgen` on your `PATH`. Or, with Go <!-- docgen:gomod field=go -->`1.27.0`<!-- /docgen:gomod --> or later:

```sh
go install github.com/USA-RedDragon/docgen@latest
```

- Go repository: needs `go` on the `PATH`, and a `go.mod`. Markers that use no Go code, such as a YAML excerpt, need neither.
- Rust repository: needs `cargo` on the `PATH` with stable Rust, and a Cargo workspace. It needs no Go and no `go.mod`, so the release binary is enough.
- A repository with both needs both.
- GitHub Action: <!-- docgen:text text="`uses: USA-RedDragon/docgen@{version}`" -->`uses: USA-RedDragon/docgen@v0.1.0`<!-- /docgen:text -->. See [Check the docs in CI](docs/start/03-check-in-ci.md#add-the-github-action).
- pre-commit hook: `id: docgen-check` of the repository `https://github.com/USA-RedDragon/docgen`, or `docgen-check-system` for a Rust repository with no Go. See [Use pre-commit](docs/guides/pre-commit.md).

## Getting started

Put a marker in any Markdown page, run `docgen update` from the root of the repository, and docgen writes the value between the two comments. For a Go package, `pkg=` is the path of the package in the module:

<!-- docgen:docgen root=internal/cli/testdata/repo module=v2 page=docs/machines/guide-const.md -->
#### Before

```
The default wait is <!-- docgen:const pkg=machine name=DefaultWait --><!-- /docgen:const -->.
```

#### After

```
The default wait is <!-- docgen:const pkg=machine name=DefaultWait -->`250ms`<!-- /docgen:const -->.
```
<!-- /docgen:docgen -->

For a Rust crate, `crate=` is the name of a package of the Cargo workspace:

<!-- docgen:docgen root=internal/cli/testdata/rust page=docs/guide-const.md -->
#### Before

```
The most jobs at once is <!-- docgen:const crate=core-a name=limits::MAX_JOBS --><!-- /docgen:const -->.
```

#### After

```
The most jobs at once is <!-- docgen:const crate=core-a name=limits::MAX_JOBS -->`8`<!-- /docgen:const -->.
```
<!-- /docgen:docgen -->

Run `docgen check` in CI. It changes nothing, and exits 1 when a page is out of date. For each step, see [Your first page](docs/start/02-first-page.md).

## Features

For the full list, see [Markers](docs/reference/markers.md).

Each of these works for Go packages with `pkg=` and for Cargo packages with `crate=`:

- Constants, inline in a sentence.
- Tables of config structs. The default of each field comes from the code that applies it, not from the doc text.
- Tables of the errors that a type names in its doc comment, or that a Rust `Error` enum has.
- State tables and SVG diagrams of state machines, recorded from what your tests do.
- Doc comments as Markdown.
- Code excerpts by symbol from Go, Rust, YAML, TOML and C files, and by marked region or line range from any file.
- The output of Examples, example programs, binaries and Rust doctests, and the `-h` usage of commands.

Also:

- `go.mod` lines and `Cargo.toml` values, the Cargo features of a crate and the crate table of a workspace, inline in a sentence.
- A version or another value from `.docgen.yaml`, in a sentence or a code block.

## Documentation

Read the docs at <https://docgen.mcswain.dev/>. Each release has its own version of the docs.

Try docgen in your browser at <https://docgen.mcswain.dev/playground/>. The playground runs the Go toolchain and docgen of the latest release on a sample repository: edit the code or a page, then run `docgen check`, `docgen update` or a shell command. The sample also has a small Rust crate. The playground has no Rust compiler, so the Rust markers that read the source work there (`docgen:doc`, `docgen:errors`, `docgen:code symbol=`, `docgen:const print=raw`, `docgen:crates`, `docgen:cargo`), and the ones that build or run Rust code (other `docgen:const` values, `docgen:config`, `docgen:configulator`, `docgen:output` and `docgen:usage` with `crate=`) are errors that say they need cargo.

- [Get started](docs/start/01-install.md)
- [Markers](docs/reference/markers.md)
- [Command line](docs/reference/command.md)
- [Configuration](docs/reference/configuration.md)
- [Versioning](docs/project/versioning.md)

## Versioning

- A patch release never breaks your pages or your CI.
- A minor release can add markers and flags, and can make a breaking change. See [Versioning](docs/project/versioning.md).
- The minimum Go version only goes up in a minor or major release.
