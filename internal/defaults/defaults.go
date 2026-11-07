// Package defaults finds the default values of config fields in the code that
// applies them.
package defaults

import (
	"go/ast"
	"go/constant"
	"go/types"
	"strings"

	"github.com/USA-RedDragon/docgen/internal/load"
)

// A Default is a value that the package assigns to a field. Source is its
// expression. Value is its constant value of type Type, and nil for an expression
// that is not constant. Literal reports a literal of the source.
type Default struct {
	Source  string
	Type    types.Type
	Value   constant.Value
	Literal bool
}

// Find returns, for each field of st, the values that the
// package assigns when the field holds its zero value. It follows the field
// through plain copies (assignments, composite literal keys and call
// arguments to functions of the package) and then finds
//
//	if x == 0 { x = V }   and   switch { case x == 0: x = V }
//
// where x is the field or a copy of it. Fields whose default the code
// computes in another way get no entry.
func Find(pi *load.Package, st *types.Struct) map[*types.Var][]Default {
	t := &tracer{pkg: pi.Pkg.Types, info: pi.Pkg.TypesInfo, alias: map[types.Object]*types.Var{}, out: map[*types.Var][]Default{}}
	for i := 0; i < st.NumFields(); i++ {
		t.alias[st.Field(i)] = st.Field(i)
	}
	for _, f := range pi.Pkg.Syntax {
		if !strings.HasSuffix(pi.Pkg.Fset.File(f.Pos()).Name(), "_test.go") {
			t.files = append(t.files, f)
		}
	}
	for t.propagate() {
	}
	for _, f := range t.files {
		ast.Inspect(f, t.findTests)
	}
	return t.out
}

func newDefault(info *types.Info, e ast.Expr) Default {
	tv := info.Types[e]
	_, literal := e.(*ast.BasicLit)
	return Default{Source: types.ExprString(e), Type: tv.Type, Value: tv.Value, Literal: literal}
}
