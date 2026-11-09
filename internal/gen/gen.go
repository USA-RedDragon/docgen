// Package gen fills the markers of the Markdown pages of a directory: it scans
// the pages, collects and runs the work of the markers, then compares each body
// with the page and writes the ones that differ.
package gen

import (
	"context"
	"errors"
	"io"
	"maps"
	"runtime"
	"slices"
	"sort"
	"sync"

	"github.com/USA-RedDragon/docgen/internal/gocmd"
	"github.com/USA-RedDragon/docgen/internal/gomod"
	"github.com/USA-RedDragon/docgen/internal/goprint"
)

// A Generator fills the pages of one repository from one Go module.
type Generator struct {
	mod      *gomod.Module
	session  *gocmd.Session
	log      io.Writer
	pages    []string
	wantDiff bool
	diff     string
	// audit makes Run scan the repository and the links of the pages. unused and
	// broken hold the result.
	audit  bool
	unused []string
	broken []string
	// scanned holds each page of the run, also one with no marker.
	scanned []*page
	// ignored reports whether git ignores a slash path of the repository. scan
	// sets it.
	ignored func(string) bool
	// roots holds the canonical directories of the docgen:docgen roots of the
	// pages. scan sets it.
	roots []string
	// regions holds the regions that the markers of this run and of its nested
	// runs excerpt.
	regions  map[regionKey]bool
	slots    *slots
	depth    int
	level    int
	settings func(root string) (Settings, error)
	printAs  string
	vars     map[string]string
	varsFile string
}

// Options holds the settings of a Generator.
type Options struct {
	// Jobs limits the work that runs at the same time, and 0 means the number
	// of CPUs. A value other than 0 is also the -p of the go commands.
	Jobs int
	// Tags holds the build tags of the go commands.
	Tags []string
	// Pages holds pages to fill in addition to the docs directory, as paths
	// relative to the repository root.
	Pages []string
	// Diff makes Run store the diff that Diff returns.
	Diff bool
	// Audit makes Run scan the repository and the links of the pages, and store
	// what Unused and Broken return.
	Audit bool
	// Depth is the number of levels of docgen:docgen markers, and 0 means none.
	Depth int
	// Print is the print= value of a marker with none: goprint.PrintGo, "" for
	// the same, goprint.PrintRaw or goprint.PrintConfig.
	Print string
	// Vars holds the values of the docgen:var and docgen:text markers.
	Vars map[string]string
	// VarsFile is the path of the file of Vars in the lines of Unused.
	VarsFile string
	// Settings returns the settings of the copy at root of a docgen:docgen
	// marker. nil means a scan of the whole copy.
	Settings func(root string) (Settings, error)
}

// A lockedWriter serializes writes, so that the lines of concurrent work stay
// whole.
type lockedWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (l *lockedWriter) Write(b []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.w.Write(b)
}

// canonical returns the absolute path of p with symbolic links resolved, so
// that two names of one file compare equal.
func canonical(p string) string {
	if r, err := gomod.Resolve(p); err == nil {
		return r
	}
	return p
}

// New returns a Generator for the repository at root and the Go module in the
// directory modDir. It writes progress to log.
func New(root, modDir string, log io.Writer, opts Options) (*Generator, error) {
	mod, err := gomod.Open(root, modDir)
	if err != nil {
		return nil, err
	}
	jobs := opts.Jobs
	if jobs <= 0 {
		jobs = runtime.NumCPU()
	}
	settings := opts.Settings
	if settings == nil {
		settings = func(string) (Settings, error) { return Settings{Docs: "."}, nil }
	}
	return &Generator{
		mod:      mod,
		session:  &gocmd.Session{Jobs: opts.Jobs, Tags: opts.Tags},
		log:      &lockedWriter{w: log},
		pages:    opts.Pages,
		wantDiff: opts.Diff,
		audit:    opts.Audit,
		slots:    newSlots(min(jobs, maxCommands)),
		depth:    opts.Depth,
		level:    1,
		settings: settings,
		printAs:  opts.Print,
		vars:     opts.Vars,
		varsFile: opts.VarsFile,
	}, nil
}

// printValue returns the print= value of a marker with the argument arg, with ""
// for the default rule of fmt.
func (g *Generator) printValue(arg string) string {
	if arg == "" {
		arg = g.printAs
	}
	if arg == goprint.PrintGo {
		return ""
	}
	return arg
}

// Diff returns the unified diff of the last Run when Options.Diff is true, and
// otherwise "".
func (g *Generator) Diff() string {
	return g.diff
}

// Unused returns the lines of the last Run for what no marker uses, in sorted
// order, when Options.Audit is true.
func (g *Generator) Unused() []string {
	return g.unused
}

// Broken returns the lines of the last Run for the links that reach nothing, in
// sorted order, when Options.Audit is true.
func (g *Generator) Broken() []string {
	return g.broken
}

// distinct returns errs with one error for each text, since the page scan and the
// repository scan can both fail on one path.
func distinct(errs []error) []error {
	seen := map[string]bool{}
	return slices.DeleteFunc(errs, func(err error) bool {
		text := err.Error()
		found := seen[text]
		seen[text] = true
		return found
	})
}

// Run fills the markers of the pages in docsDir. It returns one line for each
// body and file that differs, and an error that joins the failure of each page
// and marker. With write true it writes the changes, and it writes nothing
// unless every marker of every page succeeded. When ctx ends, Run stops, writes
// nothing and returns the cause alone. Once the writes start, they finish. Run
// removes the temporary directory of the go commands before it returns.
func (g *Generator) Run(ctx context.Context, docsDir string, write bool) ([]string, error) {
	defer g.session.Close()
	pages, errs, err := g.scan(ctx, docsDir)
	if err != nil {
		return nil, err
	}
	complete := len(errs) == 0
	g.regions = usedRegions(g.mod.Root, pages)
	p := g.newPlan()
	for _, pg := range pages {
		for _, m := range pg.markers {
			m.handler.plan(p)
		}
	}
	res, err := g.prepare(ctx, p)
	if ctx.Err() != nil {
		return nil, context.Cause(ctx)
	}
	if err != nil {
		return nil, errors.Join(append(errs, err)...)
	}
	for _, run := range p.nested {
		maps.Copy(g.regions, run.regions)
		complete = complete && run.err == nil
	}
	r := g.renderAll(ctx, res, pages)
	if ctx.Err() != nil {
		return nil, context.Cause(ctx)
	}
	if g.audit {
		errs = append(errs, g.scanRepository(ctx, pages, complete, r.texts)...)
		if ctx.Err() != nil {
			return nil, context.Cause(ctx)
		}
	}
	sort.Strings(r.stale)
	if g.wantDiff {
		g.diff = g.diffOf(r.files)
	}
	if errs = append(errs, r.errs...); len(errs) > 0 {
		return r.stale, errors.Join(distinct(errs)...)
	}
	if write {
		if err := writeFiles(r.files); err != nil {
			return nil, err
		}
	}
	return r.stale, nil
}
