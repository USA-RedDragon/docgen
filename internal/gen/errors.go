package gen

import (
	"context"
	"fmt"

	"github.com/USA-RedDragon/docgen/internal/gomod"
	"github.com/USA-RedDragon/docgen/internal/markdown"
	"github.com/USA-RedDragon/docgen/internal/marker"
)

// An errorsHandler is a docgen:errors marker: the table of the errors in the doc
// comments of the constructors and methods of the type typ.
type errorsHandler struct {
	typeArgs
}

func (h *errorsHandler) Bind(a *marker.Args) {
	h.bindType(a)
}

func (h *errorsHandler) plan(p *plan) {
	h.pkg = p.shortPkg(h.pkg)
	p.load(h.pkg)
	if gomod.HasGo(p.mod.Dir) {
		p.pkgs[p.mod.Path] = true
	}
}

func (h *errorsHandler) fill(_ context.Context, j job) (string, []outFile, error) {
	pi, err := j.loaded.Package(h.pkg)
	if err != nil {
		return "", nil, err
	}
	t, err := lookupType(pi, h.typ)
	if err != nil {
		return "", nil, err
	}
	if t.Doc == nil {
		return "", nil, fmt.Errorf("no doc for %s", h.typ)
	}
	idx, err := j.errorIndex(pi)
	if err != nil {
		return "", nil, err
	}
	found := namedErrors(pi, t, idx)
	if len(found) == 0 {
		return fmt.Sprintf("The doc comments of [`%s`](%s) and its methods name no error.", t.Name, markdown.PkgLink(pi.Path, t.Name)), nil, nil
	}
	return errorTable(found), nil, nil
}
