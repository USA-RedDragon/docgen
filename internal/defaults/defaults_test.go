package defaults

import (
	"go/types"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/USA-RedDragon/docgen/internal/gocmd"
	"github.com/USA-RedDragon/docgen/internal/gomod"
	"github.com/USA-RedDragon/docgen/internal/goprint"
	"github.com/USA-RedDragon/docgen/internal/load"
)

const configSource = `package p

import "time"

const DefaultWait = 30 * time.Second

type Config struct {
	Wait    time.Duration
	Name    string
	Handler func()
	Ratio   float64
	Local   int
	Var     int
	Param   int
	Key     int
	Paren   int
	Below   int

	CaseA int
	CaseB string
	Multi int
	Both  int
	Same  int
	Nested int

	NoZero  int
	NotEq   int
	Tagged  int
	Closure int
	Looped  int
	Ranged  int
	Other   int
	Defined int
	Pair    int
	Never   int
	Bool    bool
}

type inner struct{ Size int }

func noop() {}

func set(limit int) int {
	if limit == 0 {
		limit = 7
	}
	return limit
}

func (c *Config) Apply(on bool, n []int) {
	if c.Wait == 0 {
		c.Wait = DefaultWait
	}
	if c.Name == "" {
		c.Name = "lamp"
	}
	if c.Handler == nil {
		c.Handler = noop
	}
	if c.Ratio == 0 {
		c.Ratio = 0.5
	}
	local := c.Local
	if local == 0 {
		local = 3
	}
	var v = c.Var
	if v == 0 {
		v = 4
	}
	_ = set(c.Param)
	i := inner{Size: c.Key}
	if i.Size == 0 {
		i.Size = 9
	}
	if (c.Paren == 0) {
		c.Paren = (6)
	}
	if c.Below <= 0 {
		c.Below = 8
	}

	switch {
	case c.CaseA == 0:
		c.CaseA = 2
	case c.CaseB == "", on:
		c.CaseB = "b"
	case on:
		c.NoZero = 1
	}
	switch {
	case c.Multi == 0:
		c.Multi = 1
	default:
		c.Multi = 2
	}
	if c.Both == 0 {
		if on {
			c.Both = 1
		} else {
			c.Both = 2
		}
	}
	if c.Same == 0 {
		if on {
			c.Same = 1
		} else {
			c.Same = 1
		}
	}
	if on {
		if c.Nested == 0 {
			c.Nested = 5
		}
	}

	if c.NoZero == 5 {
		c.NoZero = 1
	}
	if c.NotEq != 0 {
		c.NotEq = 1
	}
	switch c.Tagged {
	case 0:
		c.Tagged = 1
	}
	if c.Closure == 0 {
		func() { c.Closure = 1 }()
	}
	if c.Looped == 0 {
		for range 2 {
			c.Looped = 1
		}
	}
	if c.Ranged == 0 {
		for range n {
			c.Ranged = 1
		}
	}
	if c.Other == 0 {
		c.Pair = 1
	}
	if c.Defined == 0 {
		d := 1
		_ = d
	}
	if c.Pair == 0 {
		c.Pair, c.Other = 1, 2
	}
	if !c.Bool {
		c.Bool = true
	}
}
`

const configTest = `package p

func tested(c *Config) {
	if c.Never == 0 {
		c.Never = 1
	}
}
`

func find(t *testing.T) map[string][]string {
	t.Helper()
	root := t.TempDir()
	for name, text := range map[string]string{
		"go.mod":      "module example.com/w\n\ngo 1.24.0\n",
		"p/p.go":      configSource,
		"p/p_test.go": configTest,
	} {
		p := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	m, err := gomod.Open(root, root)
	if err != nil {
		t.Fatal(err)
	}
	session := &gocmd.Session{}
	defer session.Close()
	set, err := load.Packages(t.Context(), m, session, map[string]bool{"example.com/w/p": true})
	if err != nil {
		t.Fatal(err)
	}
	pi, err := set.Package("p")
	if err != nil {
		t.Fatal(err)
	}
	st, ok := pi.Pkg.Types.Scope().Lookup("Config").Type().Underlying().(*types.Struct)
	if !ok {
		t.Fatal("Config is not a struct")
	}
	out := map[string][]string{}
	for f, ds := range Find(pi, st) {
		for _, d := range ds {
			out[f.Name()] = append(out[f.Name()], text(t, d))
		}
	}
	return out
}

func text(t *testing.T, d Default) string {
	t.Helper()
	src := "`" + d.Source + "`"
	if d.Value == nil || d.Literal {
		return src
	}
	v, err := goprint.Print(d.Type, d.Value, "")
	if err != nil {
		t.Fatal(err)
	}
	if v == d.Source {
		return src
	}
	return src + " (" + v + ")"
}

func TestFind(t *testing.T) {
	t.Parallel()
	got := find(t)
	want := map[string][]string{
		"Wait":    {"`DefaultWait` (30s)"},
		"Name":    {"`\"lamp\"`"},
		"Handler": {"`noop`"},
		"Ratio":   {"`0.5`"},
		"Local":   {"`3`"},
		"Var":     {"`4`"},
		"Param":   {"`7`"},
		"Key":     {"`9`"},
		"Paren":   {"`(6)` (6)"},
		"Below":   {"`8`"},
		"CaseA":   {"`2`"},
		"CaseB":   {"`\"b\"`"},
		"Multi":   {"`1`"},
		"Both":    {"`1`", "`2`"},
		"Same":    {"`1`"},
		"Nested":  {"`5`"},
	}
	for name, w := range want {
		if !slices.Equal(got[name], w) {
			t.Errorf("%s = %q, want %q", name, got[name], w)
		}
	}
	for name, g := range got {
		if _, ok := want[name]; !ok {
			t.Errorf("%s = %q, want no entry", name, g)
		}
	}
}
