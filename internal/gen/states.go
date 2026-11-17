package gen

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/USA-RedDragon/docgen/internal/marker"
)

// A statesHandler is a docgen:states marker: the tables and the diagram of the
// machine typ. A run value other than "" is a -run pattern of the tests.
type statesHandler struct {
	typeArgs
	run string
	// diagram is the path of the SVG file, relative to the page, or "".
	diagram string
}

func (h *statesHandler) Bind(a *marker.Args) {
	h.bindType(a)
	h.run, _ = a.Optional("run")
	a.NonEmpty("run")
	h.diagram = bindDiagram(a)
}

// bindDiagram reads diagram= of a docgen:states marker: a path relative to the
// page that ends in .svg, or "".
func bindDiagram(a *marker.Args) string {
	diagram, _ := a.Optional("diagram")
	a.NonEmpty("diagram")
	switch {
	case diagram == "":
	case strings.HasPrefix(diagram, "/"):
		a.Fail("diagram=" + diagram + " is not relative to the page")
	case !strings.HasSuffix(diagram, ".svg"):
		a.Fail("diagram=" + diagram + " does not end in .svg")
	}
	return diagram
}

// isStates reports whether h is a docgen:states marker, of Go or of Rust.
func isStates(h handler) bool {
	if l, ok := h.(*langHandler); ok {
		h = l.handler
	}
	switch h.(type) {
	case *statesHandler, *rustStatesHandler:
		return true
	}
	return false
}

func (h *statesHandler) plan(p *plan) {
	h.pkg = p.shortPkg(h.pkg)
	if p.outside(h.pkg) {
		return
	}
	p.states.Add(p.load(h.pkg), h.typ, h.run)
}

func (h *statesHandler) fill(_ context.Context, j job) (string, []outFile, error) {
	body, svg, err := j.states.Fill(h.pkg, h.typ, filepath.Dir(j.pg.path), h.diagram)
	if err != nil {
		return "", nil, err
	}
	return body, []outFile{{path: svg.Path, data: svg.Data, subject: svg.Machine}}, nil
}
