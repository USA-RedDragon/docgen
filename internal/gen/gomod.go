package gen

import (
	"context"
	"path/filepath"

	"github.com/USA-RedDragon/docgen/internal/marker"
)

// A goModHandler is a docgen:gomod marker: the value of the line field of the
// go.mod of the module, or of the go.mod at the repository path file, with the
// code= value of valueArgs.
type goModHandler struct {
	noPlan
	valueArgs
	field string
	file  string
}

func (h *goModHandler) Bind(a *marker.Args) {
	h.field = a.Required("field")
	h.file, _ = a.Optional(argFile)
	a.NonEmpty(argFile)
	h.bindCode(a)
	a.Inline()
}

func (h *goModHandler) fill(_ context.Context, j job) (string, []outFile, error) {
	p := filepath.Join(j.mod.Dir, "go.mod")
	if h.file != "" {
		var err error
		if p, err = j.repoPath(h.file); err != nil {
			return "", nil, err
		}
	}
	v, err := moduleField(p, h.field)
	if err != nil {
		return "", nil, err
	}
	return h.inline(v), nil, nil
}
