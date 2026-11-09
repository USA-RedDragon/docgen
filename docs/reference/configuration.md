# Configuration

The settings of `docgen`, with the key in `.docgen.yaml`, the environment variable and the flag of each.

## Settings

<!-- docgen:configulator pkg=internal/cli type=Config env=DOCGEN_ -->
| Key      | Type          | Default | Environment     | Flag       | Description                                                                     |
|----------|---------------|---------|-----------------|------------|---------------------------------------------------------------------------------|
| `docs`   | string        | `.`     | `DOCGEN_DOCS`   | —          | directory to scan for Markdown pages, relative to the repository root           |
| `module` | string        | `.`     | `DOCGEN_MODULE` | `--module` | directory of the Go module, relative to the repository root                     |
| `jobs`   | integer       | `0`     | `DOCGEN_JOBS`   | `--jobs`   | -p of each go command; 0 leaves it to go                                        |
| `tags`   | string        |         | `DOCGEN_TAGS`   | `--tags`   | comma-separated build tags of each go command and package load                  |
| `depth`  | integer       | `1`     | `DOCGEN_DEPTH`  | —          | levels of nested docgen:docgen markers; 0 allows none                           |
| `print`  | string        | `go`    | `DOCGEN_PRINT`  | —          | how a marker with no print= prints a value: go, raw or config; raw runs no code |
| `vars`   | map of string |         | —               | —          | values of the docgen:var and docgen:text markers, by name                       |
<!-- /docgen:configulator -->

- `vars` holds the values of the [`docgen:var`](markers.md#docgenvar) and [`docgen:text`](markers.md#docgentext) markers. It is a setting of the file only. A key has letters, digits, `_` and `-`.
- `jobs` is the `-p` of each go command, an integer up to <!-- docgen:const pkg=internal/cli name=maxJobs -->`1024`<!-- /docgen:const -->. 0 adds no `-p`. At most <!-- docgen:const pkg=internal/gen name=maxCommands -->`8`<!-- /docgen:const --> go commands run at once for any value. Use it to limit a small CI runner.
- `tags` holds letters, digits, `_` and `.`, separated by commas. A `-tags` in `GOFLAGS` also applies.
- `print` is the `print=` of each marker with no `print=`: `go`, `raw` or `config`. With `raw`, these markers run no code. For each value, see [`docgen:const`](markers.md#docgenconst).
- `depth` is at most <!-- docgen:const pkg=internal/cli name=maxDepth -->`8`<!-- /docgen:const -->.
- An unknown key is an error.

## Example

A repository with its pages in `documentation` and its module in `v2`:

```yaml
docs: documentation
module: v2
tags: integration,linux
jobs: 4
vars:
  version: v1.2.3
```

## Order of precedence

A later item overrides an earlier one:

1. Defaults.
2. The file.
3. Environment variables.
4. Flags and the directory argument.

```sh
# .docgen.yaml has jobs: 4.
DOCGEN_JOBS=2 docgen check          # jobs is 2
DOCGEN_JOBS=2 docgen check -jobs 1  # jobs is 1
```

## The file

- The file is `.docgen.yaml` at the repository root. It is optional.
- `-config` names another file, with the extension `.yaml` or `.yml`. That file must exist.
- The repository root is the nearest directory at or above the working directory with a `.docgen.yaml` file or a `.git` entry, or else the working directory.

## Choose what to scan

docgen scans each Markdown file of the repository.

| Scan | How |
|---|---|
| The whole repository | The default. |
| One directory on each run | `docs: documentation` in `.docgen.yaml` |
| One directory on one run | `docgen update documentation` |

The directory argument replaces `docs` and is relative to the working directory. A marker outside the directory stays as it is.

A scan skips:

- `testdata`, `vendor` and `node_modules`.
- Directories whose names start with `.` or `_`.
- Paths that git ignores. A directory named by the argument or `docs` is scanned also when git ignores it, and a `.gitignore` below it applies.
- Pages under the `root` of a `docgen:docgen` marker.
- Anything that is not a regular file, such as a symlink.

## Go environment

<!-- docgen:doc pkg=. section=Environment -->
docgen runs go with your environment, so `GOPROXY`, `GOWORK` and `GOFLAGS` work as they do for any go command. The go command downloads missing modules at the versions that `go.mod` and `go.sum` pin. Set `GOPROXY=off` to forbid the network. `jobs` adds `-p` to `GOFLAGS` and `tags` adds `-tags`, joined with the value of `go env -w GOFLAGS`.

`GOTMPDIR` is a new directory for each run, below docgen in the user cache directory, so that builds are not on a tmpfs. docgen removes it at the end of the run, also after an interrupt, so a go command that the interrupt ends leaves no build directory.
<!-- /docgen:doc -->

For the commands and flags, see [Command line](command.md).
