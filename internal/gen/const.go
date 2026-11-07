package gen

import (
	"context"

	"github.com/USA-RedDragon/docgen/internal/load"
	"github.com/USA-RedDragon/docgen/internal/marker"
	"github.com/USA-RedDragon/docgen/internal/types"
)

// A constHandler is a docgen:const marker: the value of the constant or the
// package-level var name of the package pkg, with the print= and code= values of
// valueArgs.
type constHandler struct {
	valueArgs
	pkg  string
	name string
}

func (h *constHandler) Bind(a *marker.Args) {
	h.pkg = a.Required(argPkg)
	h.name = a.Required(argName)
	h.bindPrint(a, true)
	h.bindCode(a)
	a.Inline()
}

func (h *constHandler) plan(p *plan) {
	h.pkg = p.shortPkg(h.pkg)
	h.planPrint(p.gen)
	p.load(h.pkg)
	p.values = append(p.values, h)
}

func (h *constHandler) addValues(vs *valueSet, loaded *load.Set) {
	pi, err := loaded.Package(h.pkg)
	if err != nil {
		return
	}
	if c, err := types.Value(pi, h.name); err == nil {
		vs.add(pi, valueRef{h.name, h.printAs}, c.Type)
	}
}

func (h *constHandler) fill(_ context.Context, j job) (string, []outFile, error) {
	pi, err := j.loaded.Package(h.pkg)
	if err != nil {
		return "", nil, err
	}
	c, err := types.Value(pi, h.name)
	if err != nil {
		return "", nil, err
	}
	out, err := h.constText(j.values, pi, h.name, c.Type, c.Value)
	if err != nil {
		return "", nil, err
	}
	return h.inline(out), nil, nil
}
