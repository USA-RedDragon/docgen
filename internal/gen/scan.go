package gen

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/USA-RedDragon/docgen/internal/gomod"
	"github.com/USA-RedDragon/docgen/internal/marker"
)

type page struct {
	path    string
	key     string
	rel     string
	base    string
	raw     string
	text    string
	crs     []int
	markers []pageMarker
	filled  []*filled
	err     error
}

// A pageMarker is a marker of a page, with the handler of its kind.
type pageMarker struct {
	marker.Marker
	handler handler
}

// skipDir reports whether a scan skips a directory: a directory that package
// patterns of go do not match, or the third-party code in node_modules.
func skipDir(name string) bool {
	return gomod.SkipDir(name) || name == "node_modules"
}

func newPage(file, rel, raw string) *page {
	pg := &page{path: file, key: canonical(file), rel: rel, raw: raw, text: strings.ReplaceAll(raw, "\r\n", "\n")}
	removed := 0
	for i := 0; i+1 < len(raw); i++ {
		if raw[i] == '\r' && raw[i+1] == '\n' {
			pg.crs = append(pg.crs, i-removed)
			removed++
		}
	}
	return pg
}

// parse sets the markers of the page, each with a new handler of its kind.
func (pg *page) parse() error {
	var handlers []handler
	ms, err := marker.Parse(pg.text, func(kind string) marker.Binder {
		h := newHandler(kind)
		if h == nil {
			return nil
		}
		handlers = append(handlers, h)
		return h
	})
	if err != nil {
		return err
	}
	for i, m := range ms {
		pg.markers = append(pg.markers, pageMarker{m, handlers[i]})
	}
	pg.filled = make([]*filled, len(ms))
	return nil
}

// rawOffset returns the offset in the file of an offset in the LF text of the
// page. An offset at a line ending is the offset of its carriage return.
func (pg *page) rawOffset(off int) int {
	return off + sort.SearchInts(pg.crs, off)
}

// crlf returns body with the line ending of most lines of the page.
func (pg *page) crlf(body string) string {
	if 2*len(pg.crs) > strings.Count(pg.text, "\n") {
		return strings.ReplaceAll(body, "\n", "\r\n")
	}
	return body
}

// scan returns the pages in docsDir and the pages of g.pages that hold markers,
// and an error for each page that does not parse and each path that does not
// read. It leaves out the paths that git ignores, and the pages under a root of a
// docgen:docgen marker, apart from the pages of g.pages. It reads regular files
// only: a symbolic link can point outside docsDir, and the page at its target is
// not part of the docs. A docsDir below the repository root that a symbolic link
// takes out of the repository is an error.
func (g *Generator) scan(ctx context.Context, docsDir string) ([]*page, []error, error) {
	if rel, ok := g.leavesRepository(docsDir); ok {
		return nil, nil, fmt.Errorf("%s %w", rel, gomod.ErrOutside)
	}
	g.ignored = ignoredPaths(ctx, g.mod.Root)
	read, failed, err := g.walkDocs(ctx, docsDir)
	if err != nil {
		return nil, nil, err
	}
	read, named, extra := g.addExtras(read)
	failed = append(failed, extra...)
	for _, pg := range read {
		pg.err = pg.parse()
	}
	g.roots = g.nestedRoots(read, named)
	var pages []*page
	var errs []error
	for _, pg := range read {
		if leftOut(g.roots, named, pg) {
			continue
		}
		g.scanned = append(g.scanned, pg)
		if pg.err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", pg.rel, pg.err))
		}
		if len(pg.markers) > 0 {
			pages = append(pages, pg)
		}
	}
	return pages, append(errs, failed...), nil
}

// addExtras returns pages with each page of g.pages that it does not hold, the
// keys of the pages of g.pages, and an error for each one that does not read.
func (g *Generator) addExtras(pages []*page) ([]*page, map[string]bool, []error) {
	seen := map[string]bool{}
	for _, pg := range pages {
		seen[pg.key] = true
	}
	named := map[string]bool{}
	var errs []error
	for _, name := range g.pages {
		b, err := g.readExtra(name)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		pg := g.readPage(g.mod.Root, filepath.ToSlash(filepath.Clean(filepath.FromSlash(name))), b)
		named[pg.key] = true
		if !seen[pg.key] {
			seen[pg.key] = true
			pages = append(pages, pg)
		}
	}
	return pages, named, errs
}

// leavesRepository returns the path of dir in the repository, and true when it is
// below the root by name and a symbolic link takes it outside.
func (g *Generator) leavesRepository(dir string) (string, bool) {
	rel, err := filepath.Rel(g.mod.Root, dir)
	if err != nil || !filepath.IsLocal(rel) {
		return "", false
	}
	return filepath.ToSlash(rel), !gomod.Inside(g.mod.Root, dir)
}

// ignoredIn returns the test of the slash paths of docsDir that a scan of docsDir
// skips. A docsDir outside the repository has no ignored path. In a docsDir that
// git ignores, only the .gitignore files of docsDir apply.
func (g *Generator) ignoredIn(ctx context.Context, docsDir string) func(string) bool {
	rel, err := filepath.Rel(canonical(g.mod.Root), canonical(docsDir))
	if err != nil || !filepath.IsLocal(rel) {
		return func(string) bool { return false }
	}
	dir := filepath.ToSlash(rel)
	if dir != "." && ignoredDir(ctx, g.mod.Root, dir) {
		return ignoredBelow(ctx, g.mod.Root, docsDir)
	}
	return func(name string) bool { return g.ignored(path.Join(dir, name)) }
}

// walkDocs returns each page in docsDir that ignoredIn does not skip, and the
// error of each path that does not read.
func (g *Generator) walkDocs(ctx context.Context, docsDir string) ([]*page, []error, error) {
	root, err := os.OpenRoot(docsDir)
	if err != nil {
		return nil, nil, err
	}
	defer root.Close()
	ignored := g.ignoredIn(ctx, docsDir)
	var names []string
	failures, err := walkFiles(root, ignored, func(name string, _ fs.DirEntry) error {
		if isPage(name) && !ignored(name) {
			names = append(names, name)
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	var pages []*page
	for _, name := range names {
		b, err := fs.ReadFile(root.FS(), name)
		if err != nil {
			failures = append(failures, walkFailure{name, err})
			continue
		}
		pages = append(pages, g.readPage(docsDir, name, b))
	}
	var errs []error
	for _, f := range failures {
		errs = append(errs, f.err)
	}
	return pages, errs, nil
}

// isPage reports whether the file name is a Markdown page.
func isPage(name string) bool {
	return strings.HasSuffix(name, ".md")
}

// A walkFailure is a path of a walk that does not read.
type walkFailure struct {
	name string
	err  error
}

// walkFiles calls visit for each regular file in root, in the directories that
// skipDir does not skip and that prune, when it is not nil, is false for. A path
// that does not read is a failure and the walk goes on. The error is the error of
// visit, which ends the walk.
func walkFiles(root *os.Root, prune func(string) bool, visit func(name string, d fs.DirEntry) error) ([]walkFailure, error) {
	var failures []walkFailure
	err := fs.WalkDir(root.FS(), ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil {
			if d == nil {
				return err
			}
			failures = append(failures, walkFailure{name, err})
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			if name != "." && (skipDir(d.Name()) || prune != nil && prune(name)) {
				return fs.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		return visit(name, d)
	})
	return failures, err
}

// readExtra returns the contents of the page name of g.pages. A path or a symbolic
// link that leaves the repository is an error.
func (g *Generator) readExtra(name string) ([]byte, error) {
	p := filepath.FromSlash(name)
	if !filepath.IsLocal(p) {
		return nil, fmt.Errorf("%s is outside the repository", name)
	}
	f, err := os.OpenInRoot(g.mod.Root, p)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	defer f.Close()
	b, err := io.ReadAll(f)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	return b, nil
}

// readPage returns the page with the contents b at the slash path name in
// docsDir. Files of the page stay in the repository, or in docsDir when that is
// outside the repository and holds the page.
func (g *Generator) readPage(docsDir, name string, b []byte) *page {
	p := filepath.Join(docsDir, filepath.FromSlash(name))
	rel, _ := filepath.Rel(g.mod.Root, p)
	pg := newPage(p, filepath.ToSlash(rel), string(b))
	pg.base = docsDir
	if gomod.Inside(g.mod.Root, p) {
		pg.base = g.mod.Root
	}
	return pg
}
