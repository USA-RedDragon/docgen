package gen

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/USA-RedDragon/docgen/internal/gomod"
	"github.com/USA-RedDragon/docgen/internal/marker"
)

// maxPasses limits the render passes that change a page. A chain of excerpts of
// pages takes one pass for each page in it, and one more pass that changes
// nothing confirms the result.
const maxPasses = 8

// filled holds what the fill of a marker returned. A fill that read a scanned
// page repeats in the next render pass. Any other fill runs once.
type filled struct {
	content  string
	files    []outFile
	err      error
	readPage bool
}

// An outFile is a file that the fill of a marker returns. subject is what the
// file shows. Two outFiles with one path and different subjects are an error.
type outFile struct {
	path    string
	data    []byte
	old     []byte
	subject string
}

// A pass holds the state of one render pass: the text of each scanned page and
// generated file, by canonical path, and the first output at each path.
type pass struct {
	texts   map[string]string
	outputs map[string]output
}

// An output is the first outFile at a path in a pass: its subject and the place
// of its marker.
type output struct {
	subject string
	place   string
}

// A result holds what the markers of one page generate.
type result struct {
	text     string
	files    []outFile
	stale    []string
	errs     []error
	texts    map[string]string
	readPage bool
}

// fillMarker returns the result of the marker at index i of pg in this pass.
func (g *Generator) fillMarker(ctx context.Context, res *prepared, ps *pass, pg *page, i int) *filled {
	f := pg.filled[i]
	if f != nil && !f.readPage {
		return f
	}
	f = &filled{}
	m := pg.markers[i]
	f.content, f.files, f.err = m.handler.fill(ctx, job{g, res, pg, m.Marker, ps, &f.readPage})
	pg.filled[i] = f
	return f
}

// render fills the markers of pg. A fill error does not change the body of that
// marker. Text outside a changed body does not change, line endings included.
func (g *Generator) render(ctx context.Context, res *prepared, ps *pass, pg *page) result {
	var r result
	var b strings.Builder
	last := 0
	for i, m := range pg.markers {
		b.WriteString(pg.raw[pg.rawOffset(last):pg.rawOffset(m.Start)])
		last = m.End
		f := g.fillMarker(ctx, res, ps, pg, i)
		r.readPage = r.readPage || f.readPage
		if f.err != nil {
			r.errs = append(r.errs, fmt.Errorf("%s:%d: %s: %w", pg.rel, m.Line, m, f.err))
			b.WriteString(pg.raw[pg.rawOffset(m.Start):pg.rawOffset(m.End)])
			continue
		}
		body := m.Wrap(strings.ReplaceAll(f.content, "\r\n", "\n"))
		if body == m.Body {
			b.WriteString(pg.raw[pg.rawOffset(m.Start):pg.rawOffset(m.End)])
		} else {
			b.WriteString(pg.crlf(body))
			r.stale = append(r.stale, fmt.Sprintf("%s:%d: %s", pg.rel, m.Line, m))
		}
		g.compareFiles(&r, ps, pg, m.Marker, f.files)
	}
	b.WriteString(pg.raw[pg.rawOffset(last):])
	r.text = b.String()
	return r
}

// compareFiles adds to r each of the files of m that differs from the file on
// disk. A file at the canonical path of an output of another subject in this pass
// is an error, and so are a file outside the boundary of pg and a file that is a
// scanned page, before any read of it.
func (g *Generator) compareFiles(r *result, ps *pass, pg *page, m marker.Marker, files []outFile) {
	for _, f := range files {
		place := fmt.Sprintf("%s:%d: %s", pg.rel, m.Line, m)
		key := canonical(f.path)
		if c, ok := ps.outputs[key]; ok && c.subject != f.subject {
			rel, _ := filepath.Rel(g.mod.Root, f.path)
			r.errs = append(r.errs, fmt.Errorf("%s and %s write the same file %s", c.place, place, filepath.ToSlash(rel)))
			continue
		}
		ps.outputs[key] = output{subject: f.subject, place: place}
		if !gomod.Inside(pg.base, f.path) {
			rel, _ := filepath.Rel(g.mod.Root, f.path)
			r.errs = append(r.errs, fmt.Errorf("%s: %s is outside %s", place, filepath.ToSlash(rel), g.boundaryName(pg)))
			continue
		}
		if i := slices.IndexFunc(g.scanned, func(s *page) bool { return s.key == key }); i >= 0 {
			rel, _ := filepath.Rel(g.mod.Root, f.path)
			r.errs = append(r.errs, fmt.Errorf("%s: %s is the page %s", place, filepath.ToSlash(rel), g.scanned[i].rel))
			continue
		}
		old, err := os.ReadFile(f.path)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			r.errs = append(r.errs, fmt.Errorf("%s:%d: %s: %w", pg.rel, m.Line, m, err))
			continue
		}
		if !bytes.Equal(old, f.data) {
			rel, _ := filepath.Rel(g.mod.Root, f.path)
			r.stale = append(r.stale, fmt.Sprintf("%s (from %s:%d: %s)", filepath.ToSlash(rel), pg.rel, m.Line, m))
			f.old = old
			r.files = append(r.files, f)
		}
	}
}

// boundaryName returns the name of the directory that holds the files of pg.
func (g *Generator) boundaryName(pg *page) string {
	if pg.base == g.mod.Root {
		return "the repository"
	}
	return "the docs directory"
}

// generated fills the markers of pages that write files and adds each file to
// texts, so that an excerpt of a file holds what update writes to it. These
// fills read no page, so they are the same in each pass.
func (g *Generator) generated(ctx context.Context, res *prepared, pages []*page, texts map[string]string) {
	ps := &pass{texts: texts, outputs: map[string]output{}}
	for _, pg := range pages {
		for i, m := range pg.markers {
			if _, ok := m.handler.(*statesHandler); !ok {
				continue
			}
			for _, f := range g.fillMarker(ctx, res, ps, pg, i).files {
				texts[canonical(f.path)] = string(f.data)
			}
		}
	}
}

// renderPass fills the markers of pages from texts and returns the changes as
// one result. Its text is not set.
func (g *Generator) renderPass(ctx context.Context, res *prepared, pages []*page, texts map[string]string) result {
	all := result{texts: map[string]string{}}
	ps := &pass{texts: texts, outputs: map[string]output{}}
	for _, pg := range pages {
		r := g.render(ctx, res, ps, pg)
		all.stale = append(all.stale, r.stale...)
		all.errs = append(all.errs, r.errs...)
		all.files = append(all.files, r.files...)
		all.texts[pg.key] = r.text
		all.readPage = all.readPage || r.readPage
		if r.text != pg.raw {
			all.files = append(all.files, outFile{path: pg.path, data: []byte(r.text), old: []byte(pg.raw)})
		}
	}
	return all
}

// renderAll repeats the render passes until the text of each page stays the
// same, so that an excerpt of a page holds what update writes to that page. A
// page that still changes in the pass after maxPasses is an error.
func (g *Generator) renderAll(ctx context.Context, res *prepared, pages []*page) result {
	texts := make(map[string]string, len(pages))
	for _, pg := range pages {
		texts[pg.key] = pg.raw
	}
	g.generated(ctx, res, pages, texts)
	var r result
	var moving []*page
	for range maxPasses + 1 {
		r = g.renderPass(ctx, res, pages, texts)
		moving = moving[:0]
		for _, pg := range pages {
			if r.texts[pg.key] != texts[pg.key] {
				moving = append(moving, pg)
			}
		}
		if !r.readPage || len(moving) == 0 || ctx.Err() != nil {
			return r
		}
		maps.Copy(texts, r.texts)
	}
	for _, pg := range moving {
		r.errs = append(r.errs, fmt.Errorf("%s: the excerpts of pages do not converge after %d render passes", pg.rel, maxPasses))
	}
	return r
}
