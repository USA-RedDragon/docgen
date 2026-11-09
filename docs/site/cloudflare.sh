#!/bin/sh
# cloudflare.sh is the build command of the Cloudflare Pages project docgen. The
# output directory is artifacts/docs-site. The production branch main gets
# all versions. Any other branch gets the current tree after docgen update,
# so a stale page still gets a preview. Both get the playground.
set -eu

root=$(cd "$(dirname "$0")/../.." && pwd)
cd "$root"

# The build image has an older Go. Go downloads the toolchain of go.mod.
export GOTOOLCHAIN=auto

PATH=$(docs/site/install-pandoc.sh "${TMPDIR:-/tmp}/pandoc"):$PATH
export PATH

# The versioned build reads every tag, so the clone needs the full history.
remote=https://github.com/USA-RedDragon/docgen.git
if [ "$(git rev-parse --is-shallow-repository)" = true ]; then
	git fetch --quiet --unshallow --tags "$remote"
else
	git fetch --quiet --tags "$remote"
fi

# /playground/ is the playground of the newest release that has one. On main,
# /main/playground/ and, on another branch, /playground/ is the playground of
# this checkout with the toolchain of that release. site.sh does not stop the
# build: on an error it puts a page there that states that the playground is
# not available.
if [ "${CF_PAGES_BRANCH:-}" = main ]; then
	docs/site/build.sh --versions
	playground/site.sh release artifacts/docs-site/playground
	playground/site.sh preview artifacts/docs-site/main/playground
else
	go run . update
	docs/site/build.sh
	playground/site.sh preview artifacts/docs-site/playground
fi
