package rustsrc

import (
	"fmt"
	"slices"
	"strings"
)

// A Crate is the module tree of one target of a crate, as the helper reads it.
type Crate struct {
	// Root is the root file of the target, such as src/lib.rs.
	Root    string    `json:"root"`
	Modules []*Module `json:"modules"`
	// Errors holds the files that do not parse and the modules with no file.
	Errors []string `json:"errors"`
	// Warnings holds what the helper skipped, such as a module with a cfg and
	// no file.
	Warnings []string `json:"warnings"`
	byPath   map[string]*Module
}

// A Module is a module of a crate.
type Module struct {
	// Path is the path from the crate root, such as net::tcp, and "" for the
	// root.
	Path string `json:"path"`
	File string `json:"file"`
	Vis  string `json:"vis"`
	Doc  string `json:"doc"`
	// Cfg holds the cfg predicates of the module and its parents.
	Cfg  []string `json:"cfg"`
	Span *Span    `json:"span"`
	// Insert is the byte offset in File where code can go into the module:
	// the end of the file, or the closing brace of an inline module.
	Insert int     `json:"insert"`
	Items  []*Item `json:"items"`
	Uses   []Use   `json:"uses"`
	// DefaultImpls holds the impl Default blocks of the module.
	DefaultImpls []DefaultImpl `json:"default_impls"`
	parent       *Module
}

// The kinds of an item.
const (
	KindConst  = "const"
	KindStatic = "static"
	KindStruct = "struct"
	KindEnum   = "enum"
	KindFn     = "fn"
	KindMod    = "mod"
	KindUnion  = "union"
	KindTrait  = "trait"
	KindType   = "type"
	KindMacro  = "macro"
)

// An Item is an item of a module. An associated const or fn of an inherent
// impl has the name Type::name.
type Item struct {
	// Kind is const, static, struct, enum, union, fn, trait, type, macro or
	// mod.
	Kind string   `json:"kind"`
	Name string   `json:"name"`
	Vis  string   `json:"vis"`
	Doc  string   `json:"doc"`
	Cfg  []string `json:"cfg"`
	Span Span     `json:"span"`
	// Type is the type of a const, static or type alias, as written.
	Type string `json:"type"`
	// Expr is the value of a const or static, as written.
	Expr string `json:"expr"`
	Mut  bool   `json:"mut"`
	// Ret is the return type of a fn as written, and empty when it has none.
	Ret      string `json:"ret"`
	Generics bool   `json:"generics"`
	// Style is named, tuple or unit for a struct.
	Style    string     `json:"style"`
	Fields   []Field    `json:"fields"`
	Variants []Variant  `json:"variants"`
	Derives  []string   `json:"derives"`
	Error    *ErrorAttr `json:"error"`
	// Configulator holds the text in each #[configulator(...)] of the item.
	Configulator []string `json:"configulator"`
	Inline       bool     `json:"inline"`
	// Serde holds the serde attributes of a struct or enum, and is nil with
	// no #[serde].
	Serde *Serde `json:"serde"`
}

// A Serde holds the serde attributes that docgen reads. Rename is the
// deserialize name of a field, and RenameAll the case of the fields of a
// struct. Default is nil with no default, "" for #[serde(default)] and the
// path of #[serde(default = "path")].
type Serde struct {
	Rename    string  `json:"rename"`
	RenameAll string  `json:"rename_all"`
	Default   *string `json:"default"`
	Skip      bool    `json:"skip"`
	Flatten   bool    `json:"flatten"`
}

// A DefaultImpl is an impl Default for a type. Fields holds the expression
// as written of each field that the tail struct literal of its default fn
// sets.
type DefaultImpl struct {
	Type   string            `json:"type"`
	Fields map[string]string `json:"fields"`
	Cfg    []string          `json:"cfg"`
}

// A Field is a field of a struct or of an enum variant. A tuple field has its
// index as its name.
type Field struct {
	Name         string   `json:"name"`
	Type         string   `json:"type"`
	Vis          string   `json:"vis"`
	Doc          string   `json:"doc"`
	From         bool     `json:"from"`
	Source       bool     `json:"source"`
	Backtrace    bool     `json:"backtrace"`
	Cfg          []string `json:"cfg"`
	Configulator []string `json:"configulator"`
	Serde        *Serde   `json:"serde"`
}

// A Variant is a variant of an enum.
type Variant struct {
	Name         string     `json:"name"`
	Doc          string     `json:"doc"`
	Style        string     `json:"style"`
	Span         Span       `json:"span"`
	Fields       []Field    `json:"fields"`
	Cfg          []string   `json:"cfg"`
	Discriminant string     `json:"discriminant"`
	Error        *ErrorAttr `json:"error"`
}

// An ErrorAttr is the #[error(...)] of thiserror.
type ErrorAttr struct {
	// Format is the format string, and nil when the first argument is not a
	// string.
	Format *string `json:"format"`
	// Args reports whether the format string has arguments after it.
	Args        bool   `json:"args"`
	Transparent bool   `json:"transparent"`
	Text        string `json:"text"`
}

// A Use is one name that a use declaration brings into a module.
type Use struct {
	Name string   `json:"name"`
	Path []string `json:"path"`
	Vis  string   `json:"vis"`
	Glob bool     `json:"glob"`
}

// A Span is the place of an item: its file, its first and last line, and its
// byte range in the file.
type Span struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	EndLine int    `json:"end_line"`
	Start   int    `json:"start"`
	End     int    `json:"end"`
}

func (c *Crate) index() {
	c.byPath = map[string]*Module{}
	for _, m := range c.Modules {
		c.byPath[m.Path] = m
	}
	for _, m := range c.Modules {
		if m.Path != "" {
			m.parent = c.byPath[parentPath(m.Path)]
		}
	}
}

func parentPath(p string) string {
	if i := strings.LastIndex(p, "::"); i >= 0 {
		return p[:i]
	}
	return ""
}

// Module returns the module at path, "" for the root, or nil.
func (c *Crate) Module(path string) *Module {
	return c.byPath[path]
}

// Parent returns the parent module of m, or nil for the root.
func (m *Module) Parent() *Module {
	return m.parent
}

// Item returns the item of m with the name, and of the kind when kind is not
// "", or nil.
func (m *Module) Item(name, kind string) *Item {
	for _, it := range m.Items {
		if it.Name == name && (kind == "" || it.Kind == kind) && it.Kind != KindMod {
			return it
		}
	}
	return nil
}

// A Found is the result of Lookup: an item and its module, or a module alone.
type Found struct {
	Module *Module
	// Item is nil when the path names a module.
	Item *Item
	// Path is the path of the item or module from the crate root, after any
	// use.
	Path string
}

// maxUses limits the use declarations that one lookup follows.
const maxUses = 16

// Lookup returns the item or module at the path name, relative to the crate
// root, such as limits::MAX_JOBS, errors::Error or Port::HTTP for an
// associated const. A leading crate:: is allowed, and "" is the root module.
// It follows use declarations of the crate. name of crate is its name in
// errors.
func (c *Crate) Lookup(crate, name string) (Found, error) {
	clean := strings.TrimPrefix(name, "crate::")
	if clean == "" || clean == "crate" {
		return Found{Module: c.Module("")}, nil
	}
	f, ok := c.lookup(splitPath(clean), 0)
	if ok {
		return f, nil
	}
	return Found{}, c.notFound(crate, name)
}

func splitPath(p string) []string {
	return strings.Split(p, "::")
}

func (c *Crate) lookup(segs []string, depth int) (Found, bool) {
	if depth > maxUses {
		return Found{}, false
	}
	if m := c.Module(strings.Join(segs, "::")); m != nil {
		return Found{Module: m, Path: m.Path}, true
	}
	for split := len(segs) - 1; split >= 0 && split >= len(segs)-2; split-- {
		m := c.Module(strings.Join(segs[:split], "::"))
		if m == nil {
			continue
		}
		name := strings.Join(segs[split:], "::")
		if it := m.Item(name, ""); it != nil {
			return Found{Module: m, Item: it, Path: joinPath(m.Path, name)}, true
		}
		if f, ok := c.viaUse(m, segs[split], segs[split+1:], depth); ok {
			return f, true
		}
	}
	return Found{}, false
}

// viaUse looks up first, and then rest below it, through the use declarations
// of m.
func (c *Crate) viaUse(m *Module, first string, rest []string, depth int) (Found, bool) {
	for _, u := range m.Uses {
		var target []string
		switch {
		case u.Glob:
			target = append(slices.Clone(u.Path), first)
		case u.Name == first:
			target = slices.Clone(u.Path)
		default:
			continue
		}
		abs, ok := absolute(m, target)
		if !ok {
			continue
		}
		if f, ok := c.lookup(append(abs, rest...), depth+1); ok {
			return f, true
		}
	}
	return Found{}, false
}

// absolute returns the path p of a use in m as a path from the crate root. A
// path that starts with the name of another crate has no such path.
func absolute(m *Module, p []string) ([]string, bool) {
	if len(p) == 0 {
		return nil, false
	}
	base := splitModule(m.Path)
	switch p[0] {
	case "crate":
		return p[1:], true
	case "self":
		return append(base, p[1:]...), true
	case "super":
		for len(p) > 0 && p[0] == "super" {
			if len(base) == 0 {
				return nil, false
			}
			base, p = base[:len(base)-1], p[1:]
		}
		return append(base, p...), true
	}
	return append(base, p...), true
}

func splitModule(p string) []string {
	if p == "" {
		return nil
	}
	return splitPath(p)
}

func joinPath(module, name string) string {
	if module == "" {
		return name
	}
	return module + "::" + name
}

// notFound returns the error of a name with no item: the module that holds no
// such name lists its items, and a path through no module names the first
// part that is missing.
func (c *Crate) notFound(crate, name string) error {
	segs := splitPath(strings.TrimPrefix(name, "crate::"))
	for i := len(segs) - 1; i >= 0; i-- {
		m := c.Module(strings.Join(segs[:i], "::"))
		if m == nil {
			continue
		}
		where := "the crate root"
		if m.Path != "" {
			where = "module " + m.Path
		}
		var names []string
		for _, it := range m.Items {
			if it.Kind != KindMod {
				names = append(names, it.Name)
			}
		}
		for _, sub := range c.Modules {
			if sub.parent == m {
				names = append(names, strings.TrimPrefix(sub.Path, joinPath(m.Path, "")))
			}
		}
		slices.Sort(names)
		if len(names) == 0 {
			return fmt.Errorf("crate %s has no item %s: %s has no items", crate, name, where)
		}
		return fmt.Errorf("crate %s has no item %s; the names in %s are %s", crate, name, where, strings.Join(names, ", "))
	}
	return fmt.Errorf("crate %s has no item %s", crate, name)
}

// Kind returns the kind of the item of f, or module for a module.
func (f Found) Kind() string {
	if f.Item == nil {
		return "module"
	}
	return f.Item.Kind
}

// Describe returns the kind and path of f for an error, such as "const
// limits::MAX_JOBS" or "module net".
func (f Found) Describe() string {
	if f.Item == nil && f.Path == "" {
		return "the crate root"
	}
	return f.Kind() + " " + f.Path
}

// Cfg returns the cfg predicates of f and of its modules.
func (f Found) Cfg() []string {
	out := slices.Clone(f.Module.Cfg)
	if f.Item != nil {
		out = append(out, f.Item.Cfg...)
	}
	return out
}
