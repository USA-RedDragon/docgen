# Check the docs in CI

In this step you run `docgen check` on each pull request, so that a change to the code cannot leave a page out of date.

## The job

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
