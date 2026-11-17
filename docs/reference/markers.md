# Markers

The marker kinds and their arguments. The text comes from the package doc of docgen.

A marker has this form. Kinds `const`, `gomod`, `cargo` (except `field=features`) and `var` go inline in a sentence. Other kinds have the body on the lines between the two comments:

```text
<!-- docgen:<kind> <arg>=<value> ... -->
<!-- /docgen:<kind> -->
```

- Package paths are relative to the module.
- `pkg=session` also matches `transport/tcp/session` when exactly one package path ends that way.
- File paths, and `cmd=`, are relative to the repository root.

Each box is the output of a [`docgen:docgen`](#docgendocgen) marker over a page of the test fixture.

## Marker kinds

### docgen:output

<!-- docgen:doc pkg=. section="docgen:output" -->
The output of a program, a command or an Example, or the checked code of a Rust doctest.

	docgen:output cmd=<dir>
	docgen:output exec="<command>"
	docgen:output test=<pkg>.<ExampleName>
	docgen:output crate=<package> example=<name> [args="<args>"] [features=<list>] [allfeatures=<bool>] [nodefaultfeatures=<bool>]
	docgen:output crate=<package> bin=<name> [args="<args>"] [features=<list>] [allfeatures=<bool>] [nodefaultfeatures=<bool>]
	docgen:output crate=<package> doctest=<path::item> [features=<list>] [allfeatures=<bool>] [nodefaultfeatures=<bool>]

The arguments:

  - `cmd=` is the standard output of `go run` on the Go program in that directory, relative to the repository root, from its nearest `go.mod`.
  - `exec=` is the standard output of a shell command, run with `sh -c` in the repository root. A nonzero exit status is an error.
  - `test=` is the expected output of an Example function, as written in its `// Output:` comment. `go test` fails when the real output differs, so the page cannot hold wrong output. Give the package and the function, such as `test=machine.ExampleLamp`. `test=machine.Example` is the Example of the package itself.

The output must be the same on each run, or check fails. A program or command with no output gives an empty block, and an Example with an empty `// Output:` comment is an error.

A marker with `crate=` is for Rust (see Rust crates). A marker with `crate=` and any of `cmd=`, `exec=`, `test=` or `pkg=` is an error, and so is one with `example=`, `bin=`, `doctest=` or `args=` and no `crate=`. It has exactly one of these:

  - `example=` is the standard output of the example of that name of the package, in a block of text. docgen builds it with `cargo build --locked --example` and runs it in the directory of the Cargo workspace. A nonzero exit status is an error that holds the standard error of the example, and an example with no output gives an empty block. An example that the package does not have is an error that lists the examples that it has.
  - `bin=` is the same for a binary target of the package, built with `cargo build --locked --bin`.
  - `args=` goes with `example=` or `bin=` and holds the arguments of the program, split on white space. A part in double quotes is one argument, also with spaces. `args=` with `doctest=` is an error.
  - `doctest=` is the code of the doctests in the doc comment of an item, as one Rust code block for each doctest, in the order of the file. Give the path of the item in the crate as rustdoc names it, such as `double`, `shapes::area` or `Counter::next`, with no leading `crate::`. A crate name or `crate` alone is the doc comment of the crate itself. rustdoc hides the lines that start with `# ` and a line that is only `#`, and the code block has none of them, and `##` shows as `#`. Rust has no output to compare, so the page shows code and not output. The markers of one crate run in one `cargo test --locked --doc`, and update and check fail when a doctest fails to compile or fails when it runs, so a page cannot show a broken example. A doctest with `no_run` only compiles, and one with `compile_fail` must not compile. A doctest with `ignore` is an error, as nothing checks it. An item with no doctest is an error that lists the items that have one, and so is a crate with no library. The code is read from `///`, `//!` and `/** */` comments; a doc comment from a `#[doc]` attribute or a macro is an error.
<!-- /docgen:doc -->

#### The output of an Example, with `test=`

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

#### The output of a program or a command, with `cmd=` or `exec=`

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

#### The output of a Rust example or binary, with `crate=` and `example=` or `bin=`

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

#### The code of a Rust doctest, with `crate=` and `doctest=`

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

### docgen:code

<!-- docgen:doc pkg=. section="docgen:code" -->
An excerpt of a file of the repository.

	docgen:code file=<repo path> symbol=<Name>
	docgen:code file=<repo path> region=<name>
	docgen:code file=<repo path> lines=<a>-<b>
	docgen:code file=<repo path>

The arguments:

  - `symbol=` in a Go file is one func, method (`Type.Method`), type, or const or var block, with no doc comment. An Example has no output comment.
  - `symbol=` in a YAML file is the block of the key at a dotted path, such as `jobs.test` of a workflow. A last segment `key[A:B]` is the items of the list at `key` from the item with name `A` to the item with name `B`, and `key[A]` is the one item `A`.
  - `symbol=` in a JSON or TOML file uses the same path form. In JSON the excerpt is `"key": value`. In TOML it is a key with its value, or a table `[a.b]` with its keys and sub-tables, and `key[A]` is an item of the array of tables `[[key]]`. An item is found by its `name` key, or by a number, its index from 0. `\.` is a dot in a key.
  - `symbol=` in a C or header file is a `#define`, a function definition or a `typedef`, by name, as whole lines with no comment above. `symbol="struct Tag"`, `"union Tag"` or `"enum Tag"` is the definition of a tagged type, with its body. A bare name never matches a tag: `struct foo {...};` is `struct foo`, and `typedef struct foo foo;` is `foo`. `typedef struct foo {...} foo_t;` is both `struct foo` and `foo_t`. A name with more than one definition, such as one per `#ifdef` branch, is an error, and so is a definition that shares a line with other code. Only top-level definitions are found.
  - `symbol=` in a Rust file is one item of the file, as whole lines from its first attribute to its last token, with no doc comment. See Rust symbols.
  - `region=` is the lines between the line `docgen:region <name>` and the next line `docgen:end`. Each mark is the whole line apart from comment syntax. In a page, a mark in code or in a marker body is text. Other excerpts of the file leave out those two lines.
  - `lines=` is those lines of the file. For a Go file the lines must hold whole declarations or statements. When the file changes, check and update fail. The error holds the new range when the old excerpt moved, and otherwise states that the range is out of date. A range past the last line is an error.
  - With none of these, the excerpt is the whole file.
  - `raw=true`, with `symbol=`, `lines=` or the whole file, keeps the text of the file: doc comments, region marks and the output comment of an Example. `symbol=package` in a Go file is then the package clause with its doc comment. In a Rust file the lines of an item start at its first doc comment or attribute, and `symbol=crate` is the lines of the `//!` comments and `#![doc]` attributes of the file, the crate or module doc, which is an error with no `raw=true`. Any other value is an error.

An excerpt with no text is an error.

On a scanned page, the excerpt is the text that update writes to that page. An excerpt of the page of its own marker is an error, and so are excerpts that do not settle in 8 passes.
<!-- /docgen:doc -->

#### A Go declaration, with `symbol=`

<!-- site:editor -->
<!-- docgen:code file=internal/cli/testdata/repo/v2/machine/machine.go symbol=Lamp.Start -->
```go
func (l *Lamp) Start(now time.Duration) error {
	if l.state == LampOn {
		return ErrBusy
	}
	if l.state == LampOff {
		l.state, l.until = LampOn, now+l.wait
	}
	return nil
}
```
<!-- /docgen:code -->
<!-- docgen:docgen root=internal/cli/testdata/repo module=v2 page=docs/machines/guide-code.md -->
#### Before

```
# Start a lamp

<!-- docgen:code file=v2/machine/machine.go symbol=Lamp.Start -->
<!-- /docgen:code -->
```

#### After

````
# Start a lamp

<!-- docgen:code file=v2/machine/machine.go symbol=Lamp.Start -->
```go
func (l *Lamp) Start(now time.Duration) error {
	if l.state == LampOn {
		return ErrBusy
	}
	if l.state == LampOff {
		l.state, l.until = LampOn, now+l.wait
	}
	return nil
}
```
<!-- /docgen:code -->
````
<!-- /docgen:docgen -->
<!-- /site:editor -->

#### A YAML key, with `symbol=`

<!-- site:editor -->
<!-- docgen:code file=internal/cli/testdata/repo/ci/workflow.yaml -->
```yaml
jobs:
  test:
    env:
      GOFLAGS: -mod=readonly
    steps:
      - name: Get the code
        uses: actions/checkout@v6
      - name: Run the tests
        run: go test ./...
      - name: Check the docs
        run: docgen check
```
<!-- /docgen:code -->
<!-- docgen:docgen root=internal/cli/testdata/repo module=v2 page=docs/machines/guide-code-yaml.md -->
#### Before

```
<!-- docgen:code file=ci/workflow.yaml symbol=jobs.test.env -->
<!-- /docgen:code -->

<!-- docgen:code file=ci/workflow.yaml symbol="jobs.test.steps[Get the code:Run the tests]" -->
<!-- /docgen:code -->

<!-- docgen:code file=ci/workflow.yaml symbol="jobs.test.steps[Check the docs]" -->
<!-- /docgen:code -->
```

#### After

````
<!-- docgen:code file=ci/workflow.yaml symbol=jobs.test.env -->
```yaml
env:
  GOFLAGS: -mod=readonly
```
<!-- /docgen:code -->

<!-- docgen:code file=ci/workflow.yaml symbol="jobs.test.steps[Get the code:Run the tests]" -->
```yaml
- name: Get the code
  uses: actions/checkout@v6
- name: Run the tests
  run: go test ./...
```
<!-- /docgen:code -->

<!-- docgen:code file=ci/workflow.yaml symbol="jobs.test.steps[Check the docs]" -->
```yaml
- name: Check the docs
  run: docgen check
```
<!-- /docgen:code -->
````
<!-- /docgen:docgen -->
<!-- /site:editor -->

#### A JSON member, with `symbol=`

<!-- site:editor -->
<!-- docgen:code file=internal/cli/testdata/repo/config/app.json -->
```json
{
  "server": {
    "port": 8080,
    "hosts": ["a.example", "b.example"]
  },
  "jobs": [
    {"name": "build", "run": "go build ./..."},
    {"name": "test", "run": "go test ./..."},
    {"name": "docs", "run": "docgen check"}
  ]
}
```
<!-- /docgen:code -->
<!-- docgen:docgen root=internal/cli/testdata/repo module=v2 page=docs/machines/guide-code-json.md -->
#### Before

```
<!-- docgen:code file=config/app.json symbol=server.port -->
<!-- /docgen:code -->

<!-- docgen:code file=config/app.json symbol="jobs[build:test]" -->
<!-- /docgen:code -->

<!-- docgen:code file=config/app.json symbol=jobs[2] -->
<!-- /docgen:code -->
```

#### After

````
<!-- docgen:code file=config/app.json symbol=server.port -->
```json
"port": 8080
```
<!-- /docgen:code -->

<!-- docgen:code file=config/app.json symbol="jobs[build:test]" -->
```json
{"name": "build", "run": "go build ./..."},
{"name": "test", "run": "go test ./..."}
```
<!-- /docgen:code -->

<!-- docgen:code file=config/app.json symbol=jobs[2] -->
```json
{"name": "docs", "run": "docgen check"}
```
<!-- /docgen:code -->
````
<!-- /docgen:docgen -->
<!-- /site:editor -->

#### A TOML table or key, with `symbol=`

<!-- site:editor -->
<!-- docgen:code file=internal/cli/testdata/repo/config/app.toml -->
```toml
[server]
port = 8080 # the default port
hosts = [
  "a.example",
  "b.example",
]

[[jobs]]
name = "build"
run = "go build ./..."

[[jobs]]
name = "test"
run = """
go vet ./...
go test ./..."""
```
<!-- /docgen:code -->
<!-- docgen:docgen root=internal/cli/testdata/repo module=v2 page=docs/machines/guide-code-toml.md -->
#### Before

```
<!-- docgen:code file=config/app.toml symbol=server -->
<!-- /docgen:code -->

<!-- docgen:code file=config/app.toml symbol=server.hosts -->
<!-- /docgen:code -->

<!-- docgen:code file=config/app.toml symbol=jobs[test] -->
<!-- /docgen:code -->
```

#### After

````
<!-- docgen:code file=config/app.toml symbol=server -->
```toml
[server]
port = 8080 # the default port
hosts = [
  "a.example",
  "b.example",
]
```
<!-- /docgen:code -->

<!-- docgen:code file=config/app.toml symbol=server.hosts -->
```toml
hosts = [
  "a.example",
  "b.example",
]
```
<!-- /docgen:code -->

<!-- docgen:code file=config/app.toml symbol=jobs[test] -->
```toml
[[jobs]]
name = "test"
run = """
go vet ./...
go test ./..."""
```
<!-- /docgen:code -->
````
<!-- /docgen:docgen -->
<!-- /site:editor -->

#### A C function, type or #define, with `symbol=`

A bare name is a `#define`, a function or a typedef. A tagged type is `"struct Tag"`, `"union Tag"` or `"enum Tag"`, so that a struct and a typedef with the same name stay apart.

<!-- site:editor -->
<!-- docgen:code file=internal/cli/testdata/repo/native/lamp.c -->
```c
#define LAMP_MAX_TRIES 3

static int tries;

struct lamp_config {
	int max_tries;
	int (*on_fail)(int count);
};

typedef struct lamp_config lamp_config_t;

int lamp_start(int count)
{
	if (count > LAMP_MAX_TRIES) {
		return -1;
	}
	tries = count;
	return 0;
}
```
<!-- /docgen:code -->
<!-- docgen:docgen root=internal/cli/testdata/repo module=v2 page=docs/machines/guide-code-c.md -->
#### Before

```
<!-- docgen:code file=native/lamp.c symbol=LAMP_MAX_TRIES -->
<!-- /docgen:code -->

<!-- docgen:code file=native/lamp.c symbol=lamp_start -->
<!-- /docgen:code -->

<!-- docgen:code file=native/lamp.c symbol="struct lamp_config" -->
<!-- /docgen:code -->

<!-- docgen:code file=native/lamp.c symbol=lamp_config_t -->
<!-- /docgen:code -->
```

#### After

````
<!-- docgen:code file=native/lamp.c symbol=LAMP_MAX_TRIES -->
```c
#define LAMP_MAX_TRIES 3
```
<!-- /docgen:code -->

<!-- docgen:code file=native/lamp.c symbol=lamp_start -->
```c
int lamp_start(int count)
{
	if (count > LAMP_MAX_TRIES) {
		return -1;
	}
	tries = count;
	return 0;
}
```
<!-- /docgen:code -->

<!-- docgen:code file=native/lamp.c symbol="struct lamp_config" -->
```c
struct lamp_config {
	int max_tries;
	int (*on_fail)(int count);
};
```
<!-- /docgen:code -->

<!-- docgen:code file=native/lamp.c symbol=lamp_config_t -->
```c
typedef struct lamp_config lamp_config_t;
```
<!-- /docgen:code -->
````
<!-- /docgen:docgen -->
<!-- /site:editor -->

#### A region, a line range or a whole file

<!-- site:editor -->
<!-- docgen:code file=internal/cli/testdata/repo/scripts/build.sh raw=true -->
```sh
#!/bin/sh
set -eu
# docgen:region setup
go mod download
go build ./...
# docgen:end
go test ./...
```
<!-- /docgen:code -->
<!-- docgen:docgen root=internal/cli/testdata/repo module=v2 page=docs/machines/guide-code-file.md -->
#### Before

```
<!-- docgen:code file=scripts/build.sh region=setup -->
<!-- /docgen:code -->

<!-- docgen:code file=scripts/build.sh lines=1-2 -->
<!-- /docgen:code -->

<!-- docgen:code file=scripts/build.sh -->
<!-- /docgen:code -->
```

#### After

````
<!-- docgen:code file=scripts/build.sh region=setup -->
```sh
go mod download
go build ./...
```
<!-- /docgen:code -->

<!-- docgen:code file=scripts/build.sh lines=1-2 -->
```sh
#!/bin/sh
set -eu
```
<!-- /docgen:code -->

<!-- docgen:code file=scripts/build.sh -->
```sh
#!/bin/sh
set -eu
go mod download
go build ./...
go test ./...
```
<!-- /docgen:code -->
````
<!-- /docgen:docgen -->
<!-- /site:editor -->

#### An excerpt with its comments and marks, with `raw=true`

<!-- site:editor -->
<!-- docgen:code file=internal/cli/testdata/repo/v2/machine/machine.go symbol=package raw=true -->
```go
// Package machine is a small state machine for the docgen tests.
//
// # Rates
//
// A Lamp blinks at these rates:
//   - slow, every [DefaultWait]
//   - fast, every period
//
// # Other
//
// Not copied.
package machine
```
<!-- /docgen:code -->
<!-- docgen:code file=internal/cli/testdata/repo/v2/machine/machine.go symbol=Lamp.Start raw=true -->
```go
// Start turns the Lamp on. It returns ErrBusy while the Lamp is on.
func (l *Lamp) Start(now time.Duration) error {
	if l.state == LampOn {
		return ErrBusy
	}
	if l.state == LampOff {
		l.state, l.until = LampOn, now+l.wait
	}
	return nil
}
```
<!-- /docgen:code -->
<!-- docgen:code file=internal/cli/testdata/repo/scripts/build.sh raw=true -->
```sh
#!/bin/sh
set -eu
# docgen:region setup
go mod download
go build ./...
# docgen:end
go test ./...
```
<!-- /docgen:code -->
<!-- docgen:docgen root=internal/cli/testdata/repo module=v2 page=docs/machines/guide-code-raw.md -->
#### Before

```
# Keep the comments

<!-- docgen:code file=v2/machine/machine.go symbol=Lamp.Start raw=true -->
<!-- /docgen:code -->

<!-- docgen:code file=v2/machine/machine.go symbol=package raw=true -->
<!-- /docgen:code -->

<!-- docgen:code file=scripts/build.sh raw=true -->
<!-- /docgen:code -->
```

#### After

````
# Keep the comments

<!-- docgen:code file=v2/machine/machine.go symbol=Lamp.Start raw=true -->
```go
// Start turns the Lamp on. It returns ErrBusy while the Lamp is on.
func (l *Lamp) Start(now time.Duration) error {
	if l.state == LampOn {
		return ErrBusy
	}
	if l.state == LampOff {
		l.state, l.until = LampOn, now+l.wait
	}
	return nil
}
```
<!-- /docgen:code -->

<!-- docgen:code file=v2/machine/machine.go symbol=package raw=true -->
```go
// Package machine is a small state machine for the docgen tests.
//
// # Rates
//
// A Lamp blinks at these rates:
//   - slow, every [DefaultWait]
//   - fast, every period
//
// # Other
//
// Not copied.
package machine
```
<!-- /docgen:code -->

<!-- docgen:code file=scripts/build.sh raw=true -->
```sh
#!/bin/sh
set -eu
# docgen:region setup
go mod download
go build ./...
# docgen:end
go test ./...
```
<!-- /docgen:code -->
````
<!-- /docgen:docgen -->
<!-- /site:editor -->

#### Rust items, with `symbol=`

<!-- docgen:doc pkg=. section="Rust symbols" -->
`symbol=` in a Rust file reads the one file, not a crate, so the file can be in no crate and `file=` is a path of the repository, as for any excerpt. The syn helper that `crate=` uses reads the file alone: a `mod name;` is not followed, and a file that does not parse is an error with the place of the error. The helper is built once with `cargo`, and no Cargo workspace is read.

An excerpt is whole lines of the file, from the first attribute of the item, such as `#[derive]` or `#[cfg]`, to its last token. A doc comment of the item, `///` or `#[doc = "..."]`, and the `//!` comments inside an inline module are not in it, and `raw=true` keeps them. A comment with `//` between the attributes stays. An item of an impl, a trait or an inline module loses the indent that its lines share. The names of `symbol=` are:

  - `name` is a `fn`, `struct`, `enum`, `union`, `trait`, `type`, `const`, `static`, `macro_rules!` macro or inline `mod` of the top level of the file, and `a::b` is such an item of the inline modules `a` and `a::b`: a name starts at the root of the file. A `mod name;` with no body is the line of the declaration.
  - `Type::name` is an associated `fn`, `const` or `type` of an impl of `Type`, with or without a trait, and `Trait::name` is an item of the `trait` definition. In a module, `a::Type::name` and `a::Trait::name`.
  - `<Type as Trait>::name` is an item of an impl of `Trait` for `Type`.
  - `impl Type` and `impl Trait for Type` are an impl block with its items. A negative impl is `impl !Trait for Type`. An impl block in an inline module has the same name as at the top level.
  - `crate`, with `raw=true`, is the `//!` comments and `#![doc]` attributes of the file, which are the crate doc in `lib.rs` and the module doc in another file, as `symbol=package raw=true` is for Go. With no `raw=true` it is an error, and so is a file with no such comment.

A type and a trait are written as in the file, or with no generic arguments, and a trait also by its last segment alone: `impl Display for Port` selects `impl std::fmt::Display for Port<u8>`, and `Port::new` selects the `new` of `impl<T> Port<T>`. Spaces do not matter. A field and an enum variant have no name. A name that selects more than one item is an error that lists each item with its line and its `#[cfg]`. This is a name in two impls, two items with different `#[cfg]`, or two impls of one trait for the same type with different generic arguments. Where the items have different full spellings, such as `<Port<u8> as Shape>::area`, the error lists them and one is the `symbol=`. Items that have the same spelling, such as `fn f` under `#[cfg(unix)]` and under `#[cfg(windows)]`, are an error that suggests `region=`. A name of no item is an error that lists the names of the file.
<!-- /docgen:doc -->

<!-- site:editor -->
<!-- docgen:docgen root=internal/cli/testdata/rust module=. page=docs/guide-code-symbol.md -->
#### Before

```
# The port

<!-- docgen:code file=loose/symbols.rs symbol=Port -->
<!-- /docgen:code -->

A method, with its doc comment:

<!-- docgen:code file=loose/symbols.rs symbol=Port::new raw=true -->
<!-- /docgen:code -->

A trait impl, and the file doc:

<!-- docgen:code file=loose/symbols.rs symbol="impl Display for Port" -->
<!-- /docgen:code -->

<!-- docgen:code file=loose/symbols.rs symbol=crate raw=true -->
<!-- /docgen:code -->
```

#### After

````
# The port

<!-- docgen:code file=loose/symbols.rs symbol=Port -->
```rust
#[derive(Debug)]
pub struct Port<T> {
    /// The number.
    number: T,
}
```
<!-- /docgen:code -->

A method, with its doc comment:

<!-- docgen:code file=loose/symbols.rs symbol=Port::new raw=true -->
```rust
/// Makes a port.
#[inline]
pub const fn new(number: T) -> Self {
    Port { number }
}
```
<!-- /docgen:code -->

A trait impl, and the file doc:

<!-- docgen:code file=loose/symbols.rs symbol="impl Display for Port" -->
```rust
impl std::fmt::Display for Port<u8> {
    fn fmt(&self, f: &mut std::fmt::Formatter) -> std::fmt::Result {
        write!(f, "{}", self.number)
    }
}
```
<!-- /docgen:code -->

<!-- docgen:code file=loose/symbols.rs symbol=crate raw=true -->
```rust
//! A file with no crate.
//! It holds one of each item.
```
<!-- /docgen:code -->
````
<!-- /docgen:docgen -->
<!-- /site:editor -->

### docgen:config

<!-- docgen:doc pkg=. section="docgen:config" -->
A table of the fields of a struct type, with the columns Field, Type, Default and Description.

	docgen:config pkg=<path> type=<Config> [print=<go|raw|config|name|verb>]

`type=` is a struct type of the package, exported or not. It can also be an alias or a defined type of a struct type, or of an instance of a generic struct type, such as `type Small = Gen[int]`.

The rows:

  - Each exported field is one row, in source order.
  - An embedded field with an exported type name, such as `Timing` or `*url.URL`, is one row. The type name is the field name.
  - An unexported field has no row.
  - An embedded field with an unexported type name has no row.
  - The fields of an embedded struct have no rows.

The Field column holds the field name. The Type column holds the field type as it is in Go source in the package of the marker:

  - A type of another package has the package name, such as `time.Duration` or `map[string]*url.URL`.
  - Pointers, slices, maps, channels, funcs, struct types with their tags and instances of generic types, such as `q.G[string]`, are as in Go source.
  - An alias is the alias name.
  - A cgo type has the prefix `C.`, such as `C.int`.

The Description column holds the doc comment of the field:

  - With no doc comment, it holds the trailing comment. With neither, it is empty.
  - Each name in one declaration, such as `A, B int // The limits.`, has that comment.
  - For an alias or a defined type of a struct, the comments come from the declaration of that struct, also in another package.
  - Line breaks and blank lines become spaces.
  - A `|` becomes `\|`. Other text, such as HTML, backticks and doc links, stays as written, so Markdown renders it.

The Default column holds each default of the field as written in the code, with its value in parentheses. Two or more defaults are joined with "or", in the order of file names and lines. A default written the same way twice is there once. A field with no default has one of these:

  - "not machine-readable; see the description", when the field doc has a phrase from the list below.
  - "zero value", when it does not.

The phrases match in any case and across a line break: `0 means`, `nil means`, `zero means`, `empty means`, `a zero <word> means`, `an empty <word> means`, `defaults to` and `by default`. Other text, such as "Zero value means" or "The default is", gives "zero value".

docgen finds a default in the files of the package of `pkg=`. Files that end in `_test.go`, and the code of other packages, do not count. For a default that is set in another package, use `pkg=` of that package and an alias of the type there, such as `type Config = server.Config`.

A default is the value V of an assignment `y = V` in this form:

	if x == 0 {
		y = V
	}

x and y are copies of the same field. A copy is one of these:

  - The field itself, such as `c.Wait`, for any value of the struct type.
  - A variable or field that gets a copy: `wait := c.Wait`, `wait = c.Wait` or `var wait = c.Wait`.
  - A field that gets a copy in a composite literal, such as `wait` in `Lamp{wait: c.Wait}`. Then `l.wait` is a copy for any `Lamp` l.
  - A parameter of a function or method of the package that gets a copy as its argument, such as `n` of `func use(n int)` in `use(c.Wait)`.

The forms of the test:

  - The condition of an `if`, or a case of a `switch` with no tag.
  - `x == Z` or `x <= Z`, where Z is `0`, `0.0`, `""`, `nil`, or a constant with the value 0 or "".
  - The `if` can have an init statement: `if x := c.Wait; x == 0`.
  - The assignment can be in a nested `if` or block of the body.

These do not count:

  - The tests `0 == x`, `x < 1`, `!x`, `x == false`, `len(x) == 0` and `x == 0 && y`.
  - A `switch x` with a tag, and the `else` of an `if`.
  - An assignment with `:=` or `+=`, with two or more targets, or in a `for` or `range` loop or a func literal.

`print=` sets the text of the value in parentheses, with the same choices as `docgen:const`. With no `print=`, the `print` setting applies. Its default is `go`.

  - `go`: the value as `fmt.Sprint` prints it, with the String, Error, Format and GoString methods of its type: `DefaultWait` (250ms). A literal, such as `3`, has no value, unless its type has one of these methods: `80` (port-80).
  - `raw`: the underlying value, with no method: `DefaultWait` (250000000). A literal has no value.
  - `config`: the value as a config file holds it, as for `docgen:const`: `SceneEvening` (evening) for a type with a MarshalText method, `5 * time.Second` (5s) for a `time.Duration`, and `ModeFade` (1) for an enum with a String method. A literal has no value, unless its type is `time.Duration` or has a MarshalText method.
  - `name`: the name of the one constant of the type with the value, as for `docgen:const`: `LevelLow + 1` (LevelHigh), or `60 * time.Second` (time.Minute). A literal has no value, and neither does a value with no such constant.
  - A fmt verb, such as `%x`: the value with that verb, also of a literal: `DefaultColor` (ffaa00).

A default that is not a constant, such as `hold` or `new(int)`, has no value with any `print=`. A value with the same text as its default, such as `-1`, has no parentheses. A value of a type with one of these methods, or with `config` of a type with a MarshalText method, comes from one `go test` run of the package, as for `docgen:const`.

These are errors:

  - `pkg=` with no package, or `type=` that is not a type of the package.
  - A type that is not a struct, a generic type, or a struct with no row.
  - An empty `print=`, or a value that is not `go`, `raw`, `config`, `name` or one fmt verb.
  - A verb that a default cannot use, such as `%d` for a string.
  - With `config`, a MarshalText method that returns an error or panics.
  - `print=name` when two constants of the type have the value.

For a Rust crate, `crate=` takes the place of `pkg=`, and `type=` is a struct from the crate root (see Rust crates):

	docgen:config crate=<package> type=<path> [print=<display|debug|raw|name|spec>] [features=<list>] [allfeatures=<bool>] [nodefaultfeatures=<bool>]

The table has the columns Field, Type, Default and Description, as in Go. The struct has named fields and no generics, and it can be private. A struct is the only kind of `type=`: an alias, an enum, a tuple struct, a unit struct and a generic struct are errors.

The rows:

  - A field is a row when it is `pub`, in source order. When the struct derives `Deserialize`, every field is a row, also a private one, since serde reads it. A field with `pub(crate)` or another restriction counts as private.
  - A field with `#[serde(skip)]` or `#[serde(skip_deserializing)]` has no row.
  - A field of a struct type, also with `#[serde(flatten)]`, is one row, as an embedded field is in Go. The fields of that struct have no rows, and its type is not read.

The Field column holds the name that a config file uses, and in Go it is the field name. That is the Rust name, unless serde renames it, in this order:

  - `#[serde(rename = "name")]` on the field, or `rename(deserialize = "name")`, which is the name for reading a file.
  - `#[serde(rename_all = "case")]` on the struct, with the case `lowercase`, `UPPERCASE`, `PascalCase`, `camelCase`, `snake_case`, `SCREAMING_SNAKE_CASE`, `kebab-case` or `SCREAMING-KEBAB-CASE`. The `deserialize` form of it applies as well. Any other case is an error.
  - The name in the source, with no `r#`.

A struct with no serde attribute has Rust names. docgen reads `#[serde]` on the struct and its fields, not what a `#[serde(with = ...)]` function or another crate does with a name.

The Type column holds the field type as the source writes it, such as `Vec<String>` or `std::time::Duration`, with no change of its path. The Description column holds the doc comment of the field, joined as in Go: line breaks and blank lines become spaces, and a `|` becomes `\|`. Rust has no trailing comment, so a field with no doc comment has an empty description.

The Default column holds the default of the field as the source writes it, then its value in parentheses, as in Go. A default has one of these sources, and the first that applies is the one:

  - `#[serde(default = "path")]` on the field: the call `path()`. The path is read in the module of the struct, as serde reads it.
  - `#[serde(default)]` on the field: `Default::default()`, which is the `Default` of the field type.
  - `#[serde(default = "path")]` on the struct: the field of the value that `path()` returns. The source of the field is not written.
  - An `impl Default for Type` in any module of the crate, or `#[derive(Default)]`, which `#[serde(default)]` on the struct uses as well: the field of `Type::default()`. The source is the expression that the `default` fn sets the field to, when its body is one struct literal such as `Self { port: DEFAULT_PORT, .. }` or `Type { .. }`, and it is not written for a derive, or for a body of another form, or a field that a `..base` fills.

So a serde attribute on a field takes the place of the `Default` of the struct for that field, as it does when serde reads a file with no such key. A field with none of these sources has no default. Its cell is "not machine-readable; see the description" when the doc has a phrase of the Go list, and else "no default". Unlike Go, Rust has no zero value to name.

Each value comes from the value test of the crate, which calls the default code, so the table holds what the code returns and not what a doc comment states. The test of one marker has one call for each field, so a default with a side effect runs once for each field. `print=` sets the text of the value with the choices of `docgen:const` for Rust:

  - With no `print=`, the value prints with `Display`, or else `Debug`: `localhost` for a `String`, and `1.5s` for a `Duration`. A value with neither impl, such as a struct with no derive of `Debug`, has no value and the cell holds only the source, or `Default::default()` when there is none.
  - `display` and `debug` use that impl, and a type with no such impl is an error that holds the message of rustc.
  - `raw` is the source as written, with no value and no code, for each field with a written source. A field with none prints as it does with no `print=`, so the value test still runs for it.
  - `name` is the path from the crate root of the one const that has the type of the field and its value, as `docgen:const` finds it, such as `server::DEFAULT_PORT`. A field with no such const has no value, and two such consts are an error.
  - A format spec of one value, such as `{:#x}` or `{:?}`, applies to each default, so it is an error for a field whose type cannot use it.

The `print` setting applies as it does for `docgen:const`, and `features=`, `allfeatures=` and `nodefaultfeatures=` choose the Cargo features of the value test.

A `Default` that panics is an error that holds the message of the panic and the field. The other markers of the run still print their values.

These are errors of the Rust form:

  - `crate=` with `pkg=`.
  - An empty `print=`, or a value that is not `display`, `debug`, `raw`, `name` or a format spec.
  - A `type=` that names no item, or an item that is not a struct, a struct with tuple fields or none, and a generic struct.
  - A struct with no row: no `pub` field, and no derive of `Deserialize`.
  - A `rename_all` that is not one of the cases above.
  - A default with no code that compiles, such as a spec that the type cannot use, or a `#[serde(default = "path")]` whose path names nothing.
  - A default that panics.
  - A struct in a module with a `#[cfg]` that is false in the test build.
  - `print=name` when two consts of the type of a field have its value.
<!-- /docgen:doc -->

#### Defaults as Go prints them

<!-- site:editor -->
<!-- docgen:code file=internal/cli/testdata/repo/v2/machine/machine.go symbol=Config -->
```go
type Config struct {
	// Wait is how long the Lamp stays on. 0 means DefaultWait.
	Wait time.Duration
	// Tries is the number of blinks. 0 means 1, and it is at most MaxTries.
	Tries int
	// Name is a label.
	Name string
}
```
<!-- /docgen:code -->
<!-- docgen:code file=internal/cli/testdata/repo/v2/machine/machine.go symbol=NewLamp -->
```go
func NewLamp(cfg Config) (*Lamp, error) {
	if cfg.Tries > MaxTries {
		return nil, &docgen.ConfigError{Field: "Tries"}
	}
	l := &Lamp{wait: cfg.Wait, tries: cfg.Tries}
	if l.wait == 0 {
		l.wait = DefaultWait
	}
	if cfg.Tries == 0 {
		l.tries = 1
	}
	return l, nil
}
```
<!-- /docgen:code -->
<!-- docgen:docgen root=internal/cli/testdata/repo module=v2 page=docs/machines/guide-config.md -->
#### Before

```
# Settings

<!-- docgen:config pkg=machine type=Config -->
<!-- /docgen:config -->
```

#### After

```
# Settings

<!-- docgen:config pkg=machine type=Config -->
| Field | Type | Default | Description |
|---|---|---|---|
| `Wait` | `time.Duration` | `DefaultWait` (250ms) | Wait is how long the Lamp stays on. 0 means DefaultWait. |
| `Tries` | `int` | `1` | Tries is the number of blinks. 0 means 1, and it is at most MaxTries. |
| `Name` | `string` | zero value | Name is a label. |
<!-- /docgen:config -->
```
<!-- /docgen:docgen -->
<!-- /site:editor -->

#### Underlying values, with `print=raw`

<!-- site:editor -->
<!-- docgen:code file=internal/cli/testdata/repo/v2/machine/machine.go symbol=DefaultWait -->
```go
const DefaultWait = 250 * time.Millisecond
```
<!-- /docgen:code -->
<!-- docgen:code file=internal/cli/testdata/repo/v2/machine/machine.go symbol=Config -->
```go
type Config struct {
	// Wait is how long the Lamp stays on. 0 means DefaultWait.
	Wait time.Duration
	// Tries is the number of blinks. 0 means 1, and it is at most MaxTries.
	Tries int
	// Name is a label.
	Name string
}
```
<!-- /docgen:code -->
<!-- docgen:code file=internal/cli/testdata/repo/v2/machine/machine.go symbol=NewLamp -->
```go
func NewLamp(cfg Config) (*Lamp, error) {
	if cfg.Tries > MaxTries {
		return nil, &docgen.ConfigError{Field: "Tries"}
	}
	l := &Lamp{wait: cfg.Wait, tries: cfg.Tries}
	if l.wait == 0 {
		l.wait = DefaultWait
	}
	if cfg.Tries == 0 {
		l.tries = 1
	}
	return l, nil
}
```
<!-- /docgen:code -->
<!-- docgen:docgen root=internal/cli/testdata/repo module=v2 page=docs/machines/guide-config-print.md -->
#### Before

```
<!-- docgen:config pkg=machine type=Config print=raw -->
<!-- /docgen:config -->
```

#### After

```
<!-- docgen:config pkg=machine type=Config print=raw -->
| Field | Type | Default | Description |
|---|---|---|---|
| `Wait` | `time.Duration` | `DefaultWait` (250000000) | Wait is how long the Lamp stays on. 0 means DefaultWait. |
| `Tries` | `int` | `1` | Tries is the number of blinks. 0 means 1, and it is at most MaxTries. |
| `Name` | `string` | zero value | Name is a label. |
<!-- /docgen:config -->
```
<!-- /docgen:docgen -->
<!-- /site:editor -->

#### Values as a config file holds them, with `print=config`

<!-- site:editor -->
<!-- docgen:code file=internal/cli/testdata/repo/v2/dimmer/dimmer.go symbol=Mode -->
```go
type Mode int
```
<!-- /docgen:code -->
<!-- docgen:code file=internal/cli/testdata/repo/v2/dimmer/dimmer.go symbol=Scene -->
```go
type Scene uint8
```
<!-- /docgen:code -->
<!-- docgen:code file=internal/cli/testdata/repo/v2/dimmer/dimmer.go symbol=Program -->
```go
type Program struct {
	// Mode is how the level changes.
	Mode Mode
	// Scene is the scene at start.
	Scene Scene
	// Hold is how long a scene stays.
	Hold time.Duration
}
```
<!-- /docgen:code -->
<!-- docgen:code file=internal/cli/testdata/repo/v2/dimmer/dimmer.go symbol=NewProgram -->
```go
func NewProgram(p Program) Program {
	if p.Mode == ModeStep {
		p.Mode = ModeFade
	}
	if p.Scene == 0 {
		p.Scene = SceneEvening
	}
	if p.Hold == 0 {
		p.Hold = 5 * time.Second
	}
	return p
}
```
<!-- /docgen:code -->
<!-- docgen:docgen root=internal/cli/testdata/repo module=v2 page=docs/dimmer/config-config.md -->
#### Before

```
<!-- docgen:config pkg=dimmer type=Program print=config -->
<!-- /docgen:config -->
```

#### After

```
<!-- docgen:config pkg=dimmer type=Program print=config -->
| Field | Type | Default | Description |
|---|---|---|---|
| `Mode` | `Mode` | `ModeFade` (1) | Mode is how the level changes. |
| `Scene` | `Scene` | `SceneEvening` (evening) | Scene is the scene at start. |
| `Hold` | `time.Duration` | `5 * time.Second` (5s) | Hold is how long a scene stays. |
<!-- /docgen:config -->
```
<!-- /docgen:docgen -->
<!-- /site:editor -->

With `print=go`, the same defaults are `ModeFade` (mode-fade), `SceneEvening` (2) and `5 * time.Second` (5s).

#### Constant names, with `print=name`

<!-- site:editor -->
<!-- docgen:code file=internal/cli/testdata/repo/v2/dimmer/dimmer.go symbol=LevelOff -->
```go
const (
	// LevelOff gives no light.
	LevelOff Level = iota
	// LevelLow gives some light.
	LevelLow
	// LevelHigh gives full light.
	LevelHigh
)
```
<!-- /docgen:code -->
<!-- docgen:code file=internal/cli/testdata/repo/v2/dimmer/dimmer.go symbol=Look -->
```go
type Look struct {
	// Level is the brightness at start.
	Level Level
	// Color is an RGB color.
	Color uint32
}
```
<!-- /docgen:code -->
<!-- docgen:code file=internal/cli/testdata/repo/v2/dimmer/dimmer.go symbol=NewLook -->
```go
func NewLook(l Look) Look {
	if l.Level == LevelOff {
		l.Level = LevelLow + 1
	}
	if l.Color == 0 {
		l.Color = DefaultColor
	}
	return l
}
```
<!-- /docgen:code -->
<!-- docgen:docgen root=internal/cli/testdata/repo module=v2 page=docs/dimmer/config-name.md -->
#### Before

```
<!-- docgen:config pkg=dimmer type=Look print=name -->
<!-- /docgen:config -->
```

#### After

```
<!-- docgen:config pkg=dimmer type=Look print=name -->
| Field | Type | Default | Description |
|---|---|---|---|
| `Level` | `Level` | `LevelLow + 1` (LevelHigh) | Level is the brightness at start. |
| `Color` | `uint32` | `DefaultColor` | Color is an RGB color. |
<!-- /docgen:config -->
```
<!-- /docgen:docgen -->
<!-- /site:editor -->

#### Values with a fmt verb, such as `print=%x`

<!-- site:editor -->
<!-- docgen:code file=internal/cli/testdata/repo/v2/dimmer/dimmer.go symbol=DefaultColor -->
```go
const DefaultColor = 0xffaa00
```
<!-- /docgen:code -->
<!-- docgen:code file=internal/cli/testdata/repo/v2/dimmer/dimmer.go symbol=Look -->
```go
type Look struct {
	// Level is the brightness at start.
	Level Level
	// Color is an RGB color.
	Color uint32
}
```
<!-- /docgen:code -->
<!-- docgen:code file=internal/cli/testdata/repo/v2/dimmer/dimmer.go symbol=NewLook -->
```go
func NewLook(l Look) Look {
	if l.Level == LevelOff {
		l.Level = LevelLow + 1
	}
	if l.Color == 0 {
		l.Color = DefaultColor
	}
	return l
}
```
<!-- /docgen:code -->
<!-- docgen:docgen root=internal/cli/testdata/repo module=v2 page=docs/dimmer/config-verb.md -->
#### Before

```
<!-- docgen:config pkg=dimmer type=Look print=%x -->
<!-- /docgen:config -->
```

#### After

```
<!-- docgen:config pkg=dimmer type=Look print=%x -->
| Field | Type | Default | Description |
|---|---|---|---|
| `Level` | `Level` | `LevelLow + 1` (2) | Level is the brightness at start. |
| `Color` | `uint32` | `DefaultColor` (ffaa00) | Color is an RGB color. |
<!-- /docgen:config -->
```
<!-- /docgen:docgen -->
<!-- /site:editor -->

#### A field whose default is not machine-readable

<!-- site:editor -->
<!-- docgen:code file=internal/cli/testdata/repo/v2/dimmer/dimmer.go symbol=Steps -->
```go
type Steps struct {
	// Step is the change of one press. 0 means 1.
	Step int
}
```
<!-- /docgen:code -->
<!-- docgen:code file=internal/cli/testdata/repo/v2/dimmer/dimmer.go symbol=NewSteps -->
```go
func NewSteps(s Steps) Steps {
	if s.Step < 1 {
		s.Step = 1
	}
	return s
}
```
<!-- /docgen:code -->
<!-- docgen:docgen root=internal/cli/testdata/repo module=v2 page=docs/dimmer/config-text.md -->
#### Before

```
<!-- docgen:config pkg=dimmer type=Steps -->
<!-- /docgen:config -->
```

#### After

```
<!-- docgen:config pkg=dimmer type=Steps -->
| Field | Type | Default | Description |
|---|---|---|---|
| `Step` | `int` | not machine-readable; see the description | Step is the change of one press. 0 means 1. |
<!-- /docgen:config -->
```
<!-- /docgen:docgen -->
<!-- /site:editor -->

#### A field with no default

<!-- site:editor -->
<!-- docgen:code file=internal/cli/testdata/repo/v2/dimmer/dimmer.go symbol=Tag -->
```go
type Tag struct {
	// Name is a label next to the Dimmer.
	Name string
}
```
<!-- /docgen:code -->
<!-- docgen:docgen root=internal/cli/testdata/repo module=v2 page=docs/dimmer/config-zero.md -->
#### Before

```
<!-- docgen:config pkg=dimmer type=Tag -->
<!-- /docgen:config -->
```

#### After

```
<!-- docgen:config pkg=dimmer type=Tag -->
| Field | Type | Default | Description |
|---|---|---|---|
| `Name` | `string` | zero value | Name is a label next to the Dimmer. |
<!-- /docgen:config -->
```
<!-- /docgen:docgen -->
<!-- /site:editor -->

#### Rows of embedded fields

<!-- site:editor -->
<!-- docgen:code file=internal/cli/testdata/repo/v2/dimmer/dimmer.go symbol=Timing -->
```go
type Timing struct {
	// Fade is the time of a change of level.
	Fade time.Duration
}
```
<!-- /docgen:code -->
<!-- docgen:code file=internal/cli/testdata/repo/v2/dimmer/dimmer.go symbol=counts -->
```go
type counts struct {
	// Presses is the number of presses.
	Presses int
}
```
<!-- /docgen:code -->
<!-- docgen:code file=internal/cli/testdata/repo/v2/dimmer/dimmer.go symbol=Setup -->
```go
type Setup struct {
	// Timing holds the times.
	Timing
	counts
	// Look is the light at start.
	Look Look
}
```
<!-- /docgen:code -->
<!-- docgen:docgen root=internal/cli/testdata/repo module=v2 page=docs/dimmer/config-embedded.md -->
#### Before

```
<!-- docgen:config pkg=dimmer type=Setup -->
<!-- /docgen:config -->
```

#### After

```
<!-- docgen:config pkg=dimmer type=Setup -->
| Field | Type | Default | Description |
|---|---|---|---|
| `Timing` | `Timing` | zero value | Timing holds the times. |
| `Look` | `Look` | zero value | Look is the light at start. |
<!-- /docgen:config -->
```
<!-- /docgen:docgen -->
<!-- /site:editor -->

#### Field types from other packages

<!-- site:editor -->
<!-- docgen:code file=internal/cli/testdata/repo/v2/dimmer/dimmer.go symbol=Group -->
```go
type Group struct {
	// Main is the first lamp.
	Main *machine.Lamp
	// Spares are the other lamps.
	Spares []machine.Lamp
	// States holds the state of each lamp by name.
	States map[string]machine.LampState
}
```
<!-- /docgen:code -->
<!-- docgen:docgen root=internal/cli/testdata/repo module=v2 page=docs/dimmer/config-types.md -->
#### Before

```
<!-- docgen:config pkg=dimmer type=Group -->
<!-- /docgen:config -->
```

#### After

```
<!-- docgen:config pkg=dimmer type=Group -->
| Field | Type | Default | Description |
|---|---|---|---|
| `Main` | `*machine.Lamp` | zero value | Main is the first lamp. |
| `Spares` | `[]machine.Lamp` | zero value | Spares are the other lamps. |
| `States` | `map[string]machine.LampState` | zero value | States holds the state of each lamp by name. |
<!-- /docgen:config -->
```
<!-- /docgen:docgen -->
<!-- /site:editor -->

#### A description from a trailing comment

<!-- site:editor -->
<!-- docgen:code file=internal/cli/testdata/repo/v2/dimmer/dimmer.go symbol=Range -->
```go
type Range struct {
	Min Level // Min is the lowest level.
	// Max is the highest level.
	Max Level // This comment is not in the table.
}
```
<!-- /docgen:code -->
<!-- docgen:docgen root=internal/cli/testdata/repo module=v2 page=docs/dimmer/config-trailing.md -->
#### Before

```
<!-- docgen:config pkg=dimmer type=Range -->
<!-- /docgen:config -->
```

#### After

```
<!-- docgen:config pkg=dimmer type=Range -->
| Field | Type | Default | Description |
|---|---|---|---|
| `Min` | `Level` | zero value | Min is the lowest level. |
| `Max` | `Level` | zero value | Max is the highest level. |
<!-- /docgen:config -->
```
<!-- /docgen:docgen -->
<!-- /site:editor -->

### docgen:configulator

<!-- docgen:doc pkg=. section="docgen:configulator" -->
The reference table of a config type of [configulator](https://github.com/USA-RedDragon/configulator): each key, with its type, default, environment variable, flag and description.

	docgen:configulator pkg=<path> type=<Config> [env=<prefix>] [envsep=<separator>] [flagsep=<separator>]

For a Rust crate, `type=` is a struct with `#[derive(Config)]` of [configulator-rs](https://github.com/USA-RedDragon/configulator-rs):

	docgen:configulator crate=<package> type=<path> [env=<prefix>] [envsep=<separator>] [flagsep=<separator>] [features=<list>] [allfeatures=<bool>] [nodefaultfeatures=<bool>]

The table is the one that `configulator --markdown` of configulator-cli prints, with no heading. docgen gets it from the markdown function of configulator-rs, on the fields that the derive of the type reports, in the value test of the crate (see Rust crates). So the table comes from the configulator-rs version of `Cargo.lock`. `env=`, `envsep=` and `flagsep=` are the `--env-prefix`, `--env-separator` and `--flag-separator` of configulator-cli, with the same defaults: no prefix, `_` and `.`. A `#[configulator(crate = "path")]` on the struct names a renamed crate.

These are errors of the Rust form:

  - `type=` that names no item, or an item that is not a struct.
  - A struct with no `#[derive(Config)]`.
  - A crate that does not build in test mode, with the message of rustc. The test calls a hidden markdown function of configulator-rs, which versions 0.3.1 to 0.5.0 have.
<!-- /docgen:doc -->

<!-- site:editor -->
<!-- docgen:code file=internal/cli/testdata/settings/config.go symbol=Config -->
```go
type Config struct {
	Name   string `name:"name" default:"lamp" description:"name of the program"`
	Server Server `name:"server"`
}
```
<!-- /docgen:code -->
<!-- docgen:code file=internal/cli/testdata/settings/config.go symbol=Server -->
```go
type Server struct {
	Port int `name:"port" default:"8080" description:"port of the server"`
}
```
<!-- /docgen:code -->
<!-- docgen:docgen root=internal/cli/testdata/settings page=docs/guide-configulator.md -->
#### Before

```
<!-- docgen:configulator pkg=. type=Config env=APP_ envsep=__ flagsep=- -->
<!-- /docgen:configulator -->
```

#### After

```
<!-- docgen:configulator pkg=. type=Config env=APP_ envsep=__ flagsep=- -->
| Key           | Type    | Default | Environment        | Flag            | Description         |
|---------------|---------|---------|--------------------|-----------------|---------------------|
| `name`        | string  | `lamp`  | `APP_NAME`         | `--name`        | name of the program |
| `server.port` | integer | `8080`  | `APP_SERVER__PORT` | `--server-port` | port of the server  |
<!-- /docgen:configulator -->
```
<!-- /docgen:docgen -->
<!-- /site:editor -->

### docgen:const

<!-- docgen:doc pkg=. section="docgen:const" -->
The value of a constant, or of a package-level var with a constant initial value, exported or not. It goes inline in a sentence.

	docgen:const pkg=<path> name=<Name> [print=<go|raw|config|name|verb>] [code=false]

`print=` sets how the value prints:

  - `print=go` is the default. The value prints as `fmt.Sprint` prints it, with the String, Error, Format and GoString methods of its type, so `250 * time.Millisecond` is `250ms`, strings have no quotes and `1e6` is `1e+06`.
  - `print=raw` is the underlying value with no method, and runs no code: `4001` for a port with a String method, `1` for an enum, and `1500000000` for a `time.Duration` of 1.5s.
  - `print=config` is the value as a config file holds it. For a type with a MarshalText method of encoding.TextMarshaler, also on the pointer, it is the text of that method. For `time.Duration`, it is the text of fmt, such as `1.5s`. Any other value is as for `print=raw`: `1` for an enum with a String method, and `1500000000` for a defined type of `time.Duration`, such as `type Ticks time.Duration`, since goccy/go-yaml and configulator parse text such as `5s` only for `time.Duration`. A MarshalText method that returns an error or panics is an error.
  - `print=name` is the name of the one constant of the type with the value, from the package of the marker and then the package of the type. A constant of the package of the type is exported and has the package name, as in Go source: `time.Minute` for `60 * time.Second`.
  - `print=%d`, or any other fmt verb, is the value with that verb.

An empty value prints as `""`, and line endings in a value print as spaces. A verb that the value cannot use is an error. An integer too large for its default type prints its exact value. A type with a print method, or with `print=config` a type with a MarshalText method, runs in one `go test` for each package, so the tests of that package must compile.

With `code=false`, the value has no backticks: 250ms in place of `250ms`. Any other value of `code=` is an error.

For a Rust crate, `crate=` takes the place of `pkg=`, and `name=` is a path from the crate root (see Rust crates):

	docgen:const crate=<package> name=<path> [print=<display|debug|raw|name|spec>] [code=false] [features=<list>] [allfeatures=<bool>] [nodefaultfeatures=<bool>]

`name=` is a `const`, a `static` that is not `mut`, or an associated const of an inherent impl, such as `limits::Port::HTTP`. Private items work too. The value comes from the value test of the crate, other than with `print=raw`:

  - With no `print=`, the value prints with its `Display` impl, or else with its `Debug` impl, so a `&str` has no quotes and a `Duration` of 1.5s is `1.5s`.
  - `print=display` and `print=debug` use that impl, as `{}` and `{:?}` do.
  - `print=raw` is the expression as the source writes it, such as `MAX_JOBS / 4`, and runs no code.
  - `print=name` is the path from the crate root of the one other const with the type of the item, as the source writes it, and the same value, such as `limits::PORT_WEB`. The type must implement `PartialEq`. A const counts when the module of the item can name it.
  - `print={:#x}`, or any other Rust format spec of one value, is the value with that spec.

The `print` setting applies to a Rust marker with no `print=` when it is `raw`, `display` or `debug`, as that `print=` does. The settings `go` and `config` are for Go markers, and leave the default of a Rust marker above. These are errors of the Rust form:

  - `crate=` with `pkg=`.
  - An empty `print=`, or a value that is not `display`, `debug`, `raw`, `name` or a format spec.
  - `name=` that names no item, or an item that is not a `const` or `static`, or a `static mut`.
  - A value with no code that compiles, such as a type with no `Display` and no `Debug` impl, or a spec that the type cannot use, such as `{:x}` for a string. The error holds the message of rustc.
  - An item in a module with a `#[cfg]` that is false in the test build.
  - `print=name` when no other const, or two other consts, have the value.

For a C file, `file=` takes the place of `pkg=`, and `name=` is an object-like `#define`:

	docgen:const file=<path> name=<DEFINE> [print=value] [code=false]

`file=` is a `.c` or `.h` file, relative to the repository root and inside it, as for `docgen:code`. The value is the replacement list of the `#define` as the source writes it, and docgen never evaluates it: `#define X (1 << 3)` is `(1 << 3)`. Whitespace around the text is gone, each backslash and line ending of a continuation is one space, and comments are gone, so `#define PORT 4001 /* default */` is `4001`. The text of a string or character literal stays as it is. The `#define` is found as `docgen:code symbol=` finds it. The value is a code span, or with `code=false` the text with no backticks. It needs no cargo and no Go toolchain, so it works with none.

`print=value` is the one `print=` value of the C form. It applies to a `#define` whose replacement text is exactly one integer literal, and prints the literal without its suffix. The literal is decimal, octal (a leading `0`), hexadecimal (`0x` or `0X`) or binary (`0b` or `0B`), with `'` between digits as C23 has it. The suffix is `u`, `l`, `ll`, `z` or `wb`, or `u` with one of the others in either order, in any case, but `ll` in one case. The base and the digits stay as written: `#define X 0x10UL` is `0x10`. A literal in parentheses, such as `(10u)`, is not one integer literal. Without `print=`, the text is as written, suffix included. These are errors of the C form:

  - `file=` that is not a `.c` or `.h` file, or with `pkg=` or `crate=`.
  - `print=` with a value other than `value`, which is the one that applies.
  - `print=value` for text that is not one integer literal, such as `(1 << 3)`, `(10u)`, `-1`, `1.5` or a string. The error names the text. A suffix that C does not have, such as `uu`, and an octal literal with a digit 8 or 9, are errors too.
  - A `name=` that is not an identifier, or that no `#define` has. The error lists the names of the `#define`s of the file.
  - A name with more than one `#define`, such as one in each branch of an `#ifdef`. The error lists the lines.
  - A function-like macro, such as `#define F(x) ...`, and a `#define` with no replacement text, such as `#define FLAG`.
<!-- /docgen:doc -->

#### A value as Go prints it

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

#### A value in another form, with `print=`

<!-- site:editor -->
<!-- docgen:code file=internal/cli/testdata/repo/v2/machine/port.go -->
```go
package machine

import "fmt"

// Port is a network port.
type Port uint16

// String returns the port as text.
func (p Port) String() string { return fmt.Sprintf("port %d", uint16(p)) }

// PortWeb is the port of the web server.
const PortWeb Port = 8080

// Listen is the port of a new Lamp.
var Listen = PortWeb
```
<!-- /docgen:code -->
<!-- docgen:docgen root=internal/cli/testdata/repo module=v2 page=docs/machines/guide-const-print.md -->
#### Before

```
- Default: <!-- docgen:const pkg=machine name=Listen --><!-- /docgen:const -->
- `go`: <!-- docgen:const pkg=machine name=Listen print=go --><!-- /docgen:const -->
- `raw`: <!-- docgen:const pkg=machine name=Listen print=raw --><!-- /docgen:const -->
- `config`: <!-- docgen:const pkg=machine name=Listen print=config --><!-- /docgen:const -->
- `name`: <!-- docgen:const pkg=machine name=Listen print=name --><!-- /docgen:const -->
- `%06d`: <!-- docgen:const pkg=machine name=Listen print=%06d --><!-- /docgen:const -->
```

#### After

```
- Default: <!-- docgen:const pkg=machine name=Listen -->`port 8080`<!-- /docgen:const -->
- `go`: <!-- docgen:const pkg=machine name=Listen print=go -->`port 8080`<!-- /docgen:const -->
- `raw`: <!-- docgen:const pkg=machine name=Listen print=raw -->`8080`<!-- /docgen:const -->
- `config`: <!-- docgen:const pkg=machine name=Listen print=config -->`8080`<!-- /docgen:const -->
- `name`: <!-- docgen:const pkg=machine name=Listen print=name -->`PortWeb`<!-- /docgen:const -->
- `%06d`: <!-- docgen:const pkg=machine name=Listen print=%06d -->`008080`<!-- /docgen:const -->
```
<!-- /docgen:docgen -->
<!-- /site:editor -->

#### A value with no backticks, with `code=false`

<!-- site:editor -->
<!-- docgen:code file=internal/cli/testdata/repo/v2/machine/machine.go symbol=DefaultWait -->
```go
const DefaultWait = 250 * time.Millisecond
```
<!-- /docgen:code -->
<!-- docgen:docgen root=internal/cli/testdata/repo module=v2 page=docs/machines/guide-const-code.md -->
#### Before

```
The default wait is <!-- docgen:const pkg=machine name=DefaultWait code=false --><!-- /docgen:const -->.
```

#### After

```
The default wait is <!-- docgen:const pkg=machine name=DefaultWait code=false -->250ms<!-- /docgen:const -->.
```
<!-- /docgen:docgen -->
<!-- /site:editor -->

### docgen:errors

<!-- docgen:doc pkg=. section="docgen:errors" -->
A table of the errors named in the doc comments of the constructors and methods of a type. Names resolve in the package, the module packages that it imports and the root package of the module. Consts and vars of any error type count.

	docgen:errors pkg=<path> type=<Type>

For a Rust crate, `type=` is an enum or a struct that derives `Error` of [thiserror](https://docs.rs/thiserror):

	docgen:errors crate=<package> type=<path>

The table has a row for each variant of the enum, or one row for the struct. The columns are the error, Returned by, its `#[error(...)]` message as the source writes it, its source and the first sentence of its doc. The message of `#[error(transparent)]` is the message of its one field. The source is a field with `#[from]` (marked "with From"), `#[source]`, or the name `source`. Unlike the Go table, the rows come from the type and not from doc comments. The error links to docs.rs when the crate does (see Rust links).

Returned by lists the functions and the methods of the crate that name the error in the `# Errors` section of their doc comment, sorted by path and written as rustdoc reaches them, such as `store::Store::get` or `repair`. A row that no function names has an empty cell. The return type alone names nothing, and a function that is not listed for an error is not claimed to avoid it. A function counts when it is `pub` and reachable from outside the crate, through public modules or a `pub use`, and has no `#[cfg]` of its own or of a module above it. It is a free function or a method or an associated function of an inherent impl whose type has a one-segment path. Methods of trait impls and trait items are not read. The `# Errors` section is the text below the heading with the text `Errors` at any level, up to the next heading of the same or a higher level, as for `section=`. It names an error in a code span, a link, or a reference definition, and not in a code block. A name counts when it is:

  - `Enum::Variant`, with any path before `Enum` that leads to the enum when read in the module of the function, as in `crate::errors::Error::NotFound`. A path to another type of the same name is not a match.
  - `Self::Variant`, in a method of an impl of the enum.
  - `Variant` alone, when the return type of the function has a path that leads to the enum, such as `Result<T, Error>`. A return type of an alias such as `Result<T>` has no such path, so the section must write `Error::Variant`.
  - For a struct, a path that leads to the struct.

Naming the enum with no variant lists the function for no row. Only the crate of `crate=` is searched. These are errors of the Rust form:

  - `type=` that names no item, or an item that is not an enum or a struct.
  - A type with no `#[derive(Error)]` or `#[derive(thiserror::Error)]`.
  - An enum with no variants.
<!-- /docgen:doc -->

<!-- site:editor -->
<!-- docgen:code file=internal/cli/testdata/repo/v2/machine/machine.go symbol=ErrBusy -->
```go
var (
	// ErrBusy is returned by Start while the Lamp is on.
	ErrBusy = errors.New("machine: busy")
	// ErrOff is returned by Stop while the Lamp is off.
	ErrOff = errors.New("machine: off")
)
```
<!-- /docgen:code -->
<!-- docgen:code file=internal/cli/testdata/repo/v2/machine/machine.go symbol=NewLamp raw=true -->
```go
// NewLamp returns a Lamp. It returns a *docgen.ConfigError for a Tries
// above MaxTries.
func NewLamp(cfg Config) (*Lamp, error) {
	if cfg.Tries > MaxTries {
		return nil, &docgen.ConfigError{Field: "Tries"}
	}
	l := &Lamp{wait: cfg.Wait, tries: cfg.Tries}
	if l.wait == 0 {
		l.wait = DefaultWait
	}
	if cfg.Tries == 0 {
		l.tries = 1
	}
	return l, nil
}
```
<!-- /docgen:code -->
<!-- docgen:code file=internal/cli/testdata/repo/v2/machine/machine.go symbol=Lamp.Start raw=true -->
```go
// Start turns the Lamp on. It returns ErrBusy while the Lamp is on.
func (l *Lamp) Start(now time.Duration) error {
	if l.state == LampOn {
		return ErrBusy
	}
	if l.state == LampOff {
		l.state, l.until = LampOn, now+l.wait
	}
	return nil
}
```
<!-- /docgen:code -->
<!-- docgen:code file=internal/cli/testdata/repo/v2/machine/machine.go symbol=Lamp.Stop raw=true -->
```go
// Stop turns the Lamp off. It returns ErrOff while the Lamp is off.
func (l *Lamp) Stop() error {
	if l.state != LampOn {
		return ErrOff
	}
	l.off()
	return nil
}
```
<!-- /docgen:code -->
<!-- docgen:code file=internal/cli/testdata/repo/v2/doc.go -->
```go
// Package docgen is the fixture root package of the docgen tests.
package docgen

// ConfigError is the error of a constructor for a field with a value out
// of range.
type ConfigError struct{ Field string }

func (e *ConfigError) Error() string { return "bad " + e.Field }
```
<!-- /docgen:code -->
<!-- docgen:docgen root=internal/cli/testdata/repo module=v2 page=docs/machines/guide-errors.md -->
#### Before

```
# Errors

<!-- docgen:errors pkg=machine type=Lamp -->
<!-- /docgen:errors -->
```

#### After

```
# Errors

<!-- docgen:errors pkg=machine type=Lamp -->
| Error | Returned by | Meaning |
|---|---|---|
| [`docgen.ConfigError`](https://pkg.go.dev/github.com/USA-RedDragon/docgen/v2#ConfigError) | `NewLamp` | ConfigError is the error of a constructor for a field with a value out of range. |
| [`machine.ErrBusy`](https://pkg.go.dev/github.com/USA-RedDragon/docgen/v2/machine#ErrBusy) | `Lamp.Start` | ErrBusy is returned by Start while the Lamp is on. |
| [`machine.ErrOff`](https://pkg.go.dev/github.com/USA-RedDragon/docgen/v2/machine#ErrOff) | `Lamp.Stop` | ErrOff is returned by Stop while the Lamp is off. |
<!-- /docgen:errors -->
```
<!-- /docgen:docgen -->
<!-- /site:editor -->

### docgen:states

<!-- docgen:doc pkg=. section="docgen:states" -->
The states and the transitions of a state machine, and an SVG diagram of them.

	docgen:states pkg=<path> type=<Type> [run=<regexp>] [diagram=<path>]

The diagram goes in `states/<pkg>-<type>.svg` next to the page. `diagram=` gives another path, relative to the page, which must end in `.svg` and must not be a scanned page. Two machines with one diagram file are an error. `type=` can be an alias of a type of the package.

Value receiver methods, promoted methods and methods with no receiver name are not recorded, and the output lists them under Not recorded.

For a Rust crate:

	docgen:states crate=<package> type=<path> [run=<filter>] [diagram=<path>] [features=<list>] [allfeatures=<bool>] [nodefaultfeatures=<bool>]

`type=` is a path from the crate root (see Rust crates) to an enum or a struct with no generics. The state of the machine is the variant of an enum:

  - For an enum, the state is its own variant.
  - For a struct with a method `fn state(&self)` in an inherent impl, the state is the variant of the enum that it returns, or that the reference that it returns points to. A `state` method with parameters or generics, or whose return type is not an enum of the crate with no generics, is an error.
  - For a struct with no `state` method, the state is the variant of its one field whose type is an enum of the crate. A struct with no such field is an error, and so is a struct with more than one, which lists them.

The rows of the state table are the variants of that enum in source order, with their doc comments and no links. A variant with fields is one state, whatever its fields hold, and its row shows it as a pattern, such as `Door::Locked { .. }` or `Door::Locked(..)`. In the transition table and the diagram, a state is the name of its variant.

docgen records the pub methods (`pub` or `pub(...)`) of the inherent impls of the type:

  - A method with `&mut self` is a call on the machine. The hook reads the state when the call starts and when it returns, and a change is a transition.
  - A method with `self` or `mut self` that returns `Self` or the type, an `Option` of it, or a `Result` with it as the `Ok` type, is a transition from the state of `self` to the state of the result. A `None` or `Err` result is no transition. A method with `self` and another return type is not recorded.
  - An associated function with no `self` that returns the type in one of those forms, such as `new`, is a constructor, and the state of the value that it returns is a first state.
  - A parameter whose type is an enum of the crate with no generics, or a reference to one, adds its variant to the name of the call, so `door.on(Event::Push)` is `on(Push)` in the tables and the diagram.
  - A call from one recorded `&mut self` method to another of the same machine is part of the outer call. A recorded call with `self` inside another one on the same thread is part of the outer call.

The output lists the pub methods that could change the state and have no hook under Not recorded, with the reason: an `async fn` (async), a `const fn` (const), a receiver with a type such as `self: Box<Self>` (self: Type), a method with `&mut self` whose return type holds a `&` or a lifetime (returns a borrow) or `impl` (returns impl Trait), and a method whose code does not compile with the hook (does not build with the hook), such as one that returns a type that borrows `self` with an elided lifetime. Methods with `&self`, the `state` method, methods that are not pub and items that a macro makes are not recorded and not listed. A machine that no constructor makes, such as an enum literal or the value of a derived `Default`, has no first state.

The hook goes into a copy of the workspace in the temporary directory of the run, as the value test does (see Rust crates), so the crate needs no dependency on docgen and your files do not change. The body of each recorded method runs in a closure with the return type of the method, so `return` and `?` work as before, and the hook reads the state before and after the closure. The hook is in every build of the copy, not only with `cfg(test)`. It allows the lints that its code can trigger, such as `unused` and `unused_qualifications`, so a `#![deny(warnings)]` stays, but a lint that the crate sets with `#![forbid]` makes each recorded method not build with the hook. docgen runs `cargo test --locked -p <crate>` once for each crate and set of features, with the machines of all its markers, so the unit tests, the integration tests and the doctests of the crate all count. `run=` is a test filter of cargo test, which runs each test whose name holds the text, such as `run=lamp` for `tests::lamp`. It applies only when each `docgen:states` marker of the crate with the same features has one, and then each filter runs. A `run=` that starts with `-` is an error.

The diagram goes in `states/<crate>-<path>.svg` next to the page, in lower case, with each run of characters other than letters and digits as `-`, such as `states/states-lamp-lamp.svg` for `crate=states-lamp type=Lamp`. The rules of `diagram=` are those of Go.

These are errors of the Rust form:

  - `type=` that names no item, an item that is not an enum or a struct, or a type with generics.
  - A struct whose state is not known, as above, and an enum of the state with no variants.
  - A type with no pub method to record and none under Not recorded.
  - Tests that do not build with the hook, other than in a recorded method, with the errors of rustc.
  - A crate that forbids a lint that the hook allows, such as `#![forbid(missing_docs)]`, with the attribute and the lint. A forbid admits no allow, so no method with the hook builds; `#![deny(...)]` keeps the lint an error and admits it.
  - A test that fails, with the failures that the test binary prints.
  - Tests that record no transition of a machine with no method under Not recorded.

The Rust form differs from the Go form in these ways. The state is always the variant of an enum: Go reads a State method with an integer type, a state type that one method returns, or the result of methods that return a bool. A first state is a state that a constructor returns, where Go marks the state of a machine at the first call that the hook records on it. Rust has no `type=` for a state type, and the names of calls hold the variants of enum parameters.
<!-- /docgen:doc -->

#### A state machine

<!-- site:editor -->
<!-- docgen:code file=internal/cli/testdata/repo/v2/machine/machine.go symbol=LampState -->
```go
type LampState uint8
```
<!-- /docgen:code -->
<!-- docgen:code file=internal/cli/testdata/repo/v2/machine/machine.go symbol=LampOff -->
```go
const (
	// LampOff gives no light.
	LampOff LampState = iota
	// LampOn gives light.
	LampOn
	// LampBroken never gives light again.
	LampBroken
)
```
<!-- /docgen:code -->
<!-- docgen:code file=internal/cli/testdata/repo/v2/machine/machine.go symbol=Lamp.State -->
```go
func (l *Lamp) State() LampState { return l.state }
```
<!-- /docgen:code -->
<!-- docgen:code file=internal/cli/testdata/repo/v2/machine/machine.go symbol=Lamp.Start -->
```go
func (l *Lamp) Start(now time.Duration) error {
	if l.state == LampOn {
		return ErrBusy
	}
	if l.state == LampOff {
		l.state, l.until = LampOn, now+l.wait
	}
	return nil
}
```
<!-- /docgen:code -->
<!-- docgen:code file=internal/cli/testdata/repo/v2/machine/machine.go symbol=Lamp.Stop -->
```go
func (l *Lamp) Stop() error {
	if l.state != LampOn {
		return ErrOff
	}
	l.off()
	return nil
}
```
<!-- /docgen:code -->
<!-- docgen:code file=internal/cli/testdata/repo/v2/machine/machine.go symbol=Lamp.Tick -->
```go
func (l *Lamp) Tick(now time.Duration) {
	if l.state == LampOn && now >= l.until {
		l.off()
	}
}
```
<!-- /docgen:code -->
<!-- docgen:code file=internal/cli/testdata/repo/v2/machine/machine.go symbol=Lamp.Break -->
```go
func (l *Lamp) Break() { l.state = LampBroken }
```
<!-- /docgen:code -->
<!-- docgen:code file=internal/cli/testdata/repo/v2/machine/machine.go symbol=Lamp.off -->
```go
func (l *Lamp) off() { l.state = LampOff }
```
<!-- /docgen:code -->
<!-- docgen:code file=internal/cli/testdata/repo/v2/machine/machine_test.go symbol=TestLamp -->
```go
func TestLamp(t *testing.T) {
	l, err := NewLamp(Config{})
	if err != nil {
		t.Fatal(err)
	}
	_ = l.Start(0)
	l.Tick(DefaultWait)
	_ = l.Start(0)
	_ = l.Stop()
	l.Break()
}
```
<!-- /docgen:code -->
<!-- docgen:docgen root=internal/cli/testdata/repo module=v2 page=docs/machines/guide-states.md -->
#### Before

```
# The lamp

<!-- docgen:states pkg=machine type=Lamp -->
<!-- /docgen:states -->
```

#### After

```
# The lamp

<!-- docgen:states pkg=machine type=Lamp -->
States of [`Lamp`](https://pkg.go.dev/github.com/USA-RedDragon/docgen/v2/machine#Lamp), from [`Lamp.State`](https://pkg.go.dev/github.com/USA-RedDragon/docgen/v2/machine#Lamp.State):

| State | Meaning | Seen in tests |
|---|---|---|
| [`LampOff`](https://pkg.go.dev/github.com/USA-RedDragon/docgen/v2/machine#LampOff) | LampOff gives no light. | yes, first state |
| [`LampOn`](https://pkg.go.dev/github.com/USA-RedDragon/docgen/v2/machine#LampOn) | LampOn gives light. | yes |
| [`LampBroken`](https://pkg.go.dev/github.com/USA-RedDragon/docgen/v2/machine#LampBroken) | LampBroken never gives light again. | yes |

Transitions seen in tests:

| From | Method call | To |
|---|---|---|
| LampOff | `Start` | LampOn |
| LampOff | `Break` | LampBroken |
| LampOn | `Stop`, `Tick` | LampOff |

![Transitions of Lamp](states/machine-lamp.svg)
<!-- /docgen:states -->
```
<!-- /docgen:docgen -->
<!-- /site:editor -->

#### Some of the tests and another diagram path, with `run=` and `diagram=`

<!-- site:editor -->
<!-- docgen:code file=internal/cli/testdata/repo/v2/door/door.go -->
```go
// Package door is a state machine with its own tests, for the run= argument.
package door

// State is the state of a Door.
type State uint8

// The State values.
const (
	// StateClosed is shut.
	StateClosed State = iota
	// StateOpen is open.
	StateOpen
)

// Door opens and closes.
type Door struct{ state State }

// State returns the state of the Door.
func (d *Door) State() State { return d.state }

// Open opens the Door.
func (d *Door) Open() { d.state = StateOpen }

// Close closes the Door.
func (d *Door) Close() { d.state = StateClosed }
```
<!-- /docgen:code -->
<!-- docgen:code file=internal/cli/testdata/repo/v2/door/door_test.go symbol=TestOpen -->
```go
func TestOpen(t *testing.T) {
	var d Door
	d.Open()
}
```
<!-- /docgen:code -->
<!-- docgen:docgen root=internal/cli/testdata/repo module=v2 page=docs/machines/guide-states-run.md -->
#### Before

```
<!-- docgen:states pkg=door type=Door run=TestOpen diagram=states/door-open.svg -->
<!-- /docgen:states -->
```

#### After

```
<!-- docgen:states pkg=door type=Door run=TestOpen diagram=states/door-open.svg -->
States of [`Door`](https://pkg.go.dev/github.com/USA-RedDragon/docgen/v2/door#Door), from [`Door.State`](https://pkg.go.dev/github.com/USA-RedDragon/docgen/v2/door#Door.State):

| State | Meaning | Seen in tests |
|---|---|---|
| [`StateClosed`](https://pkg.go.dev/github.com/USA-RedDragon/docgen/v2/door#StateClosed) | StateClosed is shut. | yes, first state |
| [`StateOpen`](https://pkg.go.dev/github.com/USA-RedDragon/docgen/v2/door#StateOpen) | StateOpen is open. | yes |

Transitions seen in tests:

| From | Method call | To |
|---|---|---|
| StateClosed | `Open` | StateOpen |

![Transitions of Door](states/door-open.svg)
<!-- /docgen:states -->
```
<!-- /docgen:docgen -->
<!-- /site:editor -->

### docgen:doc

<!-- docgen:doc pkg=. section="docgen:doc" -->
A doc comment as Markdown: the package doc, or the doc of a type, func, method (`Type.Method`), const or var, or one `# heading` section of it.

	docgen:doc pkg=<path> [name=<Name>] [section=<heading>]

A `pkg=` that names a directory outside the module, such as a tool with a `go.mod` of its own, is the package doc of that directory. Its path is relative to the repository root. A name with no doc comment and a `section=` with no text are errors. Text in backticks in a doc comment is a code span.

For a Rust crate:

	docgen:doc crate=<package> [name=<path>] [section=<heading>]

With no `name=`, it is the crate doc: the `//!` comments of the crate root. `name=` is an item or a module (see Rust crates). The doc of a module is the `///` above its `mod` line and then the `//!` at the top of its file. `#[doc = "..."]` and `#[doc = include_str!("file")]` count. On the page:

  - A heading moves down two levels, so `#` is `###` and `##` is `####`.
  - A code block with no language, or with only rustdoc words such as `no_run`, is a `rust` block, with no hidden `# ` lines.
  - An intra-doc link, such as `[Config]` or `[text](crate::Config)`, is a link to docs.rs when the crate has docs there and the path leads to a public item (see Rust links), and is its text otherwise. A reference definition of one is removed. A reference definition outside the section counts.

`section=` is the text below the heading with that text, with no backticks, up to the next heading of the same or a higher level. A name with no doc, no such heading and a heading with no text are errors, as for Go.

Go links the names of a doc comment to pkg.go.dev with no setting to turn that off, and Rust links to docs.rs the same way.
<!-- /docgen:doc -->

#### The doc comment of one declaration, with `name=`

<!-- site:editor -->
<!-- docgen:code file=internal/cli/testdata/repo/v2/machine/machine.go symbol=Lamp.Start raw=true -->
```go
// Start turns the Lamp on. It returns ErrBusy while the Lamp is on.
func (l *Lamp) Start(now time.Duration) error {
	if l.state == LampOn {
		return ErrBusy
	}
	if l.state == LampOff {
		l.state, l.until = LampOn, now+l.wait
	}
	return nil
}
```
<!-- /docgen:code -->
<!-- docgen:docgen root=internal/cli/testdata/repo module=v2 page=docs/machines/guide-doc.md -->
#### Before

```
# Start

<!-- docgen:doc pkg=machine name=Lamp.Start -->
<!-- /docgen:doc -->
```

#### After

```
# Start

<!-- docgen:doc pkg=machine name=Lamp.Start -->
Start turns the Lamp on. It returns ErrBusy while the Lamp is on.
<!-- /docgen:doc -->
```
<!-- /docgen:docgen -->
<!-- /site:editor -->

#### A package doc, or one section of it, with `section=`

<!-- site:editor -->
<!-- docgen:code file=internal/cli/testdata/repo/v2/doc.go -->
```go
// Package docgen is the fixture root package of the docgen tests.
package docgen

// ConfigError is the error of a constructor for a field with a value out
// of range.
type ConfigError struct{ Field string }

func (e *ConfigError) Error() string { return "bad " + e.Field }
```
<!-- /docgen:code -->
<!-- docgen:code file=internal/cli/testdata/repo/v2/machine/machine.go symbol=package raw=true -->
```go
// Package machine is a small state machine for the docgen tests.
//
// # Rates
//
// A Lamp blinks at these rates:
//   - slow, every [DefaultWait]
//   - fast, every period
//
// # Other
//
// Not copied.
package machine
```
<!-- /docgen:code -->
<!-- docgen:docgen root=internal/cli/testdata/repo module=v2 page=docs/machines/guide-doc-package.md -->
#### Before

```
<!-- docgen:doc pkg=. -->
<!-- /docgen:doc -->

<!-- docgen:doc pkg=machine section=Rates -->
<!-- /docgen:doc -->
```

#### After

```
<!-- docgen:doc pkg=. -->
Package docgen is the fixture root package of the docgen tests.
<!-- /docgen:doc -->

<!-- docgen:doc pkg=machine section=Rates -->
A Lamp blinks at these rates:

  - slow, every [DefaultWait](https://pkg.go.dev/github.com/USA-RedDragon/docgen/v2/machine#DefaultWait)
  - fast, every period
<!-- /docgen:doc -->
```
<!-- /docgen:docgen -->
<!-- /site:editor -->

### docgen:gomod

<!-- docgen:doc pkg=. section="docgen:gomod" -->
A line of the `go.mod` of the module, or of the `go.mod` at `file=`. It goes inline in a sentence.

	docgen:gomod field=<go|toolchain|module> [file=<repo path>] [code=false]

With `code=false`, the value has no backticks, as for `docgen:const`.
<!-- /docgen:doc -->

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

#### A Go version with no backticks, with `code=false`

<!-- site:editor -->
<!-- docgen:code file=internal/cli/testdata/repo/v2/go.mod -->
```
module github.com/USA-RedDragon/docgen/v2

go 1.24.0
```
<!-- /docgen:code -->
<!-- docgen:docgen root=internal/cli/testdata/repo module=v2 page=docs/machines/guide-gomod-code.md -->
#### Before

```
This module needs Go <!-- docgen:gomod field=go code=false --><!-- /docgen:gomod --> or later.
```

#### After

```
This module needs Go <!-- docgen:gomod field=go code=false -->1.24.0<!-- /docgen:gomod --> or later.
```
<!-- /docgen:docgen -->
<!-- /site:editor -->

### docgen:cargo

<!-- docgen:doc pkg=. section="docgen:cargo" -->
A value of a Cargo manifest of a Rust workspace, or the table of the features of a crate. A value goes inline in a sentence.

	docgen:cargo field=<key> [crate=<package>] [file=<repo path>] [code=false]
	docgen:cargo field=features crate=<package> [features=<list>] [allfeatures=<bool>] [nodefaultfeatures=<bool>]

`field=` is a dotted key path in the TOML file, such as `package.rust-version`, `workspace.package.version` or `dependencies.clap.version`. The file is the `Cargo.toml` in the module directory, the `Cargo.toml` of the Cargo package that `crate=` names (see Rust crates), or the TOML file at `file=`, such as `rust-toolchain.toml` with `field=toolchain.channel`. `crate=` and `file=` together are an error. Only `crate=` runs cargo.

Workspace inheritance applies, with the `Cargo.toml` of the workspace as the source:

  - A `package` key set to `workspace = true`, such as `version.workspace = true`, has the value of the same key in `[workspace.package]`.
  - A dependency set to `workspace = true` has the keys of the dependency in `[workspace.dependencies]`, so `dependencies.clap.version` works. `features` of both are one list.

The value is a string, number or boolean as it is, and an array of them is its items joined by a comma and a space. These are errors: a table, an array of tables or a date as the value, a file that is missing or is not valid TOML, a key that is not in the file (the error lists the keys that are there), a key below a value that is not a table, and an inherited value that `[workspace.package]` or `[workspace.dependencies]` does not have. With `code=false`, the value has no backticks, as for `docgen:const`.

With `field=features`, the marker is a table of the features of the package, in name order, with the column Default for the ones that the `default` feature enables and the column Enables for the items of each, such as other features and `dep:` items. A package with no features is an error. `field=features` needs `crate=`, and has no `file=` and no `code=`. When the `features` settings or the `features=`, `allfeatures=` or `nodefaultfeatures=` of the marker set anything, the table has the column Enabled between Default and Enables, with `yes` for each feature that the cargo commands of the marker enable: the listed ones, the default feature unless `nodefaultfeatures=true`, every feature with `allfeatures=true`, and the features of the same package that each of those enables. See Rust crates for the arguments.
<!-- /docgen:doc -->

<!-- site:editor -->
<!-- docgen:docgen root=internal/cli/testdata/rust page=docs/guide-cargo.md -->
#### Before

```
- Version: <!-- docgen:cargo field=workspace.package.version --><!-- /docgen:cargo -->
- Minimum Rust: <!-- docgen:cargo field=package.rust-version crate=cargo-b-core --><!-- /docgen:cargo -->
- Version of the core crate: <!-- docgen:cargo field=package.version crate=cargo-b-core code=false --><!-- /docgen:cargo -->
- Channel: <!-- docgen:cargo field=toolchain.channel file=rust-toolchain.toml --><!-- /docgen:cargo -->
- Components: <!-- docgen:cargo field=toolchain.components file=rust-toolchain.toml --><!-- /docgen:cargo -->
```

#### After

```
- Version: <!-- docgen:cargo field=workspace.package.version -->`0.3.0`<!-- /docgen:cargo -->
- Minimum Rust: <!-- docgen:cargo field=package.rust-version crate=cargo-b-core -->`1.85`<!-- /docgen:cargo -->
- Version of the core crate: <!-- docgen:cargo field=package.version crate=cargo-b-core code=false -->0.3.0<!-- /docgen:cargo -->
- Channel: <!-- docgen:cargo field=toolchain.channel file=rust-toolchain.toml -->`stable`<!-- /docgen:cargo -->
- Components: <!-- docgen:cargo field=toolchain.components file=rust-toolchain.toml -->`rustfmt, clippy`<!-- /docgen:cargo -->
```
<!-- /docgen:docgen -->
<!-- /site:editor -->

#### The features of a crate, with `field=features`

<!-- site:editor -->
<!-- docgen:docgen root=internal/cli/testdata/rust page=docs/guide-cargo-features.md -->
#### Before

```
<!-- docgen:cargo field=features crate=cargo-b-core -->
<!-- /docgen:cargo -->
```

#### After

```
<!-- docgen:cargo field=features crate=cargo-b-core -->
| Feature | Default | Enables |
|---|---|---|
| `checked` |  | `dep:limits`, `strict` |
| `fast` | yes |  |
| `strict` |  | `fast` |
<!-- /docgen:cargo -->
```
<!-- /docgen:docgen -->
<!-- /site:editor -->

### docgen:crates

<!-- docgen:doc pkg=. section="docgen:crates" -->
A table of the crates of a Cargo workspace.

	docgen:crates [kind=<lib|proc-macro|bin>] [deps=<normal|dev|build|all>] [match=<glob>]
	              [columns=<metadata.key>[,<metadata.key>...]]

Each row is a crate: its name, its path relative to the repository root, its kind (`lib`, `proc-macro` or `bin`, and `lib, bin` for a crate that has both), its version, the crates of the same workspace that it depends on, as `cargo metadata` reports them, and its description. Rows are in name order. The workspace is the one of the Rust markers (see Rust crates).

The description is the `description` of the `Cargo.toml` of the crate, with `description.workspace = true` taking `workspace.package.description`. A crate with none has the first line of the inner doc comment (`//!`) at the top of its crate root, which is the library, or else the first binary: the first non-empty `//!` line, with the `#` of a Markdown heading removed and no cut at a sentence end. Blank lines, line comments and `#![...]` attributes may come before that comment. A block doc comment (`/*! */`), a first doc line that is a code fence, an unreadable crate root and a crate with no comment give an empty cell. Reading them needs no cargo and no Rust helper, so the column is the same with no cargo.

  - `kind=` lists only the crates with a target of that kind.
  - `deps=` is the kind of dependency in the Depends on column: `normal`, the default, `dev`, `build`, or `all`. With `all`, a dev or build dependency has its kind in parentheses.
  - `match=` is a glob, as in Go's `path.Match`, for the names of the crates.
  - `columns=` adds a column after Description for each key of the table `[package.metadata]` of the `Cargo.toml` of a crate, in the order given. Each key is a dotted path written as `metadata.<key>[.<key>...]`, so `metadata.docs.tier` is `package.metadata.docs.tier`. The full `package.metadata.` form is not accepted. A key is bare TOML: letters, digits, `-` and `_`, so a quoted key and a key with a dot in it are not possible. The header of the column is the last key, `tier` for that one. A crate with no such key has an empty cell. A string, boolean, number or date has its text, a number as TOML writes it, and an array of those has its items joined by `, `. A table, or an array with a table or an array in it, is an error that names the crate and the key, and so is a bad key. Workspace inheritance does not apply to metadata: `[workspace.metadata]` is not read, and `owner.workspace = true` is a table, so an error. It works with no cargo too, since docgen reads the key from the `Cargo.toml` of the crate, as `cargo metadata` does.

A value that is not one of the lists, a glob that is not valid, a crate outside the repository, no crate that matches and a missing `Cargo.toml` in the module directory are errors.
<!-- /docgen:doc -->

<!-- site:editor -->
<!-- docgen:docgen root=internal/cli/testdata/rust page=docs/guide-crates.md -->
#### Before

```
<!-- docgen:crates -->
<!-- /docgen:crates -->
```

#### After

```
<!-- docgen:crates -->
| Crate | Path | Kind | Version | Depends on | Description |
|---|---|---|---|---|---|
| `cargo-b-cli` | `crates/cargo-b-cli` | bin | 0.3.0 | `cargo-b-core` |  |
| `cargo-b-core` | `crates/cargo-b-core` | lib | 0.3.0 | `limits` | Core of the cargo marker fixture |
| `cfg-app` | `crates/cfg-app` | lib | 0.1.0 |  | Config structs for the docgen:config tests. |
| `core-a` | `crates/core-a` | lib | 0.1.0 |  | The core of the fixture app. |
| `core-a-bin` | `crates/core-a-bin` | bin | 0.1.0 |  | A fixture workspace |
| `err-store` | `crates/err-store` | lib | 0.4.2 |  | A store that returns errors, for the Returned by column and docs.rs links. |
| `feat-gate` | `crates/feat-gate` | lib | 0.1.0 |  | Limits that depend on the Cargo features of the crate. |
| `limits` | `crates/limits` | lib | 0.1.0 |  | Limits of the fixture. |
| `run-c` | `crates/run-c` | lib, bin | 0.1.0 |  | Helpers to run and to test. |
| `run-c-broken` | `crates/run-c-broken` | lib | 0.1.0 |  | A crate whose doctest fails. |
| `states-door` | `crates/states-door` | lib | 0.1.0 |  | A door whose state is the variant of an enum, for docgen:states. |
| `states-lamp` | `crates/states-lamp` | lib | 0.1.0 |  | A lamp with a timer, for docgen:states. |
| `states-odd` | `crates/states-odd` | lib | 0.1.0 |  | Machines that docgen:states records in part or not at all. |
| `tool` | `crates/tool` | bin | 0.2.0 | `limits` |  |
<!-- /docgen:crates -->
```
<!-- /docgen:docgen -->
<!-- /site:editor -->

#### Only some crates, with `kind=` and `deps=`

<!-- site:editor -->
<!-- docgen:docgen root=internal/cli/testdata/rust page=docs/guide-crates-filter.md -->
#### Before

```
<!-- docgen:crates kind=bin deps=all -->
<!-- /docgen:crates -->
```

#### After

```
<!-- docgen:crates kind=bin deps=all -->
| Crate | Path | Kind | Version | Depends on | Description |
|---|---|---|---|---|---|
| `cargo-b-cli` | `crates/cargo-b-cli` | bin | 0.3.0 | `cargo-b-core`, `limits` (dev) |  |
| `core-a-bin` | `crates/core-a-bin` | bin | 0.1.0 |  | A fixture workspace |
| `run-c` | `crates/run-c` | lib, bin | 0.1.0 |  | Helpers to run and to test. |
| `tool` | `crates/tool` | bin | 0.2.0 | `limits` |  |
<!-- /docgen:crates -->
```
<!-- /docgen:docgen -->
<!-- /site:editor -->

### docgen:usage

<!-- docgen:doc pkg=. section="docgen:usage" -->
The usage of a command.

	docgen:usage cmd=<dir> [args="<args>"]
	docgen:usage crate=<package> [bin=<name>] [args="<args>"] [features=<list>] [allfeatures=<bool>] [nodefaultfeatures=<bool>]

docgen builds the command in that directory, relative to the repository root, and runs it with `-h`. With `args=`, the arguments are that value, split on spaces, such as `args="help config"` or `args=--help`. In the value, `\"` starts and ends an argument with spaces: `args="--name \"a b\""`.

With `crate=`, the command is a binary of a Cargo package (see Rust crates), not a Go command, and `cmd=` is an error with it. `bin=` is the name of the binary. It can be left out when the package has one binary, and a name that the package does not have is an error that lists the ones it has. docgen builds the binary with `cargo build --locked --bin`. The rules for the output are the ones below, so a binary that uses clap prints its usage with `-h`, and a subcommand with `args="help <name>"` or `args="<name> --help"`. `bin=` with no `crate=` is an error.

A build failure, a panic, an exit status other than 0, 1 or 2, or output with no line that starts with usage, in any letter case, and no flag line, is an error. This also applies to arguments that do not ask for help, so a command such as `version` must print a usage line or a flag line, such as `usage: tool version`.
<!-- /docgen:doc -->

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

#### The usage of a help command, with `args=`

<!-- site:editor -->
<!-- docgen:code file=internal/cli/testdata/repo/cmd/lamp/main.go symbol=main -->
```go
func main() {
	config := flag.NewFlagSet("lamp config", flag.ExitOnError)
	config.Duration("wait", 0, "the time before the lamp turns on")
	if len(os.Args) == 3 && os.Args[1] == "help" && os.Args[2] == "config" {
		config.SetOutput(os.Stdout)
		config.Usage()
		return
	}
	fmt.Println("usage: lamp <config|help> [arguments]")
}
```
<!-- /docgen:code -->
<!-- docgen:docgen root=internal/cli/testdata/repo module=v2 page=docs/machines/guide-usage-args.md -->
#### Before

```
# The config command

<!-- docgen:usage cmd=cmd/lamp args="help config" -->
<!-- /docgen:usage -->
```

#### After

````
# The config command

<!-- docgen:usage cmd=cmd/lamp args="help config" -->
```text
Usage of lamp config:
  -wait duration
    	the time before the lamp turns on
```
<!-- /docgen:usage -->
````
<!-- /docgen:docgen -->
<!-- /site:editor -->

#### A Rust binary, with `crate=`

<!-- site:editor -->
<!-- docgen:docgen root=internal/cli/testdata/rust page=docs/guide-usage.md -->
#### Before

```
<!-- docgen:usage crate=cargo-b-cli bin=demo -->
<!-- /docgen:usage -->
```

#### After

````
<!-- docgen:usage crate=cargo-b-cli bin=demo -->
```text
Inspect the demo workspace

Usage: demo [OPTIONS] <COMMAND>

Commands:
  show   Print the jobs
  label  Print the jobs with a prefix
  help   Print this message or the help of the given subcommand(s)

Options:
  -j, --jobs <JOBS>  How many jobs to run [default: 1]
  -h, --help         Print help
  -V, --version      Print version
```
<!-- /docgen:usage -->
````
<!-- /docgen:docgen -->
<!-- /site:editor -->

#### A subcommand of a Rust binary

<!-- site:editor -->
<!-- docgen:docgen root=internal/cli/testdata/rust page=docs/guide-usage-sub.md -->
#### Before

```
<!-- docgen:usage crate=cargo-b-cli bin=demo args="help label" -->
<!-- /docgen:usage -->
```

#### After

````
<!-- docgen:usage crate=cargo-b-cli bin=demo args="help label" -->
```text
Print the jobs with a prefix

Usage: demo label [OPTIONS]

Options:
      --prefix <PREFIX>  Text to put before the jobs [default: jobs]
  -h, --help             Print help
```
<!-- /docgen:usage -->
````
<!-- /docgen:docgen -->
<!-- /site:editor -->

### docgen:var

<!-- docgen:doc pkg=. section="docgen:var" -->
The value of a variable of the `vars` setting, as a code span. It goes inline in a sentence.

	docgen:var name=<key> [print=<go|raw|config|verb>] [code=false]

A key that is not in `vars` is an error. The value is a string:

  - `print=go`, the default, `print=raw` and `print=config` are the value as it is.
  - `print=%q`, or any other fmt verb, is the string with that verb: `"v1.2.3"`. A verb that a string cannot use, such as `%d`, is an error.
  - `print=name` is an error, since a value of `vars` is not a constant.
  - With `code=false`, the value has no backticks, as for `docgen:const`.
<!-- /docgen:doc -->

The boxes of `docgen:var` and `docgen:text` use this `.docgen.yaml`:

<!-- docgen:code file=internal/cli/testdata/repo/.docgen.yaml -->
```yaml
vars:
  version: v1.2.3
```
<!-- /docgen:code -->

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

#### A quoted value, with `print=%q`

<!-- site:editor -->
<!-- docgen:code file=internal/cli/testdata/repo/.docgen.yaml -->
```yaml
vars:
  version: v1.2.3
```
<!-- /docgen:code -->
<!-- docgen:docgen root=internal/cli/testdata/repo module=v2 page=docs/machines/guide-var-print.md -->
#### Before

```
The quoted version is <!-- docgen:var name=version print=%q --><!-- /docgen:var -->.
```

#### After

```
The quoted version is <!-- docgen:var name=version print=%q -->`"v1.2.3"`<!-- /docgen:var -->.
```
<!-- /docgen:docgen -->
<!-- /site:editor -->

### docgen:text

<!-- docgen:doc pkg=. section="docgen:text" -->
Text from a template, in which `vars` values replace the keys. With no `code=`, the text goes inline in a sentence. With `code=`, it is a code block in that language, and `code="text"` is a block with no highlighting, so a value can go in a code block, where markers are text.

	docgen:text text="<template>" [code=<language>]

Here `code=` is a language. In `docgen:const`, `docgen:var`, `docgen:gomod` and `docgen:cargo`, `code=false` is the one value, for a value with no backticks.

The template has these forms:

  - `{key}` is the value of the variable. A key that is not in `vars` is an error.
  - `{{` is a literal `{`.
  - `\n` is a line ending, since an argument is one line.
  - `\\` is a backslash, so `\\n` is a backslash and an `n`.
<!-- /docgen:doc -->

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

### docgen:docgen

<!-- docgen:doc pkg=. section="docgen:docgen" -->
A page of a sample directory, before and after an update.

	docgen:docgen root=<dir> page=<path in root> [module=<dir>]

docgen copies `root` to a temporary directory, runs update on the copy, and writes the page before and after it under the headings Before and After. `page` and `module` are relative to `root`. The update reads the `.docgen.yaml` of `root` for `docs`, `print` and `vars`, and uses the `jobs`, `tags` and `depth` of the outer run. The `features` settings of the outer run do not apply to a nested run, since the crates of its workspace are not those of the outer run, but the markers of its pages can set `features=`. The update always fills `page`, also outside `docs`. `GOWORK` applies only when its file is in `root`. Markers with one `root` and `module` use one copy and one update. An error of the update is an error of the marker, and a region that the update excerpts counts as used. Pages under `root` are not part of the outer run or its checks, unless `root` holds the page of the marker.
<!-- /docgen:doc -->

Each box of this page comes from a marker in this form. At the default depth this marker in a sample page is an error, so it has no box:

```text
<!-- docgen:docgen root=internal/cli/testdata/repo module=v2 page=docs/machines/guide-const.md -->
<!-- /docgen:docgen -->
```

## Rust crates

<!-- docgen:doc pkg=. section="Rust crates" -->
A marker with `crate=` is for Rust: `docgen:const`, `docgen:doc`, `docgen:errors`, `docgen:config`, `docgen:configulator`, `docgen:states`, `docgen:output`, `docgen:usage` and `docgen:cargo`. `crate=` is the name of a package of the Cargo workspace whose `Cargo.toml` is in the module directory, as `cargo metadata` reports it, and `docgen:crates` reads the same workspace. A name that is not a package of the workspace is an error that lists the packages, and a marker with both `crate=` and `pkg=` is an error.

These markers run `cargo` from the `PATH` and need stable Rust. With no `cargo` on the `PATH`, docgen reads the workspace from its `Cargo.toml` files: the members, with the globs of `workspace.members` and `workspace.exclude`, and of each package its fields with workspace inheritance, its targets as a manifest lists them or as cargo finds them in `src/lib.rs`, `src/main.rs`, `src/bin`, `examples`, `tests` and `benches`, its features, and its path dependencies on other members. The markers that read only the source then work, given a helper from `DOCGEN_RUST_HELPER` (see below): `docgen:doc`, `docgen:errors`, `docgen:const` with `print=raw`, `docgen:crates` and `docgen:cargo`, and `docgen:code symbol=` in a Rust file. A marker that needs a build, the value test or a doctest is an error that says it needs cargo: `docgen:const` with another `print=`, `docgen:config`, `docgen:configulator`, `docgen:output` and `docgen:usage` with `crate=`. Each cargo command passes `--locked`, so the run never changes `Cargo.lock`, and a `Cargo.lock` that is out of date is an error with the cargo command that updates it. Cargo downloads crates at the versions of `Cargo.lock` and builds into the temporary directory of the run, never into your `target` directory. The markers of one run read the workspace once and build each binary and example once. Each cargo command, and each program that a cargo build made, counts toward the limit on commands at once, as a go command does. Each one stops after 10 minutes, the limit of a run of a Go example, since a cold build of a crate with many dependencies can take longer than the 5 minutes of a Go build. A `jobs` value other than 0 is the `CARGO_BUILD_JOBS` of each command.

`docgen:const`, `docgen:doc`, `docgen:errors`, `docgen:config` and `docgen:configulator` read the library of the package, or else its binary with the name of the package, or else its one binary. `name=` and `type=` are paths from the crate root, such as `limits::MAX_JOBS`, with or without `crate::`. A path can go through a `use` of the crate, such as a `pub use` at the root. An associated item of an inherent impl is `Type::NAME`. A path that names nothing is an error that lists the names of the nearest module.

docgen reads the module tree with a helper program that uses syn. Your cargo builds it once, with `cargo build --release --locked`, into `rust-helper` below docgen in the user cache directory, for each version of the helper and of rustc. Two runs at the same time each build to a new directory and rename the result, so neither reads a part of the other. The helper needs Rust 1.71 or later, and an older rustc is an error. A failed build is an error with the output of cargo. The tree follows each `mod` line: inline modules, `foo.rs` and `foo/mod.rs` files and `#[path]`. A module with a `#[cfg]` is read as any other. A file that does not parse, and a module with no file and no `#[cfg]`, are errors. What the helper skips is a warning on standard error that names the crate and the file, such as a module with a `#[cfg]` and no file, a `#[doc = include_str!(...)]` whose file is missing, or a `#[doc]` value that is not a string.

The environment variable `DOCGEN_RUST_HELPER` names a helper program that was built before the run, relative to the working directory, such as a build of `internal/rustsrc/helper` of the same docgen version for another system. docgen then runs it and checks no rustc and builds nothing, so the markers that read only the source need no Rust toolchain. A path that is not a regular file is an error of each marker that needs the helper. The playground sets it to its `wasm32-wasip1` build of the helper.

A value comes from the value test of the crate. docgen copies the workspace, with no `target` directory, to the temporary directory of the run, adds a `#[cfg(test)]` module with one test to each module with a value, and runs `cargo test --locked` once for each crate of the run, so private items work. The added module allows all lints, so a `#![deny(warnings)]` stays. The crate must build in test mode with its dev-dependencies. When the code of one value does not compile, a second run prints the other values.
<!-- /docgen:doc -->

## Cargo features

<!-- docgen:doc pkg=. section="Cargo features" -->
The `features`, `all-features` and `no-default-features` settings choose the Cargo features of each cargo command that builds a crate for a marker: the build of a binary or example for `docgen:output` and `docgen:usage`, `cargo test --doc` for a doctest, the value test of `docgen:const`, `docgen:config` and `docgen:configulator`, and the test run of `docgen:states`. The setting is for the workspace as a whole, so it applies to each crate that has what it names, and cargo gets the flags of the crate of the command, with `--package`:

  - A `features` entry that is a feature, such as `serde`, enables it in each crate that has a feature of that name, and in no other. An entry `crate/feature`, such as `core-a/strict`, enables it in `crate` alone. A feature of a dependency, such as `serde/derive`, is not a valid entry; the crate that depends on it has a feature that turns it on.
  - `all-features` passes `--all-features` and `no-default-features` passes `--no-default-features`, to each command.
  - An entry that no crate has as a feature, a crate that is not in the workspace, and a feature that the crate does not have, are errors that list the crates or the features. A name has letters, digits, `_`, `-`, `+` and dots. The error of a command that cargo rejects, such as a build of an example with a `required-features` that the setting does not enable, is the message of cargo.

A marker that runs cargo can set its own with `features=`, which is a list in the form of the setting and replaces the list of the setting, and `allfeatures=` and `nodefaultfeatures=`, which are `true` or `false` and replace the setting of the same name. An argument that the marker does not set keeps the setting. `docgen:const`, `docgen:config`, `docgen:configulator`, `docgen:usage` with `crate=`, `docgen:output` with `crate=`, `docgen:states` with `crate=` and `docgen:cargo field=features` take the three. `docgen:doc`, `docgen:errors` and `docgen:crates` run no cargo command that builds, so `features=` on them is an error, and a `docgen:const` with `print=raw` reads the source and ignores the features, as does a `docgen:config` with `print=raw` when the source writes each default. docgen builds each binary and example once for each set of features, and runs one value test, one doctest run and one states test for each crate and set of features, so markers with the same features share their work. A nested `docgen:docgen` run does not use the settings of the outer run; its markers can set `features=`.

What a feature does not change: the helper that reads the module tree reads each module and item regardless of its `#[cfg]`, so `docgen:doc`, `docgen:errors` and `print=raw` show an item behind a feature that is off. `cargo metadata` runs with `--no-deps` and lists all features of a crate whatever is enabled, so `docgen:crates` and the features table are the same for every setting. A value is different: it comes from the value test, which builds the crate with the features of the marker, so a const behind `#[cfg(feature = "big")]` has a value only when `big` is enabled. With it off, the value is an error: for a const in a module that the feature gates, the test printed no value for it, and for a const that the feature gates in a module that is built, the test code does not compile because rustc cannot find the item. Both errors name the `#[cfg]` and end with a hint to enable the feature with the `features` setting or `features=`. A doctest of an item that a feature gates is not run with the feature off, so it is the error that the crate has no doctest for the item.
<!-- /docgen:doc -->

## Rust links

<!-- docgen:doc pkg=. section="Rust links" -->
Go output links names to pkg.go.dev, and Rust output links them to docs.rs, in the Error column of the table of `docgen:errors` and in the intra-doc links of `docgen:doc`. Nothing is fetched: a URL is built from the manifest and the source, and no setting turns links off, as for Go. A crate has links when:

  - Its `Cargo.toml` allows a publish to crates.io: `publish` is not set, or it lists `crates-io`. `publish = false` or a list of other registries has none, and the text of a link stays text, so the docs of a private crate have no dead links. docgen does not check crates.io, so a crate that has never been published has links that do not work yet.
  - It has a library, since docs.rs builds the library.

The URL is `[https://docs.rs/](https://docs.rs/)<package>/latest/<library>/<path>`, with the package name and the name of the library target, which has `_` for `-`. The version is `latest`, as Go links go to the latest version on pkg.go.dev, so a version that is not on docs.rs yet does not give a dead link. The path is the module path and the page of the item:

  - A module is `<module>/index.html`, and the crate root is `index.html`.
  - `struct.Name.html`, `enum.Name.html`, `union.Name.html`, `trait.Name.html`, `type.Name.html`, `fn.name.html`, `constant.NAME.html` for a const, `static.NAME.html` and `macro.name.html`. A macro with `#[macro_export]` is at the crate root, wherever it is defined. Generics change nothing: `struct Config<T>` is `struct.Config.html`.
  - A variant of an enum is the page of the enum with `#variant.Name`. A pub field of a struct is `#structfield.name`, a pub method or associated fn of an inherent impl is `#method.name` and a pub associated const is `#associatedconstant.NAME`, on the page of the type.

A link needs a public item: it is `pub`, it has no `#[cfg]` of its own or of a module above it, and each module on its path is `pub`. The page is where rustdoc puts the item: the module that defines it when it is public. When it is in a private module, the page is at the shortest `pub use` from a public module, under the name that the `pub use` gives, with a glob `pub use` too. An item that is not public, whose module has a `#[cfg]`, or that no `pub use` shows, has no link, and neither has an item that is a trait item.

An intra-doc link resolves in the module of the item that the doc belongs to, and for the doc of a module in that module and then in its parent, as `///` on the `mod` line is in the parent. The path may start with `crate::`, `self::` or `super::`, or with a name in scope that a `use` brought in. A disambiguator such as `struct@`, a `()` after a fn and a `!` after a macro are removed. A path into another crate, including another package of the workspace, or `Self`, is not resolved, and the link is its text.
<!-- /docgen:doc -->

## Marker rules

<!-- docgen:doc pkg=. section="Marker rules" -->
  - Arguments that hold spaces take double quotes: `section="Build tags"`.
  - In double quotes, `\"` is a double quote, and any other backslash stays.
  - An optional argument with an empty value, such as `region=""`, is an error.
  - A `file=`, `cmd=`, `pkg=`, `root=` or `diagram=` value that leaves the repository is an error, also through a symlink. Symlinks inside the repository work, and a symlink in the `root` of `docgen:docgen` must stay in that `root`.
  - A generated file that leaves the repository through a symlink is an error, before any read or write.
  - A comment in the form of a docgen tag that is not valid, in any letter case, is an error, and so is a close tag with no open tag.
  - Marker text in a fenced code block or an inline code span is text, and so is `\<!--`. An indented code block outside a marker is not text, so a marker shown that way is a real marker.
  - Generated text outside code shows the tag comments as `&lt;!--`.
  - The marks of a region can be in a page, as `docgen:region` and `docgen:end` comments.
  - CRLF line endings in a page stay.
  - White space or > before a close tag stays, and the lines of the body get no prefix, so a block marker in a blockquote has its generated body outside the quote.
<!-- /docgen:doc -->
