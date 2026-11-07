package types

import (
	"errors"
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	gotypes "go/types"
	"slices"
	"strings"

	"github.com/USA-RedDragon/docgen/internal/load"
)

// A Const is a constant, or a var with a constant initial value. Pkg is its
// package. Constants sets Doc to the doc comment or the trailing comment of its
// spec, and the doc of a const group is not part of it.
type Const struct {
	Name  string
	Type  gotypes.Type
	Value constant.Value
	Doc   string
	Pkg   *gotypes.Package
}

// ErrNoConstant is the error of a value with no constant of its type.
var ErrNoConstant = errors.New("no constant")

// constantsOf returns the constants of scope for which keep is true, in name order.
func constantsOf(scope *gotypes.Scope, keep func(*gotypes.Const) bool) []*gotypes.Const {
	var out []*gotypes.Const
	for _, name := range scope.Names() {
		if c, ok := scope.Lookup(name).(*gotypes.Const); ok && keep(c) {
			out = append(out, c)
		}
	}
	return out
}

// Constants returns the exported constants of the type t in value order, in name
// order for equal values and for the values of a bool or complex type, which have
// no order. The constants are in the package of t, whichever package holds the doc
// of t.
func Constants(t Type) []Const {
	n := named(t.Go)
	if n == nil || n.Obj().Pkg() == nil {
		return nil
	}
	docs := constDocs(t.pi)
	var out []Const
	for _, c := range constantsOf(n.Obj().Pkg().Scope(), func(c *gotypes.Const) bool {
		return c.Exported() && gotypes.Identical(c.Type(), n)
	}) {
		out = append(out, Const{Name: c.Name(), Type: c.Type(), Value: c.Val(), Doc: docs[c.Name()], Pkg: c.Pkg()})
	}
	slices.SortStableFunc(out, func(a, b Const) int { return compareValues(a.Value, b.Value) })
	return out
}

func compareValues(a, b constant.Value) int {
	if !ordered(a) || !ordered(b) {
		return 0
	}
	switch {
	case constant.Compare(a, token.LSS, b):
		return -1
	case constant.Compare(a, token.GTR, b):
		return 1
	}
	return 0
}

// ordered reports whether values of the kind of v have an order in constant.Compare.
func ordered(v constant.Value) bool {
	switch v.Kind() {
	case constant.Int, constant.Float, constant.String:
		return true
	default:
		return false
	}
}

// ConstantName returns the name of the one constant of type t with the value v, in
// pi or else in the package of t. An unexported or untyped constant counts. No such
// constant is an error wrapping ErrNoConstant.
func ConstantName(pi *load.Package, t gotypes.Type, v constant.Value) (string, error) {
	t = gotypes.Default(t)
	scopes := []*gotypes.Scope{pi.Pkg.Types.Scope()}
	if n := named(t); n != nil && n.Obj().Pkg() != nil && n.Obj().Pkg() != pi.Pkg.Types {
		scopes = append(scopes, n.Obj().Pkg().Scope())
	}
	for _, scope := range scopes {
		consts := constantsOf(scope, func(c *gotypes.Const) bool {
			return gotypes.Identical(gotypes.Default(c.Type()), t) && constant.Compare(c.Val(), token.EQL, v)
		})
		switch len(consts) {
		case 0:
			continue
		case 1:
			return consts[0].Name(), nil
		}
		found := make([]string, 0, len(consts))
		for _, c := range consts {
			found = append(found, c.Name())
		}
		return "", fmt.Errorf("several constants of type %s have the value %s: %s", t, v, strings.Join(found, ", "))
	}
	return "", fmt.Errorf("%w of type %s has the value %s", ErrNoConstant, t, v)
}

// Value returns the constant name of pi, or the package-level var name with a
// constant initial value, exported or not.
func Value(pi *load.Package, name string) (Const, error) {
	switch obj := pi.Pkg.Types.Scope().Lookup(name).(type) {
	case *gotypes.Const:
		return Const{Name: name, Type: obj.Type(), Value: obj.Val(), Pkg: obj.Pkg()}, nil
	case *gotypes.Var:
		if spec, i := valueSpec(pi, obj); spec != nil {
			return varValue(pi, obj, spec, i)
		}
	}
	return Const{}, fmt.Errorf("%s is not a constant or a package-level var of %s", name, pi.Path)
}

// valueSpec returns the declaration of the package-level var obj of pi and the
// index of its name there, or nil.
func valueSpec(pi *load.Package, obj *gotypes.Var) (*ast.ValueSpec, int) {
	for _, f := range pi.Pkg.Syntax {
		for _, d := range f.Decls {
			gd, ok := d.(*ast.GenDecl)
			if !ok {
				continue
			}
			for _, s := range gd.Specs {
				vs, ok := s.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for i, n := range vs.Names {
					if pi.Pkg.TypesInfo.Defs[n] == obj {
						return vs, i
					}
				}
			}
		}
	}
	return nil, 0
}

// varValue returns the constant of the var obj from its declaration vs, where its
// name is at index i. The type of a var of an interface type is the type of the
// value.
func varValue(pi *load.Package, obj *gotypes.Var, vs *ast.ValueSpec, i int) (Const, error) {
	if i >= len(vs.Values) {
		return Const{}, fmt.Errorf("var %s has no initial value", obj.Name())
	}
	tv := pi.Pkg.TypesInfo.Types[vs.Values[i]]
	if tv.Value == nil {
		return Const{}, fmt.Errorf("the initial value of var %s is not a constant expression", obj.Name())
	}
	c := Const{Name: obj.Name(), Type: obj.Type(), Value: tv.Value, Pkg: obj.Pkg()}
	if gotypes.IsInterface(obj.Type()) {
		c.Type = tv.Type
	}
	return c, nil
}
