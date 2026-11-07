# Show output

Put the output of an Example, a program or a command in a page.

## An Example

`go test` checks the `// Output:` comment, so a wrong comment fails the run.

<!-- site:editor -->
<!-- docgen:code file=internal/cli/testdata/repo/v2/machine/example_test.go symbol=ExampleLamp -->
```go
func ExampleLamp() {
	l, _ := machine.NewLamp(machine.Config{})
	_ = l.Start(0)
	fmt.Println(l.State() == machine.LampOn)
}
```
<!-- /docgen:code -->
<!-- docgen:docgen root=internal/cli/testdata/repo module=v2 page=docs/machines/guide-output.md -->
#### Before

```
# Example

<!-- docgen:output test=machine.ExampleLamp -->
<!-- /docgen:output -->
```

#### After

````
# Example

<!-- docgen:output test=machine.ExampleLamp -->
```text
true
```
<!-- /docgen:output -->
````
<!-- /docgen:docgen -->
<!-- /site:editor -->

## A program or a command

`cmd=` runs the Go program in any directory of the repository. `exec=` runs a shell command in the repository root.

<!-- site:editor -->
<!-- docgen:code file=internal/cli/testdata/repo/v2/examples/hello/main.go symbol=main -->
```go
func main() {
	fmt.Println("hello from the fixture")
}
```
<!-- /docgen:code -->
<!-- docgen:code file=internal/cli/testdata/repo/v2/go.mod -->
```
module github.com/USA-RedDragon/docgen/v2

go 1.24.0
```
<!-- /docgen:code -->
<!-- docgen:docgen root=internal/cli/testdata/repo module=v2 page=docs/machines/guide-output-run.md -->
#### Before

```
<!-- docgen:output cmd=v2/examples/hello -->
<!-- /docgen:output -->

<!-- docgen:output exec="head -n 1 v2/go.mod" -->
<!-- /docgen:output -->
```

#### After

````
<!-- docgen:output cmd=v2/examples/hello -->
```text
hello from the fixture
```
<!-- /docgen:output -->

<!-- docgen:output exec="head -n 1 v2/go.mod" -->
```text
module github.com/USA-RedDragon/docgen/v2
```
<!-- /docgen:output -->
````
<!-- /docgen:docgen -->
<!-- /site:editor -->

Both run on each `update` and `check`, also in CI. The output must be the same on each run, and a nonzero exit status is an error.

For all options, see [`docgen:output`](../reference/markers.md#docgenoutput).
