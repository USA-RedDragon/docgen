# Check the docs in CI

In this step you run `docgen check` on each pull request, so that a change to the code cannot leave a page out of date. Use [`go run`](#use-go-run), [the GitHub Action](#use-the-github-action) or [the pre-commit hook](#use-pre-commit).

## Use go run

Here is the step of the [docs workflow](../../.github/workflows/docs.yaml):

<!-- docgen:code file=.github/workflows/docs.yaml symbol="jobs.docs.steps[Check the generated parts of the docs]" -->
```yaml
- name: Check the generated parts of the docs
  run: go run . check
```
<!-- /docgen:code -->

Your repository runs docgen by its version:

<!-- docgen:text text="- name: Check the generated parts of the docs\n  run: go run github.com/USA-RedDragon/docgen@{version} check" code=yaml -->
```yaml
- name: Check the generated parts of the docs
  run: go run github.com/USA-RedDragon/docgen@v0.1.0 check
```
<!-- /docgen:text -->

No setup step is necessary. The go command downloads the modules at the versions that `go.mod` and `go.sum` pin.

## Use the GitHub Action

The action runs `docgen check` with `-diff`, so the log shows each change. Go must be on `PATH`, so set up Go first:

<!-- docgen:text text="- uses: actions/setup-go@v7\n  with:\n    go-version-file: go.mod\n- uses: USA-RedDragon/docgen@{version}" code=yaml -->
```yaml
- uses: actions/setup-go@v7
  with:
    go-version-file: go.mod
- uses: USA-RedDragon/docgen@v0.1.0
```
<!-- /docgen:text -->

The inputs:

| Input | Default | Meaning |
|---|---|---|
| `command` | `check` | `check` or `update`. |
| `directory` | | The directory argument. |
| `module` | | The `-module` flag. |
| `tags` | | The `-tags` flag. |
| `jobs` | | The `-jobs` flag. |
| `diff` | `true` | The `-diff` flag of `check`. `update` ignores it. |
| `version` | | A docgen version to install, such as `latest`. |

An empty input adds no flag, so the [settings](../reference/configuration.md) apply. With no `version`, the action builds docgen from its own ref, so a tag, a branch or a commit SHA in `uses:` runs the docgen code at that ref. The exit code of docgen is the exit code of the step. The action is in [action.yml](../../action.yml).

## Use pre-commit

Add the hook to `.pre-commit-config.yaml`:

<!-- docgen:text text="repos:\n  - repo: https://github.com/USA-RedDragon/docgen\n    rev: {version}\n    hooks:\n      - id: docgen-check\n      - id: docgen-update" code=yaml -->
```yaml
repos:
  - repo: https://github.com/USA-RedDragon/docgen
    rev: v0.1.0
    hooks:
      - id: docgen-check
      - id: docgen-update
```
<!-- /docgen:text -->

`docgen-check` runs on each commit, also when no Markdown or Go file changed, because a page can quote any file. pre-commit builds docgen with Go, and installs Go when it is not on `PATH`. `docgen-update` fills the markers, and runs only by hand:

```sh
pre-commit run --hook-stage manual docgen-update
```

The hooks are in [.pre-commit-hooks.yaml](../../.pre-commit-hooks.yaml).

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
