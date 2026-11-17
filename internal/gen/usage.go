package gen

import (
	"context"
	"errors"
	"strings"

	"github.com/USA-RedDragon/docgen/internal/marker"
)

// A usageHandler is a docgen:usage marker: the output of the command in the
// directory cmd with the arguments args, -h by default. A failed build, a crash,
// an exit status other than 0, 1 or 2 and output with no line that starts with
// usage and no flag line are errors.
//
// With crate=, the command is a binary of a Cargo package, built with cargo, and
// bin= names it.
type usageHandler struct {
	cmd   string
	crate string
	bin   string
	args  []string
	run   *toolRun
	feat  featureArgs
}

func (h *usageHandler) Bind(a *marker.Args) {
	h.crate, _ = a.Optional(argCrate)
	h.bin, _ = a.Optional(kindBin)
	a.NonEmpty(argCrate, kindBin)
	if h.crate == "" {
		if h.bin != "" {
			a.Fail("has bin= with no crate=")
		}
		h.cmd = a.Required(argCmd)
	} else if _, ok := a.Optional(argCmd); ok {
		a.Fail("has both cmd= and crate=; use one")
	} else {
		h.feat.bind(a)
	}
	h.args = []string{"-h"}
	value, ok := a.Optional("args")
	a.NonEmpty("args")
	if !ok || value == "" {
		return
	}
	var err error
	if h.args, err = splitArgs(value); err != nil {
		a.Fail("args= " + err.Error())
	}
}

func (h *usageHandler) plan(p *plan) {
	if h.crate != "" {
		feats := p.gen.features(&h.feat)
		key := "usage crate " + h.crate + "\x00" + h.bin + "\x00" + feats.Key() + "\x00" + strings.Join(h.args, "\x00")
		p.rust()
		h.run = p.cargoTool(key, func(ctx context.Context) (string, error) {
			return p.gen.rustUsageOf(ctx, h.crate, h.bin, h.args, feats)
		})
		return
	}
	key := "usage " + h.cmd + "\x00" + strings.Join(h.args, "\x00")
	h.run = p.tool(key, func(ctx context.Context) (string, error) {
		return p.gen.usageOf(ctx, h.cmd, h.args)
	})
}

func (h *usageHandler) fill(context.Context, job) (string, []outFile, error) {
	return h.run.out, nil, h.run.err
}

// splitArgs returns the arguments of the args= value s, split on white space. A
// part in double quotes is one argument, also with spaces. No argument and a
// double quote with no end are errors.
func splitArgs(s string) ([]string, error) {
	var args []string
	var cur strings.Builder
	inArg, quoted := false, false
	for _, r := range s {
		switch {
		case r == '"':
			quoted, inArg = !quoted, true
		case !quoted && (r == ' ' || r == '\t'):
			if inArg {
				args = append(args, cur.String())
				cur.Reset()
				inArg = false
			}
		default:
			cur.WriteRune(r)
			inArg = true
		}
	}
	if quoted {
		return nil, errors.New(`has a " with no end`)
	}
	if inArg {
		args = append(args, cur.String())
	}
	if len(args) == 0 {
		return nil, errors.New("has no argument")
	}
	return args, nil
}
