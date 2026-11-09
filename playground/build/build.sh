#!/bin/sh
# build.sh builds the docgen playground.
#
#   build.sh all OUT           The whole playground: the Go toolchain, the bb
#                              shell and commands, the zstd decoder, the GOROOT
#                              and GOCACHE bundles, docgen.wasm and the page.
#   build.sh preview BASE OUT  docgen.wasm and the page of this checkout, with
#                              the toolchain of BASE, an earlier output of
#                              build.sh all. The Go version is the one of BASE.
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
*) mode= ;;
esac
if [ -z "$mode" ]; then
	echo "usage: build.sh all OUT | build.sh preview BASE OUT" >&2
	exit 2
fi

here=$(cd "$(dirname "$0")" && pwd -P)
pg=$(dirname "$here")
root=$(dirname "$pg")
abs() (mkdir -p "$1" && cd "$1" && pwd -P)
if [ "$mode" = all ]; then
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

if [ "$mode" = all ]; then
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

rm -rf "$work/raw" "$work/z" "$work/ov"
mkdir -p "$work/raw" "$work/z"
python3 -I "$here/mkoverlay.py" "$goroot" "$work/ov"

wasm() {
	GOOS=js GOARCH=wasm go build -trimpath -ldflags="-s -w" "$@"
}

npm_ci() {
	rm -rf "$work/npm-$1"
	mkdir -p "$work/npm-$1"
	cp "$2/package.json" "$2/package-lock.json" "$work/npm-$1/"
	(cd "$work/npm-$1" && npm ci --ignore-scripts --no-audit --no-fund --loglevel=error)
}

# wasm_opt writes the wasm-opt -Os output of $1 to $2. The Go builds are
# reproducible, so $work/opt-cache keeps each output by the SHA-256 of its
# input and of package-lock.json, and a second build in the same work
# directory skips wasm-opt.
wasm_opt() {
	key=$(cat "$1" "$here/package-lock.json" | sha256sum | cut -d ' ' -f 1)
	mkdir -p "$work/opt-cache"
	if [ ! -f "$work/opt-cache/$key.wasm" ]; then
		"$work/npm-build/node_modules/.bin/wasm-opt" --enable-bulk-memory --enable-sign-ext --enable-nontrapping-float-to-int -Os "$1" -o "$work/opt-cache/$key.tmp"
		mv "$work/opt-cache/$key.tmp" "$work/opt-cache/$key.wasm"
	fi
	cp "$work/opt-cache/$key.wasm" "$2"
}

# compress writes the zstd file $2 of the file $1.
compress() {
	if command -v zstd >/dev/null 2>&1; then
		zstd -19 -T0 -q -f "$1" -o "$2"
	else
		(cd "$here/zstd" && go run . "$1" "$2")
	fi
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
		--banner:js="/* CodeMirror 6 and markdown-it 15 with their dependencies, MIT licenses; see licenses/. */" \
		--outfile="$work/editor-bundle.js")
}

build_docgen() {
	# No wasm-opt: it takes about 90 s, and the zstd file of its output is larger.
	(cd "$root" && wasm -overlay "$work/ov/overlay.json" -o "$work/raw/docgen.wasm" .)
}

build_toolchain() {
	rm -rf "$work/bin" "$work/ucmd"
	mkdir -p "$work/bin"
	# bb/ucmd holds the output of bb/vendor-uroot.py, so go mod tidy works in bb.
	(cd "$pg/bb" && go mod download)
	uroot=$(cd "$pg/bb" && go list -m -f '{{.Dir}}' github.com/u-root/u-root)
	# shellcheck disable=SC2046
	python3 -I "$pg/bb/vendor-uroot.py" "$uroot" "$work/ucmd" $(cd "$pg/bb/ucmd" && ls)
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
	(cd "$pg/bb" && wasm -overlay "$work/ov/overlay.json" -o "$work/bin/bb.wasm" .) &
	pids="$pids $!"
	(cd "$pg/unzstd" && wasm -o "$work/bin/unzstd.wasm" .) &
	pids="$pids $!"
	npm_ci build "$here" &
	pids="$pids $!"
	wait_all

	for t in go compile link asm vet bb unzstd; do
		wasm_opt "$work/bin/$t.wasm" "$work/raw/$t.wasm" &
		pids="$pids $!"
	done
	wait_all

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
	items="unzstd:decoder:wasm go:tool:wasm docgen:tool:wasm bb:tool:wasm compile:tool:wasm link:tool:wasm asm:tool:wasm vet:tool:wasm root:bundle:bin gocache:bundle:bin sample:bundle:bin"
else
	build_page &
	page_pid=$!
	build_docgen
	wait "$page_pid"
	items="docgen:tool:wasm"
fi

pids=
for it in $items; do
	name=${it%%:*}
	kind=${it#*:}
	kind=${kind%%:*}
	if [ "$kind" != decoder ]; then
		compress "$work/raw/$name.${it##*:}" "$work/z/$name.${it##*:}.zst" &
		pids="$pids $!"
	fi
done
wait_all

rm -rf "$out"
mkdir -p "$out"
cp "$pg/web/index.html" "$pg/web/playground.mjs" "$pg/web/explorer.mjs" "$pg/web/session.mjs" "$pg/web/kernel.mjs" \
	"$pg/web/proc.js" "$pg/web/unzstd-worker.js" "$work/ov/wasm_exec.js" "$work/editor-bundle.js" "$out/"
cp -R "$pg/web/licenses" "$out/licenses"
# shellcheck disable=SC2086
node "$here/mkmanifest.mjs" "$out" "$work" "${base:-}" "$gover" "$version" "$commit" $items
echo "build.sh: wrote $out" >&2
