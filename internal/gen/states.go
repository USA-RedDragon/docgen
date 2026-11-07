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
	h.diagram, _ = a.Optional("diagram")
	a.NonEmpty("run", "diagram")
	switch {
	case h.diagram == "":
	case strings.HasPrefix(h.diagram, "/"):
		a.Fail("diagram=" + h.diagram + " is not relative to the page")
	case !strings.HasSuffix(h.diagram, ".svg"):
		a.Fail("diagram=" + h.diagram + " does not end in .svg")
	}
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
