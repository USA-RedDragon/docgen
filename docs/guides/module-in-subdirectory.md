# Use a module in a subdirectory

Use this guide for a repository whose Go module is not at the root, such as a module in `v2`.

## Give the module directory

Put a `.docgen.yaml` at the repository root, so that each run uses the same module:

```yaml
module: v2
```

The `-module` flag also works, and overrides the file:

```sh
docgen update -module v2
```

The path is relative to the repository root. With it:

- `pkg=` paths are relative to the module, so `pkg=machine` is `v2/machine`.
- `docgen:gomod` reads `v2/go.mod`.
- The `tags` setting applies to the code of this module.
- `file=` and `cmd=` stay relative to the repository root, so an excerpt has `file=v2/machine/machine.go` and a program has `cmd=v2/examples/hello`.

docgen reads one module per run. The import path in each link comes from the `go.mod` of that module.

For a module in `v2`, `pkg=machine` reads `v2/machine`:

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

## Tools in other modules

A directory with a `go.mod` of its own is another module:

- `pkg=` does not match it as a package of the main module.
- `docgen:doc` reads its package doc when `pkg=` is its path from the repository root.
- `docgen:usage` runs it from its nearest `go.mod`.


```text
<!-- docgen:doc pkg=tools/gen -->
<!-- /docgen:doc -->

<!-- docgen:usage cmd=tools/gen -->
<!-- /docgen:usage -->
```
