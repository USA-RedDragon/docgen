package states

import (
	"go/constant"
	gotypes "go/types"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/USA-RedDragon/docgen/internal/types"
)

func constOf(pkg *gotypes.Package, name string) types.Const {
	return types.Const{Name: name, Type: gotypes.Typ[gotypes.Int], Value: constant.MakeInt64(1), Pkg: pkg}
}

func TestQualify(t *testing.T) {
	t.Parallel()
	here := gotypes.NewPackage("example.com/m/here", "here")
	one := gotypes.NewPackage("example.com/m/one", "slots")
	two := gotypes.NewPackage("example.com/other/one", "slots")
	im := &imports{here: here, alias: map[string]string{}}
	for _, c := range []struct {
		pkg  *gotypes.Package
		name string
		want string
	}{
		{here, "Local", "Local"},
		{one, "A", "__dgp0.A"},
		{one, "B", "__dgp0.B"},
		{two, "A", "__dgp1.A"},
		{here, "Again", "Again"},
		{one, "C", "__dgp0.C"},
	} {
		if got := im.qualify(constOf(c.pkg, c.name)); got != c.want {
			t.Errorf("qualify(%s.%s) = %q, want %q", c.pkg.Path(), c.name, got, c.want)
		}
	}
	want := map[string]string{"example.com/m/one": "__dgp0", "example.com/other/one": "__dgp1"}
	if len(im.alias) != len(want) {
		t.Errorf("aliases = %v, want %v", im.alias, want)
	}
	for p, a := range want {
		if im.alias[p] != a {
			t.Errorf("alias of %s = %q, want %q", p, im.alias[p], a)
		}
	}
}

func TestObserverName(t *testing.T) {
	t.Parallel()
	m := &machine{observers: []observer{{"Open", obsBool}, {"Peek", obsOK}, {"Len", obsLen}}}
	for key, want := range map[string]string{
		"true,true,true":    "Open() true, Peek() ok, Len() > 0",
		"false,false,false": "Open() false, Peek() not ok, Len() 0",
		"true":              "Open() true, Peek() not ok, Len() 0",
	} {
		if got := observerName(m, key); got != want {
			t.Errorf("observerName(%q) = %q, want %q", key, got, want)
		}
	}
}

func recordMachine(name string) *machine {
	return &machine{name: name, typ: name, initial: map[string]bool{}, edges: map[[2]string]map[string]bool{}}
}

func TestReadRecord(t *testing.T) {
	t.Parallel()
	rec := filepath.Join(t.TempDir(), "states.txt")
	text := strings.Join([]string{
		"init\tLamp\t0",
		"edge\tLamp\t0\tStart\t1",
		"edge\tLamp\t0\tKick\t1",
		"edge\tLamp\t1\tStop\t0",
		"init\tOther\t9",
		"edge\tOther\t9\tGo\t8",
		"init\tLamp",
		"init\tLamp\t1\textra",
		"edge\tLamp\t1\tShort",
		"note\tLamp\t7",
		"",
		"",
	}, "\n")
	if err := os.WriteFile(rec, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	m := recordMachine("Lamp")
	if err := readRecord(rec, &stateRun{machines: map[string]*machine{"Lamp": m}}); err != nil {
		t.Fatal(err)
	}
	if len(m.initial) != 1 || !m.initial["0"] {
		t.Errorf("initial = %v", m.initial)
	}
	if len(m.edges) != 2 {
		t.Errorf("edges = %v", m.edges)
	}
	if got := m.edges[[2]string{"0", "1"}]; !got["Start"] || !got["Kick"] || len(got) != 2 {
		t.Errorf("edge 0 to 1 = %v", got)
	}
	if got := m.edges[[2]string{"1", "0"}]; !got["Stop"] || len(got) != 1 {
		t.Errorf("edge 1 to 0 = %v", got)
	}
}

func TestReadRecordMissing(t *testing.T) {
	t.Parallel()
	err := readRecord(filepath.Join(t.TempDir(), "none"), &stateRun{})
	if err == nil || !strings.Contains(err.Error(), "the tests recorded no state") {
		t.Errorf("err = %v", err)
	}
}

func TestRecvName(t *testing.T) {
	t.Parallel()
	got := methodsOf(t, `package p
type T struct{}
type G[X any] struct{}
func (t *T) Exported() {}
func (t *T) hidden() {}
func (t T) Value() {}
func (g *G[X]) Generic() {}
func Plain() {}
`)
	if want := []string{"Exported:T", "Generic:", "Plain:", "Value:T", "hidden:"}; !slices.Equal(got, want) {
		t.Errorf("receivers = %q, want %q", got, want)
	}
}
