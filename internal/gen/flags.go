package gen

import (
	"context"

	"github.com/USA-RedDragon/docgen/internal/marker"
)

// A flagsHandler is a docgen:flags marker: it builds the command in the directory
// cmd and writes the usage that it prints for -h. A failed build, a crash, an
// exit status other than 0, 1 or 2 and output with no line that starts with
// usage and no flag line are errors.
type flagsHandler struct {
	cmd string
	run *toolRun
}

func (h *flagsHandler) Bind(a *marker.Args) {
	h.cmd = a.Required("cmd")
}

func (h *flagsHandler) plan(p *plan) {
	h.run = p.tool("flags "+h.cmd, func(ctx context.Context) (string, error) {
		return p.gen.usageOf(ctx, h.cmd)
	})
}

func (h *flagsHandler) fill(context.Context, job) (string, []outFile, error) {
	return h.run.out, nil, h.run.err
}
