package cli

import (
	"go/build"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const stateSource = `package p

// S is a state.
type S int

const (
	// Off gives no light.
	Off S = iota
	// On gives light.
	On
)
`

func page(t *testing.T, root, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, "docs", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// fill writes files in a new module, with the marker text as docs/p.md, and
// returns the page after update. A check after the update must pass.
func fill(t *testing.T, files map[string]string, markers string) string {
	t.Helper()
	root := t.TempDir()
	all := map[string]string{"go.mod": "module example.com/w\n\ngo 1.24.0\n", "docs/p.md": markers}
	maps.Copy(all, files)
	writeTree(t, root, all)
	if rc, log := docgen(t, root, "update"); rc != 0 {
		t.Fatalf("update: rc %d, log:\n%s", rc, log)
	}
	if rc, log := docgen(t, root, "check"); rc != 0 {
		t.Fatalf("check: rc %d, log:\n%s", rc, log)
	}
	return page(t, root, "p.md")
}

func wantIn(t *testing.T, text string, want ...string) {
	t.Helper()
	for _, w := range want {
		if !strings.Contains(text, w) {
			t.Errorf("no %q in:\n%s", w, text)
		}
	}
}

func wantNotIn(t *testing.T, text string, bad ...string) {
	t.Helper()
	for _, b := range bad {
		if strings.Contains(text, b) {
			t.Errorf("%q in:\n%s", b, text)
		}
	}
}

func TestStateMethodCallsExportedMethod(t *testing.T) {
	t.Parallel()
	got := fill(t, map[string]string{
		"p/p.go": `package p

// S is a state.
type S int

const (
	// Zero is no count.
	Zero S = iota
	// One is a count of one.
	One
	// Two is a count of two.
	Two
	// Three is a higher count.
	Three
)

// M counts.
type M struct{ n int }

// Count returns the count.
func (m *M) Count() int { return m.n }

// State returns the state.
func (m *M) State() S {
	if m.Count() > 2 {
		return Three
	}
	return S(m.Count())
}

// Add adds one.
func (m *M) Add() { m.n++ }

// AddTwo adds two.
func (m *M) AddTwo() { m.Add(); m.Add() }

// Clear clears the count.
func (m *M) Clear() { m.n = 0 }
`,
		"p/p_test.go": `package p

import (
	"testing"
	"time"
)

func TestM(t *testing.T) {
	defer time.AfterFunc(30*time.Second, func() { panic("deadlock") }).Stop()
	var m M
	m.Add()
	m.AddTwo()
	n := testing.AllocsPerRun(10, func() {
		m.Clear()
		m.AddTwo()
	})
	if n != 0 {
		t.Errorf("%v allocations per run", n)
	}
}
`,
	}, "<!-- docgen:states pkg=p type=M -->\n<!-- /docgen:states -->\n")
	wantIn(t, got, "| Zero | `Add` | One |", "| One | `AddTwo` | Three |")
	wantNotIn(t, got, "| One | `Add` | Two |", "`Count`")
}

func TestFloatConstants(t *testing.T) {
	t.Parallel()
	want := map[string]string{
		"Rate":    "0.5",
		"Pi":      "3.14",
		"Single":  "0.1",
		"Third":   "0.3333333333333333",
		"Whole":   "2",
		"Million": "1e+06",
		"Large":   "1e+100",
		"Small":   "1e-07",
		"Below":   "-0.25",
		"Beyond":  "1e+1000",
		"Complex": "(1.5+2i)",
		"Var":     "0.25",
		"Count":   "7",
		"Wide":    "1180591620717411303424",
		"Text":    "a b",
		"Flag":    "true",
	}
	var markers strings.Builder
	for name := range want {
		markers.WriteString(name + " is <!-- docgen:const pkg=p name=" + name + " --><!-- /docgen:const -->\n")
	}
	markers.WriteString("<!-- docgen:config pkg=p type=C -->\n<!-- /docgen:config -->\n")
	got := fill(t, map[string]string{
		"p/p.go": `package p

const (
	Rate            = 0.5
	Pi      float64 = 3.14
	Single  float32 = 0.1
	Third           = 1.0 / 3
	Whole           = 2.0
	Million         = 1000000.0
	Large           = 1e100
	Small           = 1e-7
	Below           = -0.25
	Beyond          = 1e1000
	Complex         = 1.5 + 2i
	Count           = 7
	Wide            = 1 << 70
	Text            = "a b"
	Flag            = true
)

var Var = 0.25

const half = 0.5

// C is a config.
type C struct {
	// F is a ratio.
	F float64
	// G is a ratio.
	G float64
}

// Use applies the defaults.
func Use(c C) float64 {
	f := c.F
	if f == 0 {
		f = 0.1
	}
	if c.G == 0 {
		c.G = half
	}
	return f + c.G
}
`,
	}, markers.String())
	for name, v := range want {
		wantIn(t, got, name+" is <!-- docgen:const pkg=p name="+name+" -->`"+v+"`<!--")
	}
	wantIn(t, got, "| `F` | `float64` | `0.1` | F is a ratio. |", "| `G` | `float64` | `half` (0.5) | G is a ratio. |")
}

func TestCgoPackage(t *testing.T) {
	t.Parallel()
	if !build.Default.CgoEnabled {
		t.Skip("cgo is off")
	}
	if _, err := exec.LookPath("gcc"); err != nil {
		t.Skip("no C compiler")
	}
	got := fill(t, map[string]string{
		"p/p.go": `// Package p calls C.
package p

// int one(void) { return 1; }
import "C"

// S is a state.
type S int

const (
	// Off gives no light.
	Off S = iota
	// On gives light.
	On
)

// A is a number.
const A = 1

// M is a machine.
type M struct{ s S }

// State returns the state.
func (m *M) State() S { return m.s }

// Go turns it on.
func (m *M) Go() { m.s = S(C.one()) }
`,
		"p/p_test.go": "package p\n\nimport \"testing\"\n\nfunc TestM(t *testing.T) { m := &M{}; m.Go() }\n",
		"q/q.go":      "package q\n\n// B is a number.\nconst B = 2\n",
	}, "<!-- docgen:const pkg=p name=A --><!-- /docgen:const -->\n"+
		"<!-- docgen:const pkg=q name=B --><!-- /docgen:const -->\n"+
		"<!-- docgen:doc pkg=p -->\n<!-- /docgen:doc -->\n"+
		"<!-- docgen:doc pkg=p name=M.Go -->\n<!-- /docgen:doc -->\n"+
		"<!-- docgen:states pkg=p type=M -->\n<!-- /docgen:states -->\n")
	wantIn(t, got, "`1`", "`2`", "Package p calls C.", "Go turns it on.", "Off gives no light.", "| Off | `Go` | On |")
}

func TestBrokenPackageStillChecksOtherPages(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"go.mod":    "module example.com/w\n\ngo 1.24.0\n",
		"p/p.go":    "package p\n\nconst A = 1\n",
		"q/q.go":    "package q\n\nconst B = nope\n",
		"docs/p.md": "<!-- docgen:const pkg=p name=A --><!-- /docgen:const -->\n",
		"docs/q.md": "<!-- docgen:const pkg=q name=B --><!-- /docgen:const -->\n",
	})
	if rc, log := docgen(t, root, "check"); rc != 1 || !strings.Contains(log, "docs/q.md:1") || !strings.Contains(log, "stale: docs/p.md:1") {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
	if rc, log := docgen(t, root, "update"); rc != 1 || !strings.Contains(log, "docs/q.md:1") {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
	if got := page(t, root, "p.md"); strings.Contains(got, "`1`") {
		t.Errorf("update wrote a page after a failure:\n%s", got)
	}
}

func TestStateWithNoExportedConstant(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"go.mod": "module example.com/w\n\ngo 1.24.0\n",
		"p/p.go": `package p

// S is the state.
type S int

const (
	idle S = iota
	busy
	done
	failed
)

// M is a machine.
type M struct{ s S }

// State returns the state.
func (m *M) State() S { return m.s }

// A starts.
func (m *M) A() { m.s = busy }

// B ends.
func (m *M) B() { m.s = done }

// C fails.
func (m *M) C() { m.s = failed }

// D resets.
func (m *M) D() { m.s = idle }
`,
		"p/p_test.go": "package p\n\nimport \"testing\"\n\nfunc TestM(t *testing.T) { m := &M{}; m.A(); m.B(); m.C(); m.D() }\n",
		"docs/p.md":   "<!-- docgen:states pkg=p type=M -->\n<!-- /docgen:states -->\n",
	})
	if rc, log := docgen(t, root, "update"); rc != 0 {
		t.Fatalf("update: rc %d, log:\n%s", rc, log)
	}
	for range 5 {
		if rc, log := docgen(t, root, "check"); rc != 0 {
			t.Fatalf("check: rc %d, log:\n%s", rc, log)
		}
	}
	wantIn(t, page(t, root, "p.md"),
		"| value 0 |  | yes, first state |\n| value 1 |  | yes |\n| value 2 |  | yes |\n| value 3 |  | yes |\n",
		"| value 0 | `A` | value 1 |\n| value 1 | `B` | value 2 |\n| value 2 | `C` | value 3 |\n| value 3 | `D` | value 0 |\n")
}

func TestUnlistedStateValue(t *testing.T) {
	t.Parallel()
	got := fill(t, map[string]string{
		"p/p.go": `package p

// S is a state.
type S int

const (
	// Bad is an error.
	Bad S = -1
	// Off gives no light.
	Off S = 0
	// On gives light.
	On S = 1
)

// M is a machine.
type M struct{ s S }

// State returns the state.
func (m *M) State() S { return m.s }

// A turns it on.
func (m *M) A() { m.s = On }

// B breaks it.
func (m *M) B() { m.s = Bad }

// C moves past the constants.
func (m *M) C() { m.s = 7 }

// D moves to a negative value with no constant.
func (m *M) D() { m.s = -5 }
`,
		"p/p_test.go": "package p\n\nimport \"testing\"\n\nfunc TestM(t *testing.T) { m := &M{}; m.A(); m.B(); m.C(); m.D() }\n",
	}, "<!-- docgen:states pkg=p type=M -->\n<!-- /docgen:states -->\n")
	wantIn(t, got,
		"| value -5 |  | yes |\n| [`Bad`](https://pkg.go.dev/example.com/w/p#Bad) | Bad is an error. | yes |\n| [`Off`]",
		"| value 7 |  | yes |\n\nTransitions",
		"| Bad | `C` | value 7 |\n| Off | `A` | On |\n| On | `B` | Bad |\n| value 7 | `D` | value -5 |\n")
	wantNotIn(t, got, "18446744073709551")
}

func TestPackageNamesOfTheHook(t *testing.T) {
	t.Parallel()
	got := fill(t, map[string]string{
		"p/p.go": stateSource + `
// M is a machine.
type M struct{ s S }

func sync() {}

var os = 1

const strconv = 2

// State returns the state.
func (m *M) State() S { return m.s }

// Go turns it on.
func (m *M) Go() { sync(); m.s = S(os + strconv - 2) }
`,
		"p/p_test.go": "package p\n\nimport \"testing\"\n\nfunc TestM(t *testing.T) { m := &M{}; m.Go() }\n",
	}, "<!-- docgen:states pkg=p type=M -->\n<!-- /docgen:states -->\n")
	wantIn(t, got, "| Off | `Go` | On |")
}

const twoMachines = stateSource + `
// A is a machine.
type A struct{ s S }

// State returns the state.
func (m *A) State() S { return m.s }

// Go turns it on.
func (m *A) Go() { m.s = On }

// Back turns it off.
func (m *A) Back() { m.s = Off }

// B is a machine.
type B struct{ s S }

// State returns the state.
func (m *B) State() S { return m.s }

// Go turns it on.
func (m *B) Go() { m.s = On }

// Back turns it off.
func (m *B) Back() { m.s = Off }
`

const twoMachinesTest = `package p

import "testing"

func TestA(t *testing.T)  { m := &A{}; m.Go(); m.Back() }
func TestB1(t *testing.T) { m := &B{}; m.Go() }
func TestB2(t *testing.T) { m := &B{}; m.Go(); m.Back() }
`

func TestRunOfOneMachine(t *testing.T) {
	t.Parallel()
	files := map[string]string{"p/p.go": twoMachines, "p/p_test.go": twoMachinesTest}
	got := fill(t, files, "<!-- docgen:states pkg=p type=A run=TestA|TestB1 -->\n<!-- /docgen:states -->\n"+
		"<!-- docgen:states pkg=p type=B -->\n<!-- /docgen:states -->\n")
	if n := strings.Count(got, "| On | `Back` | Off |"); n != 2 {
		t.Errorf("%d machines with the transition of TestB2, want 2:\n%s", n, got)
	}
	got = fill(t, files, "<!-- docgen:states pkg=p type=A run=TestA -->\n<!-- /docgen:states -->\n"+
		"<!-- docgen:states pkg=p type=B run=TestB1 -->\n<!-- /docgen:states -->\n")
	if n := strings.Count(got, "| On | `Back` | Off |"); n != 1 {
		t.Errorf("%d machines with the transition of TestA, want 1:\n%s", n, got)
	}
	limited := "<!-- docgen:states pkg=p type=B run=TestB1 -->\n<!-- /docgen:states -->\n"
	open := "<!-- docgen:states pkg=p type=B -->\n<!-- /docgen:states -->\n"
	for _, markers := range []string{limited + open, open + limited} {
		got = fill(t, files, markers)
		if n := strings.Count(got, "| On | `Back` | Off |"); n != 2 {
			t.Errorf("%d markers with the transition of TestB2, want 2:\n%s", n, got)
		}
	}
}

const errorSource = `// Package svc serves.
package svc

import "example.com/w/store"

// S serves.
type S struct{}

// Get gets. It returns store.ErrMissing when there is no value.
func (s *S) Get() error { return store.ErrMissing }
`

func TestErrorsOfAnImportedPackage(t *testing.T) {
	t.Parallel()
	files := map[string]string{
		"svc/s.go":   errorSource,
		"store/s.go": "// Package store stores.\npackage store\n\nimport \"errors\"\n\n// ErrMissing is missing.\nvar ErrMissing = errors.New(\"missing\")\n\n// Other is a number.\nconst Other = 1\n",
	}
	marker := "<!-- docgen:errors pkg=svc type=S -->\n<!-- /docgen:errors -->\n"
	alone := fill(t, files, marker)
	wantIn(t, alone, "| [`store.ErrMissing`](https://pkg.go.dev/example.com/w/store#ErrMissing) | `S.Get` | ErrMissing is missing. |")
	withOther := fill(t, files, marker+"<!-- docgen:const pkg=store name=Other --><!-- /docgen:const -->\n")
	if !strings.HasPrefix(withOther, alone) {
		t.Errorf("the table depends on another marker:\n%s\n%s", alone, withOther)
	}
}

func TestErrorsOfTwoPackagesWithOneName(t *testing.T) {
	t.Parallel()
	store := "// Package store stores.\npackage store\n\nimport \"errors\"\n\n// N is a number.\nconst N = 1\n\n// ErrMissing is missing here.\nvar ErrMissing = errors.New(\"missing\")\n"
	got := fill(t, map[string]string{
		"svc/s.go":       errorSource,
		"store/s.go":     strings.Replace(store, "import \"errors\"", "import (\n\t\"errors\"\n\n\t_ \"example.com/w/old/store\"\n)", 1),
		"old/store/s.go": strings.Replace(store, "here", "there", 1),
	}, "<!-- docgen:errors pkg=svc type=S -->\n<!-- /docgen:errors -->\n"+
		"<!-- docgen:const pkg=old/store name=N --><!-- /docgen:const -->\n")
	wantIn(t, got, "https://pkg.go.dev/example.com/w/store#ErrMissing", "ErrMissing is missing here.")
}

func TestVendoredPackageIsNotAMatch(t *testing.T) {
	t.Parallel()
	got := fill(t, map[string]string{
		"internal/session/s.go":               "package session\n\n// L is local.\nconst L = 1\n",
		"vendor/example.com/dep/session/s.go": "package session\n\nconst V = 2\n",
		"vendor/modules.txt":                  "",
	}, "<!-- docgen:const pkg=session name=L --><!-- /docgen:const -->\n")
	wantIn(t, got, "`1`")
}

func TestPkgWithDotSlash(t *testing.T) {
	t.Parallel()
	got := fill(t, map[string]string{
		"p/p.go":      "// Package p is a package.\npackage p\n\n// A is a number.\nconst A = 1\n",
		"p/q/q.go":    "// Package q is a package.\npackage q\n\n// B is a number.\nconst B = 2\n",
		"p/p_test.go": "package p\n\nimport \"fmt\"\n\nfunc Example() {\n\tfmt.Println(\"hi\")\n\t// Output: hi\n}\n",
	}, "<!-- docgen:const pkg=./p name=A --><!-- /docgen:const -->\n"+
		"<!-- docgen:const pkg=./p/q/ name=B --><!-- /docgen:const -->\n"+
		"<!-- docgen:doc pkg=./p -->\n<!-- /docgen:doc -->\n"+
		"<!-- docgen:output test=./p.Example -->\n<!-- /docgen:output -->\n")
	wantIn(t, got, "`1`", "`2`", "Package p is a package.", "hi")
}

func TestStateTypeOfAnotherPackage(t *testing.T) {
	t.Parallel()
	got := fill(t, map[string]string{
		"p/p.go": `package p

import "example.com/w/q"

// S has the name of the state type and is not the state type.
type S bool

// Off is not a state.
const Off S = false

// M is a machine.
type M struct{ s q.S }

// State returns the state.
func (m *M) State() q.S { return m.s }

// Go turns it on.
func (m *M) Go() { m.s = q.On }
`,
		"p/p_test.go": "package p\n\nimport \"testing\"\n\nfunc TestM(t *testing.T) { m := &M{}; m.Go() }\n",
		"q/q.go":      strings.Replace(stateSource, "package p", "package q", 1),
	}, "<!-- docgen:states pkg=p type=M -->\n<!-- /docgen:states -->\n")
	wantIn(t, got,
		"| [`Off`](https://pkg.go.dev/example.com/w/q#Off) | Off gives no light. | yes, first state |",
		"| [`On`](https://pkg.go.dev/example.com/w/q#On) | On gives light. | yes |",
		"[`M`](https://pkg.go.dev/example.com/w/p#M)")
}

func TestStateTypeOfTheStandardLibrary(t *testing.T) {
	t.Parallel()
	got := fill(t, map[string]string{
		"p/p.go": `package p

import "net/http"

// M is a machine.
type M struct{ s http.ConnState }

// State returns the state.
func (m *M) State() http.ConnState { return m.s }

// Go moves on.
func (m *M) Go() { m.s = http.StateActive }
`,
		"p/p_test.go": "package p\n\nimport \"testing\"\n\nfunc TestM(t *testing.T) { m := &M{}; m.Go() }\n",
	}, "<!-- docgen:states pkg=p type=M -->\n<!-- /docgen:states -->\n")
	wantIn(t, got, "[`StateActive`](https://pkg.go.dev/net/http#StateActive) | StateActive represents")
}

func TestDirectoryNamedLikeATestPackage(t *testing.T) {
	t.Parallel()
	got := fill(t, map[string]string{
		"load_test/l.go":      "package load\n\n// X is a number.\nconst X = 1\n",
		"load_test/l_test.go": "package load_test\n\nimport \"testing\"\n\nfunc TestX(t *testing.T) {}\n",
		"load/l.go":           "package load\n\n// Y is a number.\nconst Y = 2\n",
	}, "<!-- docgen:const pkg=load_test name=X --><!-- /docgen:const -->\n"+
		"<!-- docgen:const pkg=load name=Y --><!-- /docgen:const -->\n")
	wantIn(t, got, "`1`", "`2`")
}

func TestModulePathEndingInDotTest(t *testing.T) {
	t.Parallel()
	got := fill(t, map[string]string{
		"go.mod":    "module example.test\n\ngo 1.24.0\n",
		"r.go":      "package root\n\n// X is a number.\nconst X = 1\n",
		"r_test.go": "package root\n\nimport \"testing\"\n\nfunc TestX(t *testing.T) {}\n",
	}, "<!-- docgen:const pkg=. name=X --><!-- /docgen:const -->\n")
	wantIn(t, got, "`1`")
}

func TestRootPackageBuildConstraints(t *testing.T) {
	t.Parallel()
	got := fill(t, map[string]string{
		"tools/gen/go.mod":  "module example.com/gen\n\ngo 1.24.0\n",
		"tools/gen/main.go": "// Command gen.\npackage main\n\nfunc main() {}\n",
		"tools/gen/h.go":    "//go:build ignore\n\n// Helper.\npackage other\n",
	}, "<!-- docgen:doc pkg=tools/gen -->\n<!-- /docgen:doc -->\n")
	wantIn(t, got, "Command gen.")
	wantNotIn(t, got, "Helper.")
}

func TestRootPackageWithNoBuildableFile(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"go.mod":           "module example.com/w\n\ngo 1.24.0\n",
		"tools/gen/go.mod": "module example.com/gen\n\ngo 1.24.0\n",
		"tools/gen/h.go":   "//go:build ignore\n\n// Helper.\npackage other\n",
		"docs/p.md":        "<!-- docgen:doc pkg=tools/gen -->\n<!-- /docgen:doc -->\n",
	})
	if rc, log := docgen(t, root, "update"); rc != 1 || !strings.Contains(log, "no buildable Go file") {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
}

func TestDurationAlias(t *testing.T) {
	t.Parallel()
	got := fill(t, map[string]string{
		"p/p.go": `package p

import "time"

// D is another name of time.Duration.
type D = time.Duration

// E is a new type.
type E time.Duration

const (
	// A is a D.
	A D = 5 * time.Second
	// B is an E.
	B E = E(5 * time.Second)
)
`,
	}, "A <!-- docgen:const pkg=p name=A --><!-- /docgen:const -->\nB <!-- docgen:const pkg=p name=B --><!-- /docgen:const -->\n")
	wantIn(t, got, "`5s`", "`5000000000`")
}

func TestConfigFieldWithTrailingComment(t *testing.T) {
	t.Parallel()
	got := fill(t, map[string]string{
		"p/p.go": `package p

// C is a config.
type C struct {
	A, B int // A and B are sizes.
	// D is above.
	D int // and not this.
	E int
}
`,
	}, "<!-- docgen:config pkg=p type=C -->\n<!-- /docgen:config -->\n")
	wantIn(t, got,
		"| `A` | `int` | zero value | A and B are sizes. |",
		"| `B` | `int` | zero value | A and B are sizes. |",
		"| `D` | `int` | zero value | D is above. |",
		"| `E` | `int` | zero value |  |")
}

func TestMachineNameAndAliasOfTheStateType(t *testing.T) {
	t.Parallel()
	got := fill(t, map[string]string{
		"p/p.go": stateSource + `
// A is another name of S.
type A = S

// VIEWS is a machine.
type VIEWS struct{ s S }

// State returns the state.
func (m *VIEWS) State() A { return m.s }

// Go turns it on.
func (m *VIEWS) Go() { m.s = On }
`,
		"p/p_test.go": "package p\n\nimport \"testing\"\n\nfunc TestM(t *testing.T) { m := &VIEWS{}; m.Go() }\n",
	}, "<!-- docgen:states pkg=p type=VIEWS -->\n<!-- /docgen:states -->\n")
	wantIn(t, got, "| Off | `Go` | On |")
}
