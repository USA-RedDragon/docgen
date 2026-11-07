package load

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/USA-RedDragon/docgen/internal/gocmd"
	"github.com/USA-RedDragon/docgen/internal/gomod"
)

func writeTree(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for name, text := range files {
		p := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func loadTree(t *testing.T, files map[string]string, pkgs ...string) *Set {
	t.Helper()
	root := t.TempDir()
	writeTree(t, root, files)
	mod, err := gomod.Open(root, root)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{}
	for _, p := range pkgs {
		want[p] = true
	}
	session := &gocmd.Session{}
	defer session.Close()
	set, err := Packages(t.Context(), mod, session, want)
	if err != nil {
		t.Fatal(err)
	}
	return set
}

func TestPackages(t *testing.T) {
	t.Parallel()
	set := loadTree(t, map[string]string{
		"go.mod":      "module example.com/w\n\ngo 1.24.0\n",
		"a/a.go":      "// Package a is a package.\npackage a\n\nimport \"example.com/w/b\"\n\n// N is a number.\nconst N = b.N\n",
		"a/a_test.go": "package a\n\nfunc ExampleN() {\n\t// Output:\n}\n",
		"b/b.go":      "// Package b is a package.\npackage b\n\nimport \"example.com/w/c\"\n\n// N is a number.\nconst N = c.N\n",
		"c/c.go":      "// Package c is a package.\npackage c\n\n// N is a number.\nconst N = 1\n",
		"bad/bad.go":  "package bad\n\nconst N = \n",
	}, "example.com/w/a", "example.com/w/bad", "example.com/w/none")
	pi, err := set.Package("a")
	if err != nil {
		t.Fatal(err)
	}
	if pi.Path != "example.com/w/a" || len(pi.Examples) != 1 || pi.Doc.Name != "a" {
		t.Errorf("package a: %+v", pi)
	}
	if set.ByPath("example.com/w/a") != pi {
		t.Error("ByPath returned another package")
	}
	if set.ByPath("example.com/w/b") != nil {
		t.Error("a package that was not asked for is in the set")
	}
	if err := set.Err("example.com/w/a"); err != nil {
		t.Errorf("Err: %v", err)
	}
	if _, err := set.Package("bad"); err == nil || !strings.HasPrefix(err.Error(), "loading bad: ") {
		t.Errorf("bad: %v", err)
	}
	if err := set.Err("example.com/w/none"); err == nil || !strings.HasPrefix(err.Error(), "loading none: ") {
		t.Errorf("none: %v", err)
	}
	if _, err := set.Package("c"); err == nil || err.Error() != "package example.com/w/c not loaded" {
		t.Errorf("c: %v", err)
	}
	deps, err := set.ModuleImports(pi)
	if err != nil {
		t.Fatal(err)
	}
	if len(deps) != 2 || deps[0].Path != "example.com/w/b" || deps[1].Path != "example.com/w/c" {
		t.Errorf("ModuleImports: %+v", deps)
	}
	if got, err := set.Imported(pi, "example.com/w/c"); err != nil || got.Path != "example.com/w/c" {
		t.Errorf("Imported: %v, %v", got, err)
	}
	if _, err := set.Imported(pi, "example.com/w/none"); err == nil || err.Error() != "example.com/w/a does not import example.com/w/none" {
		t.Errorf("Imported of a missing package: %v", err)
	}
}

func TestPackagesNothing(t *testing.T) {
	t.Parallel()
	set := loadTree(t, map[string]string{"x.txt": "x\n"})
	if pi := set.ByPath("example.com/w"); pi != nil {
		t.Errorf("ByPath = %+v", pi)
	}
}

func TestPackagesNoGoMod(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mod, err := gomod.Open(root, root)
	if err != nil {
		t.Fatal(err)
	}
	set, err := Packages(t.Context(), mod, &gocmd.Session{}, map[string]bool{"example.com/w": true})
	if err != nil {
		t.Fatal(err)
	}
	if err := set.Err("example.com/w"); err == nil || !strings.Contains(err.Error(), "no go.mod in ") {
		t.Errorf("Err = %v", err)
	}
	if _, err := set.Package("."); err == nil || !strings.Contains(err.Error(), "no go.mod in ") {
		t.Errorf("Package: %v", err)
	}
}

func TestLoadErrorOfNoGoCommand(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		err  error
		want string
	}{
		{fmt.Errorf("err: %w: stderr: ", exec.ErrNotFound), "the go command is not on PATH"},
		{errors.New("err: exit status 1: stderr: go: bad"), "loading packages: go: bad"},
		{errors.New("err: exit status 1: stderr: "), "loading packages: err: exit status 1: stderr:"},
	} {
		if got := loadError(c.err).Error(); !strings.Contains(got, c.want) {
			t.Errorf("loadError = %q, want %q", got, c.want)
		}
	}
}
