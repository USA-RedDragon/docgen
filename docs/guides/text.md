# Write text with variables

`docgen:text` writes text from a template that holds [variables](variables.md). With no `code=`, the text goes in the sentence:

<!-- docgen:docgen root=internal/cli/testdata/repo module=v2 page=docs/machines/guide-text-inline.md -->
#### Before

```
Install it with <!-- docgen:text text="go install example.com/lamp@{version}" --><!-- /docgen:text -->.
```

#### After

```
Install it with <!-- docgen:text text="go install example.com/lamp@{version}" -->go install example.com/lamp@v1.2.3<!-- /docgen:text -->.
```
<!-- /docgen:docgen -->

Markers do not work inside a code block, so add `code=` to write a code block:

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
<!-- docgen:text text="go install example.com/lamp@{version}\n# {{version} holds the version." code=sh -->
<!-- /docgen:text -->
```

#### After

````
<!-- docgen:text text="go install example.com/lamp@{version}\n# {{version} holds the version." code=sh -->
```sh
go install example.com/lamp@v1.2.3
# {version} holds the version.
```
<!-- /docgen:text -->
````
<!-- /docgen:docgen -->
<!-- /site:editor -->

`{key}` is a variable, `{{` is a literal `{`, and `\n` is a line ending, and `\\` is a backslash. `code=` sets the language of the block, and `code="text"` is a plain block.

For details, see [`docgen:text`](../reference/markers.md#docgentext).
