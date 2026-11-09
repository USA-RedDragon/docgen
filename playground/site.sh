#!/bin/sh
# site.sh puts the playground into the docs site. cloudflare.sh runs it.
#
#   site.sh release OUT  The playground of the latest release, from the
#                        playground archive of that GitHub release.
#   site.sh preview OUT  The playground of this checkout: docgen.wasm and the
#                        page of this checkout, with the toolchain of the
#                        latest release.
#
# The latest release is the newest v* tag that is not a prerelease, the same
# tag as /latest/ of the docs. When there is no such release, or the release
# has no playground archive, or a download, a checksum or the build fails,
# OUT gets a page that states that the playground comes with the next
# release, the script prints a warning, and it exits 0.
#
# Environment:
#   PLAYGROUND_RELEASE_URL  base URL of the release downloads (default
#                           https://github.com/USA-RedDragon/docgen/releases/download);
#                           change it for tests only
#   PLAYGROUND_WORK         work directory (default artifacts/playground-work)
set -eu

mode=${1:-}
out=${2:-}
case $mode in
release | preview) ;;
*) mode= ;;
esac
if [ -z "$mode" ] || [ -z "$out" ] || [ $# -ne 2 ]; then
	echo "usage: site.sh release OUT | site.sh preview OUT" >&2
	exit 2
fi

pg=$(cd "$(dirname "$0")" && pwd -P)
root=$(dirname "$pg")
url=${PLAYGROUND_RELEASE_URL:-https://github.com/USA-RedDragon/docgen/releases/download}
work=${PLAYGROUND_WORK:-$root/artifacts/playground-work}
mkdir -p "$work"
work=$(cd "$work" && pwd -P)
export PLAYGROUND_WORK="$work"

pending() {
	echo "site.sh: warning: $1; $out gets the page that states that the playground comes with the next release" >&2
	rm -rf "$out"
	mkdir -p "$out"
	cp "$pg/web/pending.html" "$out/index.html"
	exit 0
}

latest_tag() {
	# Refs hold no white space, so the tags split into words safely.
	# shellcheck disable=SC2046
	tags=$(cd "$root" && pandoc lua docs/site/versions.lua order $(git tag --list 'v*'))
	# shellcheck disable=SC2086
	(cd "$root" && pandoc lua docs/site/versions.lua latest $tags)
}

# fetch_release downloads the playground archive of tag $1, checks its
# SHA-256 against the checksum file of the same release, and extracts it to
# $work/release/playground.
fetch_release() {
	name=docgen_${1#v}_playground
	dir=$work/release
	rm -rf "$dir"
	mkdir -p "$dir"
	for f in "$name.tar" "${name}_checksums.txt"; do
		if ! curl -fsSL --retry 3 -o "$dir/$f" "$url/$1/$f"; then
			pending "no $f in release $1 at $url/$1/"
		fi
	done
	if ! grep -q " $name.tar\$" "$dir/${name}_checksums.txt"; then
		pending "${name}_checksums.txt of release $1 has no line for $name.tar"
	fi
	if ! (cd "$dir" && grep " $name.tar\$" "${name}_checksums.txt" | sha256sum -c --status -); then
		pending "the SHA-256 of $name.tar does not match ${name}_checksums.txt of release $1"
	fi
	if ! tar -xf "$dir/$name.tar" -C "$dir" --no-same-owner || [ ! -f "$dir/playground/manifest.json" ]; then
		pending "$name.tar of release $1 holds no playground"
	fi
}

tag=$(latest_tag) || pending "the latest release is not known"
if [ -z "$tag" ]; then
	pending "there is no release yet"
fi
fetch_release "$tag"
echo "site.sh: the toolchain of the playground comes from release $tag" >&2

if [ "$mode" = release ]; then
	rm -rf "$out"
	mkdir -p "$(dirname "$out")"
	cp -R "$work/release/playground" "$out"
	exit 0
fi

version=${CF_PAGES_BRANCH:-$(git -C "$root" rev-parse --abbrev-ref HEAD 2>/dev/null || echo dev)}
commit=${CF_PAGES_COMMIT_SHA:-$(git -C "$root" rev-parse HEAD 2>/dev/null || echo unknown)}
if ! PLAYGROUND_VERSION=$version PLAYGROUND_COMMIT=$commit "$pg/build/build.sh" preview "$work/release/playground" "$out"; then
	pending "the build of the playground of this checkout failed"
fi
