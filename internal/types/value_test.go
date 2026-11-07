package types

import (
	"testing"

	"github.com/USA-RedDragon/docgen/internal/load"
)

const valueSource = `// Package p holds values.
package p

// The group doc.
const (
	// Own has a doc.
	Own = 1
	Trail = 2 // Trail has a trailing comment.
	Grouped = 3
)

// T is a type.
type T int

// The group doc of T.
const (
	// A has a doc.
	A T = 1
	B T = 2
)

// V is a var.
var V = T(3)

var (
	// W has a doc.
	W, X = 1, 2
)

const Bare = 4
`

func TestValueDoc(t *testing.T) {
	t.Parallel()
	pi := loadSource(t, map[string]string{"p.go": valueSource})
	checkValueDocs(t, pi)
}

func TestValueDocOfRootPackage(t *testing.T) {
	t.Parallel()
	pi := loadSource(t, map[string]string{"p.go": valueSource})
	checkValueDocs(t, &load.Package{Doc: pi.Doc, Path: pi.Path})
}

func checkValueDocs(t *testing.T, pi *load.Package) {
	t.Helper()
	for name, want := range map[string]string{
		"Own":     "Own has a doc.\n",
		"Trail":   "Trail has a trailing comment.\n",
		"Grouped": "The group doc.\n",
		"A":       "A has a doc.\n",
		"B":       "The group doc of T.\n",
		"V":       "V is a var.\n",
		"W":       "W has a doc.\n",
		"X":       "W has a doc.\n",
		"Bare":    "",
	} {
		if got, ok := ValueDoc(pi, name); !ok || got != want {
			t.Errorf("ValueDoc(%s) = %q, %v, want %q", name, got, ok, want)
		}
	}
	if got, ok := ValueDoc(pi, "Nope"); ok {
		t.Errorf("ValueDoc(Nope) = %q, want no doc", got)
	}
}

func TestConstantDocOfConversion(t *testing.T) {
	t.Parallel()
	pi := loadSource(t, map[string]string{"p.go": `package p

// State is a state.
type State int

const (
	// Idle waits.
	Idle = State(0)
	// Busy works.
	Busy = State(1)
	Done = State(2) // Done has a trailing comment.
)

// Typed has a type.
const Typed State = 3
`})
	tp, ok := Lookup(pi, "State")
	if !ok {
		t.Fatal("no State")
	}
	want := map[string]string{"Idle": "Idle waits.\n", "Busy": "Busy works.\n", "Done": "Done has a trailing comment.\n", "Typed": "Typed has a type.\n"}
	consts := Constants(tp)
	if len(consts) != len(want) {
		t.Fatalf("Constants = %+v", consts)
	}
	for _, c := range consts {
		if c.Doc != want[c.Name] {
			t.Errorf("%s: doc %q, want %q", c.Name, c.Doc, want[c.Name])
		}
	}
}
