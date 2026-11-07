package gen

import (
	"context"
	"errors"
	"slices"
	"sort"
	"strings"
	"sync"

	"github.com/USA-RedDragon/docgen/internal/gomod"
	"github.com/USA-RedDragon/docgen/internal/load"
	"github.com/USA-RedDragon/docgen/internal/states"
)

// A plan holds the work of the markers of a run. Each handler adds to it before
// any go command runs.
type plan struct {
	mod      *gomod.Module
	states   *states.Recorder
	pkgs     map[string]bool
	programs map[programRun]bool
	tests    map[string]map[string]bool
	tools    map[string]*toolRun
	toolJobs []func(ctx context.Context)
	nested   map[string]*nestedRun
	values   []valueReader
	gen      *Generator
}

func (g *Generator) newPlan() *plan {
	return &plan{
		mod:      g.mod,
		states:   states.New(g.mod, g.session, g.log),
		pkgs:     map[string]bool{},
		programs: map[programRun]bool{},
		tests:    map[string]map[string]bool{},
		tools:    map[string]*toolRun{},
		nested:   map[string]*nestedRun{},
		gen:      g,
	}
}

// shortPkg returns the pkg= value arg as a path in the module, and arg when it
// is not a package of the module.
func (p *plan) shortPkg(arg string) string {
	ip, err := p.mod.PkgPath(arg)
	if err != nil {
		return arg
	}
	return p.mod.ShortPath(ip)
}

// outside reports whether the pkg= value arg leaves the repository. No go command
// reads such a package.
func (p *plan) outside(arg string) bool {
	_, err := p.mod.PkgPath(arg)
	return errors.Is(err, gomod.ErrOutside)
}

// load adds the package of the pkg= value arg to the packages to load and
// returns its import path.
func (p *plan) load(arg string) string {
	ip := p.mod.ImportPath(arg)
	if !p.outside(arg) {
		p.pkgs[ip] = true
	}
	return ip
}

// A toolRun is the result of one run of a program for the markers of a plan.
type toolRun struct {
	out string
	err error
}

// tool returns the run with the name key, and adds it to the plan with the
// function fn when it is new.
func (p *plan) tool(key string, fn func(ctx context.Context) (string, error)) *toolRun {
	if r := p.tools[key]; r != nil {
		return r
	}
	r := &toolRun{}
	p.tools[key] = r
	p.toolJobs = append(p.toolJobs, func(ctx context.Context) { r.out, r.err = fn(ctx) })
	return r
}

// prepared holds the results of the work of a plan.
type prepared struct {
	loaded   *load.Set
	states   *states.Recorder
	programs map[programRun]programResult
	testErrs map[string]error
	values   *valueSet
}

// maxCommands limits the go commands that run at the same time. Each one runs
// its own build actions, so more commands than this slow down a build with a
// cold cache and do not speed up a warm one.
const maxCommands = 8

// loadCommands is the number of go commands of a package load at the same time:
// go/packages runs go list and a query of the compiler sizes in parallel.
const loadCommands = 2

// prepare does the work of p, in the slots of g: at most Options.Jobs units
// and maxCommands units at the same time, nested runs included. The package load holds its slots from the start, so it does not
// wait behind the other work, and runs next to the work that does not read its
// result. It returns a nil result with the error when ctx ends. A package that
// does not load is an error of the markers that read it.
func (g *Generator) prepare(ctx context.Context, p *plan) (*prepared, error) {
	res := &prepared{states: p.states, programs: map[programRun]programResult{}, testErrs: map[string]error{}, values: &valueSet{pkgs: map[string]*packageValues{}}}
	var mu sync.Mutex
	w := newPool(ctx, g.slots)
	release := w.hold(min(loadCommands, cap(g.slots.sem)))
	runs := make([]programRun, 0, len(p.programs))
	for r := range p.programs {
		runs = append(runs, r)
	}
	slices.SortFunc(runs, func(a, b programRun) int { return strings.Compare(a.String(), b.String()) })
	for _, r := range runs {
		w.run(func(ctx context.Context) {
			result := g.runProgram(ctx, r)
			mu.Lock()
			defer mu.Unlock()
			res.programs[r] = result
		})
	}
	if g.mod.Path != "" {
		for _, ip := range sortedKeys(p.tests) {
			w.run(func(ctx context.Context) {
				if err := g.runTest(ctx, ip, p.tests[ip]); err != nil {
					mu.Lock()
					defer mu.Unlock()
					res.testErrs[ip] = err
				}
			})
		}
	}
	for _, key := range sortedKeys(p.nested) {
		w.run(func(ctx context.Context) { g.update(ctx, w, p.nested[key]) })
	}
	for _, job := range p.toolJobs {
		w.run(job)
	}
	var err error
	res.loaded, err = load.Packages(ctx, g.mod, g.session, p.pkgs)
	release()
	if err == nil {
		for _, job := range p.states.Jobs(res.loaded) {
			w.run(job)
		}
		for _, r := range p.values {
			r.addValues(res.values, res.loaded)
		}
		for _, job := range res.values.jobs(g) {
			w.run(job)
		}
	}
	w.wg.Wait()
	if ctx.Err() != nil {
		return nil, context.Cause(ctx)
	}
	if err != nil {
		return nil, err
	}
	return res, nil
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
