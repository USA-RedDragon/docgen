# Run docgen on docgen

Show what docgen does to a sample directory. docgen runs itself on a copy of the directory, so the page always holds real output. This site uses it for each example.

```text
<!-- docgen:docgen root=examples/sample page=docs/page.md -->
<!-- /docgen:docgen -->
```

This is the result for `page=docs/machines/guide-const.md`, the page before and after `docgen update`:

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

The outer run skips the pages under `root`.

`module=` names the module in `root`. `depth` in `.docgen.yaml` limits nested markers.

For details, see [`docgen:docgen`](../reference/markers.md#docgendocgen).
