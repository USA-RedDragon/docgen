package types

import (
	"errors"
	"go/constant"
	gotypes "go/types"
	"slices"
	"strings"
	"testing"
)

const constantSource = `package p

// Level has constants out of value order.
type Level int

// The group doc of the levels.
const (
	// High is the last value.
	High Level = 9
	Low Level = 1 // Low is the first value.
	Mid Level = 5
	Same Level = 5
	hidden Level = 2
)

// Mixed is a group of two types, so it is not in the doc of Level.
const (
	Stray Level = 7
	Other = 3
)

// Band has a string value.
type Band string

const (
	BandA Band = "z"
	BandB Band = "a"
)

// Dir has a bool value.
type Dir bool

const (
	Up Dir = true
	Down Dir = false
)

// Wave has a complex value.
type Wave complex128

const (
	W2 Wave = 2i
	W1 Wave = 1i
)

// Ratio has a float value.
type Ratio float64

const (
	Whole Ratio = 1
	Half Ratio = 0.5
)

// LevelAlias is an alias.
type LevelAlias = Level

// NoConst has no constants.
type NoConst int

// Plain is a value of no named type.
const Plain = 3

const (
	Three = 3
	Tres = 3
	four = 4
)

var (
	Var = 12
	Typed Level = 5
	Any any = Level(1)
	None int
	Call = len("x") + f()
	Second, Third = 2, "three"
)

func f() int { return 0 }
`

func constNames(cs []Const) []string {
	out := make([]string, 0, len(cs))
	for _, c := range cs {
		out = append(out, c.Name)
	}
	return out
}

func TestConstantsOrder(t *testing.T) {
	t.Parallel()
	pi := loadSource(t, map[string]string{"p.go": constantSource})
	for typ, want := range map[string][]string{
		"Level":      {"Low", "Mid", "Same", "Stray", "High"},
		"LevelAlias": {"Low", "Mid", "Same", "Stray", "High"},
		"Band":       {"BandB", "BandA"},
		"Dir":        {"Down", "Up"},
		"Wave":       {"W1", "W2"},
		"Ratio":      {"Half", "Whole"},
		"NoConst":    nil,
	} {
		if got := constNames(Constants(lookup(t, pi, typ))); !slices.Equal(got, want) {
			t.Errorf("Constants(%s) = %v, want %v", typ, got, want)
		}
	}
}

func TestConstantsDocs(t *testing.T) {
	t.Parallel()
	pi := loadSource(t, map[string]string{"p.go": constantSource})
	want := map[string]string{
		"Low":   "Low is the first value.\n",
		"Mid":   "",
		"Same":  "",
		"Stray": "",
		"High":  "High is the last value.\n",
	}
	cs := Constants(lookup(t, pi, "Level"))
	for _, c := range cs {
		if c.Doc != want[c.Name] {
			t.Errorf("doc of %s = %q, want %q", c.Name, c.Doc, want[c.Name])
		}
		if c.Pkg != pi.Pkg.Types || c.Value == nil || c.Type == nil {
			t.Errorf("Constants: %+v", c)
		}
	}
	n := named(lookup(t, pi, "Level").Go)
	for _, c := range Constants(Of(nil, n)) {
		if c.Doc != "" {
			t.Errorf("doc of %s with no package = %q", c.Name, c.Doc)
		}
	}
}

func TestConstantsOfOtherPackage(t *testing.T) {
	t.Parallel()
	pi := loadSource(t, map[string]string{
		"p.go":   "package p\n\nimport \"example.com/m/p/q\"\n\n// Use reads Q.\nfunc Use() q.Q { return q.One }\n",
		"q/q.go": "package q\n\n// Q is a type.\ntype Q int\n\n// The values.\nconst (\n\tTwo Q = 2\n\tOne Q = 1\n)\n",
	})
	imported := pi.Pkg.Types.Imports()[0]
	n, ok := imported.Scope().Lookup("Q").Type().(*gotypes.Named)
	if !ok {
		t.Fatal("Q is not a named type")
	}
	cs := Constants(Of(nil, n))
	if got, want := constNames(cs), []string{"One", "Two"}; !slices.Equal(got, want) {
		t.Fatalf("Constants(q.Q) = %v, want %v", got, want)
	}
	if cs[0].Pkg != imported {
		t.Errorf("Pkg = %v, want %v", cs[0].Pkg, imported)
	}
}

func TestConstantsOfBasicAlias(t *testing.T) {
	t.Parallel()
	pi := loadSource(t, map[string]string{"p.go": "package p\n\n// I is an alias of a basic type.\ntype I = int\n\nconst A I = 1\n"})
	if got := Constants(lookup(t, pi, "I")); got != nil {
		t.Errorf("Constants(I) = %+v, want nil", got)
	}
}

func TestConstantName(t *testing.T) {
	t.Parallel()
	pi := loadSource(t, map[string]string{"p.go": constantSource})
	level := lookup(t, pi, "Level").Go
	intType := gotypes.Typ[gotypes.Int]
	untyped := gotypes.Typ[gotypes.UntypedInt]
	for _, c := range []struct {
		typ     gotypes.Type
		value   int64
		want    string
		wantErr string
	}{
		{level, 9, "High", ""},
		{level, 2, "hidden", ""},
		{level, 5, "", "several constants of type example.com/m/p.Level have the value 5: Mid, Same"},
		{level, 100, "", "no constant of type example.com/m/p.Level has the value 100"},
		{intType, 4, "four", ""},
		{untyped, 4, "four", ""},
		{untyped, 3, "", "several constants of type int have the value 3: Other, Plain, Three, Tres"},
	} {
		got, err := ConstantName(pi, c.typ, constant.MakeInt64(c.value))
		if c.wantErr != "" {
			if err == nil || err.Error() != c.wantErr {
				t.Errorf("ConstantName(%s, %d): got %v, want %q", c.typ, c.value, err, c.wantErr)
			}
			continue
		}
		if err != nil || got != c.want {
			t.Errorf("ConstantName(%s, %d) = %q, %v, want %q", c.typ, c.value, got, err, c.want)
		}
	}
	if _, err := ConstantName(pi, level, constant.MakeInt64(100)); !errors.Is(err, ErrNoConstant) {
		t.Errorf("got %v, want ErrNoConstant", err)
	}
}

func TestValue(t *testing.T) {
	t.Parallel()
	pi := loadSource(t, map[string]string{"p.go": constantSource})
	for _, c := range []struct{ name, typ, value string }{
		{"High", "example.com/m/p.Level", "9"},
		{"hidden", "example.com/m/p.Level", "2"},
		{"Plain", "untyped int", "3"},
		{"Var", "int", "12"},
		{"Typed", "example.com/m/p.Level", "5"},
		{"Any", "example.com/m/p.Level", "1"},
		{"Second", "int", "2"},
		{"Third", "string", `"three"`},
	} {
		got, err := Value(pi, c.name)
		if err != nil {
			t.Errorf("Value(%s): %v", c.name, err)
			continue
		}
		if got.Name != c.name || got.Type.String() != c.typ || got.Value.ExactString() != c.value || got.Pkg != pi.Pkg.Types {
			t.Errorf("Value(%s) = %+v, want type %s, value %s", c.name, got, c.typ, c.value)
		}
	}
}

func TestValueErrors(t *testing.T) {
	t.Parallel()
	pi := loadSource(t, map[string]string{"p.go": constantSource})
	for name, want := range map[string]string{
		"None":  "var None has no initial value",
		"Call":  "the initial value of var Call is not a constant expression",
		"Nope":  "Nope is not a constant or a package-level var of example.com/m/p",
		"Level": "Level is not a constant or a package-level var of example.com/m/p",
		"f":     "f is not a constant or a package-level var of example.com/m/p",
	} {
		if _, err := Value(pi, name); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("Value(%s): got %v, want %q", name, err, want)
		}
	}
}
