// Command docgen fills the generated parts of the Markdown pages of a
// repository from the code of a Go module. Each Markdown file of the repository
// is a page. An argument or the docs setting narrows the scan to one directory.
// A scan skips testdata, vendor, node_modules, directories whose names start
// with a dot or an underscore, paths that git ignores, and anything that is not
// a regular file, such as a symlink. A directory named by the argument or the docs
// setting is scanned also when git ignores it, and a `.gitignore` below it applies.
// Each generated part has an open and a close comment:
//
//	<!-- docgen:const pkg=client name=DefaultTimeout --><!-- /docgen:const -->
//
// docgen writes the body between the two comments. Prose stays outside the
// markers and docgen does not change it.
//
// Usage:
//
//	docgen update [directory]
//	docgen check [directory]
//
// update rewrites the body of every marker. check rewrites nothing, lists each
// marker whose body differs from what update writes, each region, diagram
// and variable that no marker uses, and each link that reaches nothing, and exits 1 when there
// is one. Both exit 1 on an error, for example a marker with the name of a
// type that does not exist. update writes nothing unless every marker of every
// page succeeds. check lists stale markers and errors together. Package paths are relative to the module
// ("client", "transport/tcp/session"; "." is the root package of the module).
//
// The repository root is the nearest directory at or above the working directory
// with a `.docgen.yaml` file or a `.git` entry, or else the working directory.
//
// A `.docgen.yaml` at the repository root holds the settings, so that each run
// uses the same values:
//
//	docs: documentation
//	module: v2
//	tags: integration,linux
//	jobs: 4
//	vars:
//	  version: v1.2.3
//
// The `DOCGEN_` variables, such as `DOCGEN_DOCS`, override the file,
// and the directory argument and the flags `-module`, `-tags` and `-jobs` override
// both. The argument is relative to the working directory. `-config` names
// another file, with the extension .yaml or .yml. An unknown key is an error.
//
//   - `tags` applies to each go command, each package load and the build
//     constraints of a package outside the module. A tag has letters, digits,
//     `_` and `.`. A `-tags` in `GOFLAGS` also applies.
//   - `jobs` is the `-p` of each go command. 0 adds no `-p`. The limits are on the
//     page Command line of the documentation.
//   - `depth` limits nested `docgen:docgen` markers. The default is 1, so a marker
//     in a page of the repository works and a marker in a page of its `root` is an
//     error. The value is at most 8, and 0 makes each such marker an error. Only
//     the outermost value counts.
//   - `print` applies to a marker with no `print=`: `go`, the default, or `raw`. With
//     `raw`, a marker with no `print=` runs no code.
//   - `vars` holds the values of the `docgen:var` and `docgen:text` markers. Only
//     the file sets it. A key has letters, digits, `_` and `-`.
//
// `check -diff` prints a unified diff of each stale page and generated file to
// standard output, in path order, so `docgen check -diff | patch -p0` at the root
// applies it. All other output goes to standard error. The flag goes before the
// directory argument.
//
// The module is the directory of the Go module with the code of the markers,
// relative to the repository root. The default is the root itself. The import
// path comes from the `go.mod` of that directory, and a directory below it with
// a `go.mod` of its own is in another module.
//
// # docgen:output
//
// The output of a program, a command or an Example.
//
//	docgen:output cmd=<dir>
//	docgen:output exec="<command>"
//	docgen:output test=<pkg>.<ExampleName>
//
// The arguments:
//
//   - `cmd=` is the standard output of `go run` on the Go program in that
//     directory, relative to the repository root, from its nearest `go.mod`.
//   - `exec=` is the standard output of a shell command, run with `sh -c` in the
//     repository root. A nonzero exit status is an error.
//   - `test=` is the expected output of an Example function, as written in its
//     `// Output:` comment. `go test` fails when the real output differs, so the
//     page cannot hold wrong output. Give the package and the function, such as
//     `test=machine.ExampleLamp`. `test=machine.Example` is the Example of the
//     package itself.
//
// The output must be the same on each run, or check fails. A program or command
// with no output gives an empty block, and an Example with an empty `// Output:`
// comment is an error.
//
// # docgen:code
//
// An excerpt of a file of the repository.
//
//	docgen:code file=<repo path> symbol=<Name>
//	docgen:code file=<repo path> region=<name>
//	docgen:code file=<repo path> lines=<a>-<b>
//	docgen:code file=<repo path>
//
// The arguments:
//
//   - `symbol=` in a Go file is one func, method (`Type.Method`), type, or const or
//     var block, with no doc comment. An Example has no output comment.
//   - `symbol=` in a YAML file is the block of the key at a dotted path, such as
//     `jobs.test` of a workflow. A last segment `key[A:B]` is the items of the list
//     at `key` from the item with name `A` to the item with name `B`, and `key[A]`
//     is the one item `A`.
//   - `symbol=` in a C file is the `#define` of Name or the definition of the
//     function Name.
//   - `region=` is the lines between the line `docgen:region <name>` and the next
//     line `docgen:end`. Each mark is the whole line apart from comment syntax. In
//     a page, a mark in code or in a marker body is text. Other excerpts of the
//     file leave out those two lines.
//   - `lines=` is those lines of the file. For a Go file the lines must hold whole
//     declarations or statements. When the file changes, check and update fail.
//     The error holds the new range when the old excerpt moved, and otherwise
//     states that the range is out of date. A range past the last line is an
//     error.
//   - With none of these, the excerpt is the whole file.
//   - `raw=true`, with `symbol=`, `lines=` or the whole file, keeps the text of the
//     file: doc comments, region marks and the output comment of an Example.
//     `symbol=package` in a Go file is then the package clause with its doc comment.
//     Any other value is an error.
//
// An excerpt with no text is an error.
//
// On a scanned page, the excerpt is the text that update writes to that page. An
// excerpt of the page of its own marker is an error, and so are excerpts that do
// not settle in 8 passes.
//
// # docgen:config
//
// A table of the exported fields of a struct: name, type, default and doc text.
// The doc text can be a trailing comment.
//
//	docgen:config pkg=<path> type=<Config> [print=<go|raw|name|verb>]
//
// `print=` sets how the Default column prints each value, with the same choices
// as `docgen:const`: `go`, `raw`, `name` or a fmt verb. With `print=name`, a
// default that is not a constant, such as `3`, prints as written.
//
// A struct with no exported fields is an error.
//
// # docgen:configulator
//
// The reference table of a config type of [configulator]: each key, with its
// type, default, environment variable, flag and description.
//
//	docgen:configulator pkg=<path> type=<Config> [env=<prefix>] [envsep=<separator>] [flagsep=<separator>]
//
// # docgen:const
//
// The value of a constant, or of a package-level var with a constant initial
// value, exported or not. It goes inline in a sentence.
//
//	docgen:const pkg=<path> name=<Name> [print=<go|raw|name|verb>]
//
// `print=` sets how the value prints:
//
//   - `print=go` is the default. The value prints as `fmt.Sprint` prints it, with
//     the String, Error, Format and GoString methods of its type, so
//     `250 * time.Millisecond` is `250ms`, strings have no quotes and `1e6` is
//     `1e+06`.
//   - `print=raw` is the underlying value with no method, and runs no code: `4001`
//     for a port with a String method, `1` for an enum, and `1500000000` for a
//     `time.Duration` of 1.5s.
//   - `print=name` is the name of the one constant of the type with the value,
//     from the package of the marker and then the package of the type.
//   - `print=%d`, or any other fmt verb, is the value with that verb.
//
// An empty value prints as `""`, and line endings in a value print as spaces. A
// verb that the value cannot use is an error. An integer too large for its default
// type prints its exact value. A type with a
// print method runs in one `go test` for each package, so the tests of that
// package must compile.
//
// # docgen:errors
//
// A table of the errors named in the doc comments of the constructors and
// methods of a type. Names resolve in the package, the module packages that it
// imports and the root package of the module. Consts and vars of any error type
// count.
//
//	docgen:errors pkg=<path> type=<Type>
//
// # docgen:states
//
// The states and the transitions of a state machine, and an SVG diagram of them.
//
//	docgen:states pkg=<path> type=<Type> [run=<regexp>] [diagram=<path>]
//
// The diagram goes in `states/<pkg>-<type>.svg` next to the page. `diagram=` gives
// another path, relative to the page, which must end in `.svg` and must not be a
// scanned page. Two machines with one diagram file are an error. `type=` can be an
// alias of a type of the package.
//
// Value receiver methods, promoted methods and methods with no receiver name are
// not recorded, and the output lists them under Not recorded.
//
// # docgen:doc
//
// A doc comment as Markdown: the package doc, or the doc of a type, func, method
// (`Type.Method`), const or var, or one `# heading` section of it.
//
//	docgen:doc pkg=<path> [name=<Name>] [section=<heading>]
//
// A `pkg=` that names a directory outside the module, such as a tool with a `go.mod`
// of its own, is the package doc of that directory. Its path is relative to the
// repository root. A name with no doc comment and a `section=` with no text are
// errors. Text in backticks in a doc comment is a code span.
//
// # docgen:gomod
//
// A line of the `go.mod` of the module, or of the `go.mod` at `file=`. It goes inline
// in a sentence.
//
//	docgen:gomod field=<go|toolchain|module> [file=<repo path>]
//
// # docgen:flags
//
// The usage of a command.
//
//	docgen:flags cmd=<dir>
//
// docgen builds the command in that directory, relative to the repository root,
// and runs it with `-h`. A build failure, a panic, or output with no line that
// starts with usage, in any letter case, and no flag line, is an error.
//
// # docgen:var
//
// The value of a variable of the `vars` setting, as a code span. It goes inline in
// a sentence.
//
//	docgen:var name=<key>
//
// A key that is not in `vars` is an error.
//
// # docgen:text
//
// Text from a template, in which `vars` values replace the keys. With no `code=`,
// the text goes inline in a sentence. With `code=`, it is a code block in that
// language, and `code="text"` is a block with no highlighting, so a value can go in a code block, where markers are text.
//
//	docgen:text text="<template>" [code=<language>]
//
// The template has these forms:
//
//   - `{key}` is the value of the variable. A key that is not in `vars` is an error.
//   - `{{` is a literal `{`.
//   - `\n` is a line ending, since an argument is one line.
//   - `\\` is a backslash, so `\\n` is a backslash and an `n`.
//
// # docgen:docgen
//
// A page of a sample directory, before and after an update.
//
//	docgen:docgen root=<dir> page=<path in root> [module=<dir>]
//
// docgen copies `root` to a temporary directory, runs update on the copy, and
// writes the page before and after it under the headings Before and After. `page`
// and `module` are relative to `root`. The update reads the `.docgen.yaml` of `root`
// for `docs`, `print` and `vars`, and uses the `jobs`, `tags` and `depth` of the
// outer run. The update always fills `page`, also outside `docs`. `GOWORK` applies only when its file is in `root`. Markers with one
// `root` and `module` use one copy and one update. An error of the update is an
// error of the marker, and a region that the update excerpts counts as used.
// Pages under `root` are not part of the outer run or its checks, unless `root`
// holds the page of the marker.
//
// # Marker rules
//
//   - Arguments that hold spaces take double quotes: `section="Build tags"`.
//   - In double quotes, `\"` is a double quote, and any other backslash stays.
//   - An optional argument with an empty value, such as `region=""`, is an error.
//   - A `file=`, `cmd=`, `pkg=`, `root=` or `diagram=` value that leaves the repository is
//     an error, also through a symlink. Symlinks inside the repository work, and
//     a symlink in the `root` of `docgen:docgen` must stay in that `root`.
//   - A generated file that leaves the repository through a symlink is an error,
//     before any read or write.
//   - A comment in the form of a docgen tag that is not valid, in any letter
//     case, is an error, and so is a close tag with no open tag.
//   - Marker text in a fenced code block or an inline code span is text, and so
//     is `\<!--`. An indented code block outside a marker is not text, so a marker
//     shown that way is a real marker.
//   - Generated text outside code shows the tag comments as `&lt;!--`.
//   - The marks of a region can be in a page, as `docgen:region` and `docgen:end`
//     comments.
//   - CRLF line endings in a page stay.
//   - White space or > before a close tag stays, and the lines of the body get no
//     prefix, so a block marker in a blockquote has its generated body outside
//     the quote.
//
// # Checks of check
//
// check also reads the repository, and update does not. Each result is a line on
// standard error, sorted, and check exits 1.
//
//   - `unused: path:line: docgen:region name` is a region that no `region=` of a
//     scanned page excerpts. The scan starts at the repository root, skips the
//     directories of a page scan, the paths that git ignores other than scanned
//     pages, and the pages under a `docgen:docgen` root, and skips binary files and
//     files over 1 MiB. A path that cannot be read is an error, and the other
//     results still print.
//   - `unused: path: docgen:states diagram` is an SVG file in a `states` directory
//     next to a scanned page that no `docgen:states` marker writes.
//   - `unused: .docgen.yaml: var name` is a variable of the `vars` setting that no
//     marker of a scanned page reads. The path is the config file of the run.
//   - `broken: page:line: target` is a link, image or reference definition of a
//     scanned page that reaches nothing: a missing file, a path that leaves the
//     repository, a missing anchor, or a label with no definition. Anchors follow
//     the rules of GitHub. A path that starts with `/` is relative to the
//     repository root. Links with a scheme are not fetched. Code and HTML blocks
//     are text. Line numbers refer to the text that update writes.
//
// A region mark with no pair, and a name twice in one file, are errors with path
// and line.
//
// # Defaults
//
// The Default column holds the value that your code applies, not what a doc
// comment states. In this code, the default of `Wait` is `DefaultWait`:
//
//	func NewLamp(c Config) *Lamp {
//		wait := c.Wait
//		if wait == 0 {
//			wait = DefaultWait
//		}
//		return &Lamp{wait: wait}
//	}
//
// docgen gets the default in two steps:
//
//  1. It collects the copies of each field in the package: assignments such as
//     `wait := c.Wait`, composite literal keys, and arguments of calls to
//     functions of the package.
//  2. It reads the value that an `if` or a tagless `switch` case assigns to a
//     copy, when that `if` or case tests a copy for `0`, `""` or `nil`.
//
// The column then holds one of these:
//
//   - The value, with its constant value when it has one: `DefaultWait` (250ms).
//   - "not machine-readable; see the description", when the code has no such test
//     and the doc comment states a default, for example with "0 means".
//   - "zero value", in all other cases.
//
// # States and transitions
//
// The state table lists the states of a machine. The transition table lists each
// change of state that your tests make. Both come from running the tests, so a
// transition that no test makes is not in the table. The state table marks the
// states that no test reached.
//
// `type=` names one of these:
//
//   - The machine type, when it has a State method that returns an integer type.
//   - The state type, when one method of one type of the package returns it. That
//     method can take one argument, such as `SlotState(Slot)`. docgen then reads
//     the state for each exported constant of the argument type.
//   - A machine with no State method. Its state is the result of its exported
//     methods with no argument that return a `bool` or a `(value, bool)`, and of
//     `Len()`, as 0 or not 0.
//
// The states are the constants of the state type, in value order, with their doc
// comments. A value that the tests reached with no exported constant is listed as
// `value N`, with no link.
//
// To record transitions, docgen runs `go test` with a hook at the top of each
// exported method of the machine type:
//
//   - The hook reads the state when a method call starts and when it returns. When
//     the two differ, it records both states and the method name.
//   - A call from one exported method to another of the same machine is part of the
//     outer call.
//   - The hook allocates nothing, so a test that counts allocations still passes.
//   - A test that fails under the hook fails the run.
//   - The hook exists only in a go build overlay in the user cache directory, for
//     that `go test` run. Your source and your builds do not change.
//
// A new state, a new transition or a method that no longer changes the state fails
// check until you run update.
//
// The diagram is an SVG file with one box per state, in value order. A transition
// to a later state is an arc above the boxes, and a transition to an earlier state
// is an arc below them. Each arc has the method names as its label.
//
// # Environment
//
// docgen runs go with your environment, so `GOPROXY`, `GOWORK` and `GOFLAGS` work
// as they do for any go command. The go command downloads missing modules at the
// versions that `go.mod` and `go.sum` pin. Set `GOPROXY=off` to forbid the network. `jobs`
// adds `-p` to `GOFLAGS` and `tags` adds `-tags`, joined with the value of
// `go env -w GOFLAGS`.
//
// `GOTMPDIR` is a new directory for each run, below docgen in the user cache
// directory, so that builds are not on a tmpfs. docgen removes it at the end of
// the run, also after an interrupt, so a go command that the interrupt ends
// leaves no build directory.
//
// [configulator]: https://github.com/USA-RedDragon/configulator
package main
