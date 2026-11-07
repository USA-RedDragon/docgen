package gen

import (
	"context"
	"fmt"

	"github.com/USA-RedDragon/docgen/internal/marker"
)

// A varHandler is a docgen:var marker: the value of the variable name of the vars
// setting, with the print= and code= values of valueArgs.
type varHandler struct {
	noPlan
	valueArgs
	name string
}

func (h *varHandler) Bind(a *marker.Args) {
	h.name = a.Required(argName)
	h.bindPrint(a, false)
	h.bindCode(a)
	a.Inline()
}

func (h *varHandler) fill(_ context.Context, j job) (string, []outFile, error) {
	v, err := varValue(j.vars, h.name)
	if err != nil {
		return "", nil, err
	}
	if v, err = h.stringText(v); err != nil {
		return "", nil, err
	}
	return h.inline(v), nil, nil
}

func (h *varHandler) varNames() []string {
	return []string{h.name}
}

// varValue returns the value of the variable name of vars. A name that vars does
// not hold is an error.
func varValue(vars map[string]string, name string) (string, error) {
	v, ok := vars[name]
	if !ok {
		return "", fmt.Errorf("var %s is not in the vars setting", name)
	}
	return v, nil
}
