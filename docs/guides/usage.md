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

## A Rust binary

For a binary of a Cargo package, use `crate=` and `bin=` in place of `cmd=`. docgen builds the binary with `cargo build --locked` and runs it with `-h`, which clap answers with its help.

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

`args=` works as for Go. For a subcommand, use `args="help label"` or `args="label --help"`.

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

`bin=` can be left out when the package has one binary. The build goes into a directory of the run, and one run builds a binary once for all markers. `Cargo.lock` must be up to date.

For details, see [`docgen:usage`](../reference/markers.md#docgenusage).
