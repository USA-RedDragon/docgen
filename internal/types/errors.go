package types

import (
	gotypes "go/types"

	"github.com/USA-RedDragon/docgen/internal/load"
)

// An ErrorDecl is an exported error of a package and its doc.
type ErrorDecl struct {
	Name string
	Doc  string
}

func errorType() *gotypes.Interface {
	i, _ := gotypes.Universe.Lookup("error").Type().Underlying().(*gotypes.Interface)
	return i
}

func isError(t gotypes.Type) bool {
	return gotypes.Implements(t, errorType())
}

// Errors returns the exported errors of pi: first each const and var of an error
// type, then each type T where *T is an error, in name order.
func Errors(pi *load.Package) []ErrorDecl {
	var out []ErrorDecl
	scope := pi.Pkg.Types.Scope()
	for _, v := range allValues(pi) {
		for _, name := range v.names {
			obj := scope.Lookup(name)
			if obj != nil && obj.Exported() && isError(obj.Type()) {
				out = append(out, ErrorDecl{Name: name, Doc: v.text()})
			}
		}
	}
	for _, t := range pi.Doc.Types {
		obj := scope.Lookup(t.Name)
		if obj == nil || !obj.Exported() {
			continue
		}
		if isError(gotypes.NewPointer(obj.Type())) {
			out = append(out, ErrorDecl{Name: t.Name, Doc: t.Doc})
		}
	}
	return out
}
