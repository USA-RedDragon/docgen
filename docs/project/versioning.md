# Versioning and compatibility

The version rules of docgen and its minimum Go version.

## What is stable

The compatibility surface is what your repository depends on: the marker syntax, the flags, the exit codes, and the text that docgen writes for a given marker. The Go packages of this module are `internal` and have no compatibility rule.

## Releases

- docgen has versions of the form vMAJOR.MINOR.PATCH.
- A patch release never breaks your pages or your CI. A page that passes `check` on v0.1.0 passes on v0.1.1 with no change.
- A minor release can add marker kinds, arguments and flags. It can also make a breaking change, such as a change to the text that docgen writes for a marker. Run `docgen update` after you upgrade, and read the diff.

Pin a version in CI, so that an upgrade is a change that you review. See [Install](../start/01-install.md).

## The Go version

The minimum Go version is <!-- docgen:gomod field=go -->`1.27.0`<!-- /docgen:gomod -->. It only goes up in a minor or major release.
