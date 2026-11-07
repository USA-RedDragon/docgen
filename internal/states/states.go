// Package states records the states and transitions of state machines from the
// tests of their package, and writes them as tables and an SVG diagram.
package states

import (
	"context"
	"fmt"
	"io"
	"sort"
	"sync"

	"github.com/USA-RedDragon/docgen/internal/gocmd"
	"github.com/USA-RedDragon/docgen/internal/gomod"
	"github.com/USA-RedDragon/docgen/internal/load"
)

// A Recorder records the machines of the markers of a run.
type Recorder struct {
	mod      *gomod.Module
	session  *gocmd.Session
	log      io.Writer
	loaded   *load.Set
	machines map[string]map[string][]string
	mu       sync.Mutex
	runs     map[string]*stateRun
}

// New returns a Recorder for the packages of mod. It runs go in session and
// writes progress to log.
func New(mod *gomod.Module, session *gocmd.Session, log io.Writer) *Recorder {
	return &Recorder{mod: mod, session: session, log: log, machines: map[string]map[string][]string{}}
}

// Add adds the machine typ of the package ip to the next Run. A run value other
// than "" is a -run pattern of its tests. When each Add of a package has one,
// Run runs the tests that any of the patterns match, and otherwise all tests.
func (rc *Recorder) Add(ip, typ, run string) {
	if rc.machines[ip] == nil {
		rc.machines[ip] = map[string][]string{}
	}
	rc.machines[ip][typ] = append(rc.machines[ip][typ], run)
}

// Run records each package with a machine, one after the other.
func (rc *Recorder) Run(ctx context.Context, loaded *load.Set) {
	for _, job := range rc.Jobs(loaded) {
		if ctx.Err() != nil {
			return
		}
		job(ctx)
	}
}

// Jobs returns one function for each package with a machine, from the packages
// in loaded. Each records its package and can run at the same time as the
// others. An error of a package is the result of Fill for its machines.
func (rc *Recorder) Jobs(loaded *load.Set) []func(context.Context) {
	rc.loaded = loaded
	rc.runs = map[string]*stateRun{}
	pkgs := make([]string, 0, len(rc.machines))
	for ip := range rc.machines {
		pkgs = append(pkgs, ip)
	}
	sort.Strings(pkgs)
	jobs := make([]func(context.Context), 0, len(pkgs))
	for _, ip := range pkgs {
		if loaded.Err(ip) != nil {
			continue
		}
		jobs = append(jobs, func(ctx context.Context) {
			run, err := rc.record(ctx, ip, rc.machines[ip])
			if err != nil {
				run = &stateRun{err: fmt.Errorf("states of %s: %w", rc.mod.ShortPath(ip), err)}
			}
			rc.mu.Lock()
			defer rc.mu.Unlock()
			rc.runs[ip] = run
		})
	}
	return jobs
}
