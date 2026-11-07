package gen

import (
	"context"
	"strings"

	"github.com/USA-RedDragon/docgen/internal/marker"
)

// A configulatorHandler is a docgen:configulator marker: the reference table
// that the configulator generator prints for the type typ of the package pkg,
// so that the page matches what the program loads. opts holds the options of
// the generator from the arguments of the marker.
type configulatorHandler struct {
	typeArgs
	opts []string
	run  *toolRun
}

func (h *configulatorHandler) Bind(a *marker.Args) {
	h.bindType(a)
	for _, o := range []struct{ arg, opt string }{
		{"env", "-env-prefix"},
		{"envsep", "-env-separator"},
		{"flagsep", "-flag-separator"},
	} {
		if v, ok := a.Optional(o.arg); ok {
			h.opts = append(h.opts, o.opt, v)
		}
	}
	a.NonEmpty("env", "envsep", "flagsep")
}

func (h *configulatorHandler) plan(p *plan) {
	h.pkg = p.shortPkg(h.pkg)
	ip, err := p.mod.PkgPath(h.pkg)
	if err != nil {
		h.run = &toolRun{err: err}
		return
	}
	key := strings.Join(append([]string{"configulator", ip}, h.args()...), "\x00")
	h.run = p.tool(key, func(ctx context.Context) (string, error) {
		return p.gen.referenceTable(ctx, ip, h.typ, h.args())
	})
}

func (h *configulatorHandler) fill(context.Context, job) (string, []outFile, error) {
	return h.run.out, nil, h.run.err
}

// args returns the arguments of go that run the generator.
func (h *configulatorHandler) args() []string {
	return append([]string{"run", configulatorCmd, "-type", h.typ, "-markdown", "-no-validate"}, h.opts...)
}
