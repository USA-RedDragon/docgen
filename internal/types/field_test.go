package types

import (
	"go/parser"
	"testing"
)

const fieldSource = `package p

import "time"

// Base is embedded.
type Base struct{}

// Other is embedded by pointer.
type Other struct{}

// G is generic.
type G[T any] struct{}

// Config holds fields.
type Config struct {
	// Base has a doc.
	Base
	*Other
	G[int]
	time.Time
	// Limit has a doc.
	Limit int // a trailing comment
	Name string // Name has a trailing comment.
	Size, Count int // Both have this comment.
	hidden int
	Bare bool
}

// Alias is an alias.
type Alias = Config

// Grouped has its doc in a group.
type (
	// Grouped is a struct.
	Grouped struct {
		// A has a doc.
		A int
	}
)

// Empty has no exported field.
type Empty struct{ a int }

// Number is not a struct.
type Number int
`

func TestFields(t *testing.T) {
	t.Parallel()
	pi := loadSource(t, map[string]string{"p.go": fieldSource})
	want := []struct {
		name, typ, doc string
		embedded       bool
	}{
		{"Base", "example.com/m/p.Base", "Base has a doc.\n", true},
		{"Other", "*example.com/m/p.Other", "", true},
		{"G", "example.com/m/p.G[int]", "", true},
		{"Time", "time.Time", "", true},
		{"Limit", "int", "Limit has a doc.\n", false},
		{"Name", "string", "Name has a trailing comment.\n", false},
		{"Size", "int", "Both have this comment.\n", false},
		{"Count", "int", "Both have this comment.\n", false},
		{"Bare", "bool", "", false},
	}
	got := Fields(lookup(t, pi, "Config"))
	if len(got) != len(want) {
		t.Fatalf("Fields = %+v, want %d fields", got, len(want))
	}
	for i, w := range want {
		f := got[i]
		if f.Name != w.name || f.Type.String() != w.typ || f.Doc != w.doc || f.Embedded != w.embedded || f.Var == nil || f.Var.Name() != w.name {
			t.Errorf("field %d = %+v, want %+v", i, f, w)
		}
	}
}

func TestFieldsOfOtherTypes(t *testing.T) {
	t.Parallel()
	pi := loadSource(t, map[string]string{"p.go": fieldSource})
	alias := Fields(lookup(t, pi, "Alias"))
	if len(alias) != 9 || alias[0].Doc != "Base has a doc.\n" {
		t.Errorf("Fields(Alias) = %+v, want the fields of Config with their docs", alias)
	}
	if got := Fields(lookup(t, pi, "Grouped")); len(got) != 1 || got[0].Doc != "A has a doc.\n" {
		t.Errorf("Fields(Grouped) = %+v", got)
	}
	empty := lookup(t, pi, "Empty")
	if got := Fields(empty); len(got) != 0 {
		t.Errorf("Fields(Empty) = %+v", got)
	}
	if _, ok := Struct(empty); !ok {
		t.Error("Struct(Empty) reports no struct")
	}
	number := lookup(t, pi, "Number")
	if got := Fields(number); got != nil {
		t.Errorf("Fields(Number) = %+v", got)
	}
	if _, ok := Struct(number); ok {
		t.Error("Struct(Number) reports a struct")
	}
}

func TestEmbeddedName(t *testing.T) {
	t.Parallel()
	for src, want := range map[string]string{
		"T":       "T",
		"*T":      "T",
		"p.T":     "T",
		"*p.T":    "T",
		"T[int]":  "T",
		"T[a, b]": "T",
		"(T)":     "T",
		"[]T":     "",
	} {
		e, err := parser.ParseExpr(src)
		if err != nil {
			t.Fatal(err)
		}
		if got := embeddedName(e); got != want {
			t.Errorf("embeddedName(%s) = %q, want %q", src, got, want)
		}
	}
}

func TestFieldDocsOfAliasAndDefinedTypes(t *testing.T) {
	t.Parallel()
	pi := loadSource(t, map[string]string{
		"p.go": `package p

import "example.com/m/p/q"

// Cfg holds settings.
type Cfg struct {
	// A is the first.
	A int
	B int // B is the second.
}

// CfgA is an alias.
type CfgA = Cfg

// CfgD is a defined type.
type CfgD Cfg

// CfgE is a defined type of a defined type.
type CfgE CfgD

// CfgQ is an alias of a type of another package.
type CfgQ = q.Cfg

// CfgR is a defined type of a type of another package.
type CfgR q.Cfg
`,
		"q/q.go": `// Package q holds a config.
package q

// Cfg holds settings.
type Cfg struct {
	// C is in q.
	C int
}
`,
	})
	for name, want := range map[string]map[string]string{
		"CfgA": {"A": "A is the first.\n", "B": "B is the second.\n"},
		"CfgD": {"A": "A is the first.\n", "B": "B is the second.\n"},
		"CfgE": {"A": "A is the first.\n", "B": "B is the second.\n"},
		"CfgQ": {"C": "C is in q.\n"},
		"CfgR": {"C": "C is in q.\n"},
	} {
		tp, ok := Lookup(pi, name)
		if !ok {
			t.Fatal(name)
		}
		fields := Fields(tp)
		if len(fields) != len(want) {
			t.Fatalf("%s: %d fields", name, len(fields))
		}
		for _, f := range fields {
			if f.Doc != want[f.Name] {
				t.Errorf("%s.%s: doc %q, want %q", name, f.Name, f.Doc, want[f.Name])
			}
		}
	}
}
