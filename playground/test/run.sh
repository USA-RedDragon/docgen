#!/bin/sh
# run.sh SITE OUT runs the browser test of the playground. SITE is a build of
# the docs site with the playground at /playground/, such as artifacts/docs-site
# after cloudflare.sh. The test serves SITE on 127.0.0.1 with its _headers,
# checks the headers of the playground and of a docs page, runs scenario.mjs in
# headless Firefox, and writes its results and screenshots to OUT.
#
# Environment:
#   PORT       port of the server (default 18350); Marionette uses PORT + 1
#   PAGE       path of the playground page (default /playground/)
#   DOCS       path of a docs page and the Playground link it must have,
#              such as "/index.html playground/"; empty skips that check
set -eu
site=$(cd "$1" && pwd -P)
mkdir -p "$2"
out=$(cd "$2" && pwd -P)
here=$(cd "$(dirname "$0")" && pwd -P)
port=${PORT:-18350}
mport=$((port + 1))
page=${PAGE:-/playground/}
base=http://127.0.0.1:$port

node "$here/serve.mjs" "$site" "$port" >"$out/serve.log" 2>&1 &
spid=$!
fpid=
cleanup() {
	[ -n "$fpid" ] && kill "$fpid" 2>/dev/null || :
	kill "$spid" 2>/dev/null || :
}
trap cleanup EXIT
i=0
until curl -fsS -o /dev/null "$base$page" 2>/dev/null; do
	i=$((i + 1))
	[ "$i" -lt 50 ] || { echo "run.sh: the server did not start" >&2; exit 1; }
	sleep 0.2
done

fail=0
headers=$(curl -fsSI "$base$page" | tr -d '\r' | tr 'A-Z' 'a-z')
for h in "cross-origin-opener-policy: same-origin" "cross-origin-embedder-policy: require-corp" "cross-origin-resource-policy: same-origin"; do
	if printf '%s\n' "$headers" | grep -qx "$h"; then
		echo "PASS $page has $h"
	else
		echo "FAIL $page has no $h"
		fail=1
	fi
done
asset=$(sed -n 's/.*"url": "\(assets\/[^"]*\)".*/\1/p' "$site$page/manifest.json" | head -n 1)
if [ -n "$asset" ] && curl -fsSI "$base$page$asset" | tr -d '\r' | grep -qix 'cache-control: public, max-age=31536000, immutable'; then
	echo "PASS $page$asset is cached for a year"
else
	echo "FAIL $page$asset has no long Cache-Control"
	fail=1
fi
# An asset of the encoding zstd needs Content-Encoding: zstd, and each asset
# needs the type application/wasm, which Cloudflare compresses.
bad=0
for a in $(node -e 'for (const f of JSON.parse(require("fs").readFileSync(process.argv[1], "utf8")).files) console.log(f.url + "," + f.encoding)' "$site$page/manifest.json"); do
	h=$(curl -fsSI "$base$page${a%,*}" | tr -d '\r' | tr 'A-Z' 'a-z')
	enc=$(printf '%s\n' "$h" | sed -n 's/^content-encoding: *//p')
	type=$(printf '%s\n' "$h" | sed -n 's/^content-type: *//p')
	want=${a#*,}
	[ "$want" = identity ] && want=
	if [ "$enc" != "$want" ] || [ "$type" != application/wasm ]; then
		echo "FAIL $page${a%,*} has Content-Encoding '$enc' and Content-Type '$type'"
		bad=1
	fi
done
if [ "$bad" = 0 ]; then
	echo "PASS each asset of $page has its Content-Encoding and the Content-Type application/wasm"
else
	fail=1
fi
if [ -n "${DOCS:-}" ]; then
	doc=${DOCS%% *}
	if curl -fsSI "$base$doc" | tr -d '\r' | tr 'A-Z' 'a-z' | grep -q '^cross-origin-'; then
		echo "FAIL $doc has a cross-origin isolation header"
		fail=1
	else
		echo "PASS $doc has no cross-origin isolation header"
	fi
fi

fpid=$("$here/firefox.sh" "$out/profile" "$mport" "$out/firefox.log")
if [ -n "${DOCS:-}" ]; then
	node "$here/docs-check.mjs" "$mport" "$base${DOCS%% *}" "${DOCS#* }" || fail=1
fi
# The status of the scenario counts, and so does its summary line: a run that
# stops before the summary fails.
if ! node "$here/scenario.mjs" "$mport" "$base$page" "$out" >"$out/scenario.log" 2>&1; then
	fail=1
fi
cat "$out/scenario.log"
if grep -q '^FAIL\|^FATAL' "$out/scenario.log" || ! grep -q '^checks: [0-9]* pass, 0 fail$' "$out/scenario.log"; then
	fail=1
fi
kill "$fpid" 2>/dev/null || :
fpid=
rm -rf "$out/profile"
exit "$fail"
