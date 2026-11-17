# Install

In this step you install docgen, or set up a repository to run a pinned version of it.

## What you need

| Your repository | You need |
|---|---|
| Go | `go` on the `PATH`, Go <!-- docgen:gomod field=go -->`1.27.0`<!-- /docgen:gomod --> or later, and a Go module. |
| Rust | `cargo` on the `PATH` with stable Rust, and a Cargo workspace. |
| Both | Both of the above. |

Markers that use no Go or Rust code, such as a YAML excerpt, work in a repository with no module and no workspace.

Rust repositories do not need Go. docgen is one static binary, and a run that has only Rust markers starts no `go` command and reads no `go.mod`. The repository root is the nearest directory with a `.docgen.yaml` file or a `.git` entry. Go is needed only to build docgen with `go install`, and for the Go markers.

## Install the command

Pick one:

- A release binary, for Go and Rust repositories. Download the archive for your system (Linux, macOS or Windows, on amd64 or arm64) from the [releases](https://github.com/USA-RedDragon/docgen/releases), check it against `checksums.txt` of that release, and put `docgen` on your `PATH`.
- `go install`, which needs Go <!-- docgen:gomod field=go -->`1.27.0`<!-- /docgen:gomod --> or later:

```sh
go install github.com/USA-RedDragon/docgen@latest
```

The first run on a Rust repository builds the helper that reads Rust code, with your `cargo`. It compiles `syn`, so it takes a while, and later runs use the cached build. The helper needs Rust 1.71 or later. See [How docgen works](../concepts/how-it-works.md#rust-pipeline).

## Run a pinned version

To give everyone and CI the same version, run docgen with a version in place of installing it. These options use Go; a Rust repository can pin the version of the release binary, or of the [GitHub Action](03-check-in-ci.md#check-a-rust-repository). This does not change the `go.mod` of your repository:

<!-- docgen:text text="go run github.com/USA-RedDragon/docgen@{version} check" code=sh -->
```sh
go run github.com/USA-RedDragon/docgen@v0.1.0 check
```
<!-- /docgen:text -->

Use a `go:generate` line to run it with your other generators:

<!-- docgen:text text="//go:generate go run github.com/USA-RedDragon/docgen@{version} update" code=go -->
```go
//go:generate go run github.com/USA-RedDragon/docgen@v0.1.0 update
```
<!-- /docgen:text -->

## Run docgen as a tool

A `tool` directive also pins a version:

<!-- docgen:text text="go get -tool github.com/USA-RedDragon/docgen@{version}\ngo tool docgen check" code=sh -->
```sh
go get -tool github.com/USA-RedDragon/docgen@v0.1.0
go tool docgen check
```
<!-- /docgen:text -->

It adds the docgen dependencies to your `go.mod` and raises its `go` line to the docgen version. Do not use it in a library that supports an earlier Go version.

For a GitHub Action and a pre-commit hook, including hooks for a repository with no Go, see [Check the docs in CI](03-check-in-ci.md).

## Next

Go to [Your first page](02-first-page.md).
