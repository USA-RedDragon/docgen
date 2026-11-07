// Package types reads the Go types, constants and doc comments of a loaded
// package in the terms of docgen: the fields of a config struct, the constants
// of a state type, the methods of a machine and the errors of a package.
package types

import (
	"go/doc"
	gotypes "go/types"

	"github.com/USA-RedDragon/docgen/internal/load"
)

// A Type is a named type of a package. Go is the type, for an alias the alias
// itself. Doc is its doc, or nil when the package has none. pi is the package of
// the doc, or nil.
type Type struct {
	Name string
	Go   gotypes.Type
	Doc  *doc.Type
	pi   *load.Package
}

// Lookup returns the type name of pi, and false when name is not a type.
func Lookup(pi *load.Package, name string) (Type, bool) {
	tn, ok := pi.Pkg.Types.Scope().Lookup(name).(*gotypes.TypeName)
	if !ok {
		return Type{}, false
	}
	return Type{Name: name, Go: tn.Type(), Doc: docType(pi, name), pi: pi}, true
}

// Of returns the Type of n, with its doc from pi, the package of n. With pi nil
// it has no doc.
func Of(pi *load.Package, n *gotypes.Named) Type {
	name := n.Obj().Name()
	return Type{Name: name, Go: n, Doc: docType(pi, name), pi: pi}
}

// ExportedTypes returns the exported types of pi in name order.
func ExportedTypes(pi *load.Package) []Type {
	scope := pi.Pkg.Types.Scope()
	var out []Type
	for _, name := range scope.Names() {
		if tn, ok := scope.Lookup(name).(*gotypes.TypeName); ok && tn.Exported() {
			out = append(out, Type{Name: name, Go: tn.Type(), Doc: docType(pi, name), pi: pi})
		}
	}
	return out
}

// Generic reports whether t has type parameters. An alias of a generic type has
// none of its own.
func Generic(t Type) bool {
	n, ok := t.Go.(*gotypes.Named)
	return ok && n.TypeParams().Len() > 0
}

func docType(pi *load.Package, name string) *doc.Type {
	if pi == nil {
		return nil
	}
	for _, t := range pi.Doc.Types {
		if t.Name == name {
			return t
		}
	}
	return nil
}
