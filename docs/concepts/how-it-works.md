# How docgen works

What docgen does on a run, and why the pages hold their generated text.

## Generated text lives in the page

docgen writes into the Markdown files, and you commit the result. A page is complete on GitHub, in an editor and in a pull request diff, with no build step. A change to the code shows up in the diff as a change to the docs.

The markers are HTML comments, so they do not show when the page renders. The result is plain Markdown, so any Markdown docs tool can publish it.

## One run

1. docgen gets the repository root: the nearest directory with a `.docgen.yaml` file or a `.git` entry.
2. It parses the markers of each scanned page. A marker in code is text, so you can show marker syntax in a page. See [Marker rules](../reference/markers.md#marker-rules).
3. It loads the code of each marker. A `pkg=` marker loads the Go package with its tests, with the module path from `go.mod`. A `crate=` marker reads the Cargo workspace. See [Rust pipeline](#rust-pipeline).
4. It runs what the markers need: each example program, each Example and each state recording of Go, and each value test, example, binary and doctest of Rust.
5. It computes the body of each marker and compares it with the page.
6. `update` writes each change. `check` writes nothing and reports each change.

Both compute the same marker bodies, so after `update` no marker is stale. `check` also reports what no marker uses and each broken link, and `update` does not.

## Facts come from the code

Facts come from the code and from the tests, and not from doc text:

- The default of a config field comes from the statement that assigns it.
- A transition comes from a call that a test made.
- The output of an Example comes from the comment that `go test` checks. The output of a Rust doctest comes from the code that `cargo test --doc` checks.
- A Rust constant comes from a test that prints it, so `MAX_JOBS / 4` is the number, not the expression.

A fact with no source in the code or the tests is not in the output.

## Go and Rust side by side

| | Go | Rust |
|---|---|---|
| Marker argument | `pkg=` | `crate=` |
| Unit | A package of the module | A package of the Cargo workspace |
| Reads the code with | The `go` command and its type checker | A helper that uses `syn` |
| Values from | The type checker, and `go test` for a value that needs code | `cargo test --locked` on a copy of the workspace |
| Programs | `go run` of a directory | A `cargo` build of a binary or example |
| Links | pkg.go.dev | docs.rs |
| Settings | `tags`, `jobs` | `features`, `all-features`, `no-default-features`, `jobs` |
| Needs on the `PATH` | `go` | `cargo` |

A repository can mix both. A page can hold markers for both kinds.

## Rust pipeline

A Rust marker reads the Cargo workspace of the module directory, and runs your `cargo` from the `PATH`.

This is the order of a run that has Rust markers:

1. docgen reads the workspace once, with `cargo metadata`.
2. The helper reads the module tree of a crate: each `mod`, with inline modules, `foo.rs`, `foo/mod.rs` and `#[path]`. Your cargo builds the helper once, with `cargo build --release --locked`, and docgen caches it in the user cache directory for each version of the helper and of rustc. The helper needs Rust 1.71 or later.
3. A marker that reads the source, such as `docgen:doc` or `docgen:errors`, takes its text from the helper.
4. A marker that needs a value gets it from the value test: docgen copies the workspace, with no `target` directory, adds a `#[cfg(test)]` module with one test to each module with a value, and runs `cargo test --locked` once for each crate. Private items work, and your files are not changed.
5. Binaries and examples are built once for each set of features, into the temporary directory of the run.

Every cargo command passes `--locked`, so a run never changes `Cargo.lock`. The cargo commands run with your environment, so `CARGO_HOME`, a registry mirror and `RUSTUP_TOOLCHAIN` work as they do for cargo. Choose Cargo features with the `features` settings. For the full rules, see [Rust crates](../reference/markers.md#rust-crates), [Cargo features](../reference/markers.md#cargo-features) and [Rust links](../reference/markers.md#rust-links).

## Defaults

<!-- docgen:doc pkg=. section=Defaults -->
The Default column of `docgen:config` holds the value that your code applies, not what a doc comment states. In this code, the default of `Wait` is `DefaultWait`:

	func NewLamp(c Config) *Lamp {
		wait := c.Wait
		if wait == 0 {
			wait = DefaultWait
		}
		return &Lamp{wait: wait}
	}

`wait` is a copy of `c.Wait`. The condition of the `if` is `wait == 0`, and its body has `wait = DefaultWait`. docgen finds this in the source and does not run the code to find it.
<!-- /docgen:doc -->

For the rules of the Default column, see [`docgen:config`](../reference/markers.md#docgenconfig).

## States and transitions

<!-- docgen:doc pkg=. section="States and transitions" -->
The state table lists the states of a machine. The transition table lists each change of state that your tests make. Both come from running the tests, so a transition that no test makes is not in the table. The state table marks the states that no test reached.

`type=` names one of these:

  - The machine type, when it has a State method that returns an integer type.
  - The state type, when one method of one type of the package returns it. That method can take one argument, such as `SlotState(Slot)`. docgen then reads the state for each exported constant of the argument type.
  - A machine with no State method. Its state is the result of its exported methods with no argument that return a `bool` or a `(value, bool)`, and of `Len()`, as 0 or not 0.

The states are the constants of the state type, in value order, with their doc comments. A value that the tests reached with no exported constant is listed as `value N`, with no link.

To record transitions, docgen runs `go test` with a hook at the top of each exported method of the machine type:

  - The hook reads the state when a method call starts and when it returns. When the two differ, it records both states and the method name.
  - A call from one exported method to another of the same machine is part of the outer call.
  - The hook allocates nothing, so a test that counts allocations still passes.
  - A test that fails under the hook fails the run.
  - The hook exists only in a go build overlay in the user cache directory, for that `go test` run. Your source and your builds do not change.

A new state, a new transition or a method that no longer changes the state fails check until you run update.

The diagram is an SVG file with one box per state, in value order. A transition to a later state is an arc above the boxes, and a transition to an earlier state is an arc below them. Each arc has the method names as its label.

A machine of a Rust crate has its own rules, which docgen:states lists.
<!-- /docgen:doc -->

## Limits

- A compile error in a `_test.go` file fails the markers that run the tests of that package. Other markers of the package still work.
- A program from `docgen:usage` or `exec=` that starts its own session keeps running after an interrupt, because the signal goes to the process group.
- An inline marker at the start of a line starts an HTML block in Markdown, so its backticks show as text. Put text before the marker.
- The body of a block marker gets no prefix, so a block marker in a list or a blockquote has its generated body outside the list or the quote.
- A Rust crate that does not build in test mode with its dev-dependencies fails the markers that need a value, a doctest or a state run. The markers that read only the source still work.
- A cold build of a crate with many dependencies can take minutes. Each cargo command stops after 10 minutes.
- `docgen:errors` lists only the names that resolve. A doc comment that names an error that does not exist adds no row and no error.

## Network and resources

For Go:

<!-- docgen:doc pkg=. section=Environment -->
docgen runs go with your environment, so `GOPROXY`, `GOWORK` and `GOFLAGS` work as they do for any go command. The go command downloads missing modules at the versions that `go.mod` and `go.sum` pin. Set `GOPROXY=off` to forbid the network. `jobs` adds `-p` to `GOFLAGS` and `tags` adds `-tags`, joined with the value of `go env -w GOFLAGS`.

`GOTMPDIR` is a new directory for each run, below docgen in the user cache directory, so that builds are not on a tmpfs. docgen removes it at the end of the run, also after an interrupt, so a go command that the interrupt ends leaves no build directory.
<!-- /docgen:doc -->

For Rust, cargo downloads crates at the versions of `Cargo.lock`, and `--locked` makes an out-of-date lock file an error. Builds go into the temporary directory of the run, never into your `target` directory, and the run removes it at the end. `jobs` sets `CARGO_BUILD_JOBS`. With no `cargo` on the `PATH`, the markers that read only the source still work when `DOCGEN_RUST_HELPER` names a prebuilt helper, as in the playground.
