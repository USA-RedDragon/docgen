package rustsrc

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
)

// The ways a Machine holds its state.
const (
	// StateSelf is a machine that is an enum: its variant is its state.
	StateSelf = iota
	// StateMethod is a machine with a method state(&self) that returns an
	// enum.
	StateMethod
	// StateField is a machine with one field of an enum type.
	StateField
)

// The reasons that a method has no hook.
const (
	SkipAsync  = "async"
	SkipConst  = "const"
	SkipTyped  = "self: Type"
	SkipBorrow = "returns a borrow"
	SkipImpl   = "returns impl Trait"
	SkipBuild  = "does not build with the hook"
)

// stateMethod is the name of the method that returns the state of a struct.
const stateMethod = "state"

// A Machine is a state machine of a crate, as docgen:states records it: a
// struct or enum with no generics whose state is the variant of an enum.
type Machine struct {
	// Name is the type as the marker names it, and Path its path from the
	// crate root.
	Name string
	Path string
	Type *Item
	// Enum is the enum whose variant is the state, and EnumPath its path from
	// the crate root.
	Enum     *Item
	EnumPath string
	// From is StateSelf, StateMethod or StateField, and Via is the name of
	// the method or the field.
	From int
	Via  string
	// Skipped holds the names of the pub methods with no hook, by reason.
	Skipped map[string][]string
	// First holds the states of the values that a constructor returned.
	First map[string]bool
	// Edges holds the calls of each change of state, by its states.
	Edges  map[[2]string]map[string]bool
	module *Module
	hooks  []*hook
}

// The shapes of a return type with the machine type in it.
const (
	shapeNone = iota
	shapePlain
	shapeOption
	shapeResult
)

// A hook is a method of a Machine with the hook in its body.
type hook struct {
	name   string
	item   *Item
	module *Module
	// labels holds the parameters whose type is an enum of the crate, which
	// the call label names by their variants.
	labels []string
	shape  int
}

// Resolve returns the item at the path p as code in the module from names
// it, such as State, super::State or crate::machine::State.
func (c *Crate) Resolve(from *Module, p string) (Found, bool) {
	p = strings.Join(strings.Fields(p), "")
	if p == "" || strings.ContainsAny(p, "<>()[]&*") {
		return Found{}, false
	}
	abs, ok := absolute(from, splitPath(strings.TrimPrefix(p, "::")))
	if !ok {
		return Found{}, false
	}
	return c.lookup(abs, 0)
}

// enumOf returns the enum with no generics at the type t, as written in the
// module from, with a leading & or &mut removed.
func (c *Crate) enumOf(from *Module, t string) (*Item, string, bool) {
	t = strings.TrimSpace(t)
	if r, ok := strings.CutPrefix(t, "&"); ok {
		t = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(r), "mut "))
	}
	f, ok := c.Resolve(from, t)
	if !ok || f.Item == nil || f.Item.Kind != KindEnum || f.Item.Generics {
		return nil, "", false
	}
	return f.Item, f.Path, true
}

// methodsOf returns the fns of the inherent impls of it, the item in the
// module m, with their modules, in the order of the crate.
func (c *Crate) methodsOf(it *Item) ([]*Item, []*Module) {
	var items []*Item
	var mods []*Module
	for _, m := range c.Modules {
		for _, fn := range m.Items {
			if fn.Kind != KindFn || fn.Method == nil {
				continue
			}
			typ, _, ok := strings.Cut(fn.Name, "::")
			if !ok || typ != it.Name {
				continue
			}
			if f, ok := c.Resolve(m, typ); !ok || f.Item != it {
				continue
			}
			items = append(items, fn)
			mods = append(mods, m)
		}
	}
	return items, mods
}

func methodName(fn *Item) string {
	_, name, _ := strings.Cut(fn.Name, "::")
	return name
}

// NewMachine returns the machine name of the crate c, which has the name crate
// in errors.
func NewMachine(c *Crate, crate, name string) (*Machine, error) {
	f, err := c.Lookup(crate, name)
	if err != nil {
		return nil, err
	}
	it := f.Item
	if it == nil || it.Kind != KindEnum && it.Kind != KindStruct {
		return nil, fmt.Errorf("%s is a %s, not an enum or a struct", name, f.Kind())
	}
	if it.Generics {
		return nil, fmt.Errorf("%s is generic; docgen:states records a type with no generics", name)
	}
	m := &Machine{Name: name, Path: f.Path, Type: it, module: f.Module, Skipped: map[string][]string{}, First: map[string]bool{}, Edges: map[[2]string]map[string]bool{}}
	fns, mods := c.methodsOf(it)
	if err := m.stateOf(c, crate, fns, mods); err != nil {
		return nil, err
	}
	if len(m.Enum.Variants) == 0 {
		return nil, fmt.Errorf("enum %s has no variants", m.EnumPath)
	}
	m.addHooks(c, fns, mods)
	transitions := 0
	for _, h := range m.hooks {
		if h.item.Method.Receiver != RecvNone {
			transitions++
		}
	}
	if transitions == 0 && len(m.Skipped) == 0 {
		return nil, fmt.Errorf("%s has no pub method with &mut self, or with self that returns %s, to record", name, it.Name)
	}
	return m, nil
}

// stateOf sets how m holds its state, from its type and its methods fns in
// the modules mods.
func (m *Machine) stateOf(c *Crate, crate string, fns []*Item, mods []*Module) error {
	if m.Type.Kind == KindEnum {
		m.Enum, m.EnumPath, m.From = m.Type, m.Path, StateSelf
		return nil
	}
	for i, fn := range fns {
		if methodName(fn) != stateMethod || fn.Method.Receiver != RecvRef {
			continue
		}
		e, p, ok := c.enumOf(mods[i], fn.Ret)
		if !ok || len(fn.Method.Params) > 0 || fn.Method.Generics || fn.Method.Async || fn.Method.Const {
			return fmt.Errorf("%s::%s is not a fn state(&self) that returns an enum of crate %s with no generics; it returns %q", m.Name, stateMethod, crate, fn.Ret)
		}
		m.Enum, m.EnumPath, m.From, m.Via = e, p, StateMethod, stateMethod
		return nil
	}
	var names []string
	for _, fl := range m.Type.Fields {
		if e, p, ok := c.enumOf(m.module, fl.Type); ok && !strings.HasPrefix(strings.TrimSpace(fl.Type), "&") {
			names = append(names, fl.Name)
			m.Enum, m.EnumPath, m.From, m.Via = e, p, StateField, fl.Name
		}
	}
	switch len(names) {
	case 0:
		return fmt.Errorf("struct %s has no method state(&self) and no field whose type is an enum of crate %s", m.Name, crate)
	case 1:
		return nil
	}
	return fmt.Errorf("struct %s has no method state(&self) and %d fields whose type is an enum of crate %s (%s); add a fn state(&self) that returns the state", m.Name, len(names), crate, strings.Join(names, ", "))
}

var shapeRe = regexp.MustCompile(`^(?:(?:::)?\w+::)*(Option|Result)<(\w+)(?:,.*)?>$`)

// shapeOf returns the shape of the return type ret of a method of m.
func (m *Machine) shapeOf(ret string) int {
	isSelf := func(s string) bool { return s == "Self" || s == m.Type.Name }
	ret = strings.Join(strings.Fields(ret), "")
	if isSelf(ret) {
		return shapePlain
	}
	s := shapeRe.FindStringSubmatch(ret)
	if s == nil || !isSelf(s[2]) {
		return shapeNone
	}
	if s[1] == "Option" {
		return shapeOption
	}
	return shapeResult
}

// addHooks adds a hook to each pub method of fns that can change the state
// or makes a new machine, and adds the other pub methods that could change
// the state to Skipped.
func (m *Machine) addHooks(c *Crate, fns []*Item, mods []*Module) {
	for i, fn := range fns {
		mt := fn.Method
		name := methodName(fn)
		if fn.Vis == "" || mt.Receiver == RecvRef || m.From == StateMethod && name == m.Via {
			continue
		}
		shape := m.shapeOf(fn.Ret)
		skip := ""
		switch {
		case mt.Receiver == RecvNone || mt.Receiver == RecvValue:
			if shape == shapeNone {
				continue
			}
			if mt.Async || mt.Const {
				if mt.Receiver == RecvNone {
					continue
				}
				skip = map[bool]string{true: SkipAsync, false: SkipConst}[mt.Async]
			}
		case mt.Receiver == RecvTyped:
			skip = SkipTyped
		case mt.Async:
			skip = SkipAsync
		case mt.Const:
			skip = SkipConst
		case strings.Contains(fn.Ret, "impl "):
			skip = SkipImpl
		case strings.ContainsAny(fn.Ret, "&'"):
			skip = SkipBorrow
		}
		if skip != "" {
			m.Skipped[skip] = append(m.Skipped[skip], name)
			continue
		}
		h := &hook{name: name, item: fn, module: mods[i], shape: shape}
		if mt.Receiver != RecvNone {
			for _, p := range mt.Params {
				if _, _, ok := c.enumOf(mods[i], p.Type); ok {
					h.labels = append(h.labels, p.Name)
				}
			}
		}
		m.hooks = append(m.hooks, h)
	}
}

// skip moves the hook h of m to Skipped with the reason.
func (m *Machine) skip(h *hook, reason string) {
	m.hooks = slices.DeleteFunc(m.hooks, func(x *hook) bool { return x == h })
	m.Skipped[reason] = append(m.Skipped[reason], h.name)
}

// Calls returns the number of methods of m with a hook that can change the
// state.
func (m *Machine) Calls() int {
	n := 0
	for _, h := range m.hooks {
		if h.item.Method.Receiver != RecvNone {
			n++
		}
	}
	return n
}
