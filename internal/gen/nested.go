package gen

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/USA-RedDragon/docgen/internal/excerpt"
	"github.com/USA-RedDragon/docgen/internal/gomod"
	"github.com/USA-RedDragon/docgen/internal/goprint"
)

// Settings holds what a nested run reads from the configuration of a copy.
type Settings struct {
	// Docs is the docs directory, relative to the copy.
	Docs string
	// Print is the print setting of the copy.
	Print string
	// Vars holds the variables of the copy.
	Vars map[string]string
	// VarsFile is the path of the file of Vars, relative to the copy.
	VarsFile string
}

// checkDepth returns an error when a run at this level can hold no
// docgen:docgen marker.
func (g *Generator) checkDepth() error {
	if g.depth > 0 {
		return nil
	}
	return fmt.Errorf("the marker is at level %d, above the depth limit %d", g.level, g.level-1)
}

// localTo reports whether the slash path rel stays in dir, also through a
// symbolic link.
func localTo(dir, rel string) bool {
	p := filepath.FromSlash(rel)
	return filepath.IsLocal(p) && gomod.Inside(dir, filepath.Join(dir, p))
}

// A nestedRun is one update of a copy of the directory root with the Go module
// in the directory module of root.
type nestedRun struct {
	root   string
	module string
	pages  map[string]*nestedPage
	// regions holds the regions that the run excerpts, by their paths in root.
	regions map[regionKey]bool
	err     error
}

// A nestedPage holds the text of a page before and after a nested run.
type nestedPage struct {
	before string
	after  string
}

// nestedRun returns the run of root and module, and adds it to the plan when it
// is new.
func (p *plan) nestedRun(root, module string) *nestedRun {
	root, module = path.Clean(root), path.Clean(module)
	key := root + "\x00" + module
	if r := p.nested[key]; r != nil {
		return r
	}
	r := &nestedRun{root: root, module: module, pages: map[string]*nestedPage{}}
	p.nested[key] = r
	return r
}

// update does the run r in the pool w. The nested go commands use the slots of
// the pool, so the run holds no slot while they go on.
func (g *Generator) update(ctx context.Context, w *pool, r *nestedRun) {
	work, err := g.session.WorkDir("sample-")
	if err != nil {
		r.err = err
		return
	}
	defer func() { _ = os.RemoveAll(work) }()
	session, err := g.session.TempDir()
	if err != nil {
		r.err = err
		return
	}
	clean := r.scrubber(work, session)
	inner, docs, log, err := g.nestedGenerator(ctx, r, work, session)
	if err != nil {
		r.err = errors.New(clean(err.Error()))
		return
	}
	w.withoutSlot(func() { _, err = inner.Run(ctx, docs, true) })
	r.regions = sourceRegions(inner.regions, work, g.mod.RootDir(r.root))
	if err != nil {
		if ctx.Err() == nil {
			_, _ = io.WriteString(g.log, clean(log.String()))
		}
		r.err = errors.New(clean(err.Error()))
		return
	}
	for name, pg := range r.pages {
		if pg == nil {
			continue
		}
		b, err := os.ReadFile(filepath.Join(work, filepath.FromSlash(name)))
		if err != nil {
			r.err = errors.New(clean(err.Error()))
			return
		}
		pg.before = clean(pg.before)
		pg.after = clean(string(excerpt.Source(b)))
	}
}

// sourceRegions returns regions, regions of the copy at work, with the paths of the
// same files in the directory src.
func sourceRegions(regions map[regionKey]bool, work, src string) map[regionKey]bool {
	work, src = canonical(work), canonical(src)
	out := make(map[regionKey]bool, len(regions))
	for k := range regions {
		if rel, err := filepath.Rel(work, k.path); err == nil && filepath.IsLocal(rel) {
			out[regionKey{filepath.Join(src, rel), k.name}] = true
		}
	}
	return out
}

// sampleWorkspace returns the GOWORK value of a run in the copy at work of the
// directory src, with the value current in the run around it. A workspace file in
// src has its place in the copy. Any other file names modules that are not in the
// copy, so the value is "". An empty value and off stay.
func sampleWorkspace(current, src, work string) string {
	if current == "" || current == "off" {
		return current
	}
	if !gomod.Inside(src, current) {
		return ""
	}
	rel, err := filepath.Rel(canonical(src), canonical(current))
	if err != nil {
		return ""
	}
	return filepath.Join(work, rel)
}

// nestedGenerator copies the root of r to work, reads the text of each page of r
// and returns the Generator of the run, its docs directory and its log.
func (g *Generator) nestedGenerator(ctx context.Context, r *nestedRun, work, session string) (*Generator, string, *bytes.Buffer, error) {
	if err := copyRoot(ctx, g.mod.RootDir(r.root), work, session, r.root); err != nil {
		return nil, "", nil, err
	}
	settings, err := g.settings(work)
	if err != nil {
		return nil, "", nil, err
	}
	var pages []string
	for _, name := range sortedKeys(r.pages) {
		if b, err := readPage(filepath.Join(work, filepath.FromSlash(name))); err == nil {
			r.pages[name] = &nestedPage{before: string(excerpt.Source(b))}
			pages = append(pages, name)
		}
	}
	if !localTo(work, filepath.ToSlash(settings.Docs)) {
		return nil, "", nil, fmt.Errorf("docs=%s is outside root=%s", settings.Docs, r.root)
	}
	docs := filepath.Join(work, filepath.FromSlash(settings.Docs))
	log := &bytes.Buffer{}
	inner, err := New(work, filepath.Join(work, filepath.FromSlash(r.module)), log, Options{
		Jobs:     g.session.Jobs,
		Tags:     g.session.Tags,
		Pages:    pages,
		Settings: g.settings,
		Print:    g.printAs,
		Vars:     settings.Vars,
		VarsFile: settings.VarsFile,
	})
	if err != nil {
		return nil, "", nil, err
	}
	inner.mod.Hint = "module= of the docgen:docgen marker"
	inner.session = g.session.Nested()
	inner.session.SetWorkspace(sampleWorkspace(g.session.Workspace(), g.mod.RootDir(r.root), work))
	inner.slots = g.slots
	inner.depth = g.depth - 1
	inner.level = g.level + 1
	if g.printAs != goprint.PrintRaw {
		inner.printAs = settings.Print
	}
	return inner, docs, log, nil
}

// readPage returns the contents of the regular file at p.
func readPage(p string) ([]byte, error) {
	info, err := os.Stat(p)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s is not a regular file", p)
	}
	return os.ReadFile(p)
}

// scrubber returns a function that replaces the temporary paths of r in a text:
// a path in the copy at work becomes a path in root, and any other path in the
// directory session becomes $GOTMPDIR.
func (r *nestedRun) scrubber(work, session string) func(string) string {
	tempPath := regexp.MustCompile("(?:" + regexp.QuoteMeta(session) + "|" + regexp.QuoteMeta(canonical(session)) + ")[^\\s\"'`<>()]*")
	return func(s string) string {
		s = strings.ReplaceAll(s, canonical(work), r.root)
		s = strings.ReplaceAll(s, work, r.root)
		return tempPath.ReplaceAllString(s, "$$GOTMPDIR")
	}
}
