# How docgen works

What docgen does on a run, and why the pages hold their generated text.

## Generated text lives in the page

docgen writes into the Markdown files, and you commit the result. A page is complete on GitHub, in an editor and in a pull request diff, with no build step. A change to the code shows up in the diff as a change to the docs.

The markers are HTML comments, so they do not show when the page renders. The result is plain Markdown, so any Markdown docs tool can publish it.

## One run

1. docgen gets the repository root and reads the module path from `go.mod`.
2. It parses the markers of each scanned page. A marker in code is text, so you can show marker syntax in a page. See [Marker rules](../reference/markers.md#marker-rules).
3. It loads the package of each marker, with its tests.
4. It runs each example program, each Example and each state recording in the markers.
5. It computes the body of each marker and compares it with the page.
6. `update` writes each change. `check` writes nothing and reports each change.

Both compute the same marker bodies, so after `update` no marker is stale. `check` also reports what no marker uses and each broken link, and `update` does not.

## Facts come from the code

Facts come from the code and from the tests, and not from doc text:

- The default of a config field comes from the statement that assigns it.
- A transition comes from a call that a test made.
- The output of an Example comes from the comment that `go test` checks.

A fact with no source in the code or the tests is not in the output. For a default with no source, the Default column holds "zero value". If the doc text states a default, the column holds "not machine-readable; see the description".

## Defaults

<!-- docgen:doc pkg=. section=Defaults -->
The Default column holds the value that your code applies, not what a doc comment states. In this code, the default of `Wait` is `DefaultWait`:

	func NewLamp(c Config) *Lamp {
		wait := c.Wait
		if wait == 0 {
			wait = DefaultWait
		}
		return &Lamp{wait: wait}
	}

docgen gets the default in two steps:

 1. It collects the copies of each field in the package: assignments such as `wait := c.Wait`, composite literal keys, and arguments of calls to functions of the package.
 2. It reads the value that an `if` or a tagless `switch` case assigns to a copy, when that `if` or case tests a copy for `0`, `""` or `nil`.

The column then holds one of these:

  - The value, with its constant value when it has one: `DefaultWait` (250ms).
  - "not machine-readable; see the description", when the code has no such test and the doc comment states a default, for example with "0 means".
  - "zero value", in all other cases.
<!-- /docgen:doc -->

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
<!-- /docgen:doc -->

## Limits

- A compile error in a `_test.go` file fails the markers that run the tests of that package. Other markers of the package still work.
- A program from `docgen:usage` or `exec=` that starts its own session keeps running after an interrupt, because the signal goes to the process group.
- An inline marker at the start of a line starts an HTML block in Markdown, so its backticks show as text. Put text before the marker.
- The body of a block marker gets no prefix, so a block marker in a list or a blockquote has its generated body outside the list or the quote.
- `docgen:errors` lists only the names that resolve. A doc comment that names an error that does not exist adds no row and no error.

## Network and resources

<!-- docgen:doc pkg=. section=Environment -->
docgen runs go with your environment, so `GOPROXY`, `GOWORK` and `GOFLAGS` work as they do for any go command. The go command downloads missing modules at the versions that `go.mod` and `go.sum` pin. Set `GOPROXY=off` to forbid the network. `jobs` adds `-p` to `GOFLAGS` and `tags` adds `-tags`, joined with the value of `go env -w GOFLAGS`.

`GOTMPDIR` is a new directory for each run, below docgen in the user cache directory, so that builds are not on a tmpfs. docgen removes it at the end of the run, also after an interrupt, so a go command that the interrupt ends leaves no build directory.
<!-- /docgen:doc -->
