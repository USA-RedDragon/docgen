package cli

import (
	"strings"
	"testing"
)

const configRules = `package p

import "time"

// Mode is a mode.
type Mode int

// The modes.
const (
	ModeUnset Mode = iota
	ModeFast
	ModeSlow
)

// DefaultWait is a wait.
const DefaultWait = 250 * time.Millisecond

var hold = 3

// Config holds settings.
type Config struct {
	Wait   time.Duration
	Lit    int
	Neg    int
	Expr   time.Duration
	Var    int
	Str    string
	Nil    *int
	Leq    int
	Rev    int
	Sw     int
	SwTag  int
	Init   int
	Two    int
	VarCp  int
	KeyF   int
	CallF  int
	Shad   int
	Plus   int
	Pair   int
	InFor  int
	InFn   int
	Else   int
	Float  float64
	Named  Mode
	Nested int
	Defl   int
	TestF  int
	Bool   bool
	Cond   int
	Len    string
	Assign int
	False  bool
	Less   int
	Order  int
}

type lamp struct{ keyf int }

func use(n int) int {
	if n == 0 {
		n = 11
	}
	return n
}

// New applies the defaults.
func New(c Config) Config {
	wait := c.Wait
	if wait == 0 {
		wait = DefaultWait
	}
	_ = wait
	if c.Lit == 0 {
		c.Lit = 3
	}
	if c.Neg == 0 {
		c.Neg = -1
	}
	if c.Expr == 0 {
		c.Expr = 2 * time.Second
	}
	if c.Var == 0 {
		c.Var = hold
	}
	if c.Str == "" {
		c.Str = "x"
	}
	if c.Nil == nil {
		c.Nil = new(int)
	}
	if c.Leq <= 0 {
		c.Leq = 4
	}
	if 0 == c.Rev {
		c.Rev = 4
	}
	switch {
	case c.Sw == 0:
		c.Sw = 6
	}
	switch c.SwTag {
	case 0:
		c.SwTag = 6
	}
	if x := c.Init; x == 0 {
		x = 8
		_ = x
	}
	if c.Two == 0 {
		c.Two = 1
	}
	if c.Two == 0 {
		c.Two = 2
	}
	if c.Two == 0 {
		c.Two = 1
	}
	var vc = c.VarCp
	if vc == 0 {
		vc = 12
	}
	_ = vc
	l := lamp{keyf: c.KeyF}
	if l.keyf == 0 {
		l.keyf = 13
	}
	c.CallF = use(c.CallF)
	if c.Shad == 0 {
		shad := 14
		_ = shad
	}
	if c.Plus == 0 {
		c.Plus += 15
	}
	if c.Pair == 0 {
		c.Pair, c.Lit = 16, 3
	}
	if c.InFor == 0 {
		for range 1 {
			c.InFor = 17
		}
	}
	if c.InFn == 0 {
		func() { c.InFn = 18 }()
	}
	if c.Else != 0 {
		c.Lit = 3
	} else {
		c.Else = 19
	}
	if c.Float == 0.0 {
		c.Float = 1.5
	}
	if c.Named == ModeUnset {
		c.Named = ModeSlow
	}
	if c.Nested == 0 {
		if c.Lit > 0 {
			c.Nested = 20
		}
	}
	if !c.Bool {
		c.Bool = true
	}
	if c.Cond == 0 && c.Lit > 0 {
		c.Cond = 21
	}
	if len(c.Len) == 0 {
		c.Len = "y"
	}
	var as int
	as = c.Assign
	if as == 0 {
		as = 22
	}
	_ = as
	if c.False == false {
		c.False = true
	}
	if c.Less < 1 {
		c.Less = 23
	}
	if c.Order == 0 {
		c.Order = 5
	}
	return c
}

func (c *Config) defaults() {
	if c.Defl == 0 {
		c.Defl = 10
	}
}
`

func TestConfigDefaultRules(t *testing.T) {
	t.Parallel()
	got := fill(t, map[string]string{
		"p/p.go":      configRules,
		"p/a.go":      "package p\n\nfunc a(c Config) {\n\tif c.Order == 0 {\n\t\tc.Order = 9\n\t}\n}\n",
		"p/p_test.go": "package p\n\nfunc init() {\n\tvar c Config\n\tif c.TestF == 0 {\n\t\tc.TestF = 1\n\t}\n}\n",
	}, "<!-- docgen:config pkg=p type=Config -->\n<!-- /docgen:config -->\n")
	wantIn(t, got,
		"| `Wait` | `time.Duration` | `DefaultWait` (250ms) |",
		"| `Lit` | `int` | `3` |",
		"| `Neg` | `int` | `-1` |",
		"| `Expr` | `time.Duration` | `2 * time.Second` (2s) |",
		"| `Var` | `int` | `hold` |",
		"| `Str` | `string` | `\"x\"` |",
		"| `Nil` | `*int` | `new(int)` |",
		"| `Leq` | `int` | `4` |",
		"| `Rev` | `int` | zero value |",
		"| `Sw` | `int` | `6` |",
		"| `SwTag` | `int` | zero value |",
		"| `Init` | `int` | `8` |",
		"| `Two` | `int` | `1` or `2` |",
		"| `VarCp` | `int` | `12` |",
		"| `KeyF` | `int` | `13` |",
		"| `CallF` | `int` | `11` |",
		"| `Shad` | `int` | zero value |",
		"| `Plus` | `int` | zero value |",
		"| `Pair` | `int` | zero value |",
		"| `InFor` | `int` | zero value |",
		"| `InFn` | `int` | zero value |",
		"| `Else` | `int` | zero value |",
		"| `Float` | `float64` | `1.5` |",
		"| `Named` | `Mode` | `ModeSlow` (2) |",
		"| `Nested` | `int` | `20` |",
		"| `Defl` | `int` | `10` |",
		"| `TestF` | `int` | zero value |",
		"| `Bool` | `bool` | zero value |",
		"| `Cond` | `int` | zero value |",
		"| `Len` | `string` | zero value |",
		"| `Assign` | `int` | `22` |",
		"| `False` | `bool` | zero value |",
		"| `Less` | `int` | zero value |",
		"| `Order` | `int` | `9` or `5` |",
	)
}

func TestConfigDefaultInAnotherPackage(t *testing.T) {
	t.Parallel()
	got := fill(t, map[string]string{
		"q/q.go": "package q\n\n// C holds settings.\ntype C struct{ A int }\n",
		"p/p.go": "package p\n\nimport \"example.com/w/q\"\n\n// Alias is q.C.\ntype Alias = q.C\n\n// Apply applies the default.\nfunc Apply(c *q.C) {\n\tif c.A == 0 {\n\t\tc.A = 1\n\t}\n}\n",
	}, "<!-- docgen:config pkg=q type=C -->\n<!-- /docgen:config -->\n\n<!-- docgen:config pkg=p type=Alias -->\n<!-- /docgen:config -->\n")
	if strings.Count(got, "| `A` | `int` | zero value |") != 1 || !strings.Contains(got, "| `A` | `int` | `1` |") {
		t.Errorf("page:\n%s", got)
	}
}

func TestConfigRows(t *testing.T) {
	t.Parallel()
	got := fill(t, map[string]string{
		"p/p.go": `package p

import "net/url"

// Inner is embedded.
type Inner struct {
	// IA is a field of Inner.
	IA int
}

type inner struct {
	// IB is a field of inner.
	IB int
}

// C holds settings.
type C struct {
	// Shown is exported.
	Shown  int
	hidden int
	Inner
	inner
	*url.URL
}
`,
	}, "<!-- docgen:config pkg=p type=C -->\n<!-- /docgen:config -->\n")
	wantIn(t, got, "| `Shown` | `int` |", "| `Inner` | `Inner` |", "| `URL` | `*url.URL` |")
	wantNotIn(t, got, "hidden", "`inner`", "IA", "IB")
}

func TestConfigTypes(t *testing.T) {
	t.Parallel()
	got := fill(t, map[string]string{
		"q/q.go": "package q\n\n// T is a type.\ntype T int\n\n// G is generic.\ntype G[X any] struct{ V X }\n",
		"p/p.go": `package p

import (
	"net/url"

	"example.com/w/q"
)

// Gen is generic.
type Gen[X any] struct{ V X }

// MyAlias is an alias.
type MyAlias = q.T

// C holds settings.
type C struct {
	Other q.T
	Al    MyAlias
	Ptr   *int
	Sl    []q.T
	Mp    map[string][]*url.URL
	Ch    chan<- int
	Fn    func(int) (string, error)
	Any   any
	Anon  struct{ X int ` + "`json:\"x\"`" + ` }
	GenF  q.G[string]
	Own   Gen[q.T]
	Arr   [3]byte
}
`,
	}, "<!-- docgen:config pkg=p type=C -->\n<!-- /docgen:config -->\n")
	wantIn(t, got,
		"| `Other` | `q.T` |",
		"| `Al` | `MyAlias` |",
		"| `Ptr` | `*int` |",
		"| `Sl` | `[]q.T` |",
		"| `Mp` | `map[string][]*url.URL` |",
		"| `Ch` | `chan<- int` |",
		"| `Fn` | `func(int) (string, error)` |",
		"| `Any` | `any` |",
		"| `Anon` | `struct{X int \"json:\\\"x\\\"\"}` |",
		"| `GenF` | `q.G[string]` |",
		"| `Own` | `Gen[q.T]` |",
		"| `Arr` | `[3]byte` |",
	)
}

func TestConfigDescription(t *testing.T) {
	t.Parallel()
	files := map[string]string{
		"q/q.go": "package q\n\n// C holds settings.\ntype C struct {\n\t// A is in q.\n\tA int\n}\n",
		"p/p.go": `package p

import "example.com/w/q"

// C holds settings.
type C struct {
	// Doc is a doc comment.
	Doc   int
	Trail int // Trail is a trailing comment.
	// Both has a doc.
	Both int // Both trailing.
	// Multi is line one.
	//
	// Line two | <b>html</b> and ` + "`code`" + ` and [Link].
	Multi string
	A, B  int // A and B share this.
	None  int
}

// Def is a defined type.
type Def C

// Al is an alias.
type Al = C

// QAl is an alias of a struct of another package.
type QAl = q.C
`,
	}
	got := fill(t, files, "<!-- docgen:config pkg=p type=C -->\n<!-- /docgen:config -->\n")
	wantIn(t, got,
		"| `Doc` | `int` | zero value | Doc is a doc comment. |",
		"| `Trail` | `int` | zero value | Trail is a trailing comment. |",
		"| `Both` | `int` | zero value | Both has a doc. |",
		"| `Multi` | `string` | zero value | Multi is line one. Line two \\| <b>html</b> and `code` and [Link]. |",
		"| `A` | `int` | zero value | A and B share this. |",
		"| `B` | `int` | zero value | A and B share this. |",
		"| `None` | `int` | zero value |  |",
	)
	for _, typ := range []string{"Def", "Al"} {
		wantIn(t, fill(t, files, "<!-- docgen:config pkg=p type="+typ+" -->\n<!-- /docgen:config -->\n"), "| `Trail` | `int` | zero value | Trail is a trailing comment. |")
	}
	wantIn(t, fill(t, files, "<!-- docgen:config pkg=p type=QAl -->\n<!-- /docgen:config -->\n"), "| `A` | `int` | zero value | A is in q. |")
}

func TestConfigNotMachineReadable(t *testing.T) {
	t.Parallel()
	cases := []struct {
		doc     string
		phrased bool
	}{
		{"0 means off.", true},
		{"nil means none.", true},
		{"Zero means off.", true},
		{"Empty means all.", true},
		{"A zero duration means off.", true},
		{"An empty list means all.", true},
		{"It Defaults to five.", true},
		{"On by default.", true},
		{"Zero\n\t// means off.", true},
		{"Zero value means off.", false},
		{"The empty value means off.", false},
		{"10 means ten.", false},
		{"The default is 5.", false},
	}
	var src strings.Builder
	rows := make([]string, 0, len(cases))
	src.WriteString("package p\n\n// C holds settings.\ntype C struct {\n")
	for i, c := range cases {
		name := "F" + string(rune('a'+i))
		src.WriteString("\t// " + c.doc + "\n\t" + name + " int\n")
		cell := "zero value"
		if c.phrased {
			cell = "not machine-readable; see the description"
		}
		rows = append(rows, "| `"+name+"` | `int` | "+cell+" |")
	}
	src.WriteString("\t// 0 means 7.\n\tFound int\n}\n\n// Use applies the default.\nfunc Use(c C) {\n\tif c.Found == 0 {\n\t\tc.Found = 7\n\t}\n}\n")
	got := fill(t, map[string]string{"p/p.go": src.String()}, "<!-- docgen:config pkg=p type=C -->\n<!-- /docgen:config -->\n")
	wantIn(t, got, rows...)
	wantIn(t, got, "| `Found` | `int` | `7` |")
}

func TestConfigPrintName(t *testing.T) {
	t.Parallel()
	got := fill(t, map[string]string{
		"p/p.go": `package p

import "time"

// Mode is a mode.
type Mode int

// The modes.
const (
	ModeOff Mode = iota
	ModeFast
	ModeSlow
)

// C holds settings.
type C struct {
	A Mode
	B Mode
	D int
	E Mode
	F int
	G time.Duration
	H Mode
}

var hold = 3

// New applies the defaults.
func New(c C) {
	if c.A == 0 {
		c.A = ModeFast + 1
	}
	if c.B == 0 {
		c.B = 2 * 3
	}
	if c.D == 0 {
		c.D = 255
	}
	if c.E == 0 {
		c.E = ModeFast
	}
	if c.F == 0 {
		c.F = hold
	}
	if c.G == 0 {
		c.G = 60 * time.Second
	}
	if c.H == 0 {
		c.H = 1
	}
}
`,
	}, "<!-- docgen:config pkg=p type=C print=name -->\n<!-- /docgen:config -->\n")
	wantIn(t, got,
		"| `A` | `Mode` | `ModeFast + 1` (ModeSlow) |",
		"| `B` | `Mode` | `2 * 3` |",
		"| `D` | `int` | `255` |",
		"| `E` | `Mode` | `ModeFast` |",
		"| `F` | `int` | `hold` |",
		"| `G` | `time.Duration` | `60 * time.Second` (time.Minute) |",
		"| `H` | `Mode` | `1` |",
	)
}

func TestConfigMarkerErrors(t *testing.T) {
	t.Parallel()
	src := "package p\n\n// G is generic.\ntype G[X any] struct{ V X }\n\n// N is an int.\ntype N int\n\n// E has no exported field.\ntype E struct{ x int }\n\n// S holds a string.\ntype S struct{ A string }\n\nfunc use(s S) {\n\tif s.A == \"\" {\n\t\ts.A = \"n\"\n\t}\n}\n"
	for args, want := range map[string]string{
		"pkg=p type=G":            "G is generic",
		"pkg=p type=N":            "N is not a struct",
		"pkg=p type=E":            "E has no exported field",
		"pkg=p type=Nope":         "Nope is not a type of example.com/w/p",
		"pkg=nope type=S":         "no package nope in module example.com/w",
		"pkg=p type=S print=nope": "print=nope is not go, raw, name or a fmt verb",
		"pkg=p type=S print=%d":   `the default "n": print=%d is not valid for the value: %!d(string=n)`,
		`pkg=p type=S print=""`:   "docgen:config: print= is empty",
	} {
		t.Run(args, func(t *testing.T) {
			t.Parallel()
			root := writeRepo(t, map[string]string{
				"go.mod":    "module example.com/w\n\ngo 1.24.0\n",
				"p/p.go":    src,
				"docs/p.md": "<!-- docgen:config " + args + " -->\n<!-- /docgen:config -->\n",
			})
			if rc, log := docgen(t, root, "check"); rc != 1 || !strings.Contains(log, want) {
				t.Errorf("rc %d, log:\n%s", rc, log)
			}
		})
	}
}

func TestConfigOfAGenericInstance(t *testing.T) {
	t.Parallel()
	got := fill(t, map[string]string{
		"p/p.go": "package p\n\n// G is generic.\ntype G[X comparable] struct{ V X }\n\n// A is an alias of an instance.\ntype A = G[int]\n\n// D is a defined type of an instance.\ntype D G[int]\n\nfunc use(g G[int]) {\n\tif g.V == 0 {\n\t\tg.V = 3\n\t}\n}\n",
	}, "<!-- docgen:config pkg=p type=A -->\n<!-- /docgen:config -->\n\n<!-- docgen:config pkg=p type=D -->\n<!-- /docgen:config -->\n")
	if strings.Count(got, "| `V` | `int` | `3` |") != 2 {
		t.Errorf("page:\n%s", got)
	}
}

func TestConfigDefaultsSectionExample(t *testing.T) {
	t.Parallel()
	got := fill(t, map[string]string{
		"p/p.go": `package p

import "time"

// DefaultWait is a wait.
const DefaultWait = 250 * time.Millisecond

// Config holds settings.
type Config struct{ Wait time.Duration }

// Lamp is a lamp.
type Lamp struct{ wait time.Duration }

// NewLamp returns a Lamp.
func NewLamp(c Config) *Lamp {
	wait := c.Wait
	if wait == 0 {
		wait = DefaultWait
	}
	return &Lamp{wait: wait}
}
`,
	}, "<!-- docgen:config pkg=p type=Config -->\n<!-- /docgen:config -->\n")
	wantIn(t, got, "| `Wait` | `time.Duration` | `DefaultWait` (250ms) |")
}
