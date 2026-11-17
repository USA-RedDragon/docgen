package cargo

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/USA-RedDragon/docgen/internal/gocmd"
)

const fixture = "../cli/testdata/rust"

func needCargo(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("cargo"); err != nil {
		t.Skip("cargo is not on PATH; install a Rust toolchain to run this test")
	}
}

func newRunner(t *testing.T) *Runner {
	t.Helper()
	s := &gocmd.Session{}
	t.Cleanup(func() { _ = s.Close() })
	return &Runner{Session: s}
}

func copyDir(t *testing.T, src string) string {
	t.Helper()
	dst := t.TempDir()
	err := filepath.WalkDir(src, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		if d.IsDir() {
			return os.MkdirAll(filepath.Join(dst, rel), 0o750)
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dst, rel), b, 0o600)
	})
	if err != nil {
		t.Fatal(err)
	}
	return dst
}

func TestOpenFixture(t *testing.T) {
	t.Parallel()
	needCargo(t)
	r := newRunner(t)
	ws, err := r.Open(t.Context(), fixture)
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(ws.Packages))
	for _, p := range ws.Packages {
		names = append(names, p.Name)
	}
	if want := []string{"cargo-b-cli", "cargo-b-core", "core-a", "core-a-bin", "feat-gate", "limits", "run-c", "run-c-broken", "tool"}; !slices.Equal(names, want) {
		t.Fatalf("packages %v, want %v", names, want)
	}
	limits, err := ws.Package("limits")
	if err != nil {
		t.Fatal(err)
	}
	if limits.Version != "0.1.0" || limits.Edition != "2024" || limits.RustVersion != "1.85" {
		t.Errorf("limits: %+v", limits)
	}
	if lib := limits.Lib(); lib == nil || filepath.Base(lib.Path) != "lib.rs" || lib.Name != "limits" {
		t.Errorf("lib target: %+v", lib)
	}
	if limits.Target("example", "show") == nil {
		t.Errorf("no example show in %+v", limits.Targets)
	}
	if len(limits.LocalDeps) != 0 {
		t.Errorf("limits deps %+v", limits.LocalDeps)
	}
	tool, err := ws.Package("tool")
	if err != nil {
		t.Fatal(err)
	}
	if tool.Version != "0.2.0" || tool.Edition != "2021" || tool.RustVersion != "" {
		t.Errorf("tool: %+v", tool)
	}
	if tool.Target("bin", "tool") == nil || tool.Lib() != nil {
		t.Errorf("tool targets: %+v", tool.Targets)
	}
	if len(tool.LocalDeps) != 1 || tool.LocalDeps[0].Name != "limits" || tool.LocalDeps[0].Kind != "normal" || tool.LocalDeps[0].Path != limits.Dir {
		t.Errorf("tool deps: %+v", tool.LocalDeps)
	}
}

func TestOpenDoesNotChangeFixture(t *testing.T) {
	t.Parallel()
	needCargo(t)
	before, err := os.ReadFile(filepath.Join(fixture, "Cargo.lock"))
	if err != nil {
		t.Fatal(err)
	}
	r := newRunner(t)
	if _, err := r.Open(t.Context(), fixture); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(filepath.Join(fixture, "Cargo.lock"))
	if string(before) != string(after) {
		t.Error("Cargo.lock changed")
	}
	if _, err := os.Stat(filepath.Join(fixture, "target")); err == nil {
		t.Error("a target directory appeared in the fixture")
	}
}

func TestUnknownCrate(t *testing.T) {
	t.Parallel()
	needCargo(t)
	ws, err := newRunner(t).Open(t.Context(), fixture)
	if err != nil {
		t.Fatal(err)
	}
	_, err = ws.Package("nope")
	if err == nil || !strings.Contains(err.Error(), `"nope"`) || !strings.Contains(err.Error(), "limits, run-c, run-c-broken, tool") {
		t.Errorf("error %v", err)
	}
}

func TestNoManifest(t *testing.T) {
	t.Parallel()
	_, err := newRunner(t).Open(t.Context(), t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "no Cargo.toml") {
		t.Errorf("error %v", err)
	}
}

func TestStaleLock(t *testing.T) {
	t.Parallel()
	needCargo(t)
	dir := copyDir(t, fixture)
	lock := filepath.Join(dir, "Cargo.lock")
	if err := os.WriteFile(lock, []byte("# stale\nversion = 4\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, errOut, err := newRunner(t).Run(t.Context(), dir, "build", "--locked", "--offline", "-p", "limits")
	if err == nil {
		t.Fatal("build with a stale lock succeeded")
	}
	err = LockedError("cargo build", err, errOut)
	if !strings.Contains(err.Error(), "Cargo.lock is out of date") {
		t.Fatalf("error %v", err)
	}
	if b, _ := os.ReadFile(lock); string(b) != "# stale\nversion = 4\n" {
		t.Error("Cargo.lock changed")
	}
}

func TestCargoMissing(t *testing.T) {
	t.Parallel()
	r := newRunner(t)
	r.Cargo = filepath.Join(t.TempDir(), "cargo")
	_, err := r.Open(t.Context(), fixture)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("error %v", err)
	}
}

func TestEnv(t *testing.T) {
	t.Parallel()
	r := newRunner(t)
	r.Environ = []string{"PATH=/bin", "CARGO_TARGET_DIR=/user/target", "CARGO_BUILD_JOBS=99"}
	r.Jobs = 3
	env, err := r.Env("X=1")
	if err != nil {
		t.Fatal(err)
	}
	tmp, _ := r.Session.TempDir()
	get := func(k string) []string {
		var out []string
		for _, e := range env {
			if v, ok := strings.CutPrefix(e, k+"="); ok {
				out = append(out, v)
			}
		}
		return out
	}
	if got, want := get("CARGO_TARGET_DIR"), []string{filepath.Join(tmp, TargetDirName)}; !slices.Equal(got, want) {
		t.Errorf("CARGO_TARGET_DIR %q, want %q", got, want)
	}
	if got := get("CARGO_BUILD_JOBS"); !slices.Equal(got, []string{"3"}) {
		t.Errorf("CARGO_BUILD_JOBS %q", got)
	}
	if got := get("X"); !slices.Equal(got, []string{"1"}) {
		t.Errorf("X %q", got)
	}
	if !strings.Contains(tmp, "run-") {
		t.Errorf("session dir %s", tmp)
	}
}

func TestBuildUsesSessionTargetDir(t *testing.T) {
	t.Parallel()
	needCargo(t)
	r := newRunner(t)
	_, errOut, err := r.Run(t.Context(), fixture, "build", "--locked", "--offline", "-p", "limits")
	if err != nil {
		t.Fatalf("%v\n%s", err, errOut)
	}
	tmp, _ := r.Session.TempDir()
	if _, err := os.Stat(filepath.Join(tmp, TargetDirName, "debug")); err != nil {
		t.Error(err)
	}
	if _, err := os.Stat(filepath.Join(fixture, "target")); err == nil {
		t.Error("a target directory appeared in the fixture")
	}
}

func TestAcquire(t *testing.T) {
	t.Parallel()
	needCargo(t)
	var held, released atomic.Int32
	r := newRunner(t)
	r.Acquire = func(context.Context) (func(), error) {
		held.Add(1)
		return func() { released.Add(1) }, nil
	}
	if _, err := r.Open(t.Context(), fixture); err != nil {
		t.Fatal(err)
	}
	if held.Load() != 1 || released.Load() != 1 {
		t.Errorf("held %d, released %d", held.Load(), released.Load())
	}
	r.Acquire = func(context.Context) (func(), error) { return nil, context.Canceled }
	if _, err := r.Open(t.Context(), fixture); !errors.Is(err, context.Canceled) {
		t.Errorf("error %v", err)
	}
}

func TestExec(t *testing.T) {
	t.Parallel()
	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("no sh")
	}
	var held atomic.Int32
	r := newRunner(t)
	r.Environ = []string{"PATH=" + os.Getenv("PATH")}
	r.Acquire = func(context.Context) (func(), error) {
		held.Add(1)
		return func() {}, nil
	}
	out, _, err := r.Exec(t.Context(), t.TempDir(), sh, "-c", "echo $CARGO_TERM_COLOR")
	if err != nil {
		t.Fatal(err)
	}
	if out != "never\n" || held.Load() != 1 {
		t.Errorf("output %q, %d slots", out, held.Load())
	}
}

func TestCancel(t *testing.T) {
	t.Parallel()
	needCargo(t)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := newRunner(t).Open(ctx, fixture); err == nil {
		t.Error("no error after cancel")
	}
}
