package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// unreadable makes the path p of root unreadable until the end of the test. It
// skips a test run as root, which reads anything.
func unreadable(t *testing.T, root, p string) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("root reads each file")
	}
	full := filepath.Join(root, p)
	info, err := os.Stat(full)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(full, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(full, info.Mode().Perm()); err != nil {
			t.Error(err)
		}
	})
}

func gitRepo(t *testing.T, files map[string]string) string {
	t.Helper()
	root := unusedRepo(t, files)
	if out, err := exec.CommandContext(t.Context(), "git", "-C", root, "init", "-q").CombinedOutput(); err != nil {
		t.Skipf("git init: %v\n%s", err, out)
	}
	return root
}

func TestIgnoredUnreadableDirectory(t *testing.T) {
	t.Parallel()
	root := gitRepo(t, map[string]string{".gitignore": "data/\n", "data/x.sh": regionText})
	unreadable(t, root, "data")
	for _, args := range [][]string{{"check"}, {"check", "."}, {"update"}, {"update", "."}} {
		if rc, log := docgen(t, root, args...); rc != 0 {
			t.Errorf("%q: rc %d, log:\n%s", args, rc, log)
		}
	}
}

func TestIgnoredUnreadableFile(t *testing.T) {
	t.Parallel()
	root := gitRepo(t, map[string]string{".gitignore": "*.key\n", "a.key": "x\n"})
	unreadable(t, root, "a.key")
	if rc, log := docgen(t, root, "check"); rc != 0 {
		t.Errorf("rc %d, log:\n%s", rc, log)
	}
}

func TestUnreadableDirectoryIsOneError(t *testing.T) {
	t.Parallel()
	root := gitRepo(t, map[string]string{"data/x.sh": "x\n", "docs/p.md": "Text.\n"})
	unreadable(t, root, "data")
	rc, log := docgen(t, root, "check")
	if rc != 1 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
	if n := strings.Count(log, "permission denied"); n != 1 {
		t.Errorf("%d errors, log:\n%s", n, log)
	}
}

func TestUnreadableDirectoryKeepsOtherResults(t *testing.T) {
	t.Parallel()
	root := gitRepo(t, map[string]string{
		".docgen.yaml": "docs: docs\n",
		"data/x.sh":    "x\n",
		"src.sh":       regionText,
		"docs/p.md":    "[a](nope.md)\n\n<!-- docgen:gomod field=module --><!-- /docgen:gomod -->\n",
	})
	unreadable(t, root, "data")
	rc, log := docgen(t, root, "check")
	if rc != 1 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
	wantIn(t, log, "permission denied", "unused: src.sh:1: docgen:region r", "broken: docs/p.md:1: nope.md", "stale: docs/p.md:3:")
}

func TestUnreadableFileKeepsOtherResults(t *testing.T) {
	t.Parallel()
	root := gitRepo(t, map[string]string{"x.sh": "x\n", "src.sh": regionText, "docs/p.md": "[a](nope.md)\n"})
	unreadable(t, root, "x.sh")
	rc, log := docgen(t, root, "check")
	if rc != 1 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
	wantIn(t, log, "permission denied", "unused: src.sh:1: docgen:region r", "broken: docs/p.md:1: nope.md")
}

func TestUnreadablePageDirectoryKeepsOtherResults(t *testing.T) {
	t.Parallel()
	root := gitRepo(t, map[string]string{"docs/sub/q.md": "x\n", "docs/p.md": "[a](nope.md)\n"})
	unreadable(t, root, "docs/sub")
	rc, log := docgen(t, root, "check")
	if rc != 1 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
	wantIn(t, log, "permission denied", "broken: docs/p.md:1: nope.md")
}
