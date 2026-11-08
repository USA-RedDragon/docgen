# Check the docs in CI

In this step you run `docgen check` on each pull request, so that a change to the code cannot leave a page out of date. The [GitHub Action](#add-the-github-action) is the shortest way.

## Add the GitHub Action

Add a workflow, such as `.github/workflows/docs.yaml`. The action runs `docgen check -diff`, so a failed run shows each change in its log:

<!-- docgen:text text="name: Docs\n\non:\n  pull_request:\n  push:\n    branches:\n      - main\n\npermissions:\n  contents: read\n\njobs:\n  docs:\n    runs-on: ubuntu-latest\n    steps:\n      - uses: actions/checkout@v7\n      - uses: actions/setup-go@v7\n        with:\n          go-version-file: go.mod\n      - uses: USA-RedDragon/docgen@{version}" code=yaml -->
```yaml
name: Docs

on:
  pull_request:
  push:
    branches:
      - main

permissions:
  contents: read

jobs:
  docs:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v7
      - uses: actions/setup-go@v7
        with:
          go-version-file: go.mod
      - uses: USA-RedDragon/docgen@v0.1.0
```
<!-- /docgen:text -->

docgen runs the go command to load the packages of your module, so set up Go before the action. The inputs:

| Input | Default | Meaning |
|---|---|---|
| `command` | `check` | `check` or `update`. |
| `directory` | | The directory argument. |
| `module` | | The `-module` flag. |
| `tags` | | The `-tags` flag. |
| `jobs` | | The `-jobs` flag. |
| `diff` | `true` | The `-diff` flag of `check`. `update` ignores it. |
| `version` | | A docgen version, such as `v0.1.0` or `latest`. Empty uses the ref in `uses:`. |
| `comment` | `false` | With `true`, comment on the pull request. See [Comment on the pull request](#comment-on-the-pull-request). |
| `token` | `github.token` | The token for the release download and the comment. |
| `marker` | `docgen-check` | The name in the hidden HTML comment that marks the comment. |

An empty input adds no flag, so the [settings](../reference/configuration.md) apply. The exit code of docgen is the exit code of the step. The action is in [action.yml](../../action.yml).

How the action gets docgen:

- A release tag, such as `v0.1.0` in `uses:` or in `version`, downloads the prebuilt docgen of that release with `gh`. The action checks its sha256 against the `checksums.txt` of the release, and stops on a mismatch. This takes about a second.
- A branch or a commit SHA in `uses:` builds docgen from the code at that ref. The build is cached, so the next run with the same code does not build again. Another `version`, such as `latest`, installs with `go install`. Only these two cases use Go to get docgen.
- When a release has no archive for the runner, or `gh` cannot download it, the action gets docgen from source with Go: a build of the tag in `uses:`, or `go install` of the `version`.

## Comment on the pull request

With `comment: true`, a `check` run on a `pull_request` event also puts the result in one comment on the pull request. The job needs `pull-requests: write`:

<!-- docgen:text text="permissions:\n  contents: read\n  pull-requests: write\n\njobs:\n  docs:\n    runs-on: ubuntu-latest\n    steps:\n      - uses: actions/checkout@v7\n      - uses: actions/setup-go@v7\n        with:\n          go-version-file: go.mod\n      - uses: USA-RedDragon/docgen@{version}\n        with:\n          comment: true" code=yaml -->
```yaml
permissions:
  contents: read
  pull-requests: write

jobs:
  docs:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v7
      - uses: actions/setup-go@v7
        with:
          go-version-file: go.mod
      - uses: USA-RedDragon/docgen@v0.1.0
        with:
          comment: true
```
<!-- /docgen:text -->

- The comment holds the stale, unused and broken lines, and the diff in a closed `<details>` block.
- A hidden HTML comment, `<!-- docgen-check -->` with the default `marker`, starts the comment. The next run edits that comment and adds no other.
- When the docs are current, the comment changes to one line: the docs are current. It stays, so the pull request keeps a record of the earlier result. With no earlier comment, a clean run adds none.
- A GitHub comment holds at most 65536 characters. A longer result is cut to fit, with a note, and the log of the job has all of it.
- The token of a pull request from a fork cannot write. Then the log shows a warning, and the step does not fail for that reason alone.
- The exit code of docgen stays the exit code of the step. The comment uses `gh api`, which is on GitHub-hosted runners.

## Use go run instead

Here is the step of the [docs workflow](../../.github/workflows/docs.yaml):

<!-- docgen:code file=.github/workflows/docs.yaml symbol="jobs.docs.steps[Check the generated parts of the docs]" -->
```yaml
- name: Check the generated parts of the docs
  run: go run . check
```
<!-- /docgen:code -->

In your repository, run docgen by its version:

<!-- docgen:text text="- name: Check the generated parts of the docs\n  run: go run github.com/USA-RedDragon/docgen@{version} check" code=yaml -->
```yaml
- name: Check the generated parts of the docs
  run: go run github.com/USA-RedDragon/docgen@v0.1.0 check
```
<!-- /docgen:text -->

No setup step is necessary. The go command downloads the modules at the versions that `go.mod` and `go.sum` pin.

## Check before each commit

To find stale docs before you push, run the same check in a pre-commit hook. See [Use pre-commit](../guides/pre-commit.md).

## What fails the check

- A marker body that differs from what `update` writes.
- A generated file, such as a state diagram, that differs or is missing.
- A marker with the name of something that does not exist, such as a removed constant.
- A failed test or Example, for `docgen:states` and `docgen:output`.
- An excerpt with `lines=` whose file changed.
- A region, a diagram or a variable that no marker uses, and a region mark with no pair.
- A link, image or anchor that reaches nothing.

The output gives the page and the line of each result. For the exit codes, see [Command line](../reference/command.md#exit-codes).

## Next

Go to [Document a config struct](../guides/config-tables.md) and the other guides for each marker kind.
