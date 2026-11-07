# Use variables

Keep a value such as your release version in `.docgen.yaml`, and use it in each page:

```yaml
vars:
  version: v1.2.0
```

`docgen:var` puts the value in a sentence:

<!-- site:editor -->
<!-- docgen:code file=internal/cli/testdata/repo/.docgen.yaml -->
```yaml
vars:
  version: v1.2.3
```
<!-- /docgen:code -->
<!-- docgen:docgen root=internal/cli/testdata/repo module=v2 page=docs/machines/guide-var.md -->
#### Before

```
The current version is <!-- docgen:var name=version --><!-- /docgen:var -->.
```

#### After

```
The current version is <!-- docgen:var name=version -->`v1.2.3`<!-- /docgen:var -->.
```
<!-- /docgen:docgen -->
<!-- /site:editor -->

[`docgen:text`](text.md) puts it in a code block, such as an install line:

<!-- site:editor -->
<!-- docgen:code file=internal/cli/testdata/repo/.docgen.yaml -->
```yaml
vars:
  version: v1.2.3
```
<!-- /docgen:code -->
<!-- docgen:docgen root=internal/cli/testdata/repo module=v2 page=docs/machines/guide-version.md -->
#### Before

```
Version <!-- docgen:var name=version --><!-- /docgen:var --> is out. Install it:

<!-- docgen:text text="go install example.com/lamp@{version}" lang=sh -->
<!-- /docgen:text -->
```

#### After

````
Version <!-- docgen:var name=version -->`v1.2.3`<!-- /docgen:var --> is out. Install it:

<!-- docgen:text text="go install example.com/lamp@{version}" lang=sh -->
```sh
go install example.com/lamp@v1.2.3
```
<!-- /docgen:text -->
````
<!-- /docgen:docgen -->
<!-- /site:editor -->

A release is one line in `.docgen.yaml`, then `docgen update`, then the tag. A variable that no marker uses fails `check`.

For details, see [`docgen:var`](../reference/markers.md#docgenvar).
