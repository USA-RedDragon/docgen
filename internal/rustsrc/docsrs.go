package rustsrc

import (
	"slices"
	"strings"

	"github.com/USA-RedDragon/docgen/internal/cargo"
)

// DocsRS is the host of the docs that links point to.
const DocsRS = "https://docs.rs"

// visPub is the Vis of an item that is pub.
const visPub = "pub"

// Docs builds the docs.rs URLs of the public items of one crate. Nothing
// fetches them: a URL is built from the name, the version and the path of the
// item, so it is right when docs.rs has built that version of the crate.
type Docs struct {
	crate *Crate
	base  string
}

// NewDocs returns the Docs of the library of pkg, which c is the read of. It
// returns nil when docs.rs has no docs to link to: the manifest of pkg does not
// publish to crates.io, or pkg has no library.
func NewDocs(pkg *cargo.Package, c *Crate) *Docs {
	lib := pkg.Lib()
	if lib == nil || !pkg.Published() {
		return nil
	}
	return &Docs{crate: c, base: DocsRS + "/" + pkg.Name + "/latest/" + lib.Name}
}

// A spot is where rustdoc documents an item: the module path and the name
// there, which is Type::name for an associated item.
type spot struct {
	module string
	name   string
	// owner is the kind of the type of an associated item.
	owner string
}

func (s spot) path() string {
	return joinPath(s.module, s.name)
}

// public reports whether m is reachable from outside the crate: it and each
// module above it is pub, and none has a cfg, since the docs of docs.rs are
// built with one set of features that the manifest does not tell.
func public(m *Module) bool {
	if len(m.Cfg) > 0 {
		return false
	}
	for cur := m; cur.Path != ""; cur = cur.parent {
		if cur.Vis != visPub {
			return false
		}
	}
	return true
}

func memberKind(kind string) bool {
	return kind == KindStruct || kind == KindEnum || kind == KindUnion
}

// spotOf returns where rustdoc documents the item of f, which is its own module
// when that is public, and else the module of the shortest pub use of it from a
// public module. An item that is not pub, that has a cfg or that no public
// path leads to has no spot. An associated item is on the page of its type.
func (c *Crate) spotOf(f Found) (spot, bool) {
	it := f.Item
	if it == nil || it.Vis != visPub || len(f.Cfg()) > 0 {
		return spot{}, false
	}
	if it.Kind == KindMacro {
		return spot{name: it.Name}, true
	}
	if typ, member, ok := strings.Cut(it.Name, "::"); ok {
		parent, found := c.lookup(append(splitModule(f.Module.Path), typ), 0)
		if !found || parent.Item == nil || !memberKind(parent.Item.Kind) {
			return spot{}, false
		}
		s, ok := c.spotOf(parent)
		if !ok {
			return spot{}, false
		}
		return spot{s.module, s.name + "::" + member, parent.Item.Kind}, true
	}
	if public(f.Module) {
		return spot{module: f.Module.Path, name: it.Name}, true
	}
	var best spot
	found := false
	for _, m := range c.sortedModules() {
		if !public(m) {
			continue
		}
		for _, u := range m.Uses {
			name, ok := c.reexports(m, u, f)
			if !ok {
				continue
			}
			s := spot{module: m.Path, name: name}
			if !found || better(s, best) {
				best, found = s, true
			}
		}
	}
	return best, found
}

func better(a, b spot) bool {
	if na, nb := strings.Count(a.path(), "::"), strings.Count(b.path(), "::"); na != nb {
		return na < nb
	}
	return a.path() < b.path()
}

// reexports returns the name under which the use u of m makes the item of f
// public, and whether it does.
func (c *Crate) reexports(m *Module, u Use, f Found) (string, bool) {
	if u.Vis != visPub {
		return "", false
	}
	target, name := u.Path, u.Name
	if u.Glob {
		target, name = append(slices.Clone(u.Path), f.Item.Name), f.Item.Name
	}
	if name == "_" {
		return "", false
	}
	abs, ok := absolute(m, target)
	if !ok {
		return "", false
	}
	got, ok := c.lookup(abs, 0)
	if !ok || got.Item != f.Item {
		return "", false
	}
	return name, true
}

// sortedModules returns the modules of c by path.
func (c *Crate) sortedModules() []*Module {
	mods := slices.Clone(c.Modules)
	slices.SortFunc(mods, func(a, b *Module) int { return strings.Compare(a.Path, b.Path) })
	return mods
}

// pagePrefix returns the start of the file name of the page of an item of the
// kind, such as struct for struct.Name.html, and "" for a kind with no page.
func pagePrefix(kind string) string {
	switch kind {
	case KindStruct, KindEnum, KindUnion, KindTrait, KindType, KindFn, KindStatic, KindMacro:
		return kind
	case KindConst:
		return "constant"
	}
	return ""
}

// Item returns the URL of the item or module of f, and false when docs.rs has
// no page for it that docgen can tell. A module is its index.html. An item of
// the crate that is not pub has none, and neither has one with a cfg.
func (d *Docs) Item(f Found) (string, bool) {
	if d == nil {
		return "", false
	}
	if f.Item == nil {
		if f.Module == nil || !public(f.Module) {
			return "", false
		}
		return d.moduleDir(f.Module.Path) + "index.html", true
	}
	s, ok := d.crate.spotOf(f)
	if !ok {
		return "", false
	}
	typ, member, assoc := strings.Cut(s.name, "::")
	if !assoc {
		prefix := pagePrefix(f.Item.Kind)
		if prefix == "" {
			return "", false
		}
		return d.page(s.module, prefix, s.name), true
	}
	anchor := "#method."
	if f.Item.Kind == KindConst {
		anchor = "#associatedconstant."
	}
	return d.page(s.module, pagePrefix(s.owner), typ) + anchor + member, true
}

// Variant returns the URL of the variant of the enum of f.
func (d *Docs) Variant(f Found, variant string) (string, bool) {
	if d == nil || f.Item == nil || f.Item.Kind != KindEnum {
		return "", false
	}
	for _, v := range f.Item.Variants {
		if v.Name == variant && len(v.Cfg) == 0 {
			url, ok := d.Item(f)
			return url + "#variant." + variant, ok
		}
	}
	return "", false
}

func (d *Docs) moduleDir(module string) string {
	dir := d.base + "/"
	if module != "" {
		dir += strings.ReplaceAll(module, "::", "/") + "/"
	}
	return dir
}

func (d *Docs) page(module, prefix, name string) string {
	return d.moduleDir(module) + prefix + "." + name + ".html"
}

// Link returns the URL of the intra-doc link target, written in the doc of an
// item of the module from, or in the doc of from itself. It follows what rustdoc
// does for a path that starts with crate, self, super or a name in from,
// through the use declarations of the crate; it also takes an enum variant, a
// field of a struct, and an associated const or fn of a type. A path with a
// disambiguator such as struct@, a () or a ! is the path alone. Anything else
// is false: a path into another crate, Self, a trait item, an item that is not
// public.
func (d *Docs) Link(from *Module, target string) (string, bool) {
	if d == nil || from == nil {
		return "", false
	}
	path := cleanTarget(target)
	if path == "" || strings.HasPrefix(path, "::") {
		return "", false
	}
	return d.link(from, splitPath(path))
}

// link resolves segs in m.
func (d *Docs) link(m *Module, segs []string) (string, bool) {
	abs, ok := absolute(m, segs)
	if !ok {
		return "", false
	}
	if f, ok := d.crate.lookup(abs, 0); ok {
		return d.Item(f)
	}
	if len(abs) < 2 {
		return "", false
	}
	f, ok := d.crate.lookup(abs[:len(abs)-1], 0)
	if !ok || f.Item == nil {
		return "", false
	}
	name := abs[len(abs)-1]
	switch f.Item.Kind {
	case KindEnum:
		return d.Variant(f, name)
	case KindStruct:
		for _, fl := range f.Item.Fields {
			if fl.Name == name && fl.Vis == visPub && len(fl.Cfg) == 0 {
				url, ok := d.Item(f)
				return url + "#structfield." + name, ok
			}
		}
	}
	return "", false
}

// cleanTarget returns the path of an intra-doc link target: no backticks, no
// disambiguator, and no () or !.
func cleanTarget(s string) string {
	s = strings.TrimSpace(strings.Trim(strings.TrimSpace(s), "`"))
	s = disambig.ReplaceAllString(s, "")
	s = strings.TrimSuffix(strings.TrimSuffix(s, "()"), "!")
	if !rustPath.MatchString(s) {
		return ""
	}
	return s
}

// Resolver returns the function of Markdown that resolves a link target in the
// first of the modules scopes where it leads to a public item. It is nil when d
// is nil, so Markdown writes the text of each link.
func (d *Docs) Resolver(scopes ...*Module) func(string) (string, bool) {
	if d == nil {
		return nil
	}
	return func(target string) (string, bool) {
		for _, m := range scopes {
			if url, ok := d.Link(m, target); ok {
				return url, true
			}
		}
		return "", false
	}
}
