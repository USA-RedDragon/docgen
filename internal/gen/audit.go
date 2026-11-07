package gen

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
)

// scanRepository sets g.unused and g.broken, and returns an error for each mark without its
// pair and for each repeated region name. complete is false when a page did not
// parse. Then the markers of that page are not known, and only the errors of the
// marks are valid. It reads each file from disk, after the render of pages. texts
// holds what update writes to each page, by key. A file that does not read is an
// error, and the rest of the scan goes on.
func (g *Generator) scanRepository(ctx context.Context, pages []*page, complete bool, texts map[string]string) []error {
	names, marks, errs := g.repositoryMarks(ctx)
	var unused []string
	for i, name := range names {
		key := canonical(filepath.Join(g.mod.Root, filepath.FromSlash(name)))
		regions, bad := pairMarks(name, marks[i])
		errs = append(errs, bad...)
		for _, m := range regions {
			if complete && !g.regions[regionKey{key, m.Name}] {
				unused = append(unused, fmt.Sprintf("%s:%d: docgen:region %s", name, m.Line, m.Name))
			}
		}
	}
	if complete {
		unused = append(unused, g.unusedVars(pages)...)
	}
	written, ok := writtenDiagrams(pages)
	g.scanLinks(ctx, written, texts)
	if complete && ok {
		lines, err := g.unusedDiagrams(written)
		if err != nil {
			errs = append(errs, err)
		}
		unused = append(unused, lines...)
	}
	sort.Strings(unused)
	g.unused = unused
	return errs
}
