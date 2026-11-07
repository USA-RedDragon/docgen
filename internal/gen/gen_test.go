package gen

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

const goModText = "module example.com/w\n\ngo 1.24.0\n"

const modMarker = "<!-- docgen:gomod field=module --><!-- /docgen:gomod -->"

const modFilled = "<!-- docgen:gomod field=module -->`example.com/w`<!-- /docgen:gomod -->"

func writeTree(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for name, text := range files {
		p := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func read(t *testing.T, root, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// setup writes files in a new repository and returns its root and a Generator.
func setup(t *testing.T, files map[string]string) (string, *Generator) {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	writeTree(t, root, files)
	return root, generator(t, root)
}

func generator(t *testing.T, root string) *Generator {
	t.Helper()
	g, err := New(root, root, io.Discard, Options{})
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func TestRunCheckWritesNothing(t *testing.T) {
	t.Parallel()
	text := "# Page\n\n" + modMarker + "\n"
	root, g := setup(t, map[string]string{"go.mod": goModText, "docs/p.md": text})
	stale, err := g.Run(t.Context(), filepath.Join(root, "docs"), false)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"docs/p.md:3: docgen:gomod field=module"}; !slices.Equal(stale, want) {
		t.Errorf("stale = %q, want %q", stale, want)
	}
	if got := read(t, root, "docs/p.md"); got != text {
		t.Errorf("page changed: %q", got)
	}
}

func TestRunUpdate(t *testing.T) {
	t.Parallel()
	root, g := setup(t, map[string]string{"go.mod": goModText, "docs/p.md": "before " + modMarker + " after\n"})
	docs := filepath.Join(root, "docs")
	if _, err := g.Run(t.Context(), docs, true); err != nil {
		t.Fatal(err)
	}
	if got, want := read(t, root, "docs/p.md"), "before "+modFilled+" after\n"; got != want {
		t.Errorf("page = %q, want %q", got, want)
	}
	stale, err := generator(t, root).Run(t.Context(), docs, false)
	if err != nil || len(stale) != 0 {
		t.Errorf("after update: stale %q, err %v", stale, err)
	}
}

func TestRunFailedMarkerWritesNothing(t *testing.T) {
	t.Parallel()
	good := modMarker + "\n"
	bad := "<!-- docgen:gomod field=nope --><!-- /docgen:gomod -->\n"
	root, g := setup(t, map[string]string{"go.mod": goModText, "docs/a.md": good, "docs/b.md": bad})
	stale, err := g.Run(t.Context(), filepath.Join(root, "docs"), true)
	if err == nil || !strings.HasPrefix(err.Error(), "docs/b.md:1: docgen:gomod field=nope: ") || !strings.HasSuffix(err.Error(), "is not go, toolchain or module") {
		t.Fatalf("err = %v", err)
	}
	if want := []string{"docs/a.md:1: docgen:gomod field=module"}; !slices.Equal(stale, want) {
		t.Errorf("stale = %q, want %q", stale, want)
	}
	if got := read(t, root, "docs/a.md"); got != good {
		t.Errorf("a page changed after a failure elsewhere: %q", got)
	}
}

func TestRunParseErrorNamesPage(t *testing.T) {
	t.Parallel()
	root, g := setup(t, map[string]string{"go.mod": goModText, "docs/p.md": "<!-- docgen:gomod field=module -->\n"})
	if _, err := g.Run(t.Context(), filepath.Join(root, "docs"), false); err == nil || !strings.Contains(err.Error(), "docs/p.md") {
		t.Errorf("err = %v", err)
	}
}

func TestRunMissingDocs(t *testing.T) {
	t.Parallel()
	root, g := setup(t, map[string]string{"go.mod": goModText})
	if _, err := g.Run(t.Context(), filepath.Join(root, "docs"), false); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("err = %v", err)
	}
}

func TestGoModFields(t *testing.T) {
	t.Parallel()
	mod := "module example.com/w\n\ngo 1.24.0\n\ntoolchain go1.24.1\n"
	bare := "module example.com/w\n"
	for _, c := range []struct {
		name, args, want, wantErr string
	}{
		{"go", "field=go", "`1.24.0`", ""},
		{"toolchain", "field=toolchain", "`go1.24.1`", ""},
		{"module", "field=module", "`example.com/w`", ""},
		{"file", "field=module file=sub/go.mod", "`example.com/sub`", ""},
		{"no go line", "field=go file=bare/go.mod", "", "bare/go.mod has no go line"},
		{"no toolchain line", "field=toolchain file=bare/go.mod", "", "bare/go.mod has no toolchain line"},
		{"no file", "field=go file=none/go.mod", "", "no such file"},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			page := "<!-- docgen:gomod " + c.args + " --><!-- /docgen:gomod -->\n"
			root, g := setup(t, map[string]string{
				"go.mod":      mod,
				"sub/go.mod":  "module example.com/sub\n",
				"bare/go.mod": bare,
				"docs/p.md":   page,
			})
			_, err := g.Run(t.Context(), filepath.Join(root, "docs"), true)
			if c.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), c.wantErr) {
					t.Fatalf("err = %v, want %q", err, c.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got := read(t, root, "docs/p.md"); !strings.Contains(got, ">"+c.want+"<") {
				t.Errorf("page = %q, want %q", got, c.want)
			}
		})
	}
}

func TestFillTestErrors(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ name, arg, want string }{
		{"no example", "p.ExampleMissing", "no func ExampleMissing in example.com/w/p"},
		{"no output comment", "p.ExampleQuiet", "func ExampleQuiet has no // Output: comment"},
		{"no package", "nope.ExampleSum", "no package nope"},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			root, g := setup(t, map[string]string{
				"go.mod":      goModText,
				"p/p.go":      "package p\n",
				"p/p_test.go": "package p\n\nfunc ExampleQuiet() {}\n",
				"docs/p.md":   "<!-- docgen:output test=" + c.arg + " --><!-- /docgen:output -->\n",
			})
			if _, err := g.Run(t.Context(), filepath.Join(root, "docs"), false); err == nil || !strings.Contains(err.Error(), c.want) {
				t.Errorf("err = %v, want %q", err, c.want)
			}
		})
	}
}

func TestExampleOutputFails(t *testing.T) {
	t.Parallel()
	root, g := setup(t, map[string]string{
		"go.mod":               goModText,
		"examples/bad/main.go": "package main\n\nimport \"os\"\n\nfunc main() { os.Exit(3) }\n",
		"docs/p.md":            "<!-- docgen:output cmd=examples/bad --><!-- /docgen:output -->\n",
	})
	if _, err := g.Run(t.Context(), filepath.Join(root, "docs"), false); err == nil || !strings.Contains(err.Error(), "go run examples/bad") {
		t.Errorf("err = %v", err)
	}
}

func TestExampleMissing(t *testing.T) {
	t.Parallel()
	root, g := setup(t, map[string]string{
		"go.mod":    goModText,
		"docs/p.md": "<!-- docgen:output cmd=examples/none --><!-- /docgen:output -->\n",
	})
	if _, err := g.Run(t.Context(), filepath.Join(root, "docs"), false); err == nil || !strings.Contains(err.Error(), "no Go command in examples/none") {
		t.Errorf("err = %v", err)
	}
}

func TestSplitTest(t *testing.T) {
	t.Parallel()
	for in, want := range map[string][2]string{
		"p.ExampleX":     {"p", "ExampleX"},
		"a/b.c.ExampleX": {"a/b.c", "ExampleX"},
		"ExampleX":       {"", "ExampleX"},
		".Example":       {"", "Example"},
	} {
		if pkg, name := splitTest(in); pkg != want[0] || name != want[1] {
			t.Errorf("splitTest(%q) = %q, %q, want %q", in, pkg, name, want)
		}
	}
}

func TestConfigulatorArgs(t *testing.T) {
	t.Parallel()
	base := []string{"run", configulatorCmd, "-type", "Config", "-markdown", "-no-validate"}
	args := func(tag string) []string {
		t.Helper()
		pg := newPage("p.md", "p.md", "<!-- docgen:configulator pkg=. type=Config"+tag+" -->\n<!-- /docgen:configulator -->\n")
		if err := pg.parse(); err != nil {
			t.Fatal(err)
		}
		h, ok := pg.markers[0].handler.(*configulatorHandler)
		if !ok {
			t.Fatalf("handler %T", pg.markers[0].handler)
		}
		return h.args()
	}
	want := append(slices.Clone(base), "-env-prefix", "APP_", "-env-separator", "__", "-flag-separator", "-")
	for range 50 {
		if got := args(" flagsep=- envsep=__ env=APP_"); !slices.Equal(got, want) {
			t.Fatalf("args = %q, want %q", got, want)
		}
	}
	if got := args(""); !slices.Equal(got, base) {
		t.Errorf("args = %q, want %q", got, base)
	}
}

func machineTree() map[string]string {
	return map[string]string{
		"go.mod": goModText,
		"p/p.go": `package p

// S is a state.
type S int

// The states.
const (
	Off S = iota
	On
)

// Lamp is a machine.
type Lamp struct{ s S }

// State returns the state.
func (l *Lamp) State() S { return l.s }

// Start turns the lamp on.
func (l *Lamp) Start() { l.s = On }
`,
		"p/p_test.go": `package p

import "testing"

func TestStart(t *testing.T) {
	var l Lamp
	l.Start()
}
`,
		"docs/p.md": "<!-- docgen:states pkg=p type=Lamp --><!-- /docgen:states -->\n",
	}
}

func TestRunCanceled(t *testing.T) {
	t.Parallel()
	for name, files := range map[string]map[string]string{
		"page only": {"go.mod": goModText, "docs/p.md": modMarker + "\n"},
		"states":    machineTree(),
		"example": {
			"go.mod":                 goModText,
			"examples/hello/main.go": "package main\n\nfunc main() {}\n",
			"docs/p.md":              "<!-- docgen:output cmd=examples/hello --><!-- /docgen:output -->\n",
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root, g := setup(t, files)
			want := errors.New("stopped")
			ctx, cancel := context.WithCancelCause(t.Context())
			cancel(want)
			stale, err := g.Run(ctx, filepath.Join(root, "docs"), true)
			if err == nil || err.Error() != want.Error() {
				t.Errorf("err = %v, want only the cause", err)
			}
			if stale != nil {
				t.Errorf("stale = %q", stale)
			}
			if got := read(t, root, "docs/p.md"); got != files["docs/p.md"] {
				t.Errorf("page changed: %q", got)
			}
			if _, err := os.Stat(filepath.Join(root, "docs/states")); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("states directory: %v", err)
			}
		})
	}
}

func TestJobsZeroLeavesSessionJobsAtZero(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	g, err := New(root, root, io.Discard, Options{})
	if err != nil {
		t.Fatal(err)
	}
	limit := cap(g.slots.sem)
	if g.session.Jobs != 0 || limit != min(runtime.NumCPU(), maxCommands) {
		t.Errorf("session jobs %d, worker limit %d", g.session.Jobs, limit)
	}
}
