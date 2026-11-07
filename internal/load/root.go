package load

import (
	"fmt"
	"go/ast"
	"go/build"
	"go/doc"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/USA-RedDragon/docgen/internal/gomod"
)

// RootPackage reads the package doc of a directory outside mod, with the build
// tags in tags. p is relative to the repository root.
func RootPackage(mod *gomod.Module, tags []string, p string) (*Package, error) {
	ctxt := build.Default
	ctxt.BuildTags = append(slices.Clone(ctxt.BuildTags), tags...)
	dir := mod.RootDir(p)
	fset := token.NewFileSet()
	es, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var files []*ast.File
	for _, e := range es {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		if ok, err := ctxt.MatchFile(dir, e.Name()); err != nil {
			return nil, err
		} else if !ok {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(dir, e.Name()), nil, parser.ParseComments)
		if err != nil {
			return nil, err
		}
		files = append(files, f)
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no buildable Go file in %s", p)
	}
	ip := mod.RootImportPath(p)
	d, err := doc.NewFromFiles(fset, files, ip, doc.AllDecls)
	if err != nil {
		return nil, err
	}
	return &Package{Doc: d, Path: ip}, nil
}
