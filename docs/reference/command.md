# Command line

The commands, flags, settings and exit codes of `docgen`.

## Usage

<!-- docgen:flags cmd=. -->
```text
Usage: docgen <command> [flags] [directory]

Commands:
  update  fill in the markers
  check   list stale markers, unused regions and broken links, and exit 1 if there are any

Arguments:
  directory  the directory to scan (default the repository, or docs: in .docgen.yaml)

Flags:
  -config string
    	config file (default ".docgen.yaml")
  -diff
    	with check, print a unified diff of each stale page and generated file
  -jobs int
    	-p of each go command; 0 leaves it to go
  -module string
    	directory of the Go module, relative to the repository root (default ".")
  -tags string
    	comma-separated build tags of each go command and package load
```
<!-- /docgen:flags -->

The repository root is the nearest directory at or above the working directory with a `.docgen.yaml` file or a `.git` entry, or else the working directory.

## Choose what to scan

docgen scans each Markdown file of the repository.

| Scan | How |
|---|---|
| The whole repository | The default. |
| One directory on each run | `docs: documentation` in `.docgen.yaml` |
| One directory on one run | `docgen update documentation` |

A marker outside the directory stays as it is.

A scan skips:

- `testdata`, `vendor` and `node_modules`.
- Directories whose names start with `.` or `_`.
- Paths that git ignores. A directory named by the argument or `docs` is scanned also when git ignores it, and a `.gitignore` below it applies.
- Pages under the `root` of a `docgen:docgen` marker.
- Anything that is not a regular file, such as a symlink.

## Settings

A `.docgen.yaml` at the repository root holds the settings of a repository. The order of precedence is:

1. Defaults.
2. The file.
3. Environment variables.
4. Flags and the directory argument.

The directory argument replaces `docs` and is relative to the working directory.

<!-- docgen:configulator pkg=internal/cli type=Config env=DOCGEN_ -->
| Key      | Type          | Default | Environment     | Flag       | Description                                                                |
|----------|---------------|---------|-----------------|------------|----------------------------------------------------------------------------|
| `docs`   | string        | `.`     | `DOCGEN_DOCS`   | —          | directory to scan for Markdown pages, relative to the repository root      |
| `module` | string        | `.`     | `DOCGEN_MODULE` | `--module` | directory of the Go module, relative to the repository root                |
| `jobs`   | integer       | `0`     | `DOCGEN_JOBS`   | `--jobs`   | -p of each go command; 0 leaves it to go                                   |
| `tags`   | string        |         | `DOCGEN_TAGS`   | `--tags`   | comma-separated build tags of each go command and package load             |
| `depth`  | integer       | `1`     | `DOCGEN_DEPTH`  | —          | levels of nested docgen:docgen markers; 0 allows none                      |
| `print`  | string        | `go`    | `DOCGEN_PRINT`  | —          | how a marker with no print= prints a value: go, or raw, which runs no code |
| `vars`   | map of string |         | —               | —          | values of the docgen:var and docgen:text markers, by name                  |
<!-- /docgen:configulator -->

This is the file of a repository with its pages in `documentation` and its module in `v2`:

```yaml
docs: documentation
module: v2
tags: integration,linux
jobs: 4
vars:
  version: v1.2.3
```

- `vars` holds the values of the [`docgen:var`](markers.md#docgenvar) and [`docgen:text`](markers.md#docgentext) markers. It is a setting of the file only. A key has letters, digits, `_` and `-`.
- `jobs` is the `-p` of each go command, an integer up to <!-- docgen:const pkg=internal/cli name=maxJobs -->`1024`<!-- /docgen:const -->. 0 adds no `-p`. At most <!-- docgen:const pkg=internal/gen name=maxCommands -->`8`<!-- /docgen:const --> go commands run at once for any value. Use it to limit a small CI runner.
- `tags` holds letters, digits, `_` and `.`, separated by commas. A `-tags` in `GOFLAGS` also applies.
- An unknown key is an error.
- A file named with `-config` must exist. The file at the repository root is optional.

## See a diff

`check -diff` prints a unified diff of each stale page and generated file to standard output, so `docgen check -diff | patch -p0` at the root applies it. All other output goes to standard error. Give the flag before the directory.

## Run docgen as a tool

A `tool` directive also pins a version, with `go get -tool github.com/USA-RedDragon/docgen` and then `go tool docgen check`. It adds the docgen dependencies to your `go.mod` and raises its `go` line to the docgen version. Do not use it in a library that supports an earlier Go version.

## What check reports

<!-- docgen:doc pkg=. section="Checks of check" -->
check also reads the repository, and update does not. Each result is a line on standard error, sorted, and check exits 1.

  - `unused: path:line: docgen:region name` is a region that no `region=` of a scanned page excerpts. The scan starts at the repository root, skips the directories of a page scan, the paths that git ignores other than scanned pages, and the pages under a `docgen:docgen` root, and skips binary files and files over 1 MiB. A path that cannot be read is an error, and the other results still print.
  - `unused: path: docgen:states diagram` is an SVG file in a `states` directory next to a scanned page that no `docgen:states` marker writes.
  - `unused: .docgen.yaml: var name` is a variable of the `vars` setting that no marker of a scanned page reads. The path is the config file of the run.
  - `broken: page:line: target` is a link, image or reference definition of a scanned page that reaches nothing: a missing file, a path that leaves the repository, a missing anchor, or a label with no definition. Anchors follow the rules of GitHub. A path that starts with `/` is relative to the repository root. Links with a scheme are not fetched. Code and HTML blocks are text. Line numbers refer to the text that update writes.

A region mark with no pair, and a name twice in one file, are errors with path and line.
<!-- /docgen:doc -->

## Exit codes

| Code | Meaning |
|---|---|
| 0 | The run finished. With `check`, nothing is stale, unused or broken. |
| 1 | With `check`, a marker is stale, a region, diagram or variable is unused, or a link is broken. With either command, a marker has an error, a test or Example that it uses failed, a setting has a value that is not valid, or a setting holds a directory or file that does not exist. An interrupt or a SIGTERM also stops the run with exit 1 and writes nothing. |
| 2 | The command line is not valid: no command, an unknown command, an unknown flag, a flag value that does not parse, or an extra argument. |

- `update` writes nothing when a marker fails.
- `check` lists stale markers and marker errors together, each with its page and line.
- An error that stops a package from loading names the package.

## Environment

<!-- docgen:doc pkg=. section=Environment -->
docgen runs go with your environment, so `GOPROXY`, `GOWORK` and `GOFLAGS` work as they do for any go command. The go command downloads missing modules at the versions that `go.mod` and `go.sum` pin. Set `GOPROXY=off` to forbid the network. `jobs` adds `-p` to `GOFLAGS` and `tags` adds `-tags`, joined with the value of `go env -w GOFLAGS`.

`GOTMPDIR` is a new directory for each run, below docgen in the user cache directory, so that builds are not on a tmpfs. docgen removes it at the end of the run, also after an interrupt, so a go command that the interrupt ends leaves no build directory.
<!-- /docgen:doc -->

The module of this repository is <!-- docgen:gomod field=module -->`github.com/USA-RedDragon/docgen`<!-- /docgen:gomod -->.
