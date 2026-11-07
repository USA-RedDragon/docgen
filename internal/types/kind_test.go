package types

import (
	gotypes "go/types"
	"testing"
)

const kindSource = `package p

import "time"

type N int
type U uint8
type B bool
type S string
type NA = N
type IA = int
type D = time.Duration
type L []int
type G[T any] struct{}

// P has a parameter type.
func P[T ~int]() (x T) { return }

var Stdlib time.Time
`

func TestIntegerKinds(t *testing.T) {
	t.Parallel()
	pi := loadSource(t, map[string]string{"p.go": kindSource})
	for _, c := range []struct {
		name                         string
		isInt, isUnsigned, isBool    bool
		named, integer, namedInteger bool
	}{
		{name: "N", isInt: true, named: true, integer: true, namedInteger: true},
		{name: "U", isInt: true, isUnsigned: true, named: true, integer: true, namedInteger: true},
		{name: "B", isBool: true, named: true},
		{name: "S", named: true},
		{name: "NA", isInt: true, named: true, integer: true, namedInteger: true},
		{name: "IA", isInt: true, integer: true},
		{name: "L", named: true},
	} {
		typ := lookup(t, pi, c.name).Go
		if got := IsInteger(typ); got != c.isInt {
			t.Errorf("IsInteger(%s) = %v", c.name, got)
		}
		if got := IsUnsigned(typ); got != c.isUnsigned {
			t.Errorf("IsUnsigned(%s) = %v", c.name, got)
		}
		if got := IsBool(typ); got != c.isBool {
			t.Errorf("IsBool(%s) = %v", c.name, got)
		}
		if got := IsNamed(typ); got != c.named {
			t.Errorf("IsNamed(%s) = %v", c.name, got)
		}
		if got := Integer(typ) != nil; got != c.integer {
			t.Errorf("Integer(%s) = %v", c.name, got)
		}
		if got := NamedInteger(typ) != nil; got != c.namedInteger {
			t.Errorf("NamedInteger(%s) = %v", c.name, got)
		}
	}
}

func TestIntegerHasNoAlias(t *testing.T) {
	t.Parallel()
	pi := loadSource(t, map[string]string{"p.go": kindSource})
	if _, ok := Integer(lookup(t, pi, "NA").Go).(*gotypes.Named); !ok {
		t.Error("Integer(NA) is not the named type")
	}
	if got := Integer(lookup(t, pi, "IA").Go); got != gotypes.Typ[gotypes.Int] {
		t.Errorf("Integer(IA) = %v, want int", got)
	}
}

func TestTypeParameterIsNoInteger(t *testing.T) {
	t.Parallel()
	pi := loadSource(t, map[string]string{"p.go": kindSource})
	sig, ok := pi.Pkg.Types.Scope().Lookup("P").Type().(*gotypes.Signature)
	if !ok {
		t.Fatal("P is not a func")
	}
	param := sig.Results().At(0).Type()
	if IsInteger(param) || Integer(param) != nil || IsNamed(param) {
		t.Errorf("type parameter %s counts as an integer or a named type", param)
	}
}

func TestIsDuration(t *testing.T) {
	t.Parallel()
	pi := loadSource(t, map[string]string{"p.go": kindSource})
	if !IsDuration(lookup(t, pi, "D").Go) {
		t.Error("IsDuration(alias of time.Duration) = false")
	}
	for _, typ := range []gotypes.Type{lookup(t, pi, "N").Go, gotypes.Typ[gotypes.Int64], gotypes.Universe.Lookup("error").Type(), pi.Pkg.Types.Scope().Lookup("Stdlib").Type()} {
		if IsDuration(typ) {
			t.Errorf("IsDuration(%s) = true", typ)
		}
	}
}

func TestHasNameIn(t *testing.T) {
	t.Parallel()
	here := gotypes.NewPackage("example.com/m/here", "here")
	other := gotypes.NewPackage("example.com/m/other", "other")
	namedIn := func(pkg *gotypes.Package, name string) gotypes.Type {
		return gotypes.NewNamed(gotypes.NewTypeName(0, pkg, name, nil), gotypes.Typ[gotypes.Int], nil)
	}
	for _, c := range []struct {
		typ  gotypes.Type
		want bool
	}{
		{gotypes.Typ[gotypes.Int], true},
		{namedIn(here, "lower"), true},
		{namedIn(here, "Upper"), true},
		{namedIn(other, "Upper"), true},
		{namedIn(other, "lower"), false},
		{gotypes.Universe.Lookup("error").Type(), false},
	} {
		if got := HasNameIn(c.typ, here); got != c.want {
			t.Errorf("HasNameIn(%s) = %v, want %v", c.typ, got, c.want)
		}
	}
}

func TestRawAndBasic(t *testing.T) {
	t.Parallel()
	pi := loadSource(t, map[string]string{"p.go": kindSource})
	n := lookup(t, pi, "N").Go
	if got := Raw(n); got != gotypes.Typ[gotypes.Int] {
		t.Errorf("Raw(N) = %v, want int", got)
	}
	if b, ok := Basic(n); !ok || b.Kind() != gotypes.Int {
		t.Errorf("Basic(N) = %v, %v", b, ok)
	}
	if b, ok := Basic(lookup(t, pi, "L").Go); ok {
		t.Errorf("Basic(L) = %v, want none", b)
	}
}
