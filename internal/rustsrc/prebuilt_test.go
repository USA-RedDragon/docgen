package rustsrc

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/USA-RedDragon/docgen/internal/cargo"
)

func TestHelperBin(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	bin := filepath.Join(dir, "docgen-rs-helper.wasm")
	if err := os.WriteFile(bin, []byte("\x00asm"), 0o600); err != nil {
		t.Fatal(err)
	}
	// A Runner with no Session cannot run a command, so a rustc check or a
	// build would fail the test.
	r := &cargo.Runner{Cargo: filepath.Join(dir, "no-cargo")}
	h := &Helper{CacheDir: filepath.Join(dir, "cache"), Bin: bin}
	got, err := h.Path(t.Context(), r)
	if err != nil || got != bin {
		t.Fatalf("Path() = %q, %v; want %q", got, err, bin)
	}
	if _, err := os.Stat(h.CacheDir); !os.IsNotExist(err) {
		t.Errorf("Path made the cache directory: %v", err)
	}
	for _, tc := range []struct {
		bin, want string
	}{
		{filepath.Join(dir, "missing"), "the Rust helper of DOCGEN_RUST_HELPER: stat "},
		{dir, "the Rust helper of DOCGEN_RUST_HELPER, " + dir + ", is not a regular file"},
	} {
		h := &Helper{Bin: tc.bin}
		if _, err := h.Path(t.Context(), r); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("Path() with Bin %s: error %v, want one with %q", tc.bin, err, tc.want)
		}
	}
}
