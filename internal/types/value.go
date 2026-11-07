package types

import (
	"go/ast"
	"go/doc"
	"slices"

	"github.com/USA-RedDragon/docgen/internal/load"
)

// A docValue is one spec of a const or var declaration: its names, the doc
// comment or trailing comment of the spec, and the doc of its group.
type docValue struct {
	names []string
	doc   string
	group string
}

// text returns the doc of the spec, or else the doc of its group.
func (v *docValue) text() string {
	if v.doc != "" {
		return v.doc
	}
	return v.group
}

func values(vs []*doc.Value) []*docValue {
	out := make([]*docValue, 0, len(vs))
	for _, v := range vs {
		for _, s := range v.Decl.Specs {
			spec, ok := s.(*ast.ValueSpec)
			if !ok {
				continue
			}
			d := spec.Doc.Text()
			if d == "" {
				d = spec.Comment.Text()
			}
			if d == "" && !v.Decl.Lparen.IsValid() {
				d = v.Doc
			}
			dv := &docValue{doc: d, group: v.Doc}
			for _, n := range spec.Names {
				dv.names = append(dv.names, n.Name)
			}
			out = append(out, dv)
		}
	}
	return out
}

func typeValues(pi *load.Package) []*docValue {
	out := make([]*docValue, 0, len(pi.Doc.Types))
	for _, t := range pi.Doc.Types {
		out = append(out, values(t.Consts)...)
		out = append(out, values(t.Vars)...)
	}
	return out
}

// allValues returns each const and var of pi, wherever the doc of pi lists it: go/doc
// lists a value with a type by that type, and one with a conversion as its value
// with no type.
func allValues(pi *load.Package) []*docValue {
	return slices.Concat(values(pi.Doc.Vars), values(pi.Doc.Consts), typeValues(pi))
}

// ValueDoc returns the doc of the const or var name of pi: the doc comment of its
// spec, or its trailing comment, or the doc of its group. It reports false when pi
// has no such name. It reads the doc alone, so it also works for a package that
// load.RootPackage reads.
func ValueDoc(pi *load.Package, name string) (string, bool) {
	for _, v := range allValues(pi) {
		if slices.Contains(v.names, name) {
			return v.text(), true
		}
	}
	return "", false
}

// constDocs returns the doc of each spec of the consts and vars of pi, by name, and
// none for a nil pi. The doc of a group is not part of it.
func constDocs(pi *load.Package) map[string]string {
	docs := map[string]string{}
	if pi == nil {
		return docs
	}
	for _, v := range allValues(pi) {
		for _, name := range v.names {
			docs[name] = v.doc
		}
	}
	return docs
}
