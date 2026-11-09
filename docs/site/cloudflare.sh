#!/bin/sh
# cloudflare.sh is the build command of the Cloudflare Worker docgen. The
# output directory is artifacts/docs-site. The production branch main gets
# all versions. Any other branch gets the current tree after docgen update,
# so a stale page still gets a preview.
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

if [ "${CF_PAGES_BRANCH:-}" = main ]; then
	docs/site/build.sh --versions
else
	go run . update
	docs/site/build.sh
fi
