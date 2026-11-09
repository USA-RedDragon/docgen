# Tooling

The checks of this repository, and how to run each one.

## Generated pages

docgen fills the pages of this repository. `go generate ./...` runs this line of [generate.go](../../generate.go):

<!-- docgen:code file=generate.go -->
```go
package main

//go:generate go run . update
```
<!-- /docgen:code -->

`go run . check` fails when a page differs from what that line writes. Run it after a change to a flag or to [the package doc](../../doc.go), which [Markers](../reference/markers.md) and other pages quote.

## Pre-commit

`pre-commit install` adds a hook that runs gofmt, golangci-lint and `docgen check` from this checkout on each commit. Run all of them with `pre-commit run --all-files`. The config is in [.pre-commit-config.yaml](../../.pre-commit-config.yaml).

## Tests

`go test ./...` runs docgen on the fixture in `internal/cli/testdata/repo`, a page with each marker kind except `docgen:configulator` and `docgen:docgen`, and compares the result with `internal/cli/testdata/want`. After a change that is meant to change the output, write the new result, then read the diff:

```sh
go test ./internal/cli -run 'TestFixture$' -golden
```

## Releases

A push of a `v*` tag runs [release.yaml](../../.github/workflows/release.yaml). It runs GoReleaser with [.goreleaser.yaml](../../.goreleaser.yaml) and adds a GitHub release with:

- `docgen_<version>_<os>_<arch>.tar.gz` for `linux` and `darwin`, and `.zip` for `windows`, each on `amd64` and `arm64`. Each archive holds `docgen`, `LICENSE` and `README.md`.
- `checksums.txt`, with the sha256 of each archive.

A tag with a suffix, such as `v0.2.0-rc.1`, makes a prerelease. The workflow also adds a build provenance attestation for each archive. To check an archive:

```sh
gh attestation verify docgen_0.1.0_linux_amd64.tar.gz --repo USA-RedDragon/docgen
```

To release, set `version` in [.docgen.yaml](../../.docgen.yaml) to the new tag, run `go run . update`, commit, then push the tag. `goreleaser release --snapshot --clean` makes the archives and `checksums.txt` in `dist/`, and publishes nothing.

## Lint

`golangci-lint run ./...` uses [.golangci.yaml](../../.golangci.yaml).

## Docs site

The site of this repository is a private build, and is not a feature of docgen. `docs/site/build.sh` builds it into `artifacts/docs-site` with pandoc, and `internal/sitetest` tests the script. `docs/nav.md` gives the order of the pages. The headings of a page are in a closed `<details>` under the item of the page in the navigation, so they work with no JavaScript, and `localStorage` keeps the open or closed state when it is available.

### Editor blocks

A `<!-- site:editor -->` and `<!-- /site:editor -->` pair around `docgen:code` and `docgen:docgen` markers makes one editor block. The pair can hold these markers only, and the build stops on any other content. [Show a constant](../guides/const.md) has two. A `docgen:docgen` marker outside a pair makes an editor block of its page alone. `<!-- site:large -->` on the line before a block sets a large height. In the HTML, the block is plain headings and code blocks, so it reads with no JavaScript. `docs/site/editor.js` then replaces it with a read-only CodeMirror view: an explorer, tabs, a Before and After switch for a `docgen:docgen` page, a Preview tab, and a status bar. One button shows or hides the explorer of all editors, and `localStorage` keeps that state when it is available. Another button opens the editor in a modal dialog that fills the window, and Esc or the same button closes it. When the bundle does not load, the plain block stays. `mountEditor` in that file takes a file list, so another page can use it directly.

Each block mounts near the viewport, so a page with many blocks loads fast. Until then, the block has the height of its editor. That height fits the longest file of the block, up to a limit, and is the same for each tab.

A path shows relative to the `root=` of the first `docgen:docgen` marker of the block. Two or more excerpts of one file make one file of the block, in file order, and a blank line with a `⋮` label stands for the lines between them. The build stops on two excerpts that overlap. The search text of a page leaves out the excerpts. Line numbers show only when they are true. A `docgen:docgen` page is a whole page, so its numbers start at 1. A `docgen:code` excerpt gets numbers when its text is whole lines of the file and occurs once there. In all other cases the excerpt has no line numbers. Add `raw=true` to a marker to show the doc comments and region marks, because an excerpt with region marks left out is not whole lines of the file.

`build.sh` copies `editor.js` and the bundle to the root of each version that has an editor block. Only a page with an editor block loads them. They are JavaScript modules, and some browsers load no module from a `file:` URL, so serve the site over HTTP. In those browsers the block stays plain. [docs/site/vendor/README.md](../site/vendor/README.md) holds the versions of the bundle and the command that builds it.

### Pull request preview

Each pull request gets a preview of the site on Cloudflare Pages, in two workflows:

- [docs-preview.yaml](../../.github/workflows/docs-preview.yaml) runs on `pull_request` with a read-only token and no secrets. It runs `go run . update`, builds the site, and uploads it with the number of the pull request as the `docs-preview` artifact.
- [docs-preview-deploy.yaml](../../.github/workflows/docs-preview-deploy.yaml) runs on `workflow_run` when that build succeeds. It runs no code of the pull request. It checks the artifact, deploys it with `wrangler pages deploy` to the branch `pr-<number>`, and puts the URL in one comment on the pull request.

A `workflow_run` workflow runs from the default branch only, so a change to the deploy workflow has effect after the merge. Until the secret and the account variable are set, the deploy job ends with a notice and does not fail. The first deploy creates the Pages project for direct upload, named `docgen` or the name in `CLOUDFLARE_PAGES_PROJECT`. Do not connect the repository to Cloudflare in the dashboard: a project with a Git source builds each push on Cloudflare. To set up the preview, the owner:

1. Creates an API token with the permission Account, Cloudflare Pages, Edit, and adds it as the repository secret `CLOUDFLARE_API_TOKEN`.
2. Adds the repository variable `CLOUDFLARE_ACCOUNT_ID`, and optionally `CLOUDFLARE_PAGES_PROJECT`, which has the default `docgen`.
3. Sets Settings, Actions, General, Approval for running fork pull request workflows to require approval for all external contributors, so no fork build runs before a review.

A closed pull request needs no cleanup step. Its preview stays at `pr-<number>.<project>.pages.dev` until someone deletes the deployments in the Cloudflare dashboard.

### Versioned build

`docs/site/build.sh --versions [out]` builds the site of each version, and the Docs workflow in [docs.yaml](../../.github/workflows/docs.yaml) publishes the result to GitHub Pages on a push to `main` or on a `v*` tag. A pull request builds the current tree only. The output has these paths:

| Path | Content |
| --- | --- |
| `/` | A redirect to `/latest/`. |
| `/latest/` | A copy of the newest release tag, or of `/main/` when there are no tags. |
| `/main/` | The current tree, with a banner that states it is not released. |
| `/v0.1.0/` | One directory for each tag. |
| `/versions.json` | The versions, newest first, and the latest version. |

Each tag is a semantic version that starts with `v`. The latest version is the newest tag that is not a prerelease. If all tags are prereleases, it is the newest tag. A tag with no `docs/nav.md` is left out, with a message.

The build extracts each tag with `git archive` and uses the scripts of the current tree, so a layout fix applies to all versions. It needs the full history and the tags, and it stops in a shallow clone. In a tag build, source links use the tag and `pkg.go.dev` links of this module get `@<tag>`. The Markdown files do not change.

To look at the output, serve it over HTTP, because the version picker reads `versions.json` at run time:

```sh
docs/site/build.sh --versions
python3 -m http.server -d artifacts/docs-site 8000
```
