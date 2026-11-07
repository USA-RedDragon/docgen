package types

import (
	"go/ast"
	"go/doc"
	gotypes "go/types"

	"github.com/USA-RedDragon/docgen/internal/load"
	"golang.org/x/tools/go/packages"
)

// A Field is an exported field of a struct. Doc is its doc comment or its
// trailing comment. Var is the field in go/types.
type Field struct {
	Name     string
	Type     gotypes.Type
	Doc      string
	Embedded bool
	Var      *gotypes.Var
}

// Struct returns the struct of t, and false when t is not a struct.
func Struct(t Type) (*gotypes.Struct, bool) {
	st, ok := t.Go.Underlying().(*gotypes.Struct)
	return st, ok
}

// Fields returns the exported fields of the struct t in source order, and nil
// when t is not a struct.
func Fields(t Type) []Field {
	st, ok := Struct(t)
	if !ok {
		return nil
	}
	docs := fieldDocs(t, st)
	var out []Field
	for i := 0; i < st.NumFields(); i++ {
		f := st.Field(i)
		if !f.Exported() {
			continue
		}
		out = append(out, Field{Name: f.Name(), Type: f.Type(), Doc: docs[f.Name()], Embedded: f.Embedded(), Var: f})
	}
	return out
}

// fieldDocs returns the doc of each field of st, the struct of t, by name. An alias
// or a defined type of a struct has the docs of the declaration of that struct.
func fieldDocs(t Type, st *gotypes.Struct) map[string]string {
	if s := docStruct(t.Doc); s != nil {
		return structDocs(s)
	}
	if t.pi == nil || st.NumFields() == 0 {
		return map[string]string{}
	}
	return structDocs(declaredStruct(t.pi, st.Field(0)))
}

// docStruct returns the struct in the declaration of dt, and nil when dt is nil or
// declares another type.
func docStruct(dt *doc.Type) *ast.StructType {
	if dt == nil {
		return nil
	}
	for _, spec := range dt.Decl.Specs {
		if ts, ok := spec.(*ast.TypeSpec); ok {
			if st, ok := ts.Type.(*ast.StructType); ok {
				return st
			}
		}
	}
	return nil
}

func structDocs(st *ast.StructType) map[string]string {
	out := map[string]string{}
	if st == nil {
		return out
	}
	for _, f := range st.Fields.List {
		d := f.Doc.Text()
		if d == "" {
			d = f.Comment.Text()
		}
		for _, n := range f.Names {
			out[n.Name] = d
		}
		if len(f.Names) == 0 {
			out[embeddedName(f.Type)] = d
		}
	}
	return out
}

// declaredStruct returns the struct literal that declares first, its first field,
// in the source of pi or of a package that pi imports, or nil.
func declaredStruct(pi *load.Package, first *gotypes.Var) *ast.StructType {
	p := importedPackage(pi.Pkg, first.Pkg(), map[*packages.Package]bool{})
	if p == nil {
		return nil
	}
	var found *ast.StructType
	for _, file := range p.Syntax {
		if first.Pos() < file.FileStart || first.Pos() > file.FileEnd {
			continue
		}
		ast.Inspect(file, func(n ast.Node) bool {
			if st, ok := n.(*ast.StructType); ok && len(st.Fields.List) > 0 {
				if f := st.Fields.List[0]; f.Pos() <= first.Pos() && first.Pos() < f.End() {
					found = st
				}
			}
			return true
		})
	}
	return found
}

// importedPackage returns the package with the types pkg: p, or a package that p
// imports, directly or not. It returns nil when there is none.
func importedPackage(p *packages.Package, pkg *gotypes.Package, seen map[*packages.Package]bool) *packages.Package {
	if p.Types == pkg {
		return p
	}
	seen[p] = true
	for _, q := range p.Imports {
		if seen[q] {
			continue
		}
		if found := importedPackage(q, pkg, seen); found != nil {
			return found
		}
	}
	return nil
}

// embeddedName returns the field name of an embedded field of type t.
func embeddedName(t ast.Expr) string {
	switch x := t.(type) {
	case *ast.Ident:
		return x.Name
	case *ast.StarExpr:
		return embeddedName(x.X)
	case *ast.SelectorExpr:
		return x.Sel.Name
	case *ast.IndexExpr:
		return embeddedName(x.X)
	case *ast.IndexListExpr:
		return embeddedName(x.X)
	case *ast.ParenExpr:
		return embeddedName(x.X)
	}
	return ""
}
