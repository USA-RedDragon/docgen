#!/usr/bin/env bash
# Tests install.sh offline, with a fake gh that serves release assets from a
# directory.
#
#   test.sh [ARCHIVE_DIR]
#
# ARCHIVE_DIR holds release archives of docgen, such as the dist directory of
# goreleaser. With no ARCHIVE_DIR, the test makes archives with a script for the
# docgen binary. The test also builds docgen from the source, so it needs go.
set -euo pipefail

here=$(cd "$(dirname "$0")" && pwd)
install=$here/install.sh
work=$(mktemp -d "${RUNNER_TEMP:-${TMPDIR:-/tmp}}/docgen-action-test.XXXXXX")
trap 'rm -rf "$work"' EXIT

failures=0
fail() {
  echo "FAIL: $1"
  failures=$((failures + 1))
}
pass() {
  echo "ok: $1"
}

mkdir -p "$work/bin" "$work/assets"
cat >"$work/bin/gh" <<'EOF'
#!/usr/bin/env bash
# A fake gh: release download TAG --repo R --pattern P... --dir D.
set -euo pipefail
echo "gh $*" >>"$FAKE_GH_LOG"
[ "$1 $2" = "release download" ] || { echo "fake gh: unknown command $*" >&2; exit 1; }
tag=$3
shift 3
patterns=()
dir=.
while [ $# -gt 0 ]; do
  case $1 in
  --repo) shift 2 ;;
  --pattern) patterns+=("$2"); shift 2 ;;
  --dir) dir=$2; shift 2 ;;
  *) echo "fake gh: unknown flag $1" >&2; exit 1 ;;
  esac
done
src=$FAKE_GH_ASSETS/$tag
[ -d "$src" ] || { echo "release not found" >&2; exit 1; }
n=0
for p in "${patterns[@]}"; do
  for f in "$src"/$p; do
    [ -f "$f" ] || continue
    cp "$f" "$dir/"
    n=$((n + 1))
  done
done
[ "$n" -gt 0 ] || { echo "no assets match the file pattern" >&2; exit 1; }
EOF
chmod +x "$work/bin/gh"

export FAKE_GH_LOG=$work/gh.log FAKE_GH_ASSETS=$work/assets
export PATH=$work/bin:$PATH
export RUNNER_TEMP=$work/runner-temp
mkdir -p "$RUNNER_TEMP"

good=$FAKE_GH_ASSETS/v9.9.9
mkdir -p "$good"
if [ -n "${1:-}" ]; then
  version=$(find "$1" -maxdepth 1 -name 'docgen_*_linux_amd64.tar.gz' | head -n 1 | sed 's/.*docgen_\(.*\)_linux_amd64.tar.gz/\1/')
  [ -n "$version" ] || { echo "no docgen_*_linux_amd64.tar.gz in $1"; exit 1; }
  for f in "$1"/docgen_"$version"_*; do
    cp "$f" "$good/${f##*/docgen_"$version"}"
  done
  for f in "$good"/_*; do
    mv "$f" "$good/docgen_9.9.9${f##*/}"
  done
  (cd "$good" && sha256sum docgen_* >checksums.txt)
  want_output=
else
  mkdir "$work/fixture"
  printf '#!/bin/sh\necho "docgen fixture $*"\n' >"$work/fixture/docgen"
  chmod +x "$work/fixture/docgen"
  cp "$work/fixture/docgen" "$work/fixture/docgen.exe"
  for os in linux darwin; do
    for arch in amd64 arm64; do
      tar -czf "$good/docgen_9.9.9_${os}_${arch}.tar.gz" -C "$work/fixture" docgen
    done
  done
  for arch in amd64 arm64; do
    (cd "$work/fixture" && zip -q "$good/docgen_9.9.9_windows_${arch}.zip" docgen.exe)
  done
  (cd "$good" && sha256sum docgen_* >checksums.txt)
  want_output="docgen fixture -h"
fi

# download VERSION, with the runner OS and architecture of the arguments.
# Prints the exit code.
run_download() {
  local os=$1 arch=$2 version=$3 dir=$4
  set +e
  RUNNER_OS=$os RUNNER_ARCH=$arch "$install" download "$version" "$dir" >"$work/out" 2>&1
  local rc=$?
  set -e
  echo "$rc"
}

for runner in Linux:X64 Linux:ARM64 macOS:ARM64 Windows:X64; do
  os=${runner%:*}
  arch=${runner#*:}
  exe=
  if [ "$os" = Windows ]; then exe=.exe; fi
  if [ "$os" = Windows ] && ! command -v unzip >/dev/null; then
    echo "skip: $runner, no unzip"
    continue
  fi
  dir=$work/good-$os-$arch
  rc=$(run_download "$os" "$arch" v9.9.9 "$dir")
  if [ "$rc" != 0 ]; then
    fail "good checksum on $runner: exit $rc: $(cat "$work/out")"
  elif [ ! -x "$dir/docgen$exe" ]; then
    fail "good checksum on $runner: no docgen$exe in $dir"
  else
    pass "good checksum on $runner"
  fi
done

if [ "$(uname -s)-$(uname -m)" = Linux-x86_64 ]; then
  out=$("$work/good-Linux-X64/docgen" -h 2>&1 || true)
  if [ -n "$want_output" ] && [ "$out" != "$want_output" ]; then
    fail "the downloaded docgen printed '$out'"
  elif [ -z "$want_output" ] && ! grep -q 'docgen' <<<"$out"; then
    fail "the downloaded docgen printed '$out'"
  else
    pass "the downloaded docgen runs"
  fi
fi

bad=$FAKE_GH_ASSETS/v9.9.8
mkdir -p "$bad"
cp "$good/docgen_9.9.9_linux_amd64.tar.gz" "$bad/docgen_9.9.8_linux_amd64.tar.gz"
printf '%064d  docgen_9.9.8_linux_amd64.tar.gz\n' 0 >"$bad/checksums.txt"
rc=$(run_download Linux X64 v9.9.8 "$work/bad")
if [ "$rc" != 1 ]; then
  fail "bad checksum: exit $rc, want 1"
elif [ -e "$work/bad/docgen" ]; then
  fail "bad checksum: docgen was installed"
elif ! grep -q '::error::The sha256 of docgen_9.9.8_linux_amd64.tar.gz' "$work/out"; then
  fail "bad checksum: output $(cat "$work/out")"
else
  pass "bad checksum fails"
fi

nosum=$FAKE_GH_ASSETS/v9.9.5
mkdir -p "$nosum"
cp "$good/docgen_9.9.9_linux_amd64.tar.gz" "$nosum/docgen_9.9.5_linux_amd64.tar.gz"
grep -v linux_amd64 "$good/checksums.txt" >"$nosum/checksums.txt"
rc=$(run_download Linux X64 v9.9.5 "$work/nosum")
if [ "$rc" != 1 ]; then fail "no checksum line: exit $rc, want 1"; else pass "no checksum line fails"; fi

missing=$FAKE_GH_ASSETS/v9.9.7
mkdir -p "$missing"
cp "$good/checksums.txt" "$missing/checksums.txt"
rc=$(run_download Linux X64 v9.9.7 "$work/missing")
if [ "$rc" != 2 ]; then fail "missing asset: exit $rc, want 2"; else pass "missing asset falls back"; fi

rc=$(run_download Linux X64 v9.9.6 "$work/norelease")
if [ "$rc" != 2 ]; then fail "no release: exit $rc, want 2"; else pass "no release falls back"; fi

rc=$(run_download Linux ARM v9.9.9 "$work/arm")
if [ "$rc" != 2 ]; then fail "unsupported architecture: exit $rc, want 2"; else pass "unsupported architecture falls back"; fi
rc=$(run_download FreeBSD X64 v9.9.9 "$work/freebsd")
if [ "$rc" != 2 ]; then fail "unsupported OS: exit $rc, want 2"; else pass "unsupported OS falls back"; fi

# resolve REF REPOSITORY VERSION WANT_MODE WANT_VERSION WANT_FALLBACK
check_resolve() {
  local out=$work/resolve.out
  : >"$out"
  set +e
  GITHUB_OUTPUT=$out RUNNER_OS=Linux RUNNER_ARCH=X64 ACTION_PATH=$here/.. \
    ACTION_REF=$1 ACTION_REPOSITORY=$2 INPUT_VERSION=$3 "$install" resolve >"$work/out" 2>&1
  local rc=$?
  set -e
  local got
  got=$(grep -E '^(mode|version|fallback)=' "$out" | tr '\n' ' ')
  local want="mode=$4 version=$5 fallback=$6 "
  if [ "$rc" != 0 ] || [ "$got" != "$want" ]; then
    fail "resolve ref '$1' repository '$2' version '$3': exit $rc, '$got', want '$want'"
  else
    pass "resolve ref '$1' repository '$2' version '$3': $4"
  fi
}
check_resolve v1.2.3 USA-RedDragon/docgen '' release v1.2.3 build
check_resolve v1.2.3-rc.1 usa-reddragon/docgen '' release v1.2.3-rc.1 build
check_resolve v1.2.3 someone/fork '' build '' ''
check_resolve main USA-RedDragon/docgen '' build '' ''
check_resolve 0123456789abcdef0123456789abcdef01234567 USA-RedDragon/docgen '' build '' ''
check_resolve 'v1.2.3;id' USA-RedDragon/docgen '' build '' ''
check_resolve v1 USA-RedDragon/docgen '' build '' ''
check_resolve '' '' '' build '' ''
check_resolve main USA-RedDragon/docgen v0.4.0 release v0.4.0 install
check_resolve main USA-RedDragon/docgen latest install latest ''

set +e
GITHUB_OUTPUT=$work/resolve.out RUNNER_OS=Linux RUNNER_ARCH=X64 ACTION_PATH=$here/.. \
  ACTION_REF= ACTION_REPOSITORY= INPUT_VERSION=$'v1.0.0\nbin=/x' "$install" resolve >"$work/out" 2>&1
rc=$?
set -e
if [ "$rc" != 1 ]; then fail "a version with a newline: exit $rc, want 1"; else pass "a version with a newline fails"; fi

# A release tag with no release builds docgen from the source of the action.
out=$work/install.out
: >"$out"
set +e
GITHUB_OUTPUT=$out RUNNER_OS=Linux RUNNER_ARCH=X64 ACTION_PATH=$here/.. MODE=release VERSION=v9.9.6 \
  FALLBACK=build DIR=$work/fallback "$install" install >"$work/out" 2>&1
rc=$?
set -e
if [ "$rc" != 0 ] || ! grep -q '^built=true$' "$out" || ! "$work/fallback/docgen" -h >/dev/null 2>&1; then
  fail "fallback to build: exit $rc: $(cat "$work/out")"
else
  pass "a release tag with no release builds from source"
fi

if [ "$failures" != 0 ]; then
  echo "$failures failed"
  exit 1
fi
echo "all passed"
