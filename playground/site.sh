#!/bin/sh
# site.sh puts the playground into the docs site. cloudflare.sh runs it.
#
#   site.sh release OUT  The playground of a release, from the playground
#                        archive of that GitHub release.
#   site.sh main OUT     The playground of main, from the archive of the
#                        prerelease playground-main, which the workflow
#                        playground-main.yaml replaces on each push to main.
#                        Without it, as preview.
#   site.sh preview OUT  The playground of this checkout: docgen.wasm and the
#                        page of this checkout, with the toolchain of
#                        playground-main, or else of a release.
#
# The release is the newest v* tag that is not a prerelease and that has a
# valid playground archive. When all tags are prereleases, prereleases count
# too, as for /latest/ of the docs (versions.lua). When no tag has a valid
# archive, the Go version of go.mod differs from the one of the release, or
# the build fails, OUT gets a page that states that the playground is not
# available, the script prints a warning, and it exits 0. It never stops the
# docs build.
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
release | main | preview) ;;
*) mode= ;;
esac
if [ -z "$mode" ] || [ -z "$out" ] || [ $# -ne 2 ]; then
	echo "usage: site.sh release OUT | site.sh main OUT | site.sh preview OUT" >&2
	exit 2
fi

pg=$(cd "$(dirname "$0")" && pwd -P)
root=$(dirname "$pg")
url=${PLAYGROUND_RELEASE_URL:-https://github.com/USA-RedDragon/docgen/releases/download}
work=${PLAYGROUND_WORK:-$root/artifacts/playground-work}
mkdir -p "$work"
work=$(cd "$work" && pwd -P)
export PLAYGROUND_WORK="$work"
# The number of tags to try, newest first.
tries=5

# pending writes the page with the message $2 (default: the text of the page)
# to OUT, prints the warning $1 and ends the script with exit 0.
pending() {
	echo "site.sh: warning: $1; $out gets the page that states that the playground is not available" >&2
	rm -rf "$out"
	mkdir -p "$out"
	if [ -n "${2:-}" ]; then
		sed "s|<!-- message -->.*<!-- /message -->|$2|" "$pg/web/pending.html" >"$out/index.html"
	else
		cp "$pg/web/pending.html" "$out/index.html"
	fi
	exit 0
}

# candidates prints the v* tags that the script tries, newest first: the tags
# that are not prereleases, or all tags when each one is a prerelease.
candidates() {
	# Refs hold no white space, so the tags split into words safely.
	# shellcheck disable=SC2046
	tags=$(cd "$root" && pandoc lua docs/site/versions.lua order $(git tag --list 'v*'))
	stable=$(printf '%s\n' "$tags" | grep -v '^v[0-9]*\.[0-9]*\.[0-9]*-' || :)
	if [ -n "$stable" ]; then
		printf '%s\n' "$stable"
	else
		printf '%s\n' "$tags"
	fi
}

# fetch_release downloads the playground archive of tag $1, checks its SHA-256
# against the checksum file of the same release, checks each member of the
# archive, extracts it to $work/release/playground and checks its manifest.
# It returns 1 with a warning on any problem.
fetch_release() {
	name=docgen_${1#v}_playground
	dir=$work/release
	rm -rf "$dir"
	mkdir -p "$dir"
	for f in "$name.tar" "${name}_checksums.txt"; do
		if ! curl -fsSL --retry 2 --connect-timeout 20 --max-time 300 -o "$dir/$f" "$url/$1/$f"; then
			echo "site.sh: warning: no $f in release $1 at $url/$1/" >&2
			return 1
		fi
	done
	if ! grep " $name.tar\$" "$dir/${name}_checksums.txt" | (cd "$dir" && sha256sum -c --status -); then
		echo "site.sh: warning: the SHA-256 of $name.tar does not match ${name}_checksums.txt of release $1" >&2
		return 1
	fi
	# Only regular files and directories under playground/, with plain names.
	if ! tar -tvf "$dir/$name.tar" >"$dir/list.txt" || ! tar -tf "$dir/$name.tar" >"$dir/names.txt" ||
		[ "$(wc -l <"$dir/list.txt")" != "$(wc -l <"$dir/names.txt")" ] ||
		grep -qv '^[-d]' "$dir/list.txt" ||
		grep -qvE '^playground(/[A-Za-z0-9._-]+)*/?$' "$dir/names.txt" ||
		grep -qE '(^|/)\.\.?(/|$)' "$dir/names.txt"; then
		echo "site.sh: warning: $name.tar of release $1 holds a member that is not a plain file or directory under playground/" >&2
		return 1
	fi
	if ! tar -xf "$dir/$name.tar" -C "$dir" --no-same-owner --no-same-permissions; then
		echo "site.sh: warning: $name.tar of release $1 does not extract" >&2
		return 1
	fi
	if ! relgo=$(node "$pg/build/checkmanifest.mjs" "$dir/playground"); then
		echo "site.sh: warning: the manifest of $name.tar of release $1 is not valid" >&2
		return 1
	fi
}

if [ "$mode" = main ] && fetch_release playground-main; then
	echo "site.sh: the playground of main comes from the prerelease playground-main" >&2
	rm -rf "$out"
	mkdir -p "$(dirname "$out")"
	cp -R "$work/release/playground" "$out"
	exit 0
fi

# A preview takes the toolchain of main first, so it works before the first
# release. With no v* tag, the release mode also takes the playground of main,
# as /latest/ of the docs is then main.
tag=
sources=$(candidates | head -n "$tries")
if [ "$mode" != release ] || [ -z "$sources" ]; then
	sources="playground-main $sources"
fi
for t in $sources; do
	if fetch_release "$t"; then
		tag=$t
		break
	fi
done
if [ -z "$tag" ]; then
	pending "no valid playground archive in $sources"
fi
echo "site.sh: the playground toolchain comes from release $tag, $relgo" >&2

if [ "$mode" = release ]; then
	rm -rf "$out"
	mkdir -p "$(dirname "$out")"
	cp -R "$work/release/playground" "$out"
	exit 0
fi

# The tools of the release and docgen.wasm share one wasm_exec.js, so docgen
# must build with the Go version of the release.
need=$("$pg/build/build.sh" gover) || pending "go.mod gives no Go version"
if [ "$need" != "$relgo" ]; then
	pending "go.mod needs $need and the playground toolchain of release $tag is $relgo" \
		"This build needs $need, and the newest playground release is built with $relgo. The preview of this build comes back after a release that is built with $need."
fi
version=${CF_PAGES_BRANCH:-$(git -C "$root" rev-parse --abbrev-ref HEAD 2>/dev/null || echo dev)}
commit=${CF_PAGES_COMMIT_SHA:-$(git -C "$root" rev-parse HEAD 2>/dev/null || echo unknown)}
if ! PLAYGROUND_VERSION=$version PLAYGROUND_COMMIT=$commit timeout 900 "$pg/build/build.sh" preview "$work/release/playground" "$out"; then
	pending "the build of the playground of this checkout failed or took longer than 15 minutes"
fi
