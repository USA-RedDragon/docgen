package gen

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/USA-RedDragon/docgen/internal/excerpt"
	"github.com/USA-RedDragon/docgen/internal/marker"
)

const cPrintValue = "value"

// A cConstHandler is a docgen:const marker with file=: the replacement text of
// the object-like #define name of the C file, as written and never evaluated.
// With print=value, a single integer literal prints without its suffix.
type cConstHandler struct {
	noPlan
	valueArgs
	file  string
	name  string
	value bool
}

func (h *cConstHandler) Bind(a *marker.Args) {
	h.file = a.Required(argFile)
	for _, k := range []string{argPkg, argCrate} {
		if _, ok := a.Optional(k); ok {
			a.Fail("has both file= and " + k + "=; file= is for C, pkg= for Go and crate= for Rust")
		}
	}
	if ext := filepath.Ext(h.file); ext != ".c" && ext != ".h" {
		a.Fail("file= is not a .c or .h file")
	}
	h.name = a.Required(argName)
	if p, ok := a.Optional(argPrint); ok {
		a.NonEmpty(argPrint)
		if p != cPrintValue {
			a.Fail("print=" + p + " does not apply to a C #define; the one value is print=value, an integer literal without its suffix")
		}
		h.value = true
	}
	h.bindCode(a)
	a.Inline()
}

func (h *cConstHandler) fill(_ context.Context, j job) (string, []outFile, error) {
	path, err := j.repoPath(h.file)
	if err != nil {
		return "", nil, err
	}
	src, err := j.source(path)
	if err != nil {
		return "", nil, err
	}
	v, err := excerpt.CDefineValue(src, h.name)
	if err != nil {
		return "", nil, fmt.Errorf("%s: %w", h.file, err)
	}
	if h.value {
		if v, err = excerpt.CIntegerValue(v); err != nil {
			return "", nil, fmt.Errorf("%s: %s: print=value: %w", h.file, h.name, err)
		}
	}
	return h.inline(v), nil, nil
}
