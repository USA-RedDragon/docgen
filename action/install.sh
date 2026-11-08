#!/usr/bin/env bash
# Installs docgen for action.yml.
#
#   install.sh resolve           Write the install plan to $GITHUB_OUTPUT.
#   install.sh install           Install docgen by the plan, and write its path.
#   install.sh download VER DIR  Download and verify the release archive.
#
# resolve reads INPUT_VERSION, ACTION_REF, ACTION_REPOSITORY and ACTION_PATH.
# install reads MODE, VERSION, FALLBACK, DIR and ACTION_PATH. download exits 1
# on a bad checksum and 2 when no release archive is available, so that install
# builds docgen instead.
set -euo pipefail

repo=USA-RedDragon/docgen
semver='^v[0-9]+\.[0-9]+\.[0-9]+([-+][0-9A-Za-z.-]+)?$'

exe=
if [ "${RUNNER_OS:-}" = Windows ]; then exe=.exe; fi

need_go() {
  if ! command -v go >/dev/null; then
    echo "::error::$1 needs go on PATH. Add actions/setup-go before this step, or use a release tag such as v0.1.0."
    exit 1
  fi
}

# sha256 prints the sha256 of the file, or of standard input with no file.
sha256() {
  if command -v sha256sum >/dev/null; then
    sha256sum "$@" | cut -d ' ' -f 1
  else
    shasum -a 256 "$@" | cut -d ' ' -f 1
  fi
}

# The hash of the files that go build reads, and of the go settings.
source_hash() {
  (
    cd "$ACTION_PATH"
    go env GOVERSION GOOS GOARCH CGO_ENABLED GOFLAGS
    find go.mod go.sum ./*.go internal -type f ! -path '*/testdata/*' ! -name '*_test.go' |
      LC_ALL=C sort | while IFS= read -r f; do
        printf '%s %s\n' "$(sha256 "$f")" "$f"
      done
  ) | sha256
}

resolve() {
  local version=$INPUT_VERSION from=input
  if [ -z "$version" ]; then
    from=ref
    # github.action_ref is the ref of the outer action when docgen runs inside
    # another composite action, so the ref counts only with the docgen repository.
    repository=$(printf '%s' "${ACTION_REPOSITORY:-}" | tr '[:upper:]' '[:lower:]')
    if [ "$repository" = "$(printf '%s' "$repo" | tr '[:upper:]' '[:lower:]')" ] && [[ ${ACTION_REF:-} =~ $semver ]]; then
      version=$ACTION_REF
    fi
  elif ! [[ $version =~ ^[A-Za-z0-9._+/-]+$ ]]; then
    echo "::error::version '$version' must be a tag, a branch, a commit or latest."
    exit 1
  fi
  local mode fallback= dir key=
  if [[ $version =~ $semver ]]; then
    mode=release
    if [ "$from" = input ]; then fallback=install; else fallback=build; fi
    dir=$RUNNER_TEMP/docgen-release/$version
  elif [ -n "$version" ]; then
    mode=install
    need_go "Installing docgen $version"
    dir=$(mktemp -d "$RUNNER_TEMP/docgen.XXXXXX")
  else
    mode=build
    need_go "Building docgen from the source of the action"
    key=docgen-build-${RUNNER_OS}-${RUNNER_ARCH}-$(source_hash)
    dir=$RUNNER_TEMP/docgen-build/$key
  fi
  {
    echo "mode=$mode"
    echo "version=$version"
    echo "fallback=$fallback"
    echo "key=$key"
    echo "dir=$dir"
  } >>"$GITHUB_OUTPUT"
}

# download exits 2 for each case that a build from source can fix.
download() {
  local version=$1 dir=$2 os arch ext
  case ${RUNNER_OS:-} in
  Linux) os=linux ;;
  macOS) os=darwin ;;
  Windows) os=windows ;;
  *) echo "::warning::No docgen release archive for the runner OS '${RUNNER_OS:-}'."; return 2 ;;
  esac
  case ${RUNNER_ARCH:-} in
  X64) arch=amd64 ;;
  ARM64) arch=arm64 ;;
  *) echo "::warning::No docgen release archive for the runner architecture '${RUNNER_ARCH:-}'."; return 2 ;;
  esac
  if [ "$os" = windows ]; then ext=zip; else ext=tar.gz; fi
  if ! command -v gh >/dev/null; then
    echo "::warning::gh is not on PATH, so docgen cannot download the release."
    return 2
  fi
  local archive=docgen_${version#v}_${os}_${arch}.$ext
  local dl
  dl=$(mktemp -d "$RUNNER_TEMP/docgen-download.XXXXXX")
  if ! GH_HOST=github.com gh release download "$version" --repo "$repo" --pattern "$archive" --pattern checksums.txt --dir "$dl" 2>"$dl/gh.err"; then
    echo "::warning::gh release download $version failed: $(head -c 300 "$dl/gh.err" | tr '\n' ' ')"
    return 2
  fi
  local f
  for f in "$archive" checksums.txt; do
    if [ ! -f "$dl/$f" ]; then
      echo "::warning::Release $version has no asset $f."
      return 2
    fi
  done
  local want got
  want=$(awk -v n="$archive" '$2 == n || $2 == "*" n { print $1 }' "$dl/checksums.txt")
  if ! [[ $want =~ ^[0-9a-f]{64}$ ]]; then
    echo "::error::checksums.txt of release $version has no single sha256 for $archive."
    return 1
  fi
  got=$(sha256 "$dl/$archive")
  if [ "$got" != "$want" ]; then
    echo "::error::The sha256 of $archive is $got; checksums.txt of release $version gives $want."
    return 1
  fi
  echo "Verified $archive: sha256 $got."
  local out=$dl/out
  mkdir "$out"
  if [ "$ext" = zip ]; then
    if command -v unzip >/dev/null; then
      unzip -q "$dl/$archive" "docgen$exe" -d "$out"
    elif [ -x "${SYSTEMROOT:-C:/Windows}/System32/tar.exe" ]; then
      "${SYSTEMROOT:-C:/Windows}/System32/tar.exe" -xf "$dl/$archive" -C "$out" "docgen$exe"
    else
      echo "::warning::No unzip or tar.exe to extract $archive."
      return 2
    fi
  else
    tar -xzf "$dl/$archive" -C "$out" docgen
  fi
  if [ ! -f "$out/docgen$exe" ]; then
    echo "::error::$archive holds no docgen$exe."
    return 1
  fi
  chmod +x "$out/docgen$exe"
  mkdir -p "$dir"
  mv "$out/docgen$exe" "$dir/docgen$exe"
  rm -rf "$dl"
}

build() {
  need_go "Building docgen from the source of the action"
  mkdir -p "$DIR"
  (cd "$ACTION_PATH" && GOWORK=off go build -trimpath -o "$DIR/docgen$exe" .)
  echo "built=true" >>"$GITHUB_OUTPUT"
}

go_install() {
  need_go "Installing docgen $VERSION"
  mkdir -p "$DIR"
  GOBIN=$DIR go install "github.com/USA-RedDragon/docgen@$VERSION"
}

install() {
  local bin=$DIR/docgen$exe
  if [ -x "$bin" ]; then
    echo "Using $bin."
  else
    case $MODE in
    release)
      set +e
      (set -e; download "$VERSION" "$DIR")
      rc=$?
      set -e
      if [ "$rc" = 2 ]; then
        if [ "$FALLBACK" = build ]; then
          echo "Building docgen $VERSION from the source of the action."
          build
        else
          echo "Installing docgen $VERSION with go install."
          go_install
        fi
      elif [ "$rc" != 0 ]; then
        exit "$rc"
      fi
      ;;
    build) build ;;
    install) go_install ;;
    *) echo "::error::unknown mode '$MODE'."; exit 1 ;;
    esac
  fi
  echo "bin=$bin" >>"$GITHUB_OUTPUT"
}

case ${1:-} in
resolve) resolve ;;
install) install ;;
download) download "$2" "$3" ;;
*) echo "usage: install.sh resolve | install | download VERSION DIR" >&2; exit 2 ;;
esac
