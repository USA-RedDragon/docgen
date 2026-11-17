package cargo

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

func TestNotFoundText(t *testing.T) {
	t.Parallel()
	if got := notFoundText("js"); !strings.Contains(got, "needs cargo") || !strings.Contains(got, "the playground has no Rust toolchain") {
		t.Errorf("js text %q", got)
	}
	if got := notFoundText("linux"); !strings.Contains(got, "needs cargo, which is not on PATH") || !strings.Contains(got, "rustup.rs") {
		t.Errorf("linux text %q", got)
	}
}

func TestCheckNoCargo(t *testing.T) {
	t.Parallel()
	r := &Runner{Cargo: filepath.Join(t.TempDir(), "no-cargo")}
	if err := r.Check(); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Check() = %v, want ErrNotFound", err)
	}
}
