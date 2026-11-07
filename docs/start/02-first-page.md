# Your first page

In this step you add a marker to a page, fill it, and run `check` after a change to the code.

## Add a marker

A marker is an open comment and a close comment. docgen writes the text between them. Here is a marker for a constant of package `machine`:

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

Put a marker in any Markdown page. `pkg=` is the path of the package in the module, and `.` is the root package of the module.

## Fill the marker

Run this anywhere in the repository:

```sh
docgen update
```

The page now holds the value. The comments stay, and do not show when the page renders. Commit the page. A reader on GitHub sees the value with no build step.

## See a stale page

Change `DefaultWait` to `500 * time.Millisecond` and run the check:

```sh
docgen check
```

docgen changes nothing. It lists the marker and exits 1. `docgen check -diff` also prints each change as a diff:

```text
stale: docs/wait.md:1: docgen:const pkg=machine name=DefaultWait
docgen: 1 stale; run docgen update and commit the result
```

## Write the rest by hand

Write prose outside the markers. docgen never changes it. Do not edit a marker body by hand, because the next `update` writes over it and `check` fails until then.

docgen scans each Markdown file of the repository. To narrow the scan, see [Command line](../reference/command.md#choose-what-to-scan).

## Next

Go to [Check the docs in CI](03-check-in-ci.md) to run the check on each pull request. For every marker kind, see [Markers](../reference/markers.md).
