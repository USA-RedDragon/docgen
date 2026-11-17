# Your first page

In this step you add a marker to a page, fill it, and run `check` after a change to the code.

## Add a marker

A marker is an open comment and a close comment. docgen writes the text between them. Pick the language of your code.

### In a Go module

Here is a marker for a constant of package `machine`. `pkg=` is the path of the package in the module, and `.` is the root package of the module.

<!-- site:editor -->
<!-- docgen:code file=internal/cli/testdata/repo/v2/machine/machine.go symbol=DefaultWait -->
```go
const DefaultWait = 250 * time.Millisecond
```
<!-- /docgen:code -->
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
<!-- /site:editor -->

### In a Cargo workspace

Here is a marker for a constant of the crate `core-a`. `crate=` is the name of a package of the workspace, as `cargo metadata` reports it, and `name=` is the path of the item from the crate root. A Rust-only repository needs no `go.mod`, and the workspace is the root `Cargo.toml`, or the one in the directory that `module` names.

<!-- site:editor -->
<!-- docgen:code file=internal/cli/testdata/rust/crates/core-a/src/limits.rs symbol=MAX_JOBS -->
```rust
pub const MAX_JOBS: usize = 8;
```
<!-- /docgen:code -->
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
<!-- /site:editor -->

The value comes from a test that docgen runs on a copy of the workspace, so a value that the code computes, such as `MAX_JOBS / 4`, is right too. Rust markers need `cargo` on the `PATH`.

## Fill the marker

Run this anywhere in the repository:

```sh
docgen update
```

The page now holds the value. The comments stay, and do not show when the page renders. Commit the page. A reader on GitHub sees the value with no build step.

## See a stale page

Change the constant, `DefaultWait` to `500 * time.Millisecond` or `MAX_JOBS` to `16`, and run the check:

```sh
docgen check
```

docgen changes nothing. It lists the marker and exits 1. `docgen check -diff` also prints each change as a diff:

```text
stale: docs/wait.md:1: docgen:const pkg=machine name=DefaultWait
docgen: 1 stale; run docgen update and commit the result
```

For the Rust marker, the line is `stale: docs/limits.md:1: docgen:const crate=core-a name=limits::MAX_JOBS`.

## Write the rest by hand

Write prose outside the markers. docgen never changes it. Do not edit a marker body by hand, because the next `update` writes over it and `check` fails until then.

docgen scans each Markdown file of the repository. To narrow the scan, see [Configuration](../reference/configuration.md#choose-what-to-scan).

## Next

Go to [Check the docs in CI](03-check-in-ci.md) to run the check on each pull request. For every marker kind, see [Markers](../reference/markers.md). To read Cargo values and the crates of a workspace, see [Show Cargo values and crates](../guides/cargo.md).
