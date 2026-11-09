package types

import (
	"go/token"
	gotypes "go/types"
	"slices"
	"strings"
	"testing"
)

const methodSource = `package p

import "fmt"

// Base holds promoted methods.
type Base struct{}

// Promoted is promoted to each type that embeds Base.
func (*Base) Promoted(a int, b string) (bool, error) { return false, nil }

// Inner is generic.
type Inner[T any] struct{ v T }

// Value returns the value.
func (i *Inner[T]) Value() T { return i.v }

// Outer embeds both.
type Outer struct {
	Base
	Inner[int8]
}

func (o Outer) ByValue() {}
func (o *Outer) ByPointer(x ...int) {}
func (o *Outer) hidden() {}

// Reader is an interface.
type Reader interface{ Read() }

// Stringer has a String method.
type Stringer int

func (Stringer) String() string { return "" }

// PointerStringer has a String method on the pointer only.
type PointerStringer int

func (*PointerStringer) String() string { return "" }

// Err has an Error method.
type Err struct{}

func (Err) Error() string { return "" }

// Formatter has a Format method.
type Formatter struct{}

func (Formatter) Format(fmt.State, rune) {}

// GoStringer has a GoString method.
type GoStringer struct{}

func (GoStringer) GoString() string { return "" }

// None has no print method.
type None struct{}

func (None) Print() {}

// Text has a MarshalText method.
type Text int

func (Text) MarshalText() ([]byte, error) { return nil, nil }

// PointerText has a MarshalText method on the pointer only.
type PointerText int

func (*PointerText) MarshalText() ([]uint8, error) { return nil, nil }

// WrongText has a MarshalText method of another signature.
type WrongText int

func (WrongText) MarshalText() string { return "" }
`

func signature(m Method) string {
	var b strings.Builder
	b.WriteString(m.Name + "(")
	for _, p := range m.Params {
		b.WriteString(p.String() + " ")
	}
	b.WriteString(")")
	for _, r := range m.Results {
		b.WriteString(" " + r.String())
	}
	return b.String()
}

func TestMethods(t *testing.T) {
	t.Parallel()
	pi := loadSource(t, map[string]string{"p.go": methodSource})
	var got []string
	for _, m := range Methods(lookup(t, pi, "Outer")) {
		got = append(got, signature(m))
	}
	want := []string{"ByPointer([]int )", "ByValue()", "Promoted(int string ) bool error", "Value() int8"}
	if !slices.Equal(got, want) {
		t.Errorf("Methods(Outer) = %q, want %q", got, want)
	}
	if got := Methods(lookup(t, pi, "Reader")); len(got) != 0 {
		t.Errorf("Methods(Reader) = %v, want none on a pointer to an interface", got)
	}
}

func TestHasPrintMethod(t *testing.T) {
	t.Parallel()
	pi := loadSource(t, map[string]string{"p.go": methodSource})
	for name, want := range map[string]bool{
		"Stringer":        true,
		"Err":             true,
		"Formatter":       true,
		"GoStringer":      true,
		"PointerStringer": false,
		"None":            false,
	} {
		if got := HasPrintMethod(lookup(t, pi, name).Go); got != want {
			t.Errorf("HasPrintMethod(%s) = %v, want %v", name, got, want)
		}
	}
	ptr := gotypes.NewPointer(lookup(t, pi, "PointerStringer").Go)
	if !HasPrintMethod(ptr) {
		t.Error("HasPrintMethod(*PointerStringer) = false")
	}
	if HasPrintMethod(gotypes.NewPointer(gotypes.NewNamed(gotypes.NewTypeName(token.NoPos, nil, "T", nil), gotypes.Typ[gotypes.Int], nil))) {
		t.Error("HasPrintMethod(*T) = true with no method")
	}
}

func TestIsTextMarshaler(t *testing.T) {
	t.Parallel()
	pi := loadSource(t, map[string]string{"p.go": methodSource})
	for name, want := range map[string]bool{
		"Text":        true,
		"PointerText": true,
		"WrongText":   false,
		"Stringer":    false,
	} {
		if got := IsTextMarshaler(lookup(t, pi, name).Go); got != want {
			t.Errorf("IsTextMarshaler(%s) = %v, want %v", name, got, want)
		}
	}
	if IsTextMarshaler(gotypes.Typ[gotypes.Int]) {
		t.Error("IsTextMarshaler(int) = true")
	}
}
