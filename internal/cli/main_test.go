package cli

import (
	"bytes"
	"flag"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

//nolint:gochecknoglobals // the testing flags must be registered at init
var golden = flag.Bool("golden", false, "rewrite testdata/want from the output of the fixture")

func copyFixture(t *testing.T, dst string) {
	t.Helper()
	if err := os.CopyFS(dst, os.DirFS("testdata/repo")); err != nil {
		t.Fatal(err)
	}
}

func docgen(t *testing.T, dir string, args ...string) (int, string) {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "log")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	rc := run(t.Context(), dir, args, f)
	b, _ := os.ReadFile(f.Name())
	return rc, string(b)
}

func TestMainExitCodes(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		args []string
		want int
	}{
		{nil, 2},
		{[]string{"help"}, 0},
		{[]string{"nope"}, 2},
	} {
		if got := Main(c.args, io.Discard); got != c.want {
			t.Errorf("Main(%q) = %d, want %d", c.args, got, c.want)
		}
	}
}

func TestFixture(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	copyFixture(t, root)
	if rc, log := docgen(t, root, "check", "-module", "v2"); rc != 1 || !strings.Contains(log, "stale: docs/machines/machine-lamp.md:5: docgen:config") {
		t.Fatalf("-check on empty markers: rc %d, log:\n%s", rc, log)
	}
	if rc, log := docgen(t, root, "update", "-module", "v2"); rc != 0 {
		t.Fatalf("-update: rc %d, log:\n%s", rc, log)
	}
	for _, f := range []string{"machine-lamp.md", "states/machine-lamp.svg", "states/machine-counter.svg", "states/machine-pair-sidestate.svg", "states/door-open.svg"} {
		got, err := os.ReadFile(filepath.Join(root, "docs/machines", f))
		if err != nil {
			t.Fatal(err)
		}
		want := filepath.Join("testdata/want", f)
		if *golden {
			if err := os.MkdirAll(filepath.Dir(want), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(want, got, 0o600); err != nil {
				t.Fatal(err)
			}
			continue
		}
		w, err := os.ReadFile(want)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, w) {
			t.Errorf("%s differs from %s:\n%s", f, want, got)
		}
	}
	if rc, log := docgen(t, root, "check", "-module", "v2"); rc != 0 {
		t.Fatalf("-check after -update: rc %d, log:\n%s", rc, log)
	}

	page := filepath.Join(root, "docs/machines/machine-lamp.md")
	text, _ := os.ReadFile(page)
	edited := strings.Replace(string(text), "| LampOff | `Start` | LampOn |", "| LampOff | `Start` | LampBroken |", 1)
	if err := os.WriteFile(page, []byte(edited), 0o600); err != nil {
		t.Fatal(err)
	}
	if rc, log := docgen(t, root, "check", "-module", "v2"); rc != 1 || !strings.Contains(log, "machine-lamp.md:15: docgen:states pkg=machine type=Lamp") {
		t.Fatalf("-check on an edited body: rc %d, log:\n%s", rc, log)
	}
	if err := os.WriteFile(page, text, 0o600); err != nil {
		t.Fatal(err)
	}

	src := filepath.Join(root, "v2/machine/machine.go")
	code, _ := os.ReadFile(src)
	moved := strings.Replace(string(code), "// Stop turns", "// Lamps stop.\n\n// Stop turns", 1)
	if err := os.WriteFile(src, []byte(moved), 0o600); err != nil {
		t.Fatal(err)
	}
	if rc, log := docgen(t, root, "check", "-module", "v2"); rc != 1 || !strings.Contains(log, "range drifted: the excerpt in the page is now at lines=98-104") {
		t.Fatalf("-check after the code moved: rc %d, log:\n%s", rc, log)
	}
}

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

func TestSingleModule(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"go.mod": "module example.com/flat\n\ngo 1.24.0\n",
		"flat.go": `// Package flat is at the root of its module.
package flat

import "errors"

// Limit is the most that Light accepts.
const Limit = 3

// ErrDark is returned by Light.
var ErrDark = errors.New("dark")

// Bulb is a state machine.
type Bulb struct{ on bool }

// State is the state of a Bulb.
type State int

const (
	// Off gives no light.
	Off State = iota
	// On gives light.
	On
)

// State returns the state.
func (b *Bulb) State() State {
	if b.on {
		return On
	}
	return Off
}

// Light turns the Bulb on. It returns ErrDark when the Bulb is on.
func (b *Bulb) Light() error {
	if b.on {
		return ErrDark
	}
	b.on = true
	return nil
}
`,
		"flat_test.go": `package flat

import "testing"

func TestLight(t *testing.T) {
	var b Bulb
	if err := b.Light(); err != nil {
		t.Fatal(err)
	}
}
`,
		"sub/sub.go":           "// Package sub is a package of the module.\npackage sub\n\n// Size is a size.\nconst Size = 4\n",
		"tools/gen/go.mod":     "module example.com/other/gen\n\ngo 1.24.0\n",
		"tools/gen/main.go":    "// Command gen is in a module of its own.\npackage main\n\nfunc main() {}\n",
		"tools/sub/go.mod":     "module example.com/other/sub\n\ngo 1.24.0\n",
		"tools/sub/sub/sub.go": "// Package sub has the name of a package of the main module.\npackage sub\n",
		"docs/page.md": "<!-- docgen:const pkg=. name=Limit --><!-- /docgen:const -->\n\n" +
			"<!-- docgen:const pkg=sub name=Size --><!-- /docgen:const -->\n\n" +
			"<!-- docgen:errors pkg=. type=Bulb -->\n<!-- /docgen:errors -->\n\n" +
			"<!-- docgen:states pkg=. type=Bulb -->\n<!-- /docgen:states -->\n\n" +
			"<!-- docgen:doc pkg=tools/gen -->\n<!-- /docgen:doc -->\n",
	})
	if rc, log := docgen(t, root, "update"); rc != 0 {
		t.Fatalf("-update: rc %d, log:\n%s", rc, log)
	}
	if rc, log := docgen(t, root, "check"); rc != 0 {
		t.Fatalf("-check: rc %d, log:\n%s", rc, log)
	}
	b, err := os.ReadFile(filepath.Join(root, "docs/page.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"`3`",
		"`4`",
		"https://pkg.go.dev/example.com/flat#ErrDark",
		"states/flat-bulb.svg",
		"Command gen is in a module of its own.",
	} {
		if !strings.Contains(string(b), want) {
			t.Errorf("page has no %q:\n%s", want, b)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "docs/states/flat-bulb.svg")); err != nil {
		t.Error(err)
	}
	if es, err := os.ReadDir(root); err != nil {
		t.Fatal(err)
	} else {
		for _, e := range es {
			if e.Name() == "artifacts" {
				t.Error("docgen left a directory in the repository")
			}
		}
	}
}

func TestErrorsWithNoRootPackage(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"go.mod":    "module example.com/noroot\n\ngo 1.24.0\n",
		"a/a.go":    "// Package a is the only package.\npackage a\n\nimport \"errors\"\n\n// ErrA is an error.\nvar ErrA = errors.New(\"a\")\n\n// T is a type.\ntype T struct{}\n\n// Do returns ErrA.\nfunc (T) Do() error { return ErrA }\n",
		"docs/p.md": "<!-- docgen:errors pkg=a type=T -->\n<!-- /docgen:errors -->\n",
	})
	if rc, log := docgen(t, root, "update"); rc != 0 {
		t.Fatalf("-update: rc %d, log:\n%s", rc, log)
	}
}

func TestNoGoMod(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"conf.yaml": "a:\n  b: 1\n",
		"docs/p.md": "<!-- docgen:code file=conf.yaml symbol=a.b -->\n<!-- /docgen:code -->\n",
	})
	if rc, log := docgen(t, root, "update"); rc != 0 {
		t.Fatalf("a page with no Go markers: rc %d, log:\n%s", rc, log)
	}
	writeTree(t, root, map[string]string{"docs/q.md": "<!-- docgen:const pkg=a name=B --><!-- /docgen:const -->\n"})
	if rc, log := docgen(t, root, "update"); rc != 1 || !strings.Contains(log, "no go.mod") {
		t.Fatalf("a Go marker with no go.mod: rc %d, log:\n%s", rc, log)
	}
}

func TestDocsDirectory(t *testing.T) {
	t.Parallel()
	page := "N is <!-- docgen:const pkg=. name=N --><!-- /docgen:const -->\n"
	bad := "<!-- docgen:nope -->\n"
	for _, tc := range []struct{ docs, page string }{
		{"documentation", "documentation/p.md"},
		{"site/content", "site/content/p.md"},
		{".", "README.md"},
	} {
		root := t.TempDir()
		writeTree(t, root, map[string]string{
			"go.mod":                 "module example.com/d\n\ngo 1.24.0\n",
			"d.go":                   "// Package d is a package.\npackage d\n\n// N is a number.\nconst N = 1\n",
			tc.page:                  page,
			"internal/testdata/x.md": bad,
			"vendor/y/x.md":          bad,
			"node_modules/y/x.md":    bad,
			".hidden/x.md":           bad,
			"_old/x.md":              bad,
		})
		if rc, log := docgen(t, root, "update", tc.docs); rc != 0 {
			t.Errorf("directory %s: rc %d, log:\n%s", tc.docs, rc, log)
			continue
		}
		b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(tc.page)))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(b), "`1`") {
			t.Errorf("directory %s: page not filled:\n%s", tc.docs, b)
		}
	}
}

func TestBadUsage(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{{}, {"nope"}, {"-update"}, {"check", "a", "b"}, {"check", "-nope"}, {"check", "-docs", "x"}} {
		if rc, _ := docgen(t, t.TempDir(), args...); rc != 2 {
			t.Errorf("%v: rc %d, want 2", args, rc)
		}
	}
}

func TestConfigFile(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"sub/go.mod":   "module example.com/sub\n\ngo 1.24.0\n",
		"sub/s.go":     "// Package s is a package.\npackage s\n\n// N is a number.\nconst N = 1\n",
		"content/p.md": "<!-- docgen:const pkg=. name=N --><!-- /docgen:const -->\n",
		"other/p.md":   "<!-- docgen:const pkg=. name=N --><!-- /docgen:const -->\n",
		"deep/down/x":  "",
		".docgen.yaml": "docs: content\nmodule: sub\n",
		".git/HEAD":    "ref: refs/heads/main\n",
	})
	filled := func(page string) bool {
		b, err := os.ReadFile(filepath.Join(root, page))
		if err != nil {
			t.Fatal(err)
		}
		return strings.Contains(string(b), "`1`")
	}
	if rc, log := docgen(t, filepath.Join(root, "deep", "down"), "update"); rc != 0 {
		t.Fatalf("update from a subdirectory: rc %d, log:\n%s", rc, log)
	}
	if !filled("content/p.md") || filled("other/p.md") {
		t.Fatal("the config file did not select the pages")
	}
	if rc, log := docgen(t, root, "update", "other"); rc != 0 {
		t.Fatalf("an argument over the file: rc %d, log:\n%s", rc, log)
	}
	if !filled("other/p.md") {
		t.Fatal("the argument did not override the config file")
	}
	if rc, log := docgen(t, filepath.Join(root, "other"), "check", "."); rc != 0 {
		t.Fatalf("an argument relative to the working directory: rc %d, log:\n%s", rc, log)
	}
	writeTree(t, root, map[string]string{".docgen.yaml": "docs: [\n"})
	if rc, _ := docgen(t, root, "check"); rc != 1 {
		t.Fatalf("a broken config file: rc %d, want 1", rc)
	}
}

func TestConfigulatorMarkerNeedsTheGenerator(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"go.mod":    "module example.com/c\n\ngo 1.24.0\n",
		"c.go":      "// Package c is a package.\npackage c\n\n// Config is a config.\ntype Config struct{}\n",
		"docs/p.md": "<!-- docgen:configulator pkg=. type=Config -->\n<!-- /docgen:configulator -->\n",
	})
	if rc, log := docgen(t, root, "update"); rc != 1 || !strings.Contains(log, "go get -tool") {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
}

func TestOwnDocs(t *testing.T) {
	t.Parallel()
	if rc, log := docgen(t, "../..", "check"); rc != 0 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
}

func TestHelpExitsZero(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{{"-h"}, {"help"}, {"update", "-h"}, {"check", "-help"}} {
		if rc, log := docgen(t, t.TempDir(), args...); rc != 0 || !strings.Contains(log, "-module") || !strings.Contains(log, "update") {
			t.Errorf("%v: rc %d, log:\n%s", args, rc, log)
		}
	}
}

func TestOutputAnyDirectoryAndShellCommand(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"go.mod":            "module example.com/o\n\ngo 1.24.0\n",
		"tools/say/go.mod":  "module example.com/say\n\ngo 1.24.0\n",
		"tools/say/main.go": "package main\n\nimport \"fmt\"\n\nfunc main() { fmt.Println(\"from a nested module\") }\n",
		"VERSION":           "1.2.3\n",
		".git/HEAD":         "ref: refs/heads/main\n",
		"docs/p.md": "<!-- docgen:output cmd=tools/say -->\n<!-- /docgen:output -->\n\n" +
			"<!-- docgen:output exec=\"cat VERSION | tr . -\" -->\n<!-- /docgen:output -->\n",
	})
	if rc, log := docgen(t, root, "update"); rc != 0 {
		t.Fatalf("update: rc %d, log:\n%s", rc, log)
	}
	if rc, log := docgen(t, root, "check"); rc != 0 {
		t.Fatalf("check: rc %d, log:\n%s", rc, log)
	}
	b, err := os.ReadFile(filepath.Join(root, "docs/p.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"from a nested module", "1-2-3"} {
		if !strings.Contains(string(b), want) {
			t.Errorf("page has no %q:\n%s", want, b)
		}
	}
	writeTree(t, root, map[string]string{"docs/p.md": "<!-- docgen:output exec=\"exit 3\" -->\n<!-- /docgen:output -->\n"})
	if rc, log := docgen(t, root, "update"); rc != 1 || !strings.Contains(log, "exit 3") {
		t.Fatalf("a failing command: rc %d, log:\n%s", rc, log)
	}
}

func TestDiagramPath(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	copyFixture(t, root)
	writeTree(t, root, map[string]string{"docs/p.md": "<!-- docgen:states pkg=machine type=Lamp diagram=../static/lamp.svg -->\n<!-- /docgen:states -->\n"})
	if rc, log := docgen(t, root, "update", "-module", "v2"); rc != 0 {
		t.Fatalf("update: rc %d, log:\n%s", rc, log)
	}
	if _, err := os.Stat(filepath.Join(root, "static/lamp.svg")); err != nil {
		t.Error(err)
	}
	b, err := os.ReadFile(filepath.Join(root, "docs/p.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "](../static/lamp.svg)") {
		t.Errorf("no link to the diagram:\n%s", b)
	}
}
