#!/bin/sh
set -eu

# canon prints the absolute path of $1 with symlinks resolved. The part of the
# path that does not exist stays as it is, apart from . and .. segments.
canon() (
	set -f
	path=$1
	case $path in
	/*) ;;
	*) path=$PWD/$path ;;
	esac
	cur=/
	IFS=/
	for seg in $path; do
		case $seg in
		'' | .) ;;
		..) cur=$(dirname "$cur") ;;
		*)
			if [ -d "${cur%/}/$seg" ]; then
				cur=$(cd -P "${cur%/}/$seg" && pwd -P)
			else
				cur=${cur%/}/$seg
			fi
			;;
		esac
	done
	printf '%s\n' "$cur"
)

versions=
if [ "${1:-}" = --versions ]; then
	versions=1
	shift
fi
case ${1:-} in
-*)
	echo "build.sh: unknown option $1; the usage is build.sh [--versions] [output directory]" >&2
	exit 2
	;;
esac

site=$(cd "$(dirname "$0")" && pwd -P)
root=$(git -C "$site" rev-parse --show-toplevel 2>/dev/null || (cd "$site/../.." && pwd -P))
out=$(canon "${1:-$root/artifacts/docs-site}")
cd "$root"
DOCS_SITE=${site#"$root"/}
DOCS_DIR=${DOCS_DIR:-$(dirname "$DOCS_SITE")}
docs=$(canon "$DOCS_DIR")
case $docs in
"$root")
	echo "build.sh: the pages must be in a directory below the repository root; set DOCS_DIR" >&2
	exit 2
	;;
"$root"/*) ;;
*)
	echo "build.sh: DOCS_DIR $DOCS_DIR is outside the repository root $root" >&2
	exit 2
	;;
esac
if [ ! -d "$docs" ]; then
	echo "build.sh: the pages directory $DOCS_DIR does not exist" >&2
	exit 2
fi
DOCS_DIR=${docs#"$root"/}
export DOCS_SITE DOCS_DIR
if [ -e "$out" ] && [ ! -d "$out" ]; then
	echo "build.sh: the output path $out is not a directory" >&2
	exit 2
fi
for keep in "$root" "$docs" "$site"; do
	case "$keep/" in
	"${out%/}/"*)
		echo "build.sh: the output directory $out is or holds $keep; give another directory" >&2
		exit 2
		;;
	esac
done
for keep in "$docs" "$site"; do
	case "$out/" in
	"$keep/"*)
		echo "build.sh: the output directory $out is inside $keep; give another directory" >&2
		exit 2
		;;
	esac
done
if [ -d "$out" ] && [ -n "$(ls -A "$out")" ] && [ ! -f "$out/.docgen-site" ]; then
	echo "build.sh: $out holds files and is not an earlier build of the site; give another directory" >&2
	exit 2
fi
if ! command -v pandoc >/dev/null 2>&1; then
	echo "build.sh: pandoc is not on PATH" >&2
	exit 2
fi
pages=$(pandoc lua "$DOCS_SITE/pages.lua")
if [ -z "$pages" ]; then
	echo "build.sh: $DOCS_DIR has no pages to build" >&2
	exit 2
fi
if [ -n "$versions" ]; then
	if ! git rev-parse --git-dir >/dev/null 2>&1; then
		echo "build.sh: --versions needs a git repository" >&2
		exit 2
	fi
	if [ "$(git rev-parse --is-shallow-repository)" = true ]; then
		echo "build.sh: --versions needs the full history and the tags; this clone is shallow" >&2
		exit 2
	fi
	# Refs hold no white space, so the tags split into words safely.
	# shellcheck disable=SC2046
	tags=$(pandoc lua "$DOCS_SITE/versions.lua" order $(git tag --list 'v*'))
fi

# build_pages builds the pages of the tree in the working directory into
# directory $1, from the list in file $2. The other arguments go to pandoc.
build_pages() {
	dest=$1
	list=$2
	shift 2
	while IFS="$(printf '\t')" read -r src page; do
		mkdir -p "$dest/$(dirname "$page")"
		pandoc -f gfm -t html5 --wrap=none --standalone --template "$DOCS_SITE/template.html" --lua-filter "$DOCS_SITE/site.lua" -M outdir="$dest" -M page="$page" "$@" -o "$dest/$page" "$src"
	done <"$list"
	pandoc lua "$DOCS_SITE/index.lua" "$dest"
	pandoc lua "$DOCS_SITE/llms.lua" "$dest"
	if [ -e "$dest/.editor" ]; then
		rm "$dest/.editor"
		cp "$DOCS_SITE/editor.js" "$DOCS_SITE/vendor/codemirror.js" "$dest"
	fi
}

# extract writes the tree of tag $1 to $work/$1, with the scripts of the current
# tree in place of the scripts of the tag, and lists its pages.
extract() {
	mkdir -p "$work/$1"
	git archive "refs/tags/$1" | tar -x -C "$work/$1"
	rm -rf "${work:?}/$1/$DOCS_SITE"
	mkdir -p "$(dirname "$work/$1/$DOCS_SITE")"
	cp -R "$site" "$work/$1/$DOCS_SITE"
	(cd "$work/$1" && pandoc lua "$DOCS_SITE/pages.lua") >"$work/$1.pages"
	if [ ! -s "$work/$1.pages" ]; then
		echo "build.sh: $1 has no pages to build" >&2
		exit 2
	fi
}

# build_version builds version $1 into a directory of that name.
build_version() {
	mainsha=$(git -C "$root" rev-parse --short HEAD 2>/dev/null || :)
	if [ "$1" != main ]; then
		cd "$work/$1"
	fi
	if [ -n "$kept" ]; then
		build_pages "$out/$1" "$work/$1.pages" -M version="$1" -M mainsha="$mainsha" -M latest="$latest" -M latestpages="$work/$latest.pages"
	else
		build_pages "$out/$1" "$work/$1.pages" -M version="$1" -M mainsha="$mainsha"
	fi
}

wait_jobs() {
	for pid in $pids; do
		wait "$pid" || failed=1
	done
	pids=
	running=0
}

# build_versions builds the current tree and each tag, and the top of the site.
build_versions() {
	kept=
	for tag in $tags; do
		if git cat-file -e "refs/tags/$tag:$DOCS_DIR/nav.md" 2>/dev/null; then
			kept="$kept $tag"
		else
			echo "build.sh: $tag has no $DOCS_DIR/nav.md; left out" >&2
		fi
	done
	# shellcheck disable=SC2086
	latest=$(pandoc lua "$DOCS_SITE/versions.lua" latest $kept)
	work=$out/.work
	trap 'rm -rf "$work"' EXIT
	mkdir -p "$work"
	printf '%s\n' "$pages" >"$work/main.pages"
	for tag in $kept; do
		extract "$tag"
	done
	limit=$(getconf _NPROCESSORS_ONLN 2>/dev/null || echo 4)
	pids=
	running=0
	failed=0
	for name in main $kept; do
		build_version "$name" &
		pids="$pids $!"
		running=$((running + 1))
		if [ "$running" -ge "$limit" ]; then
			wait_jobs
		fi
	done
	wait_jobs
	if [ "$failed" != 0 ]; then
		echo "build.sh: a version did not build" >&2
		exit 1
	fi
	cp -R "$out/${latest:-main}" "$out/latest"
	# shellcheck disable=SC2086
	DOCS_MAIN_SHA=$(git -C "$root" rev-parse --short HEAD 2>/dev/null || :) pandoc lua "$DOCS_SITE/versions.lua" site "$out" $kept
	pandoc lua "$DOCS_SITE/llms.lua" "$out" latest/
	# Cloudflare Pages reads _redirects, so / goes to /latest/ with no page.
	printf '/ /latest/ 302\n' >"$out/_redirects"
	: >"$out/.nojekyll"
}

rm -rf "$out"
mkdir -p "$out"
: >"$out/.docgen-site"
if [ -n "$versions" ]; then
	build_versions
else
	printf '%s\n' "$pages" >"$out/.pages"
	build_pages "$out" "$out/.pages"
	rm "$out/.pages"
fi
# Cloudflare Pages reads _headers. The playground at /playground/ and
# /main/playground/ needs cross-origin isolation for SharedArrayBuffer. The
# docs pages do not get it, so they keep their fonts from Google Fonts. The
# name of each playground asset holds its hash, so it is cached for a year.
for p in /playground /main/playground; do
	printf '%s*\n  Cross-Origin-Opener-Policy: same-origin\n  Cross-Origin-Embedder-Policy: require-corp\n  Cross-Origin-Resource-Policy: same-origin\n  X-Content-Type-Options: nosniff\n%s/assets/*\n  Cache-Control: public, max-age=31536000, immutable\n' "$p" "$p"
done >"$out/_headers"
# A host such as Cloudflare Pages serves the top index.html for a path with no
# file when the site has no 404.html, and the redirect of the top index.html
# then adds latest/ to the path again and again.
cp "$DOCS_SITE/404.html" "$out/404.html"
echo "$out/index.html"
