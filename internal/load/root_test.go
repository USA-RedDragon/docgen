package load

import (
	"strings"
	"testing"

	"github.com/USA-RedDragon/docgen/internal/gomod"
)

func TestRootPackage(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"go.mod":            "module example.com/w\n",
		"tools/x/go.mod":    "module example.com/x\n",
		"tools/x/main.go":   "// Command x runs.\npackage main\n",
		"tools/x/no.go":     "//go:build nope\n\npackage main\n",
		"tools/x/x_test.go": "package main\n",
		"tools/y/skip.go":   "//go:build nope\n\npackage y\n",
	})
	mod, err := gomod.Open(root, root)
	if err != nil {
		t.Fatal(err)
	}
	pi, err := RootPackage(mod, nil, "tools/x")
	if err != nil {
		t.Fatal(err)
	}
	if pi.Path != "example.com/x" || pi.Pkg != nil || !strings.HasPrefix(pi.Doc.Doc, "Command x runs.") {
		t.Errorf("package: %+v", pi)
	}
	if _, err := RootPackage(mod, nil, "tools/y"); err == nil || err.Error() != "no buildable Go file in tools/y" {
		t.Errorf("no file: %v", err)
	}
	if _, err := RootPackage(mod, nil, "tools/none"); err == nil {
		t.Error("a missing directory passed")
	}
}
