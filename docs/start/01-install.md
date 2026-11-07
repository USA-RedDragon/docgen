# Install

In this step you install docgen, or set up a repository to run a pinned version of it.

## What you need

- Go <!-- docgen:gomod field=go -->`1.27.0`<!-- /docgen:gomod --> or later, the version of the `go` line of the docgen module.
- A Go module to document. Markers that use no Go code, such as a YAML excerpt, work in a repository with no module.

## Install the command

```sh
go install github.com/USA-RedDragon/docgen@latest
```

## Run a pinned version

To give everyone and CI the same version, run docgen with a version in place of installing it. This does not change the `go.mod` of your repository:

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

For a GitHub Action and a pre-commit hook, see [Check the docs in CI](03-check-in-ci.md).

## Next

Go to [Your first page](02-first-page.md).
