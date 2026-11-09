package gomod

import (
	"os"
	"path/filepath"
	"testing"
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

func TestPkgPathAmbiguousOnlyInModule(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"go.mod":               "module example.com/m\n\ngo 1.24.0\n",
		"deep/down/leaf.go":    "package leaf\n",
		"tools/x/go.mod":       "module example.com/x\n\ngo 1.24.0\n",
		"tools/x/down/leaf.go": "package leaf\n",
	})
	g, err := Open(root, root)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := g.PkgPath("leaf"); err == nil {
		t.Errorf("leaf = %s, want an error: the directory is down, not leaf", got)
	}
	got, err := g.PkgPath("down")
	if err != nil {
		t.Fatal(err)
	}
	if want := "example.com/m/deep/down"; got != want {
		t.Errorf("down = %s, want %s", got, want)
	}
	if _, err := g.PkgPath("tools/x/down"); err == nil {
		t.Error("a package of a nested module passed")
	}
}

func TestResolve(t *testing.T) {
	t.Parallel()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	root, out := filepath.Join(dir, "root"), filepath.Join(dir, "out")
	for _, d := range []string{filepath.Join(root, "a/b"), out} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for name, target := range map[string]string{
		"up":       "..",
		"in":       "a/b",
		"abs":      out,
		"rel":      "../out",
		"dangling": "../out/new",
		"chain":    "rel",
		"via":      "a/b/../../rel",
		"loop":     "loop",
	} {
		if err := os.Symlink(target, filepath.Join(root, name)); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "file"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	for name, inside := range map[string]bool{
		".":          true,
		"a/b":        true,
		"in":         true,
		"in/new/x":   true,
		"new/x":      true,
		"file/x":     true,
		"up":         false,
		"in/../../x": false,
		"abs":        false,
		"abs/x":      false,
		"rel/x":      false,
		"dangling":   false,
		"dangling/x": false,
		"chain/x":    false,
		"via":        false,
		"loop":       false,
		"loop/x":     false,
	} {
		if got := Inside(root, root+"/"+name); got != inside {
			t.Errorf("Inside(%s) = %v, want %v", name, got, inside)
		}
	}
	got, err := Resolve(filepath.Join(root, "dangling/x"))
	if want := filepath.Join(out, "new/x"); err != nil || got != want {
		t.Errorf("Resolve = %q, %v, want %q", got, err, want)
	}
}

func TestUp(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(string(filepath.Separator), "a", "b", "c")
	var visited []string
	got, ok := Up(dir, func(d string) bool {
		visited = append(visited, d)
		return filepath.Base(d) == "a"
	})
	if want := filepath.Join(string(filepath.Separator), "a"); !ok || got != want {
		t.Errorf("Up = %q, %v, want %q", got, ok, want)
	}
	if len(visited) != 3 || visited[0] != dir {
		t.Errorf("visited %q", visited)
	}
	if got, ok := Up(dir, func(string) bool { return false }); ok || got != "" {
		t.Errorf("Up with no match = %q, %v", got, ok)
	}
	if got, ok := Up(dir, func(string) bool { return true }); !ok || got != dir {
		t.Errorf("Up at dir = %q, %v", got, ok)
	}
}

func TestNotFoundNamesNestedModules(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"go.mod":             "module example.com/m\n\ngo 1.24.0\n",
		"generate.go":        "package m\n",
		"v2/go.mod":          "module example.com/m/v2\n\ngo 1.24.0\n",
		"v2/trunk/trunk.go":  "package trunk\n",
		"tools/go.mod":       "module example.com/tools\n\ngo 1.24.0\n",
		"tools/lint/lint.go": "package lint\n",
	})
	g, err := Open(root, root)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ pkg, want string }{
		{"trunk", "no package trunk in module example.com/m; the module in v2 has it, so give the directory v2 with module: in .docgen.yaml or -module"},
		{"nope", "no package nope in module example.com/m; other modules are in tools, v2, and module: in .docgen.yaml or -module gives the directory of another module"},
	} {
		for range 2 {
			if _, err := g.PkgPath(tc.pkg); err == nil || err.Error() != tc.want {
				t.Errorf("pkg=%s: error %v, want %q", tc.pkg, err, tc.want)
			}
		}
	}
}
