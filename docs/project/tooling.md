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
- `docgen_<version>_playground.tar` and `docgen_<version>_playground_checksums.txt`, the [playground](#playground) of the release. A later job adds them.

A tag with a suffix, such as `v0.2.0-rc.1`, makes a prerelease. The workflow also adds a build provenance attestation for each archive. To check an archive:

```sh
gh attestation verify docgen_0.1.0_linux_amd64.tar.gz --repo USA-RedDragon/docgen
```

To release, set `version` in [.docgen.yaml](../../.docgen.yaml) to the new tag, run `go run . update`, commit, then push the tag. `goreleaser release --snapshot --clean` makes the archives and `checksums.txt` in `dist/`, and publishes nothing.

## Lint

`golangci-lint run ./...` uses [.golangci.yaml](../../.golangci.yaml).

## Checks on pull requests and on `main`

[go-static-checks.yaml](../../.github/workflows/go-static-checks.yaml), [rust-static-checks.yaml](../../.github/workflows/rust-static-checks.yaml) and [action.yaml](../../.github/workflows/action.yaml) run on each pull request and on each push to `main`, so a commit that lands on `main` has the same checks as its pull request. A new push to a branch cancels the run of the one before it, except on `main`, where every commit keeps its run. A push has no pull request, so the two steps of the `action` job that test the pull request comment run on pull requests only; the action posts no comment on a push.

## Rust checks

The Rust helper in `internal/rustsrc/helper` and the fixture in `internal/cli/testdata/rust` have their own checks in [rust-static-checks.yaml](../../.github/workflows/rust-static-checks.yaml): `cargo fmt --check` and `cargo clippy --locked --all-targets -- -D warnings` on the helper, `cargo check --locked` on the helper with Rust 1.71, which is its `rust-version`, and `cargo metadata --locked` on the fixture. `--locked` fails when `Cargo.lock` needs a change, so each lock file in the repository is the one that builds.

[Renovate](../../.github/renovate.json) updates the Cargo dependencies of both. For the helper it keeps the version ranges of `Cargo.toml` and changes `Cargo.lock` only, so the update stays within Rust 1.71. The fixture updates go in a separate group that does not merge by itself, because the golden files of the tests depend on them.

The action has the inputs `rust-toolchain` and `rust-cache`; the `rust` job of [action.yaml](../../.github/workflows/action.yaml) runs them. To change the helper cache key, see `rust` in [action/install.sh](../../action/install.sh).

## Docs site

The site of this repository is a private build, and is not a feature of docgen. `docs/site/build.sh` builds it into `artifacts/docs-site` with pandoc, and `internal/sitetest` tests the script. `docs/nav.md` gives the order of the pages. The headings of a page are in a closed `<details>` under the item of the page in the navigation, so they work with no JavaScript, and `localStorage` keeps the open or closed state when it is available.

### Editor blocks

A `<!-- site:editor -->` and `<!-- /site:editor -->` pair around `docgen:code` and `docgen:docgen` markers makes one editor block. The pair can hold these markers only, and the build stops on any other content. [Show a constant](../guides/const.md) has two. A `docgen:docgen` marker outside a pair makes an editor block of its page alone. `<!-- site:large -->` on the line before a block sets a large height. In the HTML, the block is plain headings and code blocks, so it reads with no JavaScript. `docs/site/editor.js` then replaces it with a read-only CodeMirror view: an explorer, tabs, a Before and After switch for a `docgen:docgen` page, a Preview tab, and a status bar. One button shows or hides the explorer of all editors, and `localStorage` keeps that state when it is available. Another button opens the editor in a modal dialog that fills the window, and Esc or the same button closes it. When the bundle does not load, the plain block stays. `mountEditor` in that file takes a file list, so another page can use it directly.

Each block mounts near the viewport, so a page with many blocks loads fast. Until then, the block has the height of its editor. That height fits the longest file of the block, up to a limit, and is the same for each tab.

A path shows relative to the `root=` of the first `docgen:docgen` marker of the block. Two or more excerpts of one file make one file of the block, in file order, and a blank line with a `⋮` label stands for the lines between them. The build stops on two excerpts that overlap. The search text of a page leaves out the excerpts. Line numbers show only when they are true. A `docgen:docgen` page is a whole page, so its numbers start at 1. A `docgen:code` excerpt gets numbers when its text is whole lines of the file and occurs once there. In all other cases the excerpt has no line numbers. Add `raw=true` to a marker to show the doc comments and region marks, because an excerpt with region marks left out is not whole lines of the file.

`build.sh` copies `editor.js` and the bundle to the root of each version that has an editor block. Only a page with an editor block loads them. They are JavaScript modules, and some browsers load no module from a `file:` URL, so serve the site over HTTP. In those browsers the block stays plain. [docs/site/vendor/README.md](../site/vendor/README.md) holds the versions of the bundle and the command that builds it.

### Hosting

The Cloudflare Pages project `docgen` serves the site at <https://docgen.mcswain.dev/>. The project is connected to this repository with the Cloudflare Git integration, so Cloudflare clones and builds each push, and GitHub holds no Cloudflare API token. [wrangler.jsonc](../../wrangler.jsonc) names the project and the output directory `artifacts/docs-site`. The project has the production branch `main` and the build command `docs/site/cloudflare.sh`.

[cloudflare.sh](../site/cloudflare.sh) installs pandoc with [install-pandoc.sh](../site/install-pandoc.sh), the same script that the Docs workflow uses, and fetches the full history and the tags. The build image has an older Go, and Go downloads the toolchain of `go.mod`. On `main` the build makes all versions. On another branch it runs `go run . update` and builds the current tree, so a stale page still gets a preview, and Cloudflare puts the preview URL on the pull request. Cloudflare builds no pull request from a fork. The Docs workflow builds the site of each pull request as a check.

Cloudflare Pages builds on a push to a branch, not on a tag. On a `v*` tag, the Rebuild job of [release.yaml](../../.github/workflows/release.yaml) sends a POST to the deploy hook in the repository secret `CLOUDFLARE_DEPLOY_HOOK` after the Playground job, which builds `main` again, so `/latest/` and `/playground/` show the new tag. The job also runs when the Playground job fails, so the docs of the tag still go live. Create the hook under the project, Settings, Builds, Deploy hooks, for the branch `main`. Without the secret, the job ends with a warning.

GitHub Pages serves only [redirect.html](../site/redirect.html), which sends each path under `https://usa-reddragon.github.io/docgen/` to the same path on the new domain. The Docs workflow deploys it after a push to `main`, and only when `https://docgen.mcswain.dev/versions.json` serves the docs, so GitHub Pages keeps the old site until the domain works.

### Versioned build

`docs/site/build.sh --versions [out]` builds the site of each version, and the Cloudflare Pages build of `main` serves the result. The Docs workflow in [docs.yaml](../../.github/workflows/docs.yaml) builds all versions on a push to `main` or on a `v*` tag as a check. A pull request builds the current tree only. The output has these paths:

| Path | Content |
| --- | --- |
| `/` | A redirect to `/latest/`. |
| `/404.html` | The page for a path with no file. Without it, Cloudflare Pages serves `/index.html` for such a path, and its redirect adds `latest/` to the path in a loop. |
| `/latest/` | A copy of the newest release tag, or of `/main/` when there are no tags. |
| `/main/` | The current tree, with a banner that states it is not released. |
| `/v0.1.0/` | One directory for each tag. |
| `/versions.json` | The versions, newest first, and the latest version. |
| `/_redirects` | The rule that makes Cloudflare Pages send `/` to `/latest/` with no page. |
| `/_headers` | The cross-origin isolation headers of `/playground/`, `/main/playground/` and `/latest/playground/`, and the cache time of their assets. A branch build has it too. |
| `/playground/` | The [playground](#playground) of the latest release. |
| `/main/playground/` | The playground of the main branch, built by `playground-main.yaml` on each push to `main`. |
| `/llms.txt` | The list of the Markdown copies of `/latest/`, for agents. Each version has its own `llms.txt` too. |
| `<page>.md` | The Markdown copy of each page, next to its HTML. Its links go to the Markdown copies of the other pages, and the HTML names it with `<link rel="alternate" type="text/markdown">`. |

Each tag is a semantic version that starts with `v`. The latest version is the newest tag that is not a prerelease. If all tags are prereleases, it is the newest tag. A tag with no `docs/nav.md` is left out, with a message.

The build extracts each tag with `git archive` and uses the scripts of the current tree, so a layout fix applies to all versions. It needs the full history and the tags, and it stops in a shallow clone. In a tag build, source links use the tag and `pkg.go.dev` links of this module get `@<tag>`. The Markdown files do not change.

To look at the output, serve it over HTTP, because the version picker reads `versions.json` at run time:

```sh
docs/site/build.sh --versions
python3 -m http.server -d artifacts/docs-site 8000
```

### Playground

The playground is a page that runs the Go toolchain and docgen in the browser, on a sample repository in memory. Each process is a Web Worker that runs a Go `js/wasm` program, or a WASI program: the Rust helper of docgen. The header of each docs page links to the playground of its version: a page of `/main/` links to `/main/playground/`, a page of a release links to `/playground/`, and a page of a branch build links to the `/playground/` of that build.

| Path | Content |
| --- | --- |
| `playground/web/` | The page: `index.html`, the scripts, the kernel that runs the processes, `pending.html`, and the npm packages of the editor bundle and of the WASI shim bundle. |
| `playground/overlay/` | The new files of the `go build -overlay` that gives `js/wasm` processes and pipes, in the directories of GOROOT/src: `syscall/proc_js.go` starts, waits for and kills a process through the page, and `os/exec/lp_js.go` finds a program by path. A `go.mod` makes it a module of its own, so `./...` of the root module leaves it out. |
| `playground/build/mkoverlay.py` | Writes the overlay from the GOROOT of the build: the new files, and the changed copies of some GOROOT files. Each change is an exact replacement, so a Go version with other source stops the build. It also writes `wasm_exec.js`. |
| `playground/bb/` | A Go module with a shell (`mvdan.cc/sh`) and common commands (u-root) in one binary. `bb/ucmd` is the output of `bb/vendor-uroot.py`, with the license of u-root. |
| `playground/testdata/` | The sample repository `dg` that the page starts with, with the Rust crate `crates/lamp` and its page `docs/rust.md`, and the module `loop`, a test that does not stop. The directory name keeps `docgen check` of this repository away from them. |
| `playground/build/build.sh` | Builds the playground. |
| `playground/site.sh` | Puts the playground into the docs site, for `cloudflare.sh`. |
| `playground/build/checkmanifest.mjs` | Checks the manifest of a downloaded playground. |
| `playground/test/` | The browser test, with Playwright Test. |

`playground/build/build.sh all OUT` builds the whole playground into `OUT`, with the Go version of `go.mod`:

- `go`, `compile`, `link`, `asm` and `vet` of that Go version, `bb` and `docgen`, for `js/wasm`, with `-trimpath -ldflags="-s -w"`. `go`, `docgen` and `bb` start processes, so they have the overlay. The others have no overlay, because it made `compile` 20 to 40% slower in Firefox.
- `rshelper`, the Rust helper of docgen (`internal/rustsrc/helper`), built with `cargo build --release --locked --target wasm32-wasip1`. The build needs cargo and the target (`rustup target add wasm32-wasip1`); the playground workflows add the target, and `build.sh all` stops without it.
- `wasm-opt -Os` of binaryen on each tool other than `docgen`. This step takes most of the build time. `docgen` gets no `wasm-opt`, because its output is larger after compression, and the Cloudflare build builds `docgen` on each push.
- A bundle of the source of each standard package that the sample imports, a bundle of the sample, and a bundle of a GOCACHE. To make the GOCACHE, the script runs `docgen check`, `go test` and `go build` on the sample under Node, with the kernel of the page and the same tools.
- Each tool and bundle compressed with `zstd -19` in `assets/zstd/`, with the start of the SHA-256 of the raw file in its name (see [Compression](#compression)), and `manifest.json`. The manifest gives the Go version, and the docgen version and commit. The page shows that version and commit.
- `licenses/`: the license of Go, and the notices of the Go modules in `bb` and `docgen`, of the npm packages in the editor bundle and in the WASI shim bundle (`wasi-shim.js`, [@bjorn3/browser_wasi_shim](https://github.com/bjorn3/browser_wasi_shim), MIT or Apache-2.0), and of the crates and the Rust standard library in `rshelper` (`rshelper.txt`). The build writes the notices from the module list, from the esbuild metafiles and from `cargo metadata` and the `COPYRIGHT-library.html` of the toolchain, so they match the code of each build.

`build.sh preview BASE OUT` builds only `docgen`, the Rust helper and the page of the checkout, and takes the other assets from `BASE`, an earlier output of `build.sh all`. Without cargo and the target `wasm32-wasip1`, as in the Cloudflare build, it keeps the Rust helper of `BASE`, so a change of the helper source shows in a preview only after the next `playground-main` build. It uses the Go version of `BASE`. `build.sh gover` prints the Go version of `go.mod`.

#### Rust in the playground

The playground has no Rust compiler and no cargo. It runs one Rust program, the helper of docgen that reads Rust source with syn, built for `wasm32-wasip1`. So the Rust markers that read only the source work in the tab, and the ones that build or run Rust code are errors:

| Works in the tab | Error in the tab |
| --- | --- |
| `docgen:doc crate=` | `docgen:const crate=` with `print=` other than `raw` (the value test) |
| `docgen:errors crate=` | `docgen:config crate=` and `docgen:configulator crate=` (their defaults come from the value test) |
| `docgen:const crate= print=raw` | `docgen:output crate=` with `bin=`, `example=` or `doctest=` |
| `docgen:code file=*.rs symbol=` | `docgen:usage crate=` |
| `docgen:crates`, `docgen:cargo` | |

The error of such a marker starts with the page, line and marker, and says `this marker needs cargo, and the playground has no Rust toolchain`. On a computer with no cargo the text ends with the hint to install Rust instead.

How it works:

- `session.mjs` puts the helper at `/usr/local/bin/docgen-rs-helper` and sets `DOCGEN_RUST_HELPER` to that path, so docgen runs it and does not build the helper (see the package doc). With no cargo on `PATH`, docgen reads the Cargo workspace from the `Cargo.toml` files (`cargo.ReadWorkspace`), not with `cargo metadata`.
- `proc.mjs` runs a module that imports `wasi_snapshot_preview1` and not `gojs` with `wasiproc.mjs`, in the same Worker type as a Go process. `wasiproc.mjs` uses [@bjorn3/browser_wasi_shim](https://github.com/bjorn3/browser_wasi_shim), pinned in `playground/web/package.json` and bundled by esbuild into `wasi-shim.js`. Its `Fd` classes send each file call to the kernel with the same synchronous call as a Go process, so the helper sees the memfs, its working directory is `/`, and standard input, output and error are the descriptors of the kernel. A read of a pipe with no data blocks the Worker until data comes, since a WASI program has no event loop. `proc_exit` ends the process with its status, and a trap, such as a Rust panic, is a crash of the process.
- The package is used unchanged. `wasiproc.mjs` subclasses it for three fixes: setting the times of a file succeeds, a `readlink` fails with `EINVAL` as for a file that is not a link, and the times of a stat are written at the right offsets (version 0.4.2 writes them 2 to 8 bytes early). It also replaces the `path_rename` import, which moves objects of the shim, with a rename in the kernel.
- The helper is about 1.2 MB raw, 0.8 MB after `wasm-opt -Os` and 0.2 MB with `zstd -19`. A run of the helper on the sample crate takes well under a second.

#### Compression

The playground has no compression or decompression code of its own. The build compresses each tool and bundle with the `zstd` command (`zstd -19`), and the browser decodes the files natively:

- A compressed asset is in `assets/zstd/`, with a `.zst` name, such as `assets/zstd/compile-020bd05ab708136a.wasm.zst`. The `_headers` rule of that directory gives it `Content-Encoding: zstd` and `Content-Type: application/wasm`. Cloudflare Pages sends such a file as it is, and the browser decodes it. A sitetest checks these rules, and the browser test checks the headers of each asset.
- The bundles (`root`, `gocache`, `sample`) also get `application/wasm`. The page reads their bytes and does not use the type. A test deploy showed that Cloudflare sends a file of that type with Brotli to a browser with no zstd. [Cloudflare compression](https://developers.cloudflare.com/speed/optimization/content/compression/) lists the types that it compresses; `application/wasm` is on that list, and `application/octet-stream` is not.
- The release build stops when there is no `zstd` command. GitHub's Ubuntu runners have it. The Cloudflare build image documents no `zstd` command, so a preview build without it puts `docgen.wasm` raw in `assets/`, with no `Content-Encoding`, and Cloudflare compresses it on the fly. The zone of `docgen.mcswain.dev` has a compression rule: Cloudflare sends Zstandard to a browser that accepts it, else Brotli, else gzip.
- `manifest.json` gives the encoding of each asset (`zstd` or `identity`), its URL, the size of the decoded file and its SHA-256. The page checks the size and the SHA-256 of the decoded bytes with `crypto.subtle`, and keeps them in the Cache API. The progress of the first load counts the bytes of the decoded files.
- Cloudflare Pages serves no file over 25 MiB, and each compressed asset is far below that. The build stops when a file of the output is over 25 MiB.

The release archive is a plain tar. Its assets are compressed already, and the Cloudflare build extracts it with no `zstd` command.

#### Releases and the Cloudflare build

On a `v*` tag, two jobs of [release.yaml](../../.github/workflows/release.yaml) make the playground of the release. The Playground build job has a read-only token. It runs `build.sh all`, puts the output in `docgen_<version>_playground.tar` with `docgen_<version>_playground_checksums.txt`, and hands them to the next job as a workflow artifact. After GoReleaser, the Playground upload job checks the checksum, adds a build provenance attestation for the archive, and adds both files to the release. It runs no code of the repository. Then the Rebuild job starts a Cloudflare build of `main`. The build takes about 10 minutes on four fast cores, most of it in `wasm-opt`, so only a tag runs it.

A pull request that changes `playground/`, the Go source of docgen, `go.mod`, `go.sum`, `docs/site/build.sh`, `docs/site/cloudflare.sh`, `release.yaml` or the workflow runs the same build and the browser test in [playground.yaml](../../.github/workflows/playground.yaml), with no upload. `actions/cache` keeps the `wasm-opt` outputs by the SHA-256 of their inputs, so a pull request that changes no tool skips `wasm-opt`. A pull request can restore the caches of its own runs and of `main` only, so run the workflow by hand on `main` once to fill the cache for all pull requests, and again after a change of the Go version.

`cloudflare.sh` runs `playground/site.sh`. It takes the newest `v*` tag that is not a prerelease and that has a valid playground archive, and tries up to five tags. When all tags are prereleases, prereleases count, as for `/latest/`. A tag with no archive, such as a tag whose upload is not done or failed, is skipped. For each archive, `site.sh`:

- Downloads it with time limits, and checks its SHA-256 against the checksum file of the same release.
- Stops on a member that is not a regular file or a directory under `playground/`, or whose path has `..`, before it extracts the archive.
- Checks `manifest.json` with `checkmanifest.mjs`: each URL must be `assets/` and a plain file name of a regular file.

On `main`, `site.sh release` puts that playground at `/playground/`. Each push to `main` runs [playground-main.yaml](../../.github/workflows/playground-main.yaml): it runs `build.sh all` with the cached `wasm-opt` outputs, moves the tag `playground-main` to the commit, replaces the archive `docgen_playground-main_playground.tar` and its checksum file on the prerelease `playground-main`, and starts a Cloudflare build of `main` with the deploy hook. `site.sh main` puts that archive at `/main/playground/`, so `/main/playground/` runs the toolchain and docgen of `main`. With no `v*` tag, `site.sh release` also takes that archive, and `cloudflare.sh` copies it to `/latest/playground/`, because `/latest/` is then a copy of `main`. On another branch, `site.sh preview` builds `/playground/` with `build.sh preview` from the archive of `playground-main`, or else of the newest release, so a branch preview shows docgen of that branch. A change of the toolchain on a branch, such as of the overlay, `bb` or the sample, shows after the merge, and the pull request check tests it.

The archive and its checksum file come from the same release, so the checksum finds a damaged download, not a changed release. The Cloudflare build does not check the attestation, because `gh attestation verify` needs a GitHub token, and the build has none. To check an archive by hand:

```sh
gh attestation verify docgen_0.2.0_playground.tar --repo USA-RedDragon/docgen
```

When no archive is valid, or the preview build fails or takes more than 15 minutes, `site.sh` puts `pending.html` at the path, prints a warning, and the docs build goes on. `pending.html` states that the playground is not available in this build. `PLAYGROUND_RELEASE_URL` replaces the release download URL, for tests only.

The playground needs `SharedArrayBuffer`, so its pages need cross-origin isolation. `docs/site/build.sh` writes `_headers` with `Cross-Origin-Opener-Policy`, `Cross-Origin-Embedder-Policy` and `Cross-Origin-Resource-Policy` for `/playground*` and `/main/playground*` only, so the docs pages keep their fonts from Google Fonts. The assets of the playground have their hash in their names, so `_headers` gives them a cache time of one year.

#### Change the Go version

The playground uses the Go version of the `toolchain` line of `go.mod`, or else of its `go` line. To change it, change `go.mod`, then run `build.sh all` in a pull request, or let the playground workflow run it. When `mkoverlay.py` stops on a GOROOT file that changed, change the replacement in the script. The next release then has the toolchain of that version.

The tools of a release and `docgen.wasm` use one `wasm_exec.js`, so a preview builds `docgen` with the Go version of the release. When the Go version of `go.mod` differs from that version, `site.sh preview` does not build. It puts `pending.html` at the preview path with a message that gives both versions, and prints a warning. So after a change of the Go version, `/main/playground/` and branch previews come back with the next release, and `/playground/` keeps the latest release.

When the u-root version in `playground/bb/go.mod` changes, run `vendor-uroot.py` again, as its doc comment shows. The build stops when `bb/ucmd` differs from the output of the script.

#### Test

The browser test in `playground/test/` uses [Playwright Test](https://playwright.dev/docs/intro), with the version that `playground/test/package.json` pins. `playwright.config.mjs` starts `serve.mjs`, a server on 127.0.0.1 that serves a built docs site with the rules of its `_headers`, as Cloudflare Pages does, so the browser decodes the zstd assets. The specs in `specs/` check:

- The headers of the playground and its assets, and with `PLAYGROUND_DOCS` set, that a docs page has no cross-origin isolation header, loads its fonts and links the playground.
- The first load and a load from the Cache API, and typing into the editor.
- `docgen update` after an edit, `docgen check` on a clean and on a stale page, and the output of the `exec=` markers.
- The Rust crate of the sample: `docgen update` after an edit of the crate, the helper run from the shell with its exit status, and the error of the markers that need cargo.
- Shell commands, a `go test` with a timeout, Stop during a build, and the process limit.
- The previews, and HTML in a Markdown preview that must run no script.
- The explorer: selection, keyboard, drag and drop, rename, the context menu, upload, download, the saved workspace and Reset Sample.
- The page at 390 px, and the themes.

Each test opens the page in a new browser context, so it starts with no cache and no saved workspace. The projects `firefox` and `chromium` run the specs at 1280 px, and `firefox-390` and `chromium-390` run `phone.spec.mjs` at 390 px. A failed test keeps its trace and a screenshot. The HTML report and the results go to `PLAYGROUND_TEST_OUT`, by default `artifacts/playground-test`. `PLAYGROUND_PAGE` gives the path of the page, by default `/playground/`, so the same specs test `/main/playground/`.

```sh
playground/build/build.sh all artifacts/playground
docs/site/build.sh
cp -R artifacts/playground artifacts/docs-site/playground
cd playground/test
npm ci --ignore-scripts
npx playwright install firefox chromium
PLAYGROUND_SITE=../../artifacts/docs-site PLAYGROUND_DOCS="/index.html playground/" npx playwright test
```
