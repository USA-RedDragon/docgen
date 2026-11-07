// Package load loads the Go packages of the markers of a run.
package load

import (
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/doc"
	"go/parser"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/USA-RedDragon/docgen/internal/gocmd"
	"github.com/USA-RedDragon/docgen/internal/gomod"
	"golang.org/x/tools/go/packages"
)

// A Package is a loaded package. Examples holds the Examples of its tests.
// Source holds the syntax of its source files. Pkg.Syntax can differ, because
// the compiler reads generated files for a cgo package. Pkg is nil for a
// package that RootPackage reads.
type Package struct {
	Pkg      *packages.Package
	Doc      *doc.Package
	Source   []*ast.File
	Examples []*doc.Example
	Path     string
}

// A Set holds the packages of one call of Packages, by import path.
type Set struct {
	mod      *gomod.Module
	packages map[string]*Package
	errs     map[string]error
	deps     map[string]*Package
	testErrs map[string]error
	failed   error
}

// Packages loads the packages of mod with the import paths in pkgs, with their
// tests. A package that does not load is not an error of Packages: Err and
// Package of the Set return its error. A failure of the load as a whole, such
// as no go.mod or a go.mod for a newer Go, is the error of each package.
func Packages(ctx context.Context, mod *gomod.Module, session *gocmd.Session, pkgs map[string]bool) (*Set, error) {
	s := &Set{mod: mod, packages: map[string]*Package{}, errs: map[string]error{}, deps: map[string]*Package{}, testErrs: map[string]error{}}
	if len(pkgs) == 0 {
		return s, nil
	}
	if mod.Path == "" {
		_, s.failed = mod.PkgPath(".")
		return s, nil
	}
	env, err := session.Env(ctx)
	if err != nil {
		return nil, err
	}
	var pats []string
	for p := range pkgs {
		pats = append(pats, p)
	}
	sort.Strings(pats)
	cfg := &packages.Config{
		Context: ctx,
		Mode:    packages.NeedName | packages.NeedFiles | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo | packages.NeedImports | packages.NeedDeps | packages.NeedForTest,
		Dir:     mod.Dir,
		Tests:   true,
		Env:     env,
	}
	ps, err := packages.Load(cfg, pats...)
	if err != nil {
		if ctx.Err() != nil {
			return nil, err
		}
		s.failed = loadError(err)
		return s, nil
	}
	for _, p := range ps {
		s.add(p)
	}
	for _, p := range pats {
		if s.errs[p] == nil && (s.packages[p] == nil || s.packages[p].Pkg == nil) {
			s.errs[p] = fmt.Errorf("package %s not found", mod.ShortPath(p))
		}
	}
	return s, nil
}

// loadError returns err with the message of the go command alone, when err
// holds one. go/packages keeps only the text of the error of a command that does
// not start.
func loadError(err error) error {
	if strings.Contains(err.Error(), exec.ErrNotFound.Error()) {
		return errors.New("loading packages: the go command is not on PATH")
	}
	if _, msg, ok := strings.Cut(err.Error(), "stderr: "); ok && strings.TrimSpace(msg) != "" {
		return fmt.Errorf("loading packages: %s", strings.TrimSpace(msg))
	}
	return fmt.Errorf("loading packages: %w", err)
}

// add adds what p holds to the package that p is a variant of: p itself, or the
// variant of it that its tests compile. The test main package adds nothing.
func (s *Set) add(p *packages.Package) {
	test := p.ForTest != ""
	if test && p.PkgPath == p.ForTest+".test" {
		return
	}
	base := p.PkgPath
	if test {
		base = p.ForTest
	}
	var errs []string
	for _, e := range p.Errors {
		errs = append(errs, e.Error())
	}
	if len(errs) > 0 {
		err := fmt.Errorf("loading %s: %s", s.mod.ShortPath(base), strings.Join(errs, "; "))
		if test {
			s.testErrs[base] = err
		} else {
			s.errs[base] = err
		}
		return
	}
	info := s.packages[base]
	if info == nil {
		info = &Package{Path: base}
		s.packages[base] = info
	}
	info.Examples = append(info.Examples, doc.Examples(testFiles(p)...)...)
	if !test {
		if err := info.fill(p); err != nil {
			s.errs[base] = fmt.Errorf("loading %s: %w", s.mod.ShortPath(base), err)
		}
	}
}

// testFiles returns the syntax of the _test.go files of p.
func testFiles(p *packages.Package) []*ast.File {
	var files []*ast.File
	for _, f := range p.Syntax {
		if strings.HasSuffix(p.Fset.File(f.Pos()).Name(), "_test.go") {
			files = append(files, f)
		}
	}
	return files
}

// ByPath returns the package with the import path ip, or nil when Packages did
// not load it.
func (s *Set) ByPath(ip string) *Package {
	return s.packages[ip]
}

// Err returns the error of the package with the import path ip.
func (s *Set) Err(ip string) error {
	if s.failed != nil {
		return s.failed
	}
	return s.errs[ip]
}

// TestErr returns the error of the tests of the package with the import path ip.
// The package itself can load with tests that do not.
func (s *Set) TestErr(ip string) error {
	return s.testErrs[ip]
}

// fill sets the fields of pi that come from the sources of p.
func (pi *Package) fill(p *packages.Package) error {
	src, err := sourceFiles(p)
	if err != nil {
		return err
	}
	d, err := doc.NewFromFiles(p.Fset, src, p.PkgPath, doc.PreserveAST|doc.AllDecls)
	if err != nil {
		return err
	}
	pi.Pkg, pi.Doc, pi.Source = p, d, src
	return nil
}

// sourceFiles returns the syntax of the files of p that are in the package
// directory, without the files that cgo generates.
func sourceFiles(p *packages.Package) ([]*ast.File, error) {
	parsed := make(map[string]*ast.File, len(p.Syntax))
	for _, f := range p.Syntax {
		parsed[p.Fset.File(f.Pos()).Name()] = f
	}
	files := make([]*ast.File, 0, len(p.GoFiles))
	for _, name := range p.GoFiles {
		f := parsed[name]
		if f == nil {
			var err error
			if f, err = parser.ParseFile(p.Fset, name, nil, parser.ParseComments); err != nil {
				return nil, err
			}
		}
		files = append(files, f)
	}
	return files, nil
}

// imports returns the packages that p imports, directly or not, the nearest
// first and in import path order at each distance.
func imports(p *packages.Package) []*packages.Package {
	seen := map[string]bool{p.ID: true}
	var out []*packages.Package
	for level := []*packages.Package{p}; len(level) > 0; {
		var next []*packages.Package
		for _, q := range level {
			for _, dep := range q.Imports {
				if !seen[dep.ID] {
					seen[dep.ID] = true
					next = append(next, dep)
				}
			}
		}
		sort.Slice(next, func(i, j int) bool { return next[i].PkgPath < next[j].PkgPath })
		out = append(out, next...)
		level = next
	}
	return out
}

func (s *Set) dep(p *packages.Package) (*Package, error) {
	if pi := s.packages[p.PkgPath]; pi != nil && pi.Pkg != nil {
		return pi, nil
	}
	if pi := s.deps[p.PkgPath]; pi != nil {
		return pi, nil
	}
	pi := &Package{Path: p.PkgPath}
	if err := pi.fill(p); err != nil {
		return nil, fmt.Errorf("loading %s: %w", p.PkgPath, err)
	}
	s.deps[p.PkgPath] = pi
	return pi, nil
}

// inModule reports whether p is a package of the module.
func (s *Set) inModule(p *packages.Package) bool {
	return (p.PkgPath == s.mod.Path || strings.HasPrefix(p.PkgPath, s.mod.Path+"/")) && len(p.GoFiles) > 0 && s.mod.Contains(filepath.Dir(p.GoFiles[0]))
}

// ModuleImports returns the packages of the module that pi imports, directly
// or not, the nearest first and in import path order at each distance. The
// result does not depend on the packages that Packages loaded.
func (s *Set) ModuleImports(pi *Package) ([]*Package, error) {
	var out []*Package
	for _, p := range imports(pi.Pkg) {
		if !s.inModule(p) || !s.mod.InBounds(filepath.Dir(p.GoFiles[0])) {
			continue
		}
		d, err := s.dep(p)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, nil
}

// Imported returns the package with the import path ip that pi imports,
// directly or not.
func (s *Set) Imported(pi *Package, ip string) (*Package, error) {
	for _, p := range imports(pi.Pkg) {
		if p.PkgPath == ip {
			if s.inModule(p) && !s.mod.InBounds(filepath.Dir(p.GoFiles[0])) {
				return nil, fmt.Errorf("package %s %w", s.mod.ShortPath(ip), gomod.ErrOutside)
			}
			return s.dep(p)
		}
	}
	return nil, fmt.Errorf("%s does not import %s", pi.Path, ip)
}

// Package returns the loaded package of the pkg= value of a marker.
func (s *Set) Package(p string) (*Package, error) {
	if s.failed != nil {
		return nil, s.failed
	}
	ip, err := s.mod.PkgPath(p)
	if err != nil {
		return nil, err
	}
	if err := s.errs[ip]; err != nil {
		return nil, err
	}
	pi := s.packages[ip]
	if pi == nil || pi.Pkg == nil {
		return nil, fmt.Errorf("package %s not loaded", ip)
	}
	return pi, nil
}
