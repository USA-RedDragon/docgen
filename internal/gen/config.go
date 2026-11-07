package gen

import (
	"context"
	"fmt"

	"github.com/USA-RedDragon/docgen/internal/defaults"
	"github.com/USA-RedDragon/docgen/internal/goprint"
	"github.com/USA-RedDragon/docgen/internal/load"
	"github.com/USA-RedDragon/docgen/internal/marker"
	"github.com/USA-RedDragon/docgen/internal/types"
)

// A configHandler is a docgen:config marker: the table of the exported fields
// of the struct typ of the package pkg, with the print= value of valueArgs for
// the default values.
type configHandler struct {
	typeArgs
	valueArgs
}

func (h *configHandler) Bind(a *marker.Args) {
	h.bindType(a)
	h.bindPrint(a, true)
}

func (h *configHandler) plan(p *plan) {
	h.pkg = p.shortPkg(h.pkg)
	h.planPrint(p.gen)
	p.load(h.pkg)
	p.values = append(p.values, h)
}

func (h *configHandler) addValues(vs *valueSet, loaded *load.Set) {
	pi, err := loaded.Package(h.pkg)
	if err != nil {
		return
	}
	t, err := lookupType(pi, h.typ)
	if err != nil || types.Generic(t) {
		return
	}
	st, ok := types.Struct(t)
	if !ok {
		return
	}
	for _, defs := range defaults.Find(pi, st) {
		for _, d := range defs {
			if d.Value == nil || !goprint.NeedsRun(d.Type, h.printAs) {
				continue
			}
			if expr, ok := vs.constExpr(pi, d.Type, d.Value); ok {
				vs.add(pi, valueRef{expr, h.printAs}, d.Type)
			}
		}
	}
}

func (h *configHandler) fill(_ context.Context, j job) (string, []outFile, error) {
	pi, err := j.loaded.Package(h.pkg)
	if err != nil {
		return "", nil, err
	}
	t, err := lookupType(pi, h.typ)
	if err != nil {
		return "", nil, err
	}
	if types.Generic(t) {
		return "", nil, fmt.Errorf("%s is generic", t.Name)
	}
	st, ok := types.Struct(t)
	if !ok {
		return "", nil, fmt.Errorf("%s is not a struct", t.Name)
	}
	fields := types.Fields(t)
	if len(fields) == 0 {
		return "", nil, fmt.Errorf("%s has no exported field", t.Name)
	}
	table, err := configTable(j.values, pi, fields, defaults.Find(pi, st), h.valueArgs)
	if err != nil {
		return "", nil, err
	}
	return table, nil, nil
}
