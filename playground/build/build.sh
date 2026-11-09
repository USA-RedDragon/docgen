#!/bin/sh
# build.sh builds the docgen playground.
#
#   build.sh all OUT           The whole playground: the Go toolchain, the bb
#                              shell and commands, the GOROOT
#                              and GOCACHE bundles, docgen.wasm and the page.
#   build.sh preview BASE OUT  docgen.wasm and the page of this checkout, with
#                              the toolchain of BASE, an earlier output of
#                              build.sh all. The Go version is the one of BASE.
#   build.sh gover             Print the Go version of go.mod, such as go1.27.0.
#
# OUT is a directory that the playground page serves from. The release
# workflow runs "all" on a v* tag, and cloudflare.sh runs "preview" for
# /main/playground/ and for branch builds.
#
# Environment:
#   PLAYGROUND_WORK     work directory (default artifacts/playground-work)
#   PLAYGROUND_VERSION  the docgen version that the page shows (default dev)
#   PLAYGROUND_COMMIT   the commit that the page shows (default HEAD)
set -eu

mode=${1:-}
case $mode in
all) [ $# -eq 2 ] || mode= ;;
preview) [ $# -eq 3 ] || mode= ;;
gover) [ $# -eq 1 ] || mode= ;;
*) mode= ;;
esac
if [ -z "$mode" ]; then
	echo "usage: build.sh all OUT | build.sh preview BASE OUT | build.sh gover" >&2
	exit 2
fi

here=$(cd "$(dirname "$0")" && pwd -P)
pg=$(dirname "$here")
root=$(dirname "$pg")
abs() (mkdir -p "$1" && cd "$1" && pwd -P)
if [ "$mode" = gover ]; then
	:
elif [ "$mode" = all ]; then
	out=$(abs "$2")
else
	base=$(cd "$2" && pwd -P)
	out=$(abs "$3")
fi
work=$(abs "${PLAYGROUND_WORK:-$root/artifacts/playground-work}")
version=${PLAYGROUND_VERSION:-dev}
commit=${PLAYGROUND_COMMIT:-$(git -C "$root" rev-parse HEAD 2>/dev/null || echo unknown)}
unset GOFLAGS GOOS GOARCH
export CGO_ENABLED=0

# gomod_go prints the Go toolchain of go.mod: the toolchain line, or else the
# go line, as a release name such as go1.27.0.
gomod_go() {
	v=$(sed -n 's/^toolchain[[:space:]]*\(go[0-9.]*\).*/\1/p' "$root/go.mod")
	if [ -z "$v" ]; then
		v=go$(sed -n 's/^go[[:space:]]*\([0-9.]*\).*/\1/p' "$root/go.mod")
	fi
	case $v in
	go[0-9]*.[0-9]*.[0-9]*) ;;
	go[0-9]*.[0-9]*) v=$v.0 ;;
	*)
		echo "build.sh: no Go version in go.mod" >&2
		exit 1
		;;
	esac
	echo "$v"
}

if [ "$mode" = gover ]; then
	gomod_go
	exit 0
elif [ "$mode" = all ]; then
	gover=$(gomod_go)
else
	gover=$(sed -n 's/^ *"go": "\(go[0-9.]*\)".*/\1/p' "$base/manifest.json")
	if [ -z "$gover" ]; then
		echo "build.sh: $base/manifest.json has no Go version" >&2
		exit 1
	fi
fi
export GOTOOLCHAIN="$gover"
# The work directory can be in the module of the repository, so it gets a
# go.mod of its own, and go commands on ./... of the repository leave it out.
printf 'module playground-work\n\ngo %s\n' "${gover#go}" >"$work/go.mod"
goroot=$(cd "$work" && go env GOROOT)
if [ "$(cd "$work" && go env GOVERSION)" != "$gover" ]; then
	echo "build.sh: go env GOVERSION is not $gover" >&2
	exit 1
fi
# go build -overlay replaces no file in the module cache, and a toolchain that
# GOTOOLCHAIN downloads is in the module cache, so the build uses a copy.
case $goroot in
"$(cd "$work" && go env GOMODCACHE)"/*)
	if [ ! -f "$work/goroot/VERSION" ] || [ "$(head -n 1 "$work/goroot/VERSION")" != "$gover" ]; then
		rm -rf "$work/goroot"
		cp -R "$goroot" "$work/goroot"
		chmod -R u+w "$work/goroot"
	fi
	goroot=$work/goroot
	export GOROOT="$goroot" GOTOOLCHAIN=local
	PATH=$goroot/bin:$PATH
	;;
esac
echo "build.sh: $mode with $gover in $goroot, work directory $work" >&2

rm -rf "$work/raw" "$work/z" "$work/ov" "$work/licenses"
mkdir -p "$work/raw" "$work/z" "$work/licenses"

# The assets go out compressed with zstd -19, and the browser decodes them.
# The release build needs the zstd command. A preview without it sends
# docgen.wasm as it is, and Cloudflare compresses it on the fly.
if command -v zstd >/dev/null 2>&1; then
	have_zstd=1
elif [ "$mode" = all ]; then
	echo "build.sh: the zstd command is not on PATH; install zstd" >&2
	exit 1
else
	have_zstd=0
	echo "build.sh: no zstd command, so docgen.wasm goes out as it is" >&2
fi
python3 -I "$here/mkoverlay.py" "$goroot" "$work/ov"

wasm() {
	GOOS=js GOARCH=wasm go build -trimpath -ldflags="-s -w" "$@"
}

npm_ci() {
	rm -rf "$work/npm-$1"
	mkdir -p "$work/npm-$1"
	cp "$2/package.json" "$2/package-lock.json" "$work/npm-$1/"
	(cd "$work/npm-$1" && npm ci --ignore-scripts --no-audit --no-fund --loglevel=error --fetch-timeout=60000 --fetch-retries=2)
}

# wasm_opt writes the wasm-opt -Os output of $1 to $2. The Go builds are
# reproducible, so $work/opt-cache keeps each output by the SHA-256 of its
# input and of package-lock.json, and a second build in the same work
# directory skips wasm-opt. The pull request workflow keeps opt-cache with
# actions/cache. build_toolchain moves the cache to opt-cache.old first and
# removes what this build does not use, so the cache does not grow.
wasm_opt() {
	key=$(cat "$1" "$here/package-lock.json" | sha256sum | cut -d ' ' -f 1)
	mkdir -p "$work/opt-cache"
	if [ -f "$work/opt-cache.old/$key.wasm" ]; then
		mv "$work/opt-cache.old/$key.wasm" "$work/opt-cache/$key.wasm"
	fi
	if [ ! -f "$work/opt-cache/$key.wasm" ]; then
		"$work/npm-build/node_modules/.bin/wasm-opt" --enable-bulk-memory --enable-sign-ext --enable-nontrapping-float-to-int -Os "$1" -o "$work/opt-cache/$key.tmp"
		mv "$work/opt-cache/$key.tmp" "$work/opt-cache/$key.wasm"
	fi
	cp "$work/opt-cache/$key.wasm" "$2"
}

# wait_all waits for each PID in $pids and stops when one failed.
wait_all() {
	failed=0
	for pid in $pids; do
		wait "$pid" || failed=1
	done
	pids=
	if [ "$failed" != 0 ]; then
		echo "build.sh: a build step failed" >&2
		exit 1
	fi
}

# Page: the editor bundle, the scripts and the patched wasm_exec.js.
build_page() {
	npm_ci web "$pg/web"
	cp "$pg/web/editor.entry.js" "$work/npm-web/"
	(cd "$work/npm-web" && node_modules/.bin/esbuild editor.entry.js --bundle --format=esm --minify --log-level=warning \
		--banner:js="/* CodeMirror 6 and markdown-it 15 with their dependencies; see licenses/editor-bundle.txt. */" \
		--metafile="$work/editor-meta.json" --outfile="$work/editor-bundle.js")
	node "$here/notices.mjs" npm "$work/editor-meta.json" "$work/npm-web/node_modules" >"$work/licenses/editor-bundle.txt"
}

# go_notices writes the license notices of the modules of the js/wasm build of
# the main package in directory $1 to the file $2.
go_notices() {
	(cd "$1" && GOOS=js GOARCH=wasm go list -deps -f '{{with .Module}}{{if not .Main}}{{.Path}} {{.Version}} {{.Dir}}{{end}}{{end}}' .) |
		sort -u | node "$here/notices.mjs" go >"$2"
}

build_docgen() {
	# No wasm-opt: it takes about 90 s, and its output is larger after compression.
	(cd "$root" && wasm -overlay "$work/ov/overlay.json" -o "$work/raw/docgen.wasm" .)
	go_notices "$root" "$work/licenses/docgen.txt"
}

build_toolchain() {
	rm -rf "$work/bin" "$work/ucmd"
	mkdir -p "$work/bin"
	# bb/ucmd holds the output of bb/vendor-uroot.py, so go mod tidy works in bb.
	(cd "$pg/bb" && go mod download)
	uroot=$(cd "$pg/bb" && go list -m -f '{{.Dir}}' github.com/u-root/u-root)
	# shellcheck disable=SC2046
	python3 -I "$pg/bb/vendor-uroot.py" "$uroot" "$work/ucmd" $(cd "$pg/bb/ucmd" && ls -d */ | tr -d /)
	if ! diff -r "$work/ucmd" "$pg/bb/ucmd" >&2; then
		echo "build.sh: playground/bb/ucmd differs from the output of vendor-uroot.py; run it again" >&2
		exit 1
	fi

	# go, docgen and bb start processes, so they get the overlay. compile,
	# link, asm and vet ran 20 to 40% slower in Firefox with it, so they do not.
	pids=
	(cd "$work" && wasm -overlay "$work/ov/overlay.json" -o "$work/bin/go.wasm" cmd/go) &
	pids="$pids $!"
	for t in compile link asm vet; do
		(cd "$work" && wasm -o "$work/bin/$t.wasm" "cmd/$t") &
		pids="$pids $!"
	done
	(cd "$pg/bb" && wasm -buildvcs=false -overlay "$work/ov/overlay.json" -o "$work/bin/bb.wasm" .) &
	pids="$pids $!"
	go_notices "$pg/bb" "$work/licenses/bb.txt" &
	pids="$pids $!"
	npm_ci build "$here" &
	pids="$pids $!"
	wait_all

	rm -rf "$work/opt-cache.old"
	if [ -d "$work/opt-cache" ]; then
		mv "$work/opt-cache" "$work/opt-cache.old"
	fi
	for t in go compile link asm vet bb; do
		wasm_opt "$work/bin/$t.wasm" "$work/raw/$t.wasm" &
		pids="$pids $!"
	done
	wait_all
	rm -rf "$work/opt-cache.old"

	# The GOROOT bundle holds the source of each standard package that the
	# sample imports, with its tests.
	for m in "$pg"/testdata/*/; do
		(cd "$m" && GOOS=js GOARCH=wasm go list -deps -test -f '{{if .Goroot}}{{.Dir}}{{end}}' ./...)
	done | sed "s|^$goroot/||" | LC_ALL=C sort -u >"$work/stddirs.txt"
	node "$here/mkbundle.mjs" "$work/raw/root.bin" --goroot="$goroot" --stddirs="$work/stddirs.txt"
	node "$here/mkbundle.mjs" "$work/raw/sample.bin" --mods="$pg/testdata/dg,$pg/testdata/loop"

	# The GOCACHE bundle: the cache of the usual runs on the sample, made with
	# the same tools under Node.
	cp "$work/raw/docgen.wasm" "$work/raw/go.wasm" "$work/raw/compile.wasm" "$work/raw/link.wasm" \
		"$work/raw/asm.wasm" "$work/raw/vet.wasm" "$work/raw/bb.wasm" "$work/bin/"
	EXTRA='{"GOFLAGS":"-p=4"}' node "$here/warmcache.mjs" "$work/bin" "$work/ov/wasm_exec.js" "$work/raw/gocache.bin" \
		"$work/raw/root.bin" "$work/raw/sample.bin" -- \
		'exec|/work/dg|/usr/local/bin/docgen check' \
		'!sh|/work/loop|go test -timeout 2s .' \
		'sh|/work/dg|go test -count=1 ./...' \
		'sh|/work/dg|go build ./...' >"$work/warmcache.log" 2>&1 || {
		cat "$work/warmcache.log" >&2
		exit 1
	}
	tail -n 1 "$work/warmcache.log" >&2
}

if [ "$mode" = all ]; then
	build_page &
	page_pid=$!
	build_docgen
	build_toolchain
	wait "$page_pid"
	items="go:tool:wasm docgen:tool:wasm bb:tool:wasm compile:tool:wasm link:tool:wasm asm:tool:wasm vet:tool:wasm root:bundle:bin gocache:bundle:bin sample:bundle:bin"
else
	build_page &
	page_pid=$!
	build_docgen
	wait "$page_pid"
	items="docgen:tool:wasm"
fi

rm -rf "$out"
mkdir -p "$out"
cp "$pg/web/index.html" "$pg/web/playground.mjs" "$pg/web/explorer.mjs" "$pg/web/session.mjs" "$pg/web/kernel.mjs" \
	"$pg/web/abi.mjs" "$pg/web/proc.mjs" "$work/ov/wasm_exec.js" "$work/editor-bundle.js" "$out/"
if [ "$have_zstd" = 1 ]; then
	pids=
	for it in $items; do
		f=${it%%:*}.${it##*:}
		zstd -19 -T0 -q -f "$work/raw/$f" -o "$work/z/$f.zst" &
		pids="$pids $!"
	done
	wait_all
fi

# The licenses of Go (the tools), bb, docgen and the editor bundle.
mkdir -p "$out/licenses"
if [ "$mode" = all ]; then
	cp "$goroot/LICENSE" "$out/licenses/go.txt"
	cp "$work/licenses/bb.txt" "$out/licenses/"
else
	cp "$base/licenses/go.txt" "$base/licenses/bb.txt" "$out/licenses/"
fi
cp "$work/licenses/docgen.txt" "$work/licenses/editor-bundle.txt" "$out/licenses/"
# shellcheck disable=SC2086
node "$here/mkmanifest.mjs" "$out" "$work" "${base:-}" "$gover" "$version" "$commit" $items
# Cloudflare Pages serves no file over 25 MiB.
big=$(find "$out" -type f -size +25600k)
if [ -n "$big" ]; then
	echo "build.sh: these files are over 25 MiB, the file limit of Cloudflare Pages:" >&2
	echo "$big" >&2
	exit 1
fi
echo "build.sh: wrote $out" >&2
