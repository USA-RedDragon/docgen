# Command line

The commands, flags and exit codes of `docgen`. For the settings, see [Configuration](configuration.md).

## Usage

<!-- docgen:usage cmd=. -->
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
<!-- /docgen:usage -->

A flag goes before the directory. The flags `-module`, `-tags` and `-jobs` override the settings of the same name, and the directory overrides `docs`. For the repository root, see [The file](configuration.md#the-file). For the scan, see [Choose what to scan](configuration.md#choose-what-to-scan).

## See a diff

`check -diff` prints a unified diff of each stale page and generated file to standard output, so `docgen check -diff | patch -p0` at the root applies it. All other output goes to standard error.

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
