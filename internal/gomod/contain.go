package gomod

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// ErrOutside is the error of a path that leaves the repository.
var ErrOutside = errors.New("is outside the repository")

// maxLinks limits the symbolic links of one path, as the kernel does.
const maxLinks = 255

// Resolve returns the absolute path that p reaches with each symbolic link
// followed, also a link with a target that does not exist. Elements that do not
// exist stay as they are, so the result is the place where a write to p lands.
func Resolve(p string) (string, error) {
	p, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	sep := string(filepath.Separator)
	vol := filepath.VolumeName(p)
	cur := vol + sep
	rest := strings.Split(p[len(vol):], sep)
	links := 0
	for len(rest) > 0 {
		name := rest[0]
		rest = rest[1:]
		switch name {
		case "", ".":
			continue
		case "..":
			cur = filepath.Dir(cur)
			continue
		}
		next := filepath.Join(cur, name)
		info, err := os.Lstat(next)
		if err != nil && !errors.Is(err, fs.ErrNotExist) && !isNotDir(next) {
			return "", err
		}
		if err != nil || info.Mode()&fs.ModeSymlink == 0 {
			cur = next
			continue
		}
		if links++; links > maxLinks {
			return "", fmt.Errorf("%s: too many symbolic links", p)
		}
		target, err := os.Readlink(next)
		if err != nil {
			return "", err
		}
		if filepath.IsAbs(target) {
			vol = filepath.VolumeName(target)
			cur = vol + sep
			target = target[len(vol):]
		}
		rest = append(strings.Split(target, sep), rest...)
	}
	return cur, nil
}

// isNotDir reports whether the parent of p is a file.
func isNotDir(p string) bool {
	info, err := os.Stat(filepath.Dir(p))
	return err == nil && !info.IsDir()
}

// Inside reports whether path, with each symbolic link followed, is root or a
// path below it. A path that does not exist passes when the place that it
// reaches is below root, and the read of it then fails. A path that does not
// resolve, such as a loop of links, does not pass.
func Inside(root, path string) bool {
	base, err := Resolve(root)
	if err != nil {
		return false
	}
	resolved, err := Resolve(path)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(base, resolved)
	return err == nil && filepath.IsLocal(rel)
}

// goDirInside reports whether dir and each Go file in it are inside root. A
// link to a Go file counts: the go command and go/build read through it.
func goDirInside(root, dir string) bool {
	return Inside(root, dir) && GoLinkOutside(root, dir) == ""
}

// GoLinkOutside returns the name of the first link to a Go file in dir that leaves
// root, and "" when there is none.
func GoLinkOutside(root, dir string) string {
	es, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	for _, e := range es {
		if e.Type()&fs.ModeSymlink != 0 && strings.HasSuffix(e.Name(), ".go") && !Inside(root, filepath.Join(dir, e.Name())) {
			return e.Name()
		}
	}
	return ""
}

// boundary returns the directory that the paths of the module must stay in: the
// repository, or the module when its directory is outside the repository.
func (m *Module) boundary() string {
	if Inside(m.Root, m.Dir) {
		return m.Root
	}
	return m.Dir
}

// InBounds reports whether the package directory dir of the module and its Go
// files stay in the repository.
func (m *Module) InBounds(dir string) bool {
	return goDirInside(m.boundary(), dir)
}

// LocalDir returns an error wrapping ErrOutside when the directory rel, relative
// to the repository root, or a Go file in it, leaves the repository. key and rel
// are the argument of the marker in the error.
func (m *Module) LocalDir(key, rel string) error {
	p := filepath.FromSlash(rel)
	if !filepath.IsLocal(p) || !goDirInside(m.Root, filepath.Join(m.Root, p)) {
		return fmt.Errorf("%s=%s %w", key, rel, ErrOutside)
	}
	return nil
}
