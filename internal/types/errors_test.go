package types

import (
	"slices"
	"testing"
)

const errorSource = `package p

import "errors"

// The errors of a group.
var (
	ErrGroup = errors.New("group")
	// ErrOwn has a doc.
	ErrOwn = errors.New("own")
	ErrTrail = errors.New("trail") // ErrTrail has a trailing comment.
	errHidden = errors.New("hidden")
	Number = 1
)

// ValueError has a value receiver.
type ValueError struct{}

func (ValueError) Error() string { return "" }

// PointerError has a pointer receiver.
type PointerError struct{}

func (*PointerError) Error() string { return "" }

// ErrPointer is a pointer, so the doc of the package lists it with no type.
var ErrPointer = &PointerError{}

// ErrAttached has the type ValueError, so the doc lists it with that type.
var ErrAttached ValueError

// Plain is not an error.
type Plain struct{}

type lower struct{}

func (lower) Error() string { return "" }
`

func TestErrors(t *testing.T) {
	t.Parallel()
	pi := loadSource(t, map[string]string{"p.go": errorSource})
	want := []ErrorDecl{
		{"ErrGroup", "The errors of a group.\n"},
		{"ErrOwn", "ErrOwn has a doc.\n"},
		{"ErrTrail", "ErrTrail has a trailing comment.\n"},
		{"ErrPointer", "ErrPointer is a pointer, so the doc of the package lists it with no type.\n"},
		{"ErrAttached", "ErrAttached has the type ValueError, so the doc lists it with that type.\n"},
		{"PointerError", "PointerError has a pointer receiver.\n"},
		{"ValueError", "ValueError has a value receiver.\n"},
	}
	if got := Errors(pi); !slices.Equal(got, want) {
		t.Errorf("Errors = %+v, want %+v", got, want)
	}
}

func TestErrorsOfConstAndTypedVar(t *testing.T) {
	t.Parallel()
	pi := loadSource(t, map[string]string{"p.go": `package p

// Error is a text error.
type Error string

func (e Error) Error() string { return string(e) }

// ErrX is a const error.
const ErrX Error = "x"

// Code is a number error.
type Code int

func (c Code) Error() string { return "code" }

// ErrTyped is a typed var.
var ErrTyped Code = 2

const errHidden Error = "hidden"

// NotError is a const of another type.
const NotError = 1
`})
	got := map[string]string{}
	for _, e := range Errors(pi) {
		got[e.Name] = e.Doc
	}
	for name, doc := range map[string]string{"ErrX": "ErrX is a const error.\n", "ErrTyped": "ErrTyped is a typed var.\n"} {
		if got[name] != doc {
			t.Errorf("Errors[%s] = %q, want %q", name, got[name], doc)
		}
	}
	for _, name := range []string{"errHidden", "NotError"} {
		if _, ok := got[name]; ok {
			t.Errorf("Errors holds %s", name)
		}
	}
}
