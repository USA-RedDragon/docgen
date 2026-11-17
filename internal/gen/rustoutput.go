package gen

import (
	"context"
	"fmt"
	"strings"

	"github.com/USA-RedDragon/docgen/internal/markdown"
	"github.com/USA-RedDragon/docgen/internal/marker"
)

const (
	kindExample = "example"
	kindBin     = "bin"
	kindDoctest = "doctest"
)

// A rustOutput is the Rust form of a docgen:output marker: the standard output
// of an example or a binary of the Cargo package crate, or the doctest of the
// item.
type rustOutput struct {
	crate string
	kind  string
	name  string
	args  []string
	run   *toolRun
	batch *doctestBatch
	feat  featureArgs
}

// bind reads the Rust arguments of a. It returns false when the marker has none
// of them, so that it is a marker for Go.
func (r *rustOutput) bind(a *marker.Args) bool {
	crate, hasCrate := a.Optional(argCrate)
	example, hasExample := a.Optional(kindExample)
	bin, hasBin := a.Optional(kindBin)
	doctest, hasDoctest := a.Optional(kindDoctest)
	args, hasArgs := a.Optional("args")
	if !hasCrate && !hasExample && !hasBin && !hasDoctest && !hasArgs {
		return false
	}
	if !hasCrate {
		a.Fail("example=, bin=, doctest= and args= need crate=")
		return true
	}
	a.NonEmpty(argCrate, kindExample, kindBin, kindDoctest, "args")
	for _, key := range []string{argCmd, "exec", "test", argPkg} {
		if _, has := a.Optional(key); has {
			a.Fail("with crate=, there is no " + key + "=; give one of example=, bin= or doctest=")
			return true
		}
	}
	r.feat.bind(a)
	n := 0
	for _, has := range []bool{hasExample, hasBin, hasDoctest} {
		if has {
			n++
		}
	}
	want := 2 + r.feat.count
	if hasArgs {
		want++
	}
	if n != 1 || a.Len() != want {
		a.Fail("with crate=, must have exactly one of example=, bin= or doctest=")
		return true
	}
	if hasArgs && hasDoctest {
		a.Fail("args= goes with example= or bin=, not doctest=")
		return true
	}
	r.crate = crate
	switch {
	case hasExample:
		r.kind, r.name = kindExample, example
	case hasBin:
		r.kind, r.name = kindBin, bin
	default:
		r.kind, r.name = kindDoctest, doctest
	}
	if hasArgs {
		var err error
		if r.args, err = splitArgs(args); err != nil {
			a.Fail("args= " + err.Error())
		}
	}
	return true
}

func (r *rustOutput) plan(p *plan) {
	if r.kind == kindDoctest {
		r.batch = p.rust().batchOf(p, r.crate, p.gen.features(&r.feat))
		r.batch.add(r.name)
		return
	}
	feats := p.gen.features(&r.feat)
	key := strings.Join(append([]string{"rust", r.kind, r.crate, r.name, feats.Key()}, r.args...), "\x00")
	p.rust()
	r.run = p.cargoTool(key, func(ctx context.Context) (string, error) {
		return p.gen.runTarget(ctx, r.crate, r.kind, r.name, r.args, feats)
	})
}

func (r *rustOutput) fill() (string, []outFile, error) {
	if r.kind == kindDoctest {
		res, ok := r.batch.results[r.name]
		if !ok {
			return "", nil, fmt.Errorf("no result for doctest %s of %s", r.name, r.crate)
		}
		return res.text, nil, res.err
	}
	if r.run.err != nil {
		return "", nil, r.run.err
	}
	return markdown.Fence("text", r.run.out), nil, nil
}
