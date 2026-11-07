// Package gomod maps the paths of the markers to the directories and the import
// paths of a Go module in a repository.
package gomod

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"golang.org/x/mod/modfile"
)

// A Module is the Go module in the directory Dir of the repository at Root.
// Path is its import path, and "" when Dir has no go.mod. Hint names what sets the
// directory of the module in the error of a Dir with no go.mod, and "" means the
// -module flag.
type Module struct {
	Root string
	Dir  string
	Path string
	Hint string
	dirs []string
}

// Open returns the Module for the repository at root and the directory modDir.
// A directory with no go.mod is not an error here, because a page can hold
// only markers that read no Go code. A directory that does not exist is an
// error.
func Open(root, modDir string) (*Module, error) {
	m := &Module{Root: root, Dir: modDir}
	if info, err := os.Stat(modDir); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("module directory %s does not exist", modDir)
		}
		return nil, err
	} else if !info.IsDir() {
		return nil, fmt.Errorf("module path %s is not a directory", modDir)
	}
	b, err := os.ReadFile(filepath.Join(modDir, "go.mod"))
	if errors.Is(err, fs.ErrNotExist) {
		return m, nil
	}
	if err != nil {
		return nil, err
	}
	if m.Path = modfile.ModulePath(b); m.Path == "" {
		return nil, fmt.Errorf("%s has no module line", filepath.Join(modDir, "go.mod"))
	}
	return m, nil
}

// cleanPath returns the pkg= of a marker with no ./ prefix and no / at either
// end, and "." for the root.
func cleanPath(p string) string {
	return path.Clean(strings.Trim(p, "/"))
}

// ImportPath returns the import path of p, a path in the module or an import
// path of the module.
func (m *Module) ImportPath(p string) string {
	p = cleanPath(p)
	switch {
	case p == "." || p == m.Path:
		return m.Path
	case strings.HasPrefix(p, m.Path+"/"):
		return p
	}
	return m.Path + "/" + p
}

// ShortPath returns the import path ip relative to the module, or "." for the
// root package of the module.
func (m *Module) ShortPath(ip string) string {
	if ip == m.Path {
		return "."
	}
	return strings.TrimPrefix(ip, m.Path+"/")
}

// PkgPath returns the import path of the pkg= value of a marker: a path
// in the module, or the last elements of exactly one package path in the
// module ("session" for transport/tcp/session). A directory below a go.mod
// of its own is in another module. A path that leaves the repository through a
// symbolic link is an error wrapping ErrOutside.
func (m *Module) PkgPath(p string) (string, error) {
	if m.Path == "" {
		hint := m.Hint
		if hint == "" {
			hint = "-module"
		}
		return "", fmt.Errorf("no go.mod in %s; give the directory of the module with %s", m.Dir, hint)
	}
	ip := m.ImportPath(p)
	if dir := filepath.Join(m.Dir, filepath.FromSlash(m.ShortPath(ip))); HasGo(dir) && m.Contains(dir) {
		return m.bounded(p, ip, dir)
	}
	dirs, err := m.packageDirs()
	if err != nil {
		return "", err
	}
	var found []string
	for _, d := range dirs {
		if strings.HasSuffix("/"+d, "/"+cleanPath(p)) {
			found = append(found, d)
		}
	}
	switch len(found) {
	case 0:
		return "", fmt.Errorf("no package %s in module %s", p, m.Path)
	case 1:
		return m.bounded(p, m.ImportPath(found[0]), filepath.Join(m.Dir, filepath.FromSlash(found[0])))
	}
	return "", fmt.Errorf("pkg=%s matches several packages: %s", p, strings.Join(found, ", "))
}

// bounded returns ip, and an error wrapping ErrOutside when the directory dir of
// the pkg= value p or a Go file in it leaves the repository.
func (m *Module) bounded(p, ip, dir string) (string, error) {
	if !m.InBounds(dir) {
		return "", fmt.Errorf("pkg=%s %w", p, ErrOutside)
	}
	return ip, nil
}

// packageDirs returns the directories with Go files of the module, relative to
// Dir. It reads the tree once.
func (m *Module) packageDirs() ([]string, error) {
	if m.dirs != nil {
		return m.dirs, nil
	}
	err := filepath.WalkDir(m.Dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		if path != m.Dir && (SkipDir(d.Name()) || isModule(path)) {
			return filepath.SkipDir
		}
		if HasGo(path) {
			rel, _ := filepath.Rel(m.Dir, path)
			m.dirs = append(m.dirs, filepath.ToSlash(rel))
		}
		return nil
	})
	return m.dirs, err
}

// SkipDir reports whether a package pattern of go does not match a directory
// with that name.
func SkipDir(name string) bool {
	return name == "testdata" || name == "vendor" || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_")
}

func isModule(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, "go.mod"))
	return err == nil
}

// Contains reports whether no go.mod lies between dir and the module root.
func (m *Module) Contains(dir string) bool {
	for dir != m.Dir {
		if isModule(dir) {
			return false
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return false
		}
		dir = parent
	}
	return true
}

// HasGo reports whether dir holds a Go file.
func HasGo(dir string) bool {
	es, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range es {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".go") {
			return true
		}
	}
	return false
}

// RootDir returns the directory of a pkg= value, relative to the repository
// root, for a command outside the module.
func (m *Module) RootDir(p string) string {
	return filepath.Join(m.Root, filepath.FromSlash(strings.Trim(p, "/")))
}

// Up returns the nearest directory at or above dir for which found is true, and
// false when there is none.
func Up(dir string, found func(dir string) bool) (string, bool) {
	for d := dir; ; {
		if found(d) {
			return d, true
		}
		parent := filepath.Dir(d)
		if parent == d {
			return "", false
		}
		d = parent
	}
}

// ModuleRoot returns the nearest directory at or above dir with a go.mod.
func ModuleRoot(dir string) (string, error) {
	mod, ok := Up(dir, isModule)
	if !ok {
		return "", errors.New("no go.mod above it")
	}
	return mod, nil
}

// RootImportPath returns the import path of the directory of a pkg= value
// relative to the repository root, from the nearest go.mod, or the path of the
// directory in the repository when it has none.
func (m *Module) RootImportPath(p string) string {
	dir := m.RootDir(p)
	p = cleanPath(p)
	mod, err := ModuleRoot(dir)
	if err != nil {
		return p
	}
	b, err := os.ReadFile(filepath.Join(mod, "go.mod"))
	if err != nil {
		return p
	}
	rel, err := filepath.Rel(mod, dir)
	if err != nil {
		return p
	}
	return path.Join(modfile.ModulePath(b), filepath.ToSlash(rel))
}
