package states

import (
	"fmt"
	"math/big"
	"net/url"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/USA-RedDragon/docgen/internal/goprint"
	"github.com/USA-RedDragon/docgen/internal/load"
	"github.com/USA-RedDragon/docgen/internal/markdown"
	"github.com/USA-RedDragon/docgen/internal/types"
)

// A Diagram is the SVG file of a machine, with the path to write it to. Machine
// is the import path and type of the machine.
type Diagram struct {
	Path    string
	Data    []byte
	Machine string
}

type stateDef struct {
	key      string
	name     string
	doc      string
	constant bool
}

func stateValue(key string) *big.Int {
	v, _ := new(big.Int).SetString(key, 10)
	return v
}

// reachedStates returns the keys of the states that the tests of m reached.
func reachedStates(m *machine) map[string]bool {
	reached := map[string]bool{}
	for k := range m.initial {
		reached[k] = true
	}
	for k := range m.edges {
		reached[k[0]], reached[k[1]] = true, true
	}
	return reached
}

// stateDefs returns the states of m in value order: the exported constants of
// the state type in the package pi, and each value of the tests with no
// constant. A machine with no state type has only the values of the tests.
func stateDefs(pi *load.Package, m *machine) []stateDef {
	seen := reachedStates(m)
	if m.stateType == nil {
		out := make([]stateDef, 0, len(seen))
		for k := range seen {
			out = append(out, stateDef{key: k, name: observerName(m, k)})
		}
		sort.Slice(out, func(i, j int) bool { return out[i].key < out[j].key })
		return out
	}
	var out []stateDef
	if st := m.namedState(); st != nil {
		for _, c := range types.Constants(types.Of(pi, st)) {
			key := c.Value.ExactString()
			delete(seen, key)
			out = append(out, stateDef{key: key, name: c.Name, doc: c.Doc, constant: true})
		}
	}
	for k := range seen {
		out = append(out, stateDef{key: k, name: "value " + k})
	}
	sort.Slice(out, func(i, j int) bool {
		if c := stateValue(out[i].key).Cmp(stateValue(out[j].key)); c != 0 {
			return c < 0
		}
		return out[i].name < out[j].name
	})
	return out
}

// statePackage returns the package of the state type of m, which pi imports
// when it is not pi.
func (rc *Recorder) statePackage(pi *load.Package, m *machine) (*load.Package, error) {
	st := m.namedState()
	if st == nil || st.Obj().Pkg() == nil || st.Obj().Pkg().Path() == pi.Path {
		return pi, nil
	}
	return rc.loaded.Imported(pi, st.Obj().Pkg().Path())
}

func observerName(m *machine, key string) string {
	vals := strings.Split(key, ",")
	var parts []string
	for i, o := range m.observers {
		v := i < len(vals) && vals[i] == "true"
		switch o.kind {
		case obsBool:
			parts = append(parts, fmt.Sprintf("%s() %s", o.name, map[bool]string{true: "true", false: "false"}[v]))
		case obsOK:
			parts = append(parts, fmt.Sprintf("%s() %s", o.name, map[bool]string{true: "ok", false: "not ok"}[v]))
		case obsLen:
			parts = append(parts, fmt.Sprintf("%s() %s", o.name, map[bool]string{true: "> 0", false: "0"}[v]))
		}
	}
	return strings.Join(parts, ", ")
}

var slugRe = regexp.MustCompile(`[^a-z0-9]+`)

func (rc *Recorder) stateView(pkg, typ string) (string, *machine, error) {
	ip, err := rc.mod.PkgPath(pkg)
	if err != nil {
		return "", nil, err
	}
	if err := rc.loaded.Err(ip); err != nil {
		return "", nil, err
	}
	run := rc.runs[ip]
	if run == nil {
		return "", nil, fmt.Errorf("no state run for %s", ip)
	}
	if run.err != nil {
		return "", nil, run.err
	}
	m := run.machines[typ]
	if m == nil {
		return "", nil, fmt.Errorf("no state view %s", typ)
	}
	return ip, m, nil
}

// Fill returns the tables of the machine typ of the package of pkgArg,
// and its diagram for a page in pageDir.
func (rc *Recorder) Fill(pkgArg, typ, pageDir, diagram string) (string, Diagram, error) {
	ip, m, err := rc.stateView(pkgArg, typ)
	if err != nil {
		return "", Diagram{}, err
	}
	pi := rc.loaded.ByPath(ip)
	sp, err := rc.statePackage(pi, m)
	if err != nil {
		return "", Diagram{}, err
	}
	defs := stateDefs(sp, m)
	if len(m.edges) == 0 && !m.hasUnrecorded() {
		return "", Diagram{}, fmt.Errorf("the tests of %s recorded no transition of %s", rc.mod.ShortPath(ip), m.name)
	}
	names := map[string]string{}
	for _, d := range defs {
		names[d.key] = d.name
	}
	trs := transitions(m, defs)
	var b strings.Builder
	if m.stateType != nil {
		writeStateTable(&b, pi, sp.Path, m, defs)
	} else {
		writeObserverTable(&b, ip, m, defs)
	}
	writeTransitions(&b, trs, names)
	if diagram == "" {
		diagram = "states/" + diagramSlug(rc.mod.ShortPath(ip), ip, m) + ".svg"
	}
	fmt.Fprintf(&b, "\n![Transitions of %s](%s)\n", m.typ, (&url.URL{Path: diagram}).String())
	writeUnrecorded(&b, m)
	svgPath := filepath.Join(pageDir, filepath.FromSlash(diagram))
	return b.String(), Diagram{Path: svgPath, Data: diagramOf(m, defs, trs, names), Machine: ip + "." + typ}, nil
}

// hasUnrecorded reports whether m has an exported method that the hook does not
// record, which can change the state with no record.
func (m *machine) hasUnrecorded() bool {
	return len(m.skipped)+len(m.valueRecv)+len(m.promoted) > 0
}

// writeUnrecorded writes a line for each group of methods of m that the hook does
// not record. The hook holds its state by the pointer to the machine, and a value
// receiver is a new copy on each call.
func writeUnrecorded(b *strings.Builder, m *machine) {
	for _, g := range []struct {
		reason  string
		methods []string
	}{{"no receiver name", m.skipped}, {"value receiver", m.valueRecv}, {"promoted", m.promoted}} {
		if len(g.methods) > 0 {
			fmt.Fprintf(b, "\nNot recorded (%s): %s.\n", g.reason, strings.Join(g.methods, ", "))
		}
	}
}

// writeStateTable writes the table of the states of a machine with a state
// type. spPath is the import path of the package of that type.
func writeStateTable(b *strings.Builder, pi *load.Package, spPath string, m *machine, defs []stateDef) {
	reached := reachedStates(m)
	acc := m.typ + "." + m.accessor
	each := ""
	if len(m.args) > 0 {
		each = " for each `" + goprint.Type(m.argType, pi.Pkg.Types) + "`"
	}
	fmt.Fprintf(b, "States of [`%s`](%s), from [`%s`](%s)%s:\n\n", m.typ, markdown.PkgLink(pi.Path, m.typ), acc, markdown.PkgLink(pi.Path, acc), each)
	b.WriteString("| State | Meaning | Seen in tests |\n|---|---|---|\n")
	for _, d := range defs {
		seen := "no"
		if reached[d.key] {
			seen = "yes"
		}
		if m.initial[d.key] {
			seen += ", first state"
		}
		state := d.name
		if d.constant {
			state = fmt.Sprintf("[`%s`](%s)", d.name, markdown.PkgLink(spPath, d.name))
		}
		fmt.Fprintf(b, "| %s | %s | %s |\n", state, markdown.Cell(d.doc), seen)
	}
}

// writeObserverTable writes the table of the states of a machine with no state
// type, in the package with the import path ip.
func writeObserverTable(b *strings.Builder, ip string, m *machine, defs []stateDef) {
	obs := make([]string, 0, len(m.observers))
	for _, o := range m.observers {
		obs = append(obs, "`"+o.name+"()`")
	}
	fmt.Fprintf(b, "[`%s`](%s) has no State method. The state in this table is the result of %s. States seen in tests:\n\n", m.name, markdown.PkgLink(ip, m.name), strings.Join(obs, ", "))
	b.WriteString("| State | Seen in tests |\n|---|---|\n")
	for _, d := range defs {
		seen := "yes"
		if m.initial[d.key] {
			seen += ", first state"
		}
		fmt.Fprintf(b, "| %s | %s |\n", markdown.Cell(d.name), seen)
	}
}

// A transition is one pair of states of a machine and the methods that move
// the machine from the first to the second, in name order.
type transition struct {
	from, to string
	calls    []string
}

// transitions returns the transitions of m in the order of defs.
func transitions(m *machine, defs []stateDef) []transition {
	trs := make([]transition, 0, len(m.edges))
	for k, set := range m.edges {
		t := transition{from: k[0], to: k[1]}
		for c := range set {
			t.calls = append(t.calls, c)
		}
		sort.Strings(t.calls)
		trs = append(trs, t)
	}
	order := map[string]int{}
	for i, d := range defs {
		order[d.key] = i
	}
	sort.Slice(trs, func(i, j int) bool {
		if order[trs[i].from] != order[trs[j].from] {
			return order[trs[i].from] < order[trs[j].from]
		}
		return order[trs[i].to] < order[trs[j].to]
	})
	return trs
}

func writeTransitions(b *strings.Builder, trs []transition, names map[string]string) {
	b.WriteString("\nTransitions seen in tests:\n\n| From | Method call | To |\n|---|---|---|\n")
	for _, t := range trs {
		cs := make([]string, 0, len(t.calls))
		for _, c := range t.calls {
			cs = append(cs, "`"+c+"`")
		}
		fmt.Fprintf(b, "| %s | %s | %s |\n", markdown.Cell(names[t.from]), strings.Join(cs, ", "), markdown.Cell(names[t.to]))
	}
}

// diagramSlug returns the file name of the diagram of m without its extension.
// pkg is the path of its package in the module and ip the import path.
func diagramSlug(pkg, ip string, m *machine) string {
	if pkg == "." {
		pkg = path.Base(ip)
	}
	slug := slugRe.ReplaceAllString(strings.ToLower(pkg+"-"+m.typ), "-")
	if m.name != m.typ {
		slug += "-" + strings.ToLower(m.name)
	}
	return slug
}

// diagramOf draws the states and the transitions of m. Each key of defs is one
// box.
func diagramOf(m *machine, defs []stateDef, trs []transition, names map[string]string) []byte {
	var nodes, keys []string
	idx := map[string]int{}
	for _, d := range defs {
		if _, ok := idx[d.key]; !ok {
			idx[d.key] = len(keys)
			keys = append(keys, d.key)
			nodes = append(nodes, names[d.key])
		}
	}
	arcs := make([]arc, 0, len(trs))
	for _, t := range trs {
		arcs = append(arcs, arc{from: idx[t.from], to: idx[t.to], label: strings.Join(t.calls, ", ")})
	}
	return stateSVG(nodes, arcs, m.initial, keys)
}
