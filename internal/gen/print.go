package gen

import (
	"go/constant"
	gotypes "go/types"
	"strings"

	"github.com/USA-RedDragon/docgen/internal/goprint"
	"github.com/USA-RedDragon/docgen/internal/load"
	"github.com/USA-RedDragon/docgen/internal/markdown"
	"github.com/USA-RedDragon/docgen/internal/marker"
	"github.com/USA-RedDragon/docgen/internal/types"
)

const argCode = "code"

// valueArgs holds the print= and code= arguments of a marker with a value:
// docgen:const, docgen:config, docgen:var and docgen:gomod. printAs is the
// print= value, and plain is true for code=false.
type valueArgs struct {
	printAs string
	plain   bool
}

// bindPrint reads print=. A value that is not go, raw, name or one fmt verb is an
// error, and so is name when names is false.
func (v *valueArgs) bindPrint(a *marker.Args, names bool) {
	v.printAs, _ = a.Optional(argPrint)
	a.NonEmpty(argPrint)
	if err := goprint.CheckPrint(v.printAs); err != nil {
		a.Fail(err.Error())
	} else if !names && v.printAs == goprint.PrintName {
		a.Fail("print=name is not valid for a value with no constant")
	}
}

// bindCode reads code= of an inline marker. false is the one valid value.
func (v *valueArgs) bindCode(a *marker.Args) {
	code, ok := a.Optional(argCode)
	a.NonEmpty(argCode)
	if ok && code != "" && code != "false" {
		a.Fail("code= can only be false")
	}
	v.plain = code == "false"
}

// planPrint sets the print= value of a marker with none from the print setting
// of g.
func (v *valueArgs) planPrint(g *Generator) {
	v.printAs = g.printValue(v.printAs)
}

// constText returns the text of the constant value val of type t in pi. expr is
// the source of the value for a run of the code.
func (v valueArgs) constText(vs *valueSet, pi *load.Package, expr string, t gotypes.Type, val constant.Value) (string, error) {
	if v.printAs == goprint.PrintName {
		return types.ConstantName(pi, t, val)
	}
	return vs.text(pi, valueRef{expr, v.printAs}, t, val)
}

// stringText returns the text of the string s. go and raw are s, and a fmt
// verb applies to s.
func (v valueArgs) stringText(s string) (string, error) {
	return goprint.Print(gotypes.Typ[gotypes.String], constant.MakeString(s), v.printAs)
}

// inline returns s as the body of an inline marker: a code span, or with
// code=false the text with each line ending as a space. An empty s is "".
func (v valueArgs) inline(s string) string {
	if !v.plain {
		return markdown.CodeSpan(s)
	}
	if s == "" {
		return `""`
	}
	return strings.NewReplacer("\r\n", " ", "\n", " ").Replace(s)
}
