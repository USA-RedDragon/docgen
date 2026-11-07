package gen

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/USA-RedDragon/docgen/internal/excerpt"
)

// A regionKey names a region: the canonical path of its file and its name.
type regionKey struct {
	path string
	name string
}

// usedRegions returns the regions that the docgen:code markers of pages excerpt.
func usedRegions(root string, pages []*page) map[regionKey]bool {
	used := map[regionKey]bool{}
	for _, pg := range pages {
		for _, m := range pg.markers {
			if h, ok := m.handler.(*codeHandler); ok && h.region != "" {
				used[regionKey{canonical(filepath.Join(root, filepath.FromSlash(h.file))), h.region}] = true
			}
		}
	}
	return used
}

// A varUser is a handler that reads variables.
type varUser interface {
	varNames() []string
}

// unusedVars returns a line for each variable that no marker of pages reads.
func (g *Generator) unusedVars(pages []*page) []string {
	used := map[string]bool{}
	for _, pg := range pages {
		for _, m := range pg.markers {
			if h, ok := m.handler.(varUser); ok {
				for _, name := range h.varNames() {
					used[name] = true
				}
			}
		}
	}
	var lines []string
	for _, name := range sortedKeys(g.vars) {
		if !used[name] {
			lines = append(lines, g.varsFile+": var "+name)
		}
	}
	return lines
}

// pairMarks returns the mark that opens each region of the file name with an end,
// the first of each name, with an error for each mark out of place. These are the
// rules of excerpt.Region: the mark after an open mark must be a docgen:end.
func pairMarks(name string, marks []excerpt.Mark) ([]excerpt.Mark, []error) {
	var regions []excerpt.Mark
	var errs []error
	first := map[string]int{}
	var open *excerpt.Mark
	for i, m := range marks {
		switch {
		case m.Name == "" && open == nil:
			errs = append(errs, fmt.Errorf("%s:%d: docgen:end has no docgen:region", name, m.Line))
		case m.Name == "":
			if first[open.Name] == open.Line {
				regions = append(regions, *open)
			}
			open = nil
		default:
			if open != nil {
				errs = append(errs, fmt.Errorf("%s:%d: docgen:region %s has no docgen:end", name, open.Line, open.Name))
			}
			open = &marks[i]
			if line, ok := first[m.Name]; ok {
				errs = append(errs, fmt.Errorf("%s:%d: docgen:region %s is also at line %d", name, m.Line, m.Name, line))
				continue
			}
			first[m.Name] = m.Line
		}
	}
	if open != nil {
		errs = append(errs, fmt.Errorf("%s:%d: docgen:region %s has no docgen:end", name, open.Line, open.Name))
	}
	return regions, errs
}

// writtenDiagrams returns the canonical paths of the files of the docgen:states
// markers of pages, and false when a marker failed, since its path is then not
// known.
func writtenDiagrams(pages []*page) (map[string]bool, bool) {
	written := map[string]bool{}
	complete := true
	for _, pg := range pages {
		for i, m := range pg.markers {
			if _, ok := m.handler.(*statesHandler); !ok {
				continue
			}
			f := pg.filled[i]
			if f == nil || f.err != nil {
				complete = false
				continue
			}
			for _, out := range f.files {
				written[canonical(out.path)] = true
			}
		}
	}
	return written, complete
}

// unusedDiagrams returns a line for each .svg file in a states directory next to
// a scanned page that is not in written.
func (g *Generator) unusedDiagrams(written map[string]bool) ([]string, error) {
	var lines []string
	var errs []error
	dirs := map[string]bool{}
	for _, pg := range g.scanned {
		dirs[filepath.Dir(pg.path)] = true
	}
	for _, dir := range sortedKeys(dirs) {
		states := filepath.Join(dir, "states")
		if info, err := os.Lstat(states); err != nil || !info.IsDir() {
			continue
		}
		entries, err := os.ReadDir(states)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		for _, e := range entries {
			p := filepath.Join(states, e.Name())
			if !e.Type().IsRegular() || !strings.HasSuffix(e.Name(), ".svg") || written[canonical(p)] {
				continue
			}
			rel, err := filepath.Rel(g.mod.Root, p)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			lines = append(lines, filepath.ToSlash(rel)+": docgen:states diagram")
		}
	}
	return lines, errors.Join(errs...)
}
