package gen

import (
	"context"
	"fmt"
	"net/url"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"

	"github.com/USA-RedDragon/docgen/internal/cargo"
	"github.com/USA-RedDragon/docgen/internal/markdown"
	"github.com/USA-RedDragon/docgen/internal/marker"
	"github.com/USA-RedDragon/docgen/internal/rustsrc"
	"github.com/USA-RedDragon/docgen/internal/states"
)

// A rustStatesHandler is a docgen:states marker with crate=: the tables and
// the diagram of the machine typ of the crate, from a run of its tests with a
// hook on a copy of the workspace.
type rustStatesHandler struct {
	crate   string
	typ     string
	run     string
	diagram string
	feat    featureArgs
	rp      *rustPlan
	req     *rustStates
}

func (h *rustStatesHandler) Bind(a *marker.Args) {
	h.feat.bind(a)
	h.crate = bindCrate(a)
	h.typ = a.Required(argType)
	h.run, _ = a.Optional("run")
	h.diagram = bindDiagram(a)
	a.NonEmpty("run")
	if strings.HasPrefix(h.run, "-") {
		a.Fail("run=" + h.run + " starts with -, which cargo test reads as an option")
	}
}

func (h *rustStatesHandler) plan(p *plan) {
	h.rp = p.rust()
	h.req = h.rp.addStates(&rustStates{crate: h.crate, typ: h.typ, run: h.run, feats: p.gen.features(&h.feat)})
}

func (h *rustStatesHandler) fill(_ context.Context, j job) (string, []outFile, error) {
	if h.req.err != nil {
		return "", nil, h.req.err
	}
	m := h.req.machine
	if m == nil {
		if _, err := h.rp.crate(h.crate); err != nil {
			return "", nil, err
		}
		return "", nil, fmt.Errorf("crate %s was not read", h.crate)
	}
	if len(m.Edges) == 0 && len(m.Skipped) == 0 {
		return "", nil, fmt.Errorf("the tests of crate %s recorded no transition of %s", h.crate, h.typ)
	}
	var b strings.Builder
	switch m.From {
	case rustsrc.StateSelf:
		fmt.Fprintf(&b, "States of %s, from its variant:\n\n", markdown.CodeSpan(m.Type.Name))
	case rustsrc.StateMethod:
		fmt.Fprintf(&b, "States of %s, from %s:\n\n", markdown.CodeSpan(m.Type.Name), markdown.CodeSpan(m.Type.Name+"::"+m.Via))
	default:
		fmt.Fprintf(&b, "States of %s, from its field %s:\n\n", markdown.CodeSpan(m.Type.Name), markdown.CodeSpan(m.Via))
	}
	reached := map[string]bool{}
	for k := range m.First {
		reached[k] = true
	}
	for k := range m.Edges {
		reached[k[0]], reached[k[1]] = true, true
	}
	b.WriteString("| State | Meaning | Seen in tests |\n|---|---|---|\n")
	order := map[string]int{}
	nodes := make([]string, 0, len(m.Enum.Variants))
	first := make([]bool, 0, len(m.Enum.Variants))
	for i, v := range m.Enum.Variants {
		order[v.Name] = i
		nodes = append(nodes, v.Name)
		first = append(first, m.First[v.Name])
		seen := "no"
		if reached[v.Name] {
			seen = cellYes
		}
		if m.First[v.Name] {
			seen += ", first state"
		}
		fmt.Fprintf(&b, "| %s | %s | %s |\n", markdown.CodeSpan(variantPattern(m.Enum.Name, v)), markdown.Cell(v.Doc), seen)
	}
	keys := make([][2]string, 0, len(m.Edges))
	for k := range m.Edges {
		keys = append(keys, k)
	}
	slices.SortFunc(keys, func(x, y [2]string) int {
		if c := order[x[0]] - order[y[0]]; c != 0 {
			return c
		}
		return order[x[1]] - order[y[1]]
	})
	b.WriteString("\nTransitions seen in tests:\n\n| From | Method call | To |\n|---|---|---|\n")
	arcs := make([]states.Arc, 0, len(keys))
	for _, k := range keys {
		calls := slices.Sorted(func(yield func(string) bool) {
			for c := range m.Edges[k] {
				if !yield(c) {
					return
				}
			}
		})
		spans := make([]string, 0, len(calls))
		for _, c := range calls {
			spans = append(spans, markdown.CodeSpan(c))
		}
		fmt.Fprintf(&b, "| %s | %s | %s |\n", markdown.Cell(k[0]), strings.Join(spans, ", "), markdown.Cell(k[1]))
		arcs = append(arcs, states.Arc{From: order[k[0]], To: order[k[1]], Label: strings.Join(calls, ", ")})
	}
	diagram := h.diagram
	if diagram == "" {
		diagram = "states/" + slugRe.ReplaceAllString(strings.ToLower(h.crate+"-"+m.Path), "-") + ".svg"
	}
	fmt.Fprintf(&b, "\n![Transitions of %s](%s)\n", m.Type.Name, (&url.URL{Path: diagram}).String())
	for _, reason := range []string{rustsrc.SkipAsync, rustsrc.SkipConst, rustsrc.SkipTyped, rustsrc.SkipBorrow, rustsrc.SkipImpl, rustsrc.SkipBuild} {
		if names := m.Skipped[reason]; len(names) > 0 {
			fmt.Fprintf(&b, "\nNot recorded (%s): %s.\n", reason, strings.Join(slices.Sorted(slices.Values(names)), ", "))
		}
	}
	svg := outFile{
		path:    filepath.Join(filepath.Dir(j.pg.path), filepath.FromSlash(diagram)),
		data:    states.Draw(nodes, arcs, first),
		subject: "crate " + h.crate + " " + m.Path,
	}
	return b.String(), []outFile{svg}, nil
}

var slugRe = regexp.MustCompile(`[^a-z0-9]+`)

// cellYes is the text of a table cell that is true.
const cellYes = "yes"

// variantPattern returns the variant v of the enum as a pattern: with { .. }
// for a variant with named fields and (..) for a tuple variant.
func variantPattern(enum string, v rustsrc.Variant) string {
	switch v.Style {
	case "named":
		return enum + "::" + v.Name + " { .. }"
	case "tuple":
		return enum + "::" + v.Name + "(..)"
	}
	return enum + "::" + v.Name
}

// A rustStates is the machine of a docgen:states marker with crate=, and its
// results once the plan is prepared.
type rustStates struct {
	crate   string
	typ     string
	run     string
	feats   cargo.Features
	machine *rustsrc.Machine
	err     error
}

// addStates adds the machine s to the machines to record.
func (rp *rustPlan) addStates(s *rustStates) *rustStates {
	rp.crates[s.crate] = true
	rp.states = append(rp.states, s)
	return s
}

// A statesGroup is the machines of one crate and one set of features, which
// one cargo test records.
type statesGroup struct {
	target  *rustsrc.StatesTarget
	byPath  map[string]*rustsrc.Machine
	members []*rustStates
}

// rustStates records the machines of rp with one cargo test for each crate
// and set of features. Each test runs in wg.
func (g *Generator) rustStates(ctx context.Context, r *cargo.Runner, ws *cargo.Workspace, rp *rustPlan, wg *sync.WaitGroup) {
	groups := map[string]*statesGroup{}
	for _, s := range rp.states {
		c, err := rp.crate(s.crate)
		if err != nil {
			s.err = err
			continue
		}
		flags, err := g.featureFlags(ctx, s.crate, s.feats)
		if err != nil {
			s.err = err
			continue
		}
		key := s.crate + "\x00" + s.feats.Key()
		grp := groups[key]
		if grp == nil {
			grp = &statesGroup{target: &rustsrc.StatesTarget{Package: s.crate, Crate: c.crate, Flags: flags}, byPath: map[string]*rustsrc.Machine{}}
			groups[key] = grp
		}
		m, err := rustsrc.NewMachine(c.crate, s.crate, s.typ)
		if err != nil {
			s.err = err
			continue
		}
		if prev := grp.byPath[m.Path]; prev != nil {
			m = prev
		} else {
			grp.byPath[m.Path] = m
			grp.target.Machines = append(grp.target.Machines, m)
		}
		s.machine = m
		grp.members = append(grp.members, s)
	}
	for _, key := range sortedKeys(groups) {
		grp := groups[key]
		if len(grp.members) == 0 {
			continue
		}
		grp.target.Filters = statesFilters(grp.members)
		wg.Go(func() {
			work, err := g.session.WorkDir("rust-states-")
			if err == nil {
				err = rustsrc.RunStates(ctx, r, ws.Root, work, grp.target)
			}
			if err != nil {
				for _, s := range grp.members {
					s.err = err
				}
			}
		})
	}
}

// statesFilters returns the run= values of members, in order with no
// duplicates, when each has one, and nil otherwise.
func statesFilters(members []*rustStates) []string {
	var out []string
	for _, s := range members {
		if s.run == "" {
			return nil
		}
		out = append(out, s.run)
	}
	slices.Sort(out)
	return slices.Compact(out)
}
