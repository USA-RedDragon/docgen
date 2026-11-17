package gen

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/USA-RedDragon/docgen/internal/excerpt"
	"github.com/USA-RedDragon/docgen/internal/marker"
)

// A cConstHandler is a docgen:const marker with file=: the replacement text of
// the object-like #define name of the C file, as written and never evaluated.
type cConstHandler struct {
	noPlan
	valueArgs
	file string
	name string
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
	if _, ok := a.Optional(argPrint); ok {
		a.Fail("print= does not apply to a C #define; the text is as written")
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
	return h.inline(v), nil, nil
}
