# Show command usage

Put the `-h` output of a Go command in a page, so that the page changes when a flag changes.

<!-- site:editor -->
<!-- docgen:code file=internal/cli/testdata/repo/cmd/tool/main.go symbol=main -->
```go
func main() {
	n := flag.Int("n", 3, "how many lines to print")
	flag.Parse()
	for i := 0; i < *n; i++ {
		fmt.Println(i)
	}
}
```
<!-- /docgen:code -->
<!-- docgen:docgen root=internal/cli/testdata/repo module=v2 page=docs/machines/guide-usage.md -->
#### Before

```
# Flags

<!-- docgen:usage cmd=cmd/tool -->
<!-- /docgen:usage -->
```

#### After

````
# Flags

<!-- docgen:usage cmd=cmd/tool -->
```text
Usage of tool:
  -n int
    	how many lines to print (default 3)
```
<!-- /docgen:usage -->
````
<!-- /docgen:docgen -->
<!-- /site:editor -->

The directory is relative to the repository root. A build failure, a crash, or output with no usage is an error.

With `args=`, docgen runs the command with other arguments, such as `args="help config"` for a help command. The output must still hold a usage line or a flag line.

For details, see [`docgen:usage`](../reference/markers.md#docgenusage).
