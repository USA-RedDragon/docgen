package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindRoot(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	deep := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := findRoot(deep); got != deep {
		t.Errorf("no .git: %s, want %s", got, deep)
	}
	if err := os.WriteFile(filepath.Join(root, ".git"), []byte("gitdir: x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := findRoot(deep); got != root {
		t.Errorf("a .git file above: %s, want %s", got, root)
	}
}

func TestRootFromConfigFile(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	deep := filepath.Join(root, "a", "b")
	writeTree(t, root, map[string]string{"a/b/x": "", ".docgen.yaml": "docs: d\n"})
	if got := findRoot(deep); got != root {
		t.Errorf("findRoot = %s, want %s", got, root)
	}
}
