package gen

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/USA-RedDragon/docgen/internal/excerpt"
	"github.com/USA-RedDragon/docgen/internal/gomod"
	"github.com/USA-RedDragon/docgen/internal/links"
)

// A linkCheck holds what the check of the links of a run reads: the text of the
// scanned pages and the files that the run generates, by canonical path.
type linkCheck struct {
	g       *Generator
	texts   map[string]string
	written map[string]bool
	anchors map[string]map[string]bool
}

// scanLinks sets g.broken from the links of each scanned page, in the text that
// update writes to it, so a stale body adds no line. rendered holds that text by
// key, for a page with a marker. The file of a generated diagram counts as present,
// since the stale line of its marker covers it.
func (g *Generator) scanLinks(ctx context.Context, written map[string]bool, rendered map[string]string) {
	c := &linkCheck{g: g, texts: map[string]string{}, written: written, anchors: map[string]map[string]bool{}}
	for _, pg := range g.scanned {
		c.texts[pg.key] = pg.text
		if text, ok := rendered[pg.key]; ok {
			c.texts[pg.key] = string(excerpt.Source([]byte(text)))
		}
	}
	var broken []string
	for _, pg := range g.scanned {
		if ctx.Err() != nil {
			return
		}
		for _, l := range links.Scan(c.texts[pg.key]) {
			if !c.valid(pg, l) {
				target := l.Target
				if target == "" {
					target = "()"
				}
				broken = append(broken, fmt.Sprintf("%s:%d: %s", pg.rel, l.Line, target))
			}
		}
	}
	sort.Strings(broken)
	g.broken = broken
}

// valid reports whether the link l of pg reaches a file in the boundary of pg, and
// its anchor when it has one. A path that starts with / is relative to the
// repository root, as on GitHub.
func (c *linkCheck) valid(pg *page, l links.Link) bool {
	if l.Undefined || l.Target == "" {
		return false
	}
	path, fragment, local := links.Split(l.Target)
	if !local {
		return true
	}
	full := pg.path
	switch {
	case strings.HasPrefix(path, "/"):
		full = filepath.Join(c.g.mod.Root, filepath.FromSlash(path))
	case path != "":
		full = filepath.Join(filepath.Dir(pg.path), filepath.FromSlash(path))
	}
	if !gomod.Inside(pg.base, full) {
		return false
	}
	info, err := os.Stat(full)
	if err != nil {
		return c.written[canonical(full)]
	}
	if strings.HasSuffix(path, "/") && !info.IsDir() {
		return false
	}
	if fragment == "" || info.IsDir() || filepath.Ext(full) != ".md" {
		return true
	}
	return c.anchorsOf(full)[fragment]
}

// anchorsOf returns the anchors of the Markdown file at path.
func (c *linkCheck) anchorsOf(path string) map[string]bool {
	key := canonical(path)
	if a, ok := c.anchors[key]; ok {
		return a
	}
	text, ok := c.texts[key]
	if !ok {
		b, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		text = string(excerpt.Source(b))
	}
	a := links.Anchors(text)
	c.anchors[key] = a
	return a
}
