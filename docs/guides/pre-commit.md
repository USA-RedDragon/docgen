# Use pre-commit

Run `docgen check` before each commit with [pre-commit](https://pre-commit.com), so that stale docs never reach a pull request.

## Add the hooks

Add docgen to `.pre-commit-config.yaml` at the repository root:

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

Then install the git hook:

```sh
pre-commit install
```

pre-commit builds docgen with Go at the `rev` that you give, and installs Go when it is not on `PATH`. For a repository with no Go, see [Use pre-commit without Go](#use-pre-commit-without-go).

## Use pre-commit without Go

The `docgen-check` and `docgen-update` hooks build docgen with Go, so pre-commit installs Go when a repository has none. A Rust-only repository can use the `-system` hooks, which run the `docgen` on `PATH`:

<!-- docgen:text text="repos:\n  - repo: https://github.com/USA-RedDragon/docgen\n    rev: {version}\n    hooks:\n      - id: docgen-check-system\n      - id: docgen-update-system" code=yaml -->
```yaml
repos:
  - repo: https://github.com/USA-RedDragon/docgen
    rev: v0.1.0
    hooks:
      - id: docgen-check-system
      - id: docgen-update-system
```
<!-- /docgen:text -->

Install docgen first. Download the archive of a [release](https://github.com/USA-RedDragon/docgen/releases) for your system, check it against `checksums.txt` of that release, and put `docgen` on `PATH`. pre-commit does not download or verify a binary itself, so each developer and each CI job installs it. In CI, the [GitHub Action](../start/03-check-in-ci.md#check-a-rust-repository) does that with a checked download.

These hooks also run `cargo` for markers with `crate=`, so `cargo` must be on `PATH` too. The first run builds the helper that reads Rust code, which takes a while; later runs use the cached build. The `-system` hooks take `args:` and run on the same stages as the others.

## What each hook does

| Hook | Runs | Does |
|---|---|---|
| `docgen-check` | on each commit | `docgen check`. The commit stops when a page is stale. |
| `docgen-update` | only when you ask | `docgen update`. It fills the markers. |

`docgen-check` runs on each commit, also when no Markdown or Go file changed, because a page can quote any file.

To also check before each push, install that hook type too: `pre-commit install --hook-type pre-push`.

## Fix stale docs

When `docgen-check` stops a commit, fill the markers, then add the changes and commit again:

```sh
pre-commit run --hook-stage manual docgen-update
git add -A
git commit
```

## Check all files

Run the check on the whole repository, for example after you add the hooks:

```sh
pre-commit run docgen-check --all-files
```

## Add flags

Give flags with `args:`, such as a module in a subdirectory:

```yaml
      - id: docgen-check
        args: [-module, v2]
```

The settings in [`.docgen.yaml`](../reference/configuration.md) also apply, so most repositories need no `args:`.

The hooks are in [.pre-commit-hooks.yaml](../../.pre-commit-hooks.yaml).
