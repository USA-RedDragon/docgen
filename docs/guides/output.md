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

## A Rust example or binary

`crate=` names a Cargo package of the Cargo workspace in the module directory. `example=` builds an example of the package with `cargo build --locked --example` and runs it, and `bin=` does the same for a binary. `args=` gives the arguments of the program. Builds go to the temporary directory of the run, and the markers of one run, `docgen:usage` included, build each example and binary once.

<!-- site:editor -->
<!-- docgen:code file=internal/cli/testdata/rust/crates/run-c/examples/greet.rs -->
```rust
fn main() {
    let mut names: Vec<String> = std::env::args().skip(1).collect();
    if names.is_empty() {
        names.push("world".to_string());
    }
    for name in names {
        println!("{}", run_c::greeting(&name));
    }
}
```
<!-- /docgen:code -->
<!-- docgen:docgen root=internal/cli/testdata/rust module=. page=docs/guide-run.md -->
#### Before

```
<!-- docgen:output crate=run-c example=greet -->
<!-- /docgen:output -->

<!-- docgen:output crate=run-c example=greet args="Ada Lovelace" -->
<!-- /docgen:output -->

<!-- docgen:output crate=run-c bin=run-c -->
<!-- /docgen:output -->
```

#### After

````
<!-- docgen:output crate=run-c example=greet -->
```text
Hello, world!
```
<!-- /docgen:output -->

<!-- docgen:output crate=run-c example=greet args="Ada Lovelace" -->
```text
Hello, Ada!
Hello, Lovelace!
```
<!-- /docgen:output -->

<!-- docgen:output crate=run-c bin=run-c -->
```text
Hello, bin!
```
<!-- /docgen:output -->
````
<!-- /docgen:docgen -->
<!-- /site:editor -->

As for `cmd=`, the output must be the same on each run, and a nonzero exit status is an error that holds the standard error of the program.

## A Rust doctest

A doctest has no expected output, so `doctest=` shows the code of the doctests of an item as Rust code blocks, without the lines that rustdoc hides. It also runs `cargo test --doc` on them, so check and update fail when a doctest does not compile or fails, and the page cannot show a broken example. Doctests of one crate run in one cargo command.

<!-- site:editor -->
<!-- docgen:code file=internal/cli/testdata/rust/crates/run-c/src/lib.rs region=area -->
````rust
/// The area of a rectangle.
///
/// ```
/// use run_c::shapes::area;
///
/// assert_eq!(area(3, 4), 12);
/// ```
///
/// A square is a rectangle:
///
/// ```
/// assert_eq!(run_c::shapes::area(5, 5), 25);
/// ```
pub fn area(w: u32, h: u32) -> u32 {
    w * h
}
````
<!-- /docgen:code -->
<!-- docgen:docgen root=internal/cli/testdata/rust module=. page=docs/guide-doctest.md -->
#### Before

```
<!-- docgen:output crate=run-c doctest=shapes::area -->
<!-- /docgen:output -->
```

#### After

````
<!-- docgen:output crate=run-c doctest=shapes::area -->
```rust
use run_c::shapes::area;

assert_eq!(area(3, 4), 12);
```

```rust
assert_eq!(run_c::shapes::area(5, 5), 25);
```
<!-- /docgen:output -->
````
<!-- /docgen:docgen -->
<!-- /site:editor -->

The path of the item is the one that rustdoc prints, such as `shapes::area` or `Counter::next`. A doctest with `ignore`, an item with no doctest and a crate with no library are errors.

For all options, see [`docgen:output`](../reference/markers.md#docgenoutput).
