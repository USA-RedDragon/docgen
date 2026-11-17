#!/bin/sh
# build.sh builds the docgen playground.
#
#   build.sh all OUT           The whole playground: the Go toolchain, the bb
#                              shell and commands, the GOROOT
#                              and GOCACHE bundles, docgen.wasm, the Rust
#                              helper and the page.
#   build.sh preview BASE OUT  docgen.wasm, the Rust helper and the page of
#                              this checkout, with the toolchain of BASE, an
#                              earlier output of build.sh all. The Go version
#                              is the one of BASE.
#   build.sh gover             Print the Go version of go.mod, such as go1.27.0.
#
# The Rust helper (internal/rustsrc/helper) needs cargo and the Rust target
# wasm32-wasip1 (rustup target add wasm32-wasip1). build.sh all stops without
# them. A preview without them keeps the helper of BASE, if it has one.
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

# have_rust is 1 when cargo can build for wasm32-wasip1.
have_rust=0
if command -v cargo >/dev/null 2>&1 && command -v rustc >/dev/null 2>&1 &&
	[ -d "$(rustc --print target-libdir --target wasm32-wasip1 2>/dev/null)" ]; then
	have_rust=1
elif [ "$mode" = all ]; then
	echo "build.sh: the Rust helper needs cargo and the Rust target wasm32-wasip1; install Rust and run rustup target add wasm32-wasip1" >&2
	exit 1
else
	echo "build.sh: no cargo with the target wasm32-wasip1, so the preview keeps the Rust helper of $base" >&2
fi

wasm() {
	GOOS=js GOARCH=wasm go build -trimpath -ldflags="-s -w" "$@"
}

npm_ci() {
	rm -rf "$work/npm-$1"
	mkdir -p "$work/npm-$1"
	cp "$2/package.json" "$2/package-lock.json" "$work/npm-$1/"
	(cd "$work/npm-$1" && npm ci --ignore-scripts --no-audit --no-fund --loglevel=error --fetch-timeout=60000 --fetch-retries=2)
}

# wasm_opt writes the wasm-opt -Os output of $1 to $2, with the wasm-opt
# flags that follow. The builds are reproducible, so $work/opt-cache keeps
# each output by the SHA-256 of its input, of the flags and of
# package-lock.json, and a second build in the same work directory skips
# wasm-opt. The pull request workflow keeps opt-cache with actions/cache.
# build_toolchain moves the cache to opt-cache.old first and removes what
# this build does not use, so the cache does not grow.
wasm_opt() {
	in=$1
	to=$2
	shift 2
	key=$({ cat "$in" "$here/package-lock.json" && echo "$@"; } | sha256sum | cut -d ' ' -f 1)
	mkdir -p "$work/opt-cache"
	if [ -f "$work/opt-cache.old/$key.wasm" ]; then
		mv "$work/opt-cache.old/$key.wasm" "$work/opt-cache/$key.wasm"
	fi
	if [ ! -f "$work/opt-cache/$key.wasm" ]; then
		"$work/npm-build/node_modules/.bin/wasm-opt" "$@" -Os "$in" -o "$work/opt-cache/$key.tmp"
		mv "$work/opt-cache/$key.tmp" "$work/opt-cache/$key.wasm"
	fi
	cp "$work/opt-cache/$key.wasm" "$to"
}

# The wasm-opt flags of the Go tools: the features of Go's wasm output, which
# has no target features section.
go_opt_flags="--enable-bulk-memory --enable-sign-ext --enable-nontrapping-float-to-int"

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

# Page: the editor bundle, the WASI shim bundle, the scripts and the patched
# wasm_exec.js.
build_page() {
	npm_ci web "$pg/web"
	cp "$pg/web/editor.entry.js" "$pg/web/wasi.entry.js" "$work/npm-web/"
	(cd "$work/npm-web" && node_modules/.bin/esbuild editor.entry.js --bundle --format=esm --minify --log-level=warning \
		--banner:js="/* CodeMirror 6 and markdown-it 15 with their dependencies; see licenses/editor-bundle.txt. */" \
		--metafile="$work/editor-meta.json" --outfile="$work/editor-bundle.js")
	node "$here/notices.mjs" npm "$work/editor-meta.json" "$work/npm-web/node_modules" >"$work/licenses/editor-bundle.txt"
	(cd "$work/npm-web" && node_modules/.bin/esbuild wasi.entry.js --bundle --format=esm --minify --log-level=warning \
		--banner:js="/* @bjorn3/browser_wasi_shim; see licenses/wasi-shim.txt. */" \
		--metafile="$work/wasi-meta.json" --outfile="$work/wasi-shim.js")
	node "$here/notices.mjs" npm "$work/wasi-meta.json" "$work/npm-web/node_modules" >"$work/licenses/wasi-shim.txt"
}

# wait_page waits for build_page, which runs in the background, once.
wait_page() {
	if [ -n "$page_pid" ]; then
		wait "$page_pid"
		page_pid=
	fi
}

# rshelper_cargo builds the Rust helper of docgen for wasm32-wasip1 and
# writes the license notices of its crates and of the Rust standard library.
# The target directory stays in the work directory, so a second build is
# fast.
rshelper_cargo() {
	helper=$root/internal/rustsrc/helper
	cargo build --release --locked --quiet --target wasm32-wasip1 \
		--manifest-path "$helper/Cargo.toml" --target-dir "$work/rshelper-target"
	cp "$work/rshelper-target/wasm32-wasip1/release/docgen-rs-helper.wasm" "$work/rshelper.wasm"
	cargo metadata --format-version 1 --locked --filter-platform wasm32-wasip1 --manifest-path "$helper/Cargo.toml" |
		node "$here/notices.mjs" cargo "$(rustc --print sysroot)" >"$work/licenses/rshelper.txt"
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
	rshelper_cargo &
	pids="$pids $!"
	wait_all

	rm -rf "$work/opt-cache.old"
	if [ -d "$work/opt-cache" ]; then
		mv "$work/opt-cache" "$work/opt-cache.old"
	fi
	for t in go compile link asm vet bb; do
		# shellcheck disable=SC2086
		wasm_opt "$work/bin/$t.wasm" "$work/raw/$t.wasm" $go_opt_flags &
		pids="$pids $!"
	done
	wasm_opt "$work/rshelper.wasm" "$work/raw/rshelper.wasm" &
	pids="$pids $!"
	wait_all
	rm -rf "$work/opt-cache.old"
}

# build_bundles writes the GOROOT, sample and GOCACHE bundles. The GOCACHE
# runs need the tools, docgen, the Rust helper and the WASI shim bundle.
build_bundles() {
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
		"$work/raw/asm.wasm" "$work/raw/vet.wasm" "$work/raw/bb.wasm" "$work/raw/rshelper.wasm" "$work/bin/"
	EXTRA='{"GOFLAGS":"-p=4"}' node "$here/warmcache.mjs" "$work/bin" "$work/ov/wasm_exec.js" "$work/wasi-shim.js" "$work/raw/gocache.bin" \
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
	wait_page
	build_bundles
	items="go:tool:wasm docgen:tool:wasm bb:tool:wasm compile:tool:wasm link:tool:wasm asm:tool:wasm vet:tool:wasm rshelper:tool:wasm root:bundle:bin gocache:bundle:bin sample:bundle:bin"
else
	build_page &
	page_pid=$!
	build_docgen
	items="docgen:tool:wasm"
	if [ "$have_rust" = 1 ]; then
		pids=
		npm_ci build "$here" &
		pids="$pids $!"
		rshelper_cargo &
		pids="$pids $!"
		wait_all
		wasm_opt "$work/rshelper.wasm" "$work/raw/rshelper.wasm"
		items="$items rshelper:tool:wasm"
	fi
	wait_page
fi

rm -rf "$out"
mkdir -p "$out"
cp "$pg/web/index.html" "$pg/web/playground.mjs" "$pg/web/explorer.mjs" "$pg/web/session.mjs" "$pg/web/kernel.mjs" \
	"$pg/web/abi.mjs" "$pg/web/paths.mjs" "$pg/web/bundle.mjs" "$pg/web/dom.mjs" \
	"$pg/web/proc.mjs" "$pg/web/wasiproc.mjs" "$work/ov/wasm_exec.js" "$work/editor-bundle.js" "$work/wasi-shim.js" "$out/"
if [ "$have_zstd" = 1 ]; then
	pids=
	for it in $items; do
		f=${it%%:*}.${it##*:}
		zstd -19 -T0 -q -f "$work/raw/$f" -o "$work/z/$f.zst" &
		pids="$pids $!"
	done
	wait_all
fi

# The licenses of Go (the tools), bb, docgen, the Rust helper, the editor
# bundle and the WASI shim bundle.
mkdir -p "$out/licenses"
if [ "$mode" = all ]; then
	cp "$goroot/LICENSE" "$out/licenses/go.txt"
	cp "$work/licenses/bb.txt" "$out/licenses/"
else
	cp "$base/licenses/go.txt" "$base/licenses/bb.txt" "$out/licenses/"
fi
if [ "$have_rust" = 1 ]; then
	cp "$work/licenses/rshelper.txt" "$out/licenses/"
elif [ -f "$base/licenses/rshelper.txt" ]; then
	cp "$base/licenses/rshelper.txt" "$out/licenses/"
fi
cp "$work/licenses/docgen.txt" "$work/licenses/editor-bundle.txt" "$work/licenses/wasi-shim.txt" "$out/licenses/"
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
