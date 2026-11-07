package gen

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/USA-RedDragon/docgen/internal/excerpt"
)

const (
	// maxScanSize is the size in bytes of the largest file that the scan for
	// region marks reads.
	maxScanSize = 1 << 20
	// binaryPrefix is the number of bytes at the start of a file that show it is
	// binary, when they hold a NUL byte. git reads the same number.
	binaryPrefix = 8000
)

// repositoryMarks returns the slash paths of the text files of the repository, in
// walk order, and the region marks of each one. It does not enter a directory that
// git ignores, and it leaves out the pages under a root of a docgen:docgen marker.
// It holds the scanned pages that git ignores. The errors are the failure of each
// path that does not read, and the result holds the other paths.
func (g *Generator) repositoryMarks(ctx context.Context) ([]string, [][]excerpt.Mark, []error) {
	root, err := os.OpenRoot(g.mod.Root)
	if err != nil {
		return nil, nil, []error{err}
	}
	defer root.Close()
	names := g.ignoredPages()
	failures, err := walkFiles(root, g.ignored, func(name string, d fs.DirEntry) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		info, err := d.Info()
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		if info.Size() <= maxScanSize && !g.ignored(name) && !g.underRoot(name) {
			names = append(names, name)
		}
		return nil
	})
	if err != nil {
		return nil, nil, []error{err}
	}
	var errs []error
	for _, f := range failures {
		errs = append(errs, f.err)
	}
	marks := make([][]excerpt.Mark, len(names))
	failed := make([]error, len(names))
	g.each(ctx, len(names), func(i int) {
		marks[i], failed[i] = fileMarks(root, names[i])
	})
	for _, err := range failed {
		if err != nil {
			errs = append(errs, err)
		}
	}
	return names, marks, errs
}

// underRoot reports whether the slash path name of the repository is a page under
// a root of a docgen:docgen marker.
func (g *Generator) underRoot(name string) bool {
	return isPage(name) && holds(g.roots, canonical(filepath.Join(g.mod.Root, filepath.FromSlash(name))))
}

// ignoredPages returns the slash paths of the scanned pages that git ignores, with
// the size limit of the scan for region marks.
func (g *Generator) ignoredPages() []string {
	var names []string
	for _, pg := range g.scanned {
		rel, err := filepath.Rel(canonical(g.mod.Root), pg.key)
		if name := filepath.ToSlash(rel); err == nil && g.ignored(name) && len(pg.raw) <= maxScanSize {
			names = append(names, name)
		}
	}
	return names
}

// ignoredPaths returns a test for the slash paths under root that git ignores,
// directories included. Outside a git repository, or with no git command, the test
// is false for each path.
func ignoredPaths(ctx context.Context, root string) func(string) bool {
	return ignoredList(exec.CommandContext(ctx, "git", "-C", root, "ls-files", "-z", "--others", "--ignored", "--exclude-standard", "--directory"))
}

// ignoredDir reports whether git ignores the directory at the slash path name of
// root, with its parents. A file of the directory that git tracks does not change
// the result.
func ignoredDir(ctx context.Context, root, name string) bool {
	return exec.CommandContext(ctx, "git", "-C", root, "check-ignore", "-q", "--no-index", "--", name).Run() == nil
}

// ignoredBelow returns a test for the slash paths under dir, a directory of the
// repository at root, that the .gitignore files in dir match. Each path under a
// directory that git ignores is ignored in the work tree of the repository, so the
// git command takes dir as its work tree, with no index and no other exclude files.
func ignoredBelow(ctx context.Context, root, dir string) func(string) bool {
	out, err := exec.CommandContext(ctx, "git", "-C", root, "rev-parse", "--absolute-git-dir").Output()
	if err != nil {
		return func(string) bool { return false }
	}
	gitDir := strings.TrimSpace(string(out))
	cmd := exec.CommandContext(ctx, "git", "ls-files", "-z", "--others", "--ignored", "--exclude-per-directory=.gitignore", "--directory")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_DIR="+gitDir, "GIT_WORK_TREE="+dir, "GIT_INDEX_FILE="+filepath.Join(gitDir, "docgen-no-index"))
	return ignoredList(cmd)
}

// ignoredList returns a test for the paths that cmd, a git ls-files command with
// the options -z and --directory, lists. A failed command lists no path.
func ignoredList(cmd *exec.Cmd) func(string) bool {
	out, err := cmd.Output()
	if err != nil {
		return func(string) bool { return false }
	}
	files := map[string]bool{}
	var dirs []string
	for p := range strings.SplitSeq(string(out), "\x00") {
		switch {
		case p == "":
		case strings.HasSuffix(p, "/"):
			dirs = append(dirs, p)
		default:
			files[p] = true
		}
	}
	return func(name string) bool {
		return files[name] || slices.ContainsFunc(dirs, func(d string) bool { return strings.HasPrefix(name+"/", d) })
	}
}

// fileMarks returns the region marks of the file name of root, with none for a
// binary file and a file that no longer exists.
func fileMarks(root *os.Root, name string) ([]excerpt.Mark, error) {
	b, err := root.ReadFile(name)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if bytes.IndexByte(b[:min(len(b), binaryPrefix)], 0) >= 0 {
		return nil, nil
	}
	return excerpt.Marks(name, string(excerpt.Source(b))), nil
}
