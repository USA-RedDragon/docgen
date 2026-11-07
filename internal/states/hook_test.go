package states

import (
	"encoding/json"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/USA-RedDragon/docgen/internal/gocmd"
	"github.com/USA-RedDragon/docgen/internal/gomod"
	"github.com/USA-RedDragon/docgen/internal/load"
	"golang.org/x/tools/go/packages"
)

const hookMachines = `package p

import "example.com/w/slots"

// Level is a state with a signed value.
type Level int

// The levels.
const (
	Low Level = iota
	High
)

// Lamp has a State method.
type Lamp struct{ level Level }

// State returns the level.
func (l *Lamp) State() Level { return l.level }

// Raise moves the lamp up.
func (l *Lamp) Raise() { l.level = High }

// Gear is a state with an unsigned value.
type Gear uint8

// The gears.
const (
	Neutral Gear = iota
	First
)

// Box has an unsigned state.
type Box struct{ gear Gear }

// State returns the gear.
func (b *Box) State() Gear { return b.gear }

// Shift moves the gear.
func (b *Box) Shift() { b.gear = First }

// Power is the state of one slot.
type Power int

// The powers.
const (
	Dead Power = iota
	Live
)

// Repeater has one state for each slot of another package.
type Repeater struct{ slot [2]Power }

// SlotState returns the state of a slot.
func (r *Repeater) SlotState(s slots.Slot) Power { return r.slot[s] }

// Key turns a slot on.
func (r *Repeater) Key(s slots.Slot) { r.slot[s] = Live }

// Queue has no State method.
type Queue struct {
	open  bool
	items []int
}

// Open reports whether the queue is open.
func (q *Queue) Open() bool { return q.open }

// Peek returns the first item.
func (q *Queue) Peek() (int, bool) {
	if len(q.items) == 0 {
		return 0, false
	}
	return q.items[0], true
}

// Len returns the number of items.
func (q *Queue) Len() int { return len(q.items) }

// Push adds an item.
func (q *Queue) Push(v int) { q.items = append(q.items, v) }
`

const hookSlots = `package slots

// Slot is a time slot.
type Slot int

// The slots.
const (
	SlotA Slot = iota
	SlotB
)
`

func hookRecording(t *testing.T) (*load.Package, *recording, string) {
	t.Helper()
	root := t.TempDir()
	for name, text := range map[string]string{
		"go.mod":         "module example.com/w\n\ngo 1.24.0\n",
		"p/p.go":         hookMachines,
		"slots/slots.go": hookSlots,
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
	r, err := resolveAll(pi, map[string][]string{"Lamp": {""}, "Box": {""}, "Power": {""}, "Queue": {""}})
	if err != nil {
		t.Fatal(err)
	}
	if r.hook, err = r.hookSource(pi); err != nil {
		t.Fatal(err)
	}
	return pi, r, root
}

// overlayFiles writes the overlay of r and returns its files by the path that
// each replaces.
func overlayFiles(t *testing.T, pi *load.Package, r *recording) map[string][]byte {
	t.Helper()
	ovPath, err := r.writeOverlay(t.TempDir(), filepath.Dir(pi.Pkg.GoFiles[0]))
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(ovPath)
	if err != nil {
		t.Fatal(err)
	}
	var ov struct{ Replace map[string]string }
	if err := json.Unmarshal(b, &ov); err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{}
	for target, src := range ov.Replace {
		if files[target], err = os.ReadFile(src); err != nil {
			t.Fatal(err)
		}
	}
	return files
}

// TestHookTypeChecks type-checks the hook file with the instrumented sources of
// a package with each kind of machine, so a fault in the template fails here.
func TestHookTypeChecks(t *testing.T) {
	t.Parallel()
	pi, r, root := hookRecording(t)
	if _, err := parser.ParseFile(token.NewFileSet(), hookFile, r.hook, parser.SkipObjectResolution); err != nil {
		t.Fatalf("parse: %v\n%s", err, r.hook)
	}
	session := &gocmd.Session{}
	defer session.Close()
	env, err := session.Env(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	ps, err := packages.Load(&packages.Config{
		Context: t.Context(),
		Mode:    packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles | packages.NeedImports | packages.NeedDeps | packages.NeedTypes | packages.NeedSyntax | packages.NeedTypesInfo,
		Dir:     root,
		Env:     env,
		Overlay: overlayFiles(t, pi, r),
	}, "example.com/w/p")
	if err != nil {
		t.Fatal(err)
	}
	if len(ps) != 1 {
		t.Fatalf("%d packages, want 1", len(ps))
	}
	p := ps[0]
	for _, e := range p.Errors {
		t.Errorf("type check: %v", e)
	}
	if t.Failed() {
		t.Fatalf("hook source:\n%s", r.hook)
	}
	if !slices.ContainsFunc(p.CompiledGoFiles, func(f string) bool { return filepath.Base(f) == hookFile }) {
		t.Fatalf("%s is not in %q", hookFile, p.CompiledGoFiles)
	}
	for _, typ := range []string{"Lamp", "Box", "Repeater", "Queue"} {
		for _, fn := range []string{"__dgIn", "__dgOut", "__dgDown", "__dgState", "__dgFmt", "__dgSeen"} {
			if p.Types.Scope().Lookup(fn+typ) == nil {
				t.Errorf("no %s%s in the hook", fn, typ)
			}
		}
	}
}

func TestHookValues(t *testing.T) {
	t.Parallel()
	_, r, _ := hookRecording(t)
	src := string(r.hook)
	for _, want := range []string{
		"package p\n",
		`__dgp0 "example.com/w/slots"`,
		"type __dgCompsLamp = [1]uint64",
		"type __dgCompsRepeater = [2]uint64",
		`var __dgViewsRepeater = __dgViewNamesRepeater{"Power", "Power"}`,
		"__dgCompsRepeater{uint64(r.SlotState(__dgp0.SlotA)), uint64(r.SlotState(__dgp0.SlotB))}",
		"case 1:\n\t\tb = __dgstrconv.AppendInt(b, int64(v), 10)\n",
		"b = __dgstrconv.AppendUint(b, v, 10)",
		"__dgstrconv.AppendBool(b, v>>1&1 != 0)",
	} {
		if !strings.Contains(src, want) {
			t.Errorf("no %q in the hook:\n%s", want, src)
		}
	}
	for _, bad := range []string{"{{", "<no value>"} {
		if strings.Contains(src, bad) {
			t.Errorf("%q in the hook", bad)
		}
	}
}

func TestHookStable(t *testing.T) {
	t.Parallel()
	d := hookData{
		Package:  "p",
		Imports:  []hookImport{{Alias: "__dgp0", Path: "example.com/a"}},
		Machines: []hookMachine{{Type: "T", Comps: []hookComp{{View: "T", Value: "uint64(r.State())", Format: "__dgstrconv.AppendInt(b, int64(v), 10)"}}}},
	}
	first, err := renderHook(d)
	if err != nil {
		t.Fatal(err)
	}
	second, err := renderHook(d)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Error("two renders differ")
	}
}
