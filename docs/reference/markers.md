# Markers

The marker kinds and their arguments. The text comes from the package doc of docgen.

A marker has this form. Kinds `const`, `gomod` and `var` go inline in a sentence. Other kinds have the body on the lines between the two comments:

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
The output of a program, a command or an Example.

	docgen:output cmd=<dir>
	docgen:output exec="<command>"
	docgen:output test=<pkg>.<ExampleName>

The arguments:

  - `cmd=` is the standard output of `go run` on the Go program in that directory, relative to the repository root, from its nearest `go.mod`.
  - `exec=` is the standard output of a shell command, run with `sh -c` in the repository root. A nonzero exit status is an error.
  - `test=` is the expected output of an Example function, as written in its `// Output:` comment. `go test` fails when the real output differs, so the page cannot hold wrong output. Give the package and the function, such as `test=machine.ExampleLamp`. `test=machine.Example` is the Example of the package itself.

The output must be the same on each run, or check fails. A program or command with no output gives an empty block, and an Example with an empty `// Output:` comment is an error.
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
  - `symbol=` in a C file is the `#define` of Name or the definition of the function Name.
  - `region=` is the lines between the line `docgen:region <name>` and the next line `docgen:end`. Each mark is the whole line apart from comment syntax. In a page, a mark in code or in a marker body is text. Other excerpts of the file leave out those two lines.
  - `lines=` is those lines of the file. For a Go file the lines must hold whole declarations or statements. When the file changes, check and update fail. The error holds the new range when the old excerpt moved, and otherwise states that the range is out of date. A range past the last line is an error.
  - With none of these, the excerpt is the whole file.
  - `raw=true`, with `symbol=`, `lines=` or the whole file, keeps the text of the file: doc comments, region marks and the output comment of an Example. `symbol=package` in a Go file is then the package clause with its doc comment. Any other value is an error.

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

#### A C function or #define, with `symbol=`

<!-- site:editor -->
<!-- docgen:code file=internal/cli/testdata/repo/native/lamp.c -->
```c
#define LAMP_MAX_TRIES 3

static int tries;

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

### docgen:usage

<!-- docgen:doc pkg=. section="docgen:usage" -->
The usage of a command.

	docgen:usage cmd=<dir> [args="<args>"]

docgen builds the command in that directory, relative to the repository root, and runs it with `-h`. With `args=`, the arguments are that value, split on spaces, such as `args="help config"` or `args=--help`. In the value, `\"` starts and ends an argument with spaces: `args="--name \"a b\""`.

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

Here `code=` is a language. In `docgen:const`, `docgen:var` and `docgen:gomod`, `code=false` is the one value, for a value with no backticks.

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

docgen copies `root` to a temporary directory, runs update on the copy, and writes the page before and after it under the headings Before and After. `page` and `module` are relative to `root`. The update reads the `.docgen.yaml` of `root` for `docs`, `print` and `vars`, and uses the `jobs`, `tags` and `depth` of the outer run. The update always fills `page`, also outside `docs`. `GOWORK` applies only when its file is in `root`. Markers with one `root` and `module` use one copy and one update. An error of the update is an error of the marker, and a region that the update excerpts counts as used. Pages under `root` are not part of the outer run or its checks, unless `root` holds the page of the marker.
<!-- /docgen:doc -->

Each box of this page comes from a marker in this form. At the default depth this marker in a sample page is an error, so it has no box:

```text
<!-- docgen:docgen root=internal/cli/testdata/repo module=v2 page=docs/machines/guide-const.md -->
<!-- /docgen:docgen -->
```

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
