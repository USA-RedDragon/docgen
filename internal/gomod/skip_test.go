package gomod

import "testing"

func TestSkipDir(t *testing.T) {
	t.Parallel()
	for name, want := range map[string]bool{
		"testdata":     true,
		"vendor":       true,
		".git":         true,
		"_build":       true,
		"internal":     false,
		"node_modules": false,
		"Testdata":     false,
		"vendored":     false,
		"a_b":          false,
		"a.b":          false,
	} {
		if got := SkipDir(name); got != want {
			t.Errorf("SkipDir(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestPkgPathSkipsDirs(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"go.mod":                    "module example.com/m\n\ngo 1.24.0\n",
		"a/testdata/one/one.go":     "package one\n",
		"a/vendor/two/two.go":       "package two\n",
		"a/.hidden/three/three.go":  "package three\n",
		"a/_private/four/four.go":   "package four\n",
		"a/sub/five/go.mod":         "module example.com/five\n",
		"a/sub/five/five.go":        "package five\n",
		"a/node_modules/six/six.go": "package six\n",
		"a/plain/seven/seven.go":    "package seven\n",
	})
	m, err := Open(root, root)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"one", "two", "three", "four", "five"} {
		if got, err := m.PkgPath(p); err == nil {
			t.Errorf("%s = %s, want an error", p, got)
		}
	}
	for p, want := range map[string]string{
		"six":   "example.com/m/a/node_modules/six",
		"seven": "example.com/m/a/plain/seven",
	} {
		got, err := m.PkgPath(p)
		if err != nil {
			t.Errorf("%s: %v", p, err)
			continue
		}
		if got != want {
			t.Errorf("%s = %s, want %s", p, got, want)
		}
	}
}
