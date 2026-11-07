# Show a go.mod value

Put the Go version, the toolchain or the module path in a sentence.

<!-- site:editor -->
<!-- docgen:code file=internal/cli/testdata/repo/v2/go.mod -->
```
module github.com/USA-RedDragon/docgen/v2

go 1.24.0
```
<!-- /docgen:code -->
<!-- docgen:code file=internal/cli/testdata/repo/cmd/tool/go.mod -->
```
module example.com/tool

go 1.24.0

toolchain go1.24.1
```
<!-- /docgen:code -->
<!-- docgen:docgen root=internal/cli/testdata/repo module=v2 page=docs/machines/guide-gomod-lines.md -->
#### Before

```
- Module: <!-- docgen:gomod field=module --><!-- /docgen:gomod -->
- Go: <!-- docgen:gomod field=go --><!-- /docgen:gomod -->
- Module of the tool: <!-- docgen:gomod field=module file=cmd/tool/go.mod --><!-- /docgen:gomod -->
- Toolchain of the tool: <!-- docgen:gomod field=toolchain file=cmd/tool/go.mod --><!-- /docgen:gomod -->
```

#### After

```
- Module: <!-- docgen:gomod field=module -->`github.com/USA-RedDragon/docgen/v2`<!-- /docgen:gomod -->
- Go: <!-- docgen:gomod field=go -->`1.24.0`<!-- /docgen:gomod -->
- Module of the tool: <!-- docgen:gomod field=module file=cmd/tool/go.mod -->`example.com/tool`<!-- /docgen:gomod -->
- Toolchain of the tool: <!-- docgen:gomod field=toolchain file=cmd/tool/go.mod -->`go1.24.1`<!-- /docgen:gomod -->
```
<!-- /docgen:docgen -->
<!-- /site:editor -->

`field=` is `go`, `toolchain` or `module`. `file=` reads another `go.mod`.

For details, see [`docgen:gomod`](../reference/markers.md#docgengomod).
