// Package rustsrc reads the items of a Rust crate for docgen: a helper program,
// built once with syn, reads the module tree, and a test on a copy of the crate
// prints the values of its items.
package rustsrc

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"github.com/USA-RedDragon/docgen/internal/cargo"
)

//go:embed helper/Cargo.toml helper/Cargo.lock helper/src/main.rs
var helperSource embed.FS

// MinRust is the oldest rustc that builds the helper.
const MinRust = "1.71"

// helperName is the name of the helper program.
const helperName = "docgen-rs-helper"

// A Helper builds and runs the helper program. Runs that share a Helper build
// it once.
type Helper struct {
	// CacheDir holds the built helper. An empty CacheDir is rust-helper in
	// the docgen directory of the user cache.
	CacheDir string
	// Bin, when it is not empty, is the path of a helper program that was
	// built before the run from the helper source of this docgen version,
	// such as the wasm32-wasip1 build of the playground. Path returns it with
	// no rustc check and no build, so the helper markers need no Rust
	// toolchain.
	Bin string
	mu  sync.Mutex
}

// BinEnv is the environment variable that sets Helper.Bin for a run.
const BinEnv = "DOCGEN_RUST_HELPER"

// prebuilt returns h.Bin, or an error when it is not a regular file.
func (h *Helper) prebuilt() (string, error) {
	info, err := os.Stat(h.Bin)
	if err != nil {
		return "", fmt.Errorf("the Rust helper of %s: %w", BinEnv, err)
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("the Rust helper of %s, %s, is not a regular file", BinEnv, h.Bin)
	}
	return h.Bin, nil
}

func (h *Helper) cacheDir() (string, error) {
	if h.CacheDir != "" {
		return h.CacheDir, nil
	}
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "docgen", "rust-helper"), nil
}

var releaseLine = regexp.MustCompile(`(?m)^release: (\d+)\.(\d+)\.(\d+)`)

// ErrNoRustc is the error of a missing rustc when docgen has to build the
// helper.
var ErrNoRustc = errors.New("reading Rust code needs rustc to build the docgen helper, and there is no rustc; install a Rust toolchain from https://rustup.rs, or set DOCGEN_RUST_HELPER to a prebuilt helper")

// rustc returns the output of rustc -vV in dir, run with r.Exec. A rustc older
// than MinRust is an error, and a missing rustc is ErrNoRustc.
func (h *Helper) rustc(ctx context.Context, r *cargo.Runner, dir string) (string, error) {
	env, err := r.Env()
	if err != nil {
		return "", err
	}
	rustc := "rustc"
	for _, e := range env {
		if v, ok := strings.CutPrefix(e, "RUSTC="); ok && v != "" {
			rustc = v
		}
	}
	out, errOut, err := r.Exec(ctx, dir, rustc, "-vV")
	if errors.Is(err, exec.ErrNotFound) || errors.Is(err, fs.ErrNotExist) {
		return "", ErrNoRustc
	}
	if err != nil {
		return "", fmt.Errorf("rustc -vV: %w\n%s", err, strings.TrimSpace(errOut))
	}
	if err := checkRelease(out); err != nil {
		return "", err
	}
	return out, nil
}

// checkRelease returns an error when the output of rustc -vV has no release
// line or a release older than MinRust.
func checkRelease(out string) error {
	m := releaseLine.FindStringSubmatch(out)
	if m == nil {
		return fmt.Errorf("rustc -vV printed no release line:\n%s", out)
	}
	if !atLeast(m[1], m[2], MinRust) {
		return fmt.Errorf("rustc %s.%s.%s is too old for docgen, which needs Rust %s or later to read Rust code; run rustup update", m[1], m[2], m[3], MinRust)
	}
	return nil
}

// atLeast reports whether the version major.minor is min or later.
func atLeast(major, minor, minVersion string) bool {
	wantMajor, wantMinor, _ := strings.Cut(minVersion, ".")
	a, _ := strconv.Atoi(major)
	b, _ := strconv.Atoi(minor)
	x, _ := strconv.Atoi(wantMajor)
	y, _ := strconv.Atoi(wantMinor)
	return a > x || a == x && b >= y
}

// Path returns the path of the helper program, with r for the commands: Bin
// when it is set, or else a build in the cache. The first call for a version
// of the helper source and of rustc builds it with cargo build --release
// --locked in a new directory and renames it into the cache, so that a run at
// the same time never reads a part of it.
func (h *Helper) Path(ctx context.Context, r *cargo.Runner) (string, error) {
	if h.Bin != "" {
		return h.prebuilt()
	}
	dir, err := h.cacheDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", err
	}
	version, err := h.rustc(ctx, r, dir)
	if err != nil {
		return "", err
	}
	key, err := helperKey(version)
	if err != nil {
		return "", err
	}
	exe := ""
	if runtime.GOOS == "windows" {
		exe = ".exe"
	}
	bin := filepath.Join(dir, helperName+"-"+key+exe)
	h.mu.Lock()
	defer h.mu.Unlock()
	if info, err := os.Stat(bin); err == nil && info.Mode().IsRegular() {
		return bin, nil
	}
	work, err := os.MkdirTemp(dir, "build-")
	if err != nil {
		return "", err
	}
	defer func() { _ = os.RemoveAll(work) }()
	if err := writeSource(work); err != nil {
		return "", err
	}
	target := filepath.Join(work, "target")
	_, errOut, err := r.Run(ctx, work, "build", "--release", "--locked", "--manifest-path", filepath.Join(work, "Cargo.toml"), "--target-dir", target)
	if err != nil {
		if errors.Is(err, cargo.ErrNotFound) {
			return "", err
		}
		return "", fmt.Errorf("cargo build of the docgen Rust helper: %w\n%s", err, strings.TrimSpace(errOut))
	}
	if err := os.Rename(filepath.Join(target, "release", helperName+exe), bin); err != nil {
		if info, serr := os.Stat(bin); serr == nil && info.Mode().IsRegular() {
			return bin, nil
		}
		return "", err
	}
	return bin, nil
}

// helperKey returns the hash of the helper source and of the rustc version.
func helperKey(rustc string) (string, error) {
	sum := sha256.New()
	err := fs.WalkDir(helperSource, "helper", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := helperSource.ReadFile(p)
		if err != nil {
			return err
		}
		fmt.Fprintf(sum, "%s\x00%d\x00", p, len(b))
		sum.Write(b)
		return nil
	})
	if err != nil {
		return "", err
	}
	sum.Write([]byte(rustc))
	return hex.EncodeToString(sum.Sum(nil))[:16], nil
}

func writeSource(dir string) error {
	return fs.WalkDir(helperSource, "helper", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		to := filepath.Join(dir, filepath.FromSlash(strings.TrimPrefix(p, "helper")))
		if d.IsDir() {
			return os.MkdirAll(to, 0o750)
		}
		b, err := helperSource.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(to, b, 0o600)
	})
}

// Read runs the helper bin on the root files of crate targets with r.Exec, and
// returns one Crate for each root, in order.
func Read(ctx context.Context, r *cargo.Runner, bin string, roots []string) ([]*Crate, error) {
	out, errOut, err := r.Exec(ctx, filepath.Dir(bin), bin, roots...)
	if err != nil {
		return nil, fmt.Errorf("%s: %w\n%s", helperName, err, strings.TrimSpace(errOut))
	}
	return ParseCrates([]byte(out))
}

// ParseCrates reads the JSON output of the helper.
func ParseCrates(b []byte) ([]*Crate, error) {
	var crates []*Crate
	if err := json.Unmarshal(b, &crates); err != nil {
		return nil, fmt.Errorf("%s output: %w", helperName, err)
	}
	for _, c := range crates {
		c.index()
	}
	return crates, nil
}
