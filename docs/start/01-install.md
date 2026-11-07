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

A `tool` directive also works. See [Run docgen as a tool](../reference/command.md#run-docgen-as-a-tool).

## Next

Go to [Your first page](02-first-page.md).
