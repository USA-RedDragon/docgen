package states

import (
	"fmt"
	gotypes "go/types"
	"slices"
	"strings"

	"github.com/USA-RedDragon/docgen/internal/load"
	"github.com/USA-RedDragon/docgen/internal/types"
)

// A stateRun holds what the instrumented tests of one package recorded.
type stateRun struct {
	machines map[string]*machine
	err      error
}

// A machine is one view of the state of a machine type: the state type
// that one of its methods returns, or the results of its methods with no
// argument.
type machine struct {
	name      string
	typ       string
	stateType gotypes.Type
	accessor  string
	args      []string
	argType   gotypes.Type
	observers []observer
	methods   []string
	skipped   []string
	valueRecv []string
	promoted  []string
	initial   map[string]bool
	edges     map[[2]string]map[string]bool
}

type observer struct {
	name string
	kind string
}

const (
	obsBool = "bool"
	obsOK   = "ok"
	obsLen  = "len"
)

// namedState returns the named state type of m, or nil when m has none. An
// unnamed integer type has no constants of its own.
func (m *machine) namedState() *gotypes.Named {
	n, _ := m.stateType.(*gotypes.Named)
	return n
}

func stateMethod(t types.Type) gotypes.Type {
	for _, f := range types.Methods(t) {
		if f.Name == "State" && len(f.Params) == 0 && len(f.Results) == 1 {
			return types.Integer(f.Results[0])
		}
	}
	return nil
}

func observersOf(t types.Type) []observer {
	var out []observer
	for _, f := range types.Methods(t) {
		if len(f.Params) != 0 {
			continue
		}
		r := f.Results
		switch {
		case len(r) == 1 && types.IsBool(r[0]):
			out = append(out, observer{f.Name, obsBool})
		case len(r) == 2 && types.IsBool(r[1]):
			out = append(out, observer{f.Name, obsOK})
		case len(r) == 1 && types.IsInteger(r[0]) && f.Name == "Len":
			out = append(out, observer{f.Name, obsLen})
		}
	}
	return out
}

type imports struct {
	here  *gotypes.Package
	alias map[string]string
}

func (im *imports) qualify(c types.Const) string {
	if c.Pkg == im.here {
		return c.Name
	}
	a, ok := im.alias[c.Pkg.Path()]
	if !ok {
		a = fmt.Sprintf("__dgp%d", len(im.alias))
		im.alias[c.Pkg.Path()] = a
	}
	return a + "." + c.Name
}

// resolveView maps the type= of a marker to the machine type, the method
// that returns its state and the arguments of that method.
func resolveView(pi *load.Package, name string, im *imports) (*machine, error) {
	t, ok := types.Lookup(pi, name)
	if !ok {
		return nil, fmt.Errorf("%s is not a type", name)
	}
	m := &machine{name: name, typ: name, initial: map[string]bool{}, edges: map[[2]string]map[string]bool{}}
	if st := stateMethod(t); st != nil {
		m.stateType, m.accessor = st, "State"
		return m, m.resolveAlias(pi, t)
	}
	if st := types.NamedInteger(t.Go); st != nil {
		return m, m.resolveAccessor(pi, t, st, im)
	}
	m.observers = observersOf(t)
	if len(m.observers) == 0 {
		return nil, fmt.Errorf("%s has no State method and no method without arguments that returns a bool or a count", name)
	}
	return m, m.resolveAlias(pi, t)
}

// resolveAlias sets the machine type of m to the type that t names, since the
// methods of the machine have that type. The overlay holds the files of one package,
// so an alias of a type of another package is an error.
func (m *machine) resolveAlias(pi *load.Package, t types.Type) error {
	name, ok := aliasTarget(pi, t)
	if !ok {
		return fmt.Errorf("%s is an alias of a type of another package; use a marker in that package", t.Name)
	}
	m.typ = name
	return nil
}

// aliasTarget returns the name of the type of pi that t names, which is the name of
// t for a type that is not an alias of a defined type. It reports false for an alias
// of a type of another package.
func aliasTarget(pi *load.Package, t types.Type) (string, bool) {
	n, ok := gotypes.Unalias(t.Go).(*gotypes.Named)
	if !ok {
		return t.Name, true
	}
	return n.Obj().Name(), n.Obj().Pkg() == pi.Pkg.Types
}

// resolveAccessor sets m from the one exported method of an exported type of pi
// other than state that returns the state type st.
func (m *machine) resolveAccessor(pi *load.Package, state types.Type, st *gotypes.Named, im *imports) error {
	var found []string
	seen := map[string]bool{}
	for _, t := range types.ExportedTypes(pi) {
		name, ok := aliasTarget(pi, t)
		if !ok || seen[name] || gotypes.Identical(t.Go, st) {
			continue
		}
		seen[name] = true
		for _, f := range types.Methods(t) {
			if len(f.Results) != 1 || !gotypes.Identical(f.Results[0], st) || len(f.Params) > 1 {
				continue
			}
			args, argType, ok := accessorArgs(f, im)
			if !ok {
				continue
			}
			found = append(found, name+"."+f.Name)
			m.typ, m.accessor, m.args, m.argType, m.stateType = name, f.Name, args, argType, st
		}
	}
	switch len(found) {
	case 0:
		return fmt.Errorf("no method of the package returns %s", state.Name)
	case 1:
		return nil
	}
	return fmt.Errorf("several methods return %s (%s); give the machine type", state.Name, strings.Join(found, ", "))
}

// accessorArgs returns the arguments to read a state with the method f and the
// type of its parameter: none for a method with no parameter, and each exported
// constant of a named parameter type, in name order. It reports false for any
// other method.
func accessorArgs(f types.Method, im *imports) ([]string, gotypes.Type, bool) {
	if len(f.Params) == 0 {
		return nil, nil, true
	}
	argType := f.Params[0]
	at, ok := argType.(*gotypes.Named)
	if !ok {
		return nil, nil, false
	}
	consts := types.Constants(types.Of(nil, at))
	slices.SortFunc(consts, func(a, b types.Const) int { return strings.Compare(a.Name, b.Name) })
	var args []string
	for _, c := range consts {
		args = append(args, im.qualify(c))
	}
	return args, argType, len(args) > 0
}

// stateComps returns, for each state component of m, a Go expression of
// type uint64 on the receiver r and a Go expression of type string that
// appends a value v of that component to b as the record holds it.
func (m *machine) stateComps() (vals, fmts []string) {
	if m.stateType != nil {
		args := m.args
		if len(args) == 0 {
			args = []string{""}
		}
		format := "__dgstrconv.AppendInt(b, int64(v), 10)"
		if types.IsUnsigned(m.stateType) {
			format = "__dgstrconv.AppendUint(b, v, 10)"
		}
		for _, a := range args {
			vals = append(vals, fmt.Sprintf("uint64(r.%s(%s))", m.accessor, a))
			fmts = append(fmts, format)
		}
		return vals, fmts
	}
	var bits, strs []string
	for i, o := range m.observers {
		var b string
		switch o.kind {
		case obsBool:
			b = fmt.Sprintf("r.%s()", o.name)
		case obsOK:
			b = fmt.Sprintf("func() bool { _, ok := r.%s(); return ok }()", o.name)
		case obsLen:
			b = fmt.Sprintf("r.%s() > 0", o.name)
		}
		bits = append(bits, fmt.Sprintf("__dgBit(%s)<<%d", b, i))
		if i > 0 {
			strs = append(strs, "append(b, ',')")
		}
		strs = append(strs, fmt.Sprintf("__dgstrconv.AppendBool(b, v>>%d&1 != 0)", i))
	}
	return []string{strings.Join(bits, " | ")}, []string{strings.Join(strs, "\n\t\tb = ")}
}
