package rustsrc

import (
	"regexp"
	"slices"
	"strings"
)

const errorsHeading = "Errors"

var retPath = regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]*(?:::[A-Za-z_][A-Za-z0-9_]*)*`)

// ReturnedBy returns the functions and methods that document each error of typ,
// which is an enum or a struct: by the name of a variant of the enum, or by ""
// for the struct, the paths of the functions, sorted. A function counts when it
// has a spot (see spotOf), so it is pub and reachable from outside the crate,
// and has a section with the heading Errors in its doc comment that names the
// error:
//
//   - Enum::Variant, with any path before Enum, where the path leads to the
//     enum from the module of the function. Self::Variant counts in a method
//     of the enum.
//   - Variant alone, when the return type of the function has a path that
//     leads to the enum, as Result<T, Enum> does.
//   - A path that leads to the struct, for a struct.
//
// A name counts in a code span, a link or a reference definition of that
// section, and not in a code block. Naming the enum alone names no variant, and
// the return type alone names nothing.
func (c *Crate) ReturnedBy(typ Found) map[string][]string {
	out := map[string][]string{}
	if typ.Item == nil || typ.Item.Kind != KindEnum && typ.Item.Kind != KindStruct {
		return out
	}
	for _, m := range c.sortedModules() {
		for _, it := range m.Items {
			if it.Kind != KindFn {
				continue
			}
			f := Found{Module: m, Item: it, Path: joinPath(m.Path, it.Name)}
			s, ok := c.spotOf(f)
			if !ok {
				continue
			}
			for _, v := range c.named(f, typ) {
				if !slices.Contains(out[v], s.path()) {
					out[v] = append(out[v], s.path())
				}
			}
		}
	}
	for _, fns := range out {
		slices.Sort(fns)
	}
	return out
}

// named returns the errors of typ that the Errors section of the doc of the fn
// f names: each variant once, or "" for a struct.
func (c *Crate) named(f Found, typ Found) []string {
	cands := errorNames(f.Item.Doc)
	if len(cands) == 0 {
		return nil
	}
	var out []string
	add := func(v string) {
		if !slices.Contains(out, v) {
			out = append(out, v)
		}
	}
	for _, name := range cands {
		segs := splitPath(name)
		if typ.Item.Kind == KindStruct {
			if r, ok := c.resolve(f, segs); ok && r.Item == typ.Item {
				add("")
			}
			continue
		}
		variant := segs[len(segs)-1]
		if !hasVariant(typ.Item, variant) {
			continue
		}
		if len(segs) == 1 {
			if c.returns(f, typ) {
				add(variant)
			}
			continue
		}
		if r, ok := c.resolve(f, segs[:len(segs)-1]); ok && r.Item == typ.Item {
			add(variant)
		}
	}
	return out
}

func hasVariant(it *Item, name string) bool {
	return slices.ContainsFunc(it.Variants, func(v Variant) bool { return v.Name == name })
}

// resolve returns what the path segs leads to from the module of the fn f. Self
// is the type of the impl of a method.
func (c *Crate) resolve(f Found, segs []string) (Found, bool) {
	if segs[0] == "Self" {
		typ, _, ok := strings.Cut(f.Item.Name, "::")
		if !ok || len(segs) > 1 {
			return Found{}, false
		}
		return c.lookup(append(splitModule(f.Module.Path), typ), 0)
	}
	abs, ok := absolute(f.Module, segs)
	if !ok {
		return Found{}, false
	}
	return c.lookup(abs, 0)
}

// returns reports whether the return type of the fn f has a path that leads to
// typ.
func (c *Crate) returns(f Found, typ Found) bool {
	for _, p := range retPath.FindAllString(f.Item.Ret, -1) {
		if r, ok := c.resolve(f, splitPath(p)); ok && r.Item == typ.Item {
			return true
		}
	}
	return false
}

// errorNames returns the paths that the Errors section of doc names, in order.
func errorNames(doc string) []string {
	lines, err := sectionLines(strings.Split(doc, "\n"), errorsHeading, "")
	if err != nil {
		return nil
	}
	var out []string
	add := func(s string) {
		if p := cleanTarget(s); p != "" && !slices.Contains(out, p) {
			out = append(out, p)
		}
	}
	fence := ""
	for _, l := range lines {
		next := fenceState(fence, l)
		inCode := fence != "" || next != ""
		fence = next
		if inCode {
			continue
		}
		if m := refDef.FindStringSubmatch(l); m != nil {
			add(m[2])
			continue
		}
		spans := codeSpans(l)
		for _, s := range spans {
			add(strings.Trim(l[s[0]:s[1]], "`"))
		}
		for _, m := range link.FindAllStringSubmatchIndex(l, -1) {
			if inSpan(spans, m[0]) {
				continue
			}
			switch {
			case m[4] >= 0:
				add(l[m[4]:m[5]])
			case m[6] >= 0:
				add(l[m[6]:m[7]])
			default:
				add(l[m[2]:m[3]])
			}
		}
	}
	return out
}
