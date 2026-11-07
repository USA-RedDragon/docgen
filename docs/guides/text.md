# Write a code block with variables

Markers do not work inside a code block, so use `docgen:text` to write a block that holds a [variable](variables.md):

<!-- site:editor -->
<!-- docgen:code file=internal/cli/testdata/repo/.docgen.yaml -->
```yaml
vars:
  version: v1.2.3
```
<!-- /docgen:code -->
<!-- docgen:docgen root=internal/cli/testdata/repo module=v2 page=docs/machines/guide-text.md -->
#### Before

```
<!-- docgen:text text="go install example.com/lamp@{version}\n# {{version} holds the version." lang=sh -->
<!-- /docgen:text -->
```

#### After

````
<!-- docgen:text text="go install example.com/lamp@{version}\n# {{version} holds the version." lang=sh -->
```sh
go install example.com/lamp@v1.2.3
# {version} holds the version.
```
<!-- /docgen:text -->
````
<!-- /docgen:docgen -->
<!-- /site:editor -->

`{key}` is a variable, `{{` is a literal `{`, and `\n` is a line ending, and `\\` is a backslash. `lang=` sets the language of the block.

For details, see [`docgen:text`](../reference/markers.md#docgentext).
