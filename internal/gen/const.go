package gen

import (
	"context"

	"github.com/USA-RedDragon/docgen/internal/goprint"
	"github.com/USA-RedDragon/docgen/internal/load"
	"github.com/USA-RedDragon/docgen/internal/markdown"
	"github.com/USA-RedDragon/docgen/internal/marker"
	"github.com/USA-RedDragon/docgen/internal/types"
)

// A constHandler is a docgen:const marker: the value of the constant or the
// package-level var name of the package pkg, with the print= value printAs: as
// fmt prints it, as the underlying value, with a fmt verb, or as the name of the
// constant with that value.
type constHandler struct {
	pkg     string
	name    string
	printAs string
}

func (h *constHandler) Bind(a *marker.Args) {
	h.pkg = a.Required(argPkg)
	h.name = a.Required(argName)
	h.printAs, _ = a.Optional(argPrint)
	a.NonEmpty(argPrint)
	a.Inline()
}

func (h *constHandler) plan(p *plan) {
	h.pkg = p.shortPkg(h.pkg)
	h.printAs = p.gen.printValue(h.printAs)
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
	var out string
	if h.printAs == goprint.PrintName {
		out, err = types.ConstantName(pi, c.Type, c.Value)
	} else {
		out, err = j.values.text(pi, valueRef{h.name, h.printAs}, c.Type, c.Value)
	}
	if err != nil {
		return "", nil, err
	}
	return markdown.CodeSpan(out), nil, nil
}
