package cli

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func writeRepo(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	writeTree(t, root, files)
	return root
}

func TestBacktickValues(t *testing.T) {
	t.Parallel()
	got := fill(t, map[string]string{
		"p/p.go":   "package p\n\nconst Str = \"a`b\"\n\nconst Tick = \"`\"\n\nvar Two = \"x``y\"\n",
		"x/go.mod": "module example.com/x\n\ngo 1.24.0\n\ntoolchain a`b\n",
	}, "<!-- docgen:const pkg=p name=Str --><!-- /docgen:const -->\n\n"+
		"<!-- docgen:const pkg=p name=Tick --><!-- /docgen:const -->\n\n"+
		"<!-- docgen:const pkg=p name=Two --><!-- /docgen:const -->\n\n"+
		"<!-- docgen:gomod field=toolchain file=x/go.mod --><!-- /docgen:gomod -->\n")
	wantIn(t, got, "``a`b``", "`` ` ``", "```x``y```")
}

func TestDocHeadingHasNoID(t *testing.T) {
	t.Parallel()
	got := fill(t, map[string]string{
		"p/p.go": "// Package p is a package.\n//\n// # Build tags\n//\n// The tags.\npackage p\n",
	}, "<!-- docgen:doc pkg=p -->\n<!-- /docgen:doc -->\n")
	wantIn(t, got, "### Build tags\n")
	wantNotIn(t, got, "{#")
}

func TestStateMethodOfUnnamedInteger(t *testing.T) {
	t.Parallel()
	for _, typ := range []string{"int", "uint8"} {
		t.Run(typ, func(t *testing.T) {
			t.Parallel()
			got := fill(t, map[string]string{
				"p/p.go":      "package p\n\n// M is a machine.\ntype M struct{ s " + typ + " }\n\n// State returns the state.\nfunc (m *M) State() " + typ + " { return m.s }\n\n// Go moves the machine.\nfunc (m *M) Go() { m.s = 2 }\n\n// Back moves it back.\nfunc (m *M) Back() { m.s = 0 }\n",
				"p/p_test.go": "package p\n\nimport \"testing\"\n\nfunc TestM(t *testing.T) { m := &M{}; m.Go(); m.Back() }\n",
			}, "<!-- docgen:states pkg=p type=M -->\n<!-- /docgen:states -->\n")
			wantIn(t, got, "| value 0 |  | yes, first state |", "| value 2 |  | yes |", "| value 0 | `Go` | value 2 |", "| value 2 | `Back` | value 0 |")
		})
	}
}

func TestExcerptOfAPage(t *testing.T) {
	t.Parallel()
	const gomod = "<!-- docgen:gomod field=module --><!-- /docgen:gomod -->\n"
	root := pageRepo(t, map[string]string{
		"docs/a.md": "<!-- docgen:region r -->\n" + gomod + "<!-- docgen:end -->\n",
		"docs/b.md": "<!-- docgen:code file=docs/a.md region=r -->\n<!-- /docgen:code -->\n",
		"docs/c.md": "<!-- docgen:code file=docs/b.md -->\n<!-- /docgen:code -->\n",
		"docs/d.md": "<!-- docgen:code file=docs/c.md lines=1-2 -->\n<!-- /docgen:code -->\n",
	})
	wantIn(t, updateTwice(t, root, "docs/b.md"), "`example.com/p`")
	wantIn(t, readPage(t, root, "docs/c.md"), "`example.com/p`")
}

func TestExcerptOfTheReadme(t *testing.T) {
	t.Parallel()
	root := pageRepo(t, map[string]string{
		"README.md": "# R\n\n" + goodPair + "\n",
		"docs/p.md": "<!-- docgen:code file=README.md -->\n<!-- /docgen:code -->\n",
	})
	if rc, log := docgen(t, root, "update", "."); rc != 0 {
		t.Fatalf("update: rc %d, log:\n%s", rc, log)
	}
	if rc, log := docgen(t, root, "check", "."); rc != 0 {
		t.Fatalf("check: rc %d, log:\n%s", rc, log)
	}
	wantIn(t, readPage(t, root, "docs/p.md"), "-->`1`<!--")
}

func TestExcerptOfItsOwnPage(t *testing.T) {
	t.Parallel()
	text := "<!-- docgen:code file=docs/p.md -->\n<!-- /docgen:code -->\n"
	root := pageRepo(t, map[string]string{"docs/p.md": text})
	rc, log := docgen(t, root, "update")
	if rc != 1 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
	wantIn(t, log, "docs/p.md:1:", "of the page of the marker, docs/p.md")
	if got := readPage(t, root, "docs/p.md"); got != text {
		t.Errorf("the page changed:\n%s", got)
	}
}

func TestExcerptsWithNoFixedPoint(t *testing.T) {
	t.Parallel()
	a := "<!-- docgen:code file=docs/b.md -->\n<!-- /docgen:code -->\n"
	b := "<!-- docgen:code file=docs/a.md -->\n<!-- /docgen:code -->\n"
	root := pageRepo(t, map[string]string{"docs/a.md": a, "docs/b.md": b})
	rc, log := docgen(t, root, "update")
	if rc != 1 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
	wantIn(t, log, "docs/a.md: ", "docs/b.md: ", "do not converge")
	if got := readPage(t, root, "docs/a.md"); got != a {
		t.Errorf("the page changed:\n%s", got)
	}
}

const collidingMachines = `package p

// S is a state.
type S int

// The states.
const (
	Off S = iota
	On
)

// Ab is a machine.
type Ab struct{ s S }

// State returns the state.
func (m *Ab) State() S { return m.s }

// Go moves the machine.
func (m *Ab) Go() { m.s = On }

// AB is a machine.
type AB struct{ s S }

// State returns the state.
func (m *AB) State() S { return m.s }

// Go moves the machine.
func (m *AB) Go() { m.s = On }
`

func TestDiagramNameCollision(t *testing.T) {
	t.Parallel()
	root := writeRepo(t, map[string]string{
		"go.mod":      "module example.com/w\n\ngo 1.24.0\n",
		"p/p.go":      collidingMachines,
		"p/p_test.go": "package p\n\nimport \"testing\"\n\nfunc TestM(t *testing.T) { (&Ab{}).Go(); (&AB{}).Go() }\n",
		"docs/p.md":   "<!-- docgen:states pkg=p type=Ab -->\n<!-- /docgen:states -->\n\n<!-- docgen:states pkg=p type=AB -->\n<!-- /docgen:states -->\n",
	})
	rc, log := docgen(t, root, "update")
	if rc != 1 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
	wantIn(t, log, "docs/p.md:1: docgen:states pkg=p type=Ab", "docs/p.md:4: docgen:states pkg=p type=AB", "docs/states/p-ab.svg")
	if names := dirNames(t, filepath.Join(root, "docs/states")); len(names) != 0 {
		t.Errorf("diagrams written: %q", names)
	}
}

func TestOneMachineTwiceIsNoCollision(t *testing.T) {
	t.Parallel()
	got := fill(t, map[string]string{
		"p/p.go":      collidingMachines,
		"p/p_test.go": "package p\n\nimport \"testing\"\n\nfunc TestM(t *testing.T) { (&Ab{}).Go() }\n",
	}, "<!-- docgen:states pkg=p type=Ab -->\n<!-- /docgen:states -->\n\n<!-- docgen:states pkg=p type=Ab -->\n<!-- /docgen:states -->\n")
	wantIn(t, got, "states/p-ab.svg")
}

func TestModuleDirectory(t *testing.T) {
	t.Parallel()
	root := writeRepo(t, map[string]string{"docs/p.md": "Text.\n", "plain/x.txt": "x\n"})
	if rc, log := docgen(t, root, "check", "-module", "nonexistent"); rc != 1 || !strings.Contains(log, "nonexistent") {
		t.Errorf("a missing directory: rc %d, log:\n%s", rc, log)
	}
	if rc, log := docgen(t, root, "check", "-module", "plain"); rc != 0 {
		t.Errorf("a directory with no go.mod: rc %d, log:\n%s", rc, log)
	}
}

func TestAbsolutePathsAreClean(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{
		"/r/v2/":    "/r/v2",
		"/r/./v2":   "/r/v2",
		"/r/x/../y": "/r/y",
		"v2/":       "/root/v2",
		".":         "/root",
	} {
		if got := abs("/root", in); got != filepath.FromSlash(want) {
			t.Errorf("abs(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestModuleWithTrailingSlash(t *testing.T) {
	t.Parallel()
	root := writeRepo(t, map[string]string{
		"v2/go.mod":         "module example.com/w/v2\n\ngo 1.24.0\n",
		"v2/store/store.go": "package store\n\nimport \"errors\"\n\n// ErrMissing is an error.\nvar ErrMissing = errors.New(\"missing\")\n",
		"v2/s/s.go":         "package s\n\nimport \"example.com/w/v2/store\"\n\n// S is a store.\ntype S struct{}\n\n// Get returns [store.ErrMissing].\nfunc (s *S) Get() error { return store.ErrMissing }\n",
		"docs/p.md":         "<!-- docgen:errors pkg=s type=S -->\n<!-- /docgen:errors -->\n",
	})
	if rc, log := docgen(t, root, "update", "-module", root+"/v2/"); rc != 0 {
		t.Fatalf("update: rc %d, log:\n%s", rc, log)
	}
	if rc, log := docgen(t, root, "check", "-module", "v2"); rc != 0 {
		t.Fatalf("check: rc %d, log:\n%s\npage:\n%s", rc, log, page(t, root, "p.md"))
	}
}

func TestCRLFExcerpts(t *testing.T) {
	t.Parallel()
	crlf := func(s string) string { return strings.ReplaceAll(s, "\n", "\r\n") }
	got := fill(t, map[string]string{
		"c.yaml": crlf("a:\n  b:\n    c: 1\nz: 2\n"),
		"c.h":    crlf("#define MAX(a, b) \\\n  ((a) > (b) ? \\\n   (a) : (b))\n\n#define OPEN(x) \\\n  if (x) {\n\nint one(void)\n{\n  return 1;\n}\n"),
		"r.txt":  crlf("x\n# docgen:region r\none\n# docgen:end\ny\n"),
		"s.go":   crlf("package p\n\nfunc F() {\n\tprintln(1)\n}\n"),
		"l.txt":  crlf("one\ntwo\nthree\n"),
	}, "<!-- docgen:code file=c.yaml symbol=a.b -->\n<!-- /docgen:code -->\n\n"+
		"<!-- docgen:code file=c.h symbol=MAX -->\n<!-- /docgen:code -->\n\n"+
		"<!-- docgen:code file=c.h symbol=one -->\n<!-- /docgen:code -->\n\n"+
		"<!-- docgen:code file=r.txt region=r -->\n<!-- /docgen:code -->\n\n"+
		"<!-- docgen:code file=s.go symbol=F -->\n<!-- /docgen:code -->\n\n"+
		"<!-- docgen:code file=l.txt lines=2-3 -->\n<!-- /docgen:code -->\n\n"+
		"<!-- docgen:code file=l.txt -->\n<!-- /docgen:code -->\n")
	wantIn(t, got, "```yaml\nb:\n  c: 1\n```", "```c\n#define MAX(a, b) \\\n  ((a) > (b) ? \\\n   (a) : (b))\n```",
		"```c\nint one(void)\n{\n  return 1;\n}\n```", "```\none\n```", "func F() {\n\tprintln(1)\n}\n", "```\ntwo\nthree\n```")
	wantNotIn(t, got, "\r")
}

func TestGoLoadFailureIsAnErrorOfGoMarkers(t *testing.T) {
	t.Parallel()
	const staleCode = "<!-- docgen:code file=conf.txt -->\n<!-- /docgen:code -->\n"
	for name, c := range map[string]struct {
		files map[string]string
		want  string
	}{
		"no go.mod": {map[string]string{}, "-module"},
		"newer go":  {map[string]string{"go.mod": "module example.com/w\n\ngo 1.999.0\n", "p.go": "package p\n\nconst N = 1\n"}, "1.999"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			text := goodPair + "\n\n" + staleCode
			root := writeRepo(t, c.files)
			writeTree(t, root, map[string]string{"conf.txt": "x\n", "docs/p.md": text})
			rc, log := docgen(t, root, "check")
			if rc != 1 {
				t.Fatalf("rc %d, log:\n%s", rc, log)
			}
			wantIn(t, log, "stale: docs/p.md:3: docgen:code file=conf.txt", "docs/p.md:1: docgen:const pkg=. name=N: ", c.want)
			wantNotIn(t, log, "stderr:", "err: exit status")
			if rc, _ := docgen(t, root, "update"); rc != 1 {
				t.Fatalf("update: rc %d", rc)
			}
			if got := page(t, root, "p.md"); got != text {
				t.Errorf("the page changed:\n%s", got)
			}
		})
	}
}

func goModOfTheRepository(t *testing.T) map[string]string {
	t.Helper()
	mod, err := os.ReadFile("../../go.mod")
	if err != nil {
		t.Fatal(err)
	}
	sum, err := os.ReadFile("../../go.sum")
	if err != nil {
		t.Fatal(err)
	}
	text := regexp.MustCompile(`(?m)^module .*$`).ReplaceAllString(string(mod), "module example.com/c")
	return map[string]string{"go.mod": text, "go.sum": string(sum)}
}

const configulatorType = `// Package c is a package.
package c

// Config is a config.
type Config struct {
	Port int    ` + "`name:\"port\" default:\"8080\" description:\"port to listen on\"`" + `
	Host string ` + "`name:\"host\" description:\"host name\"`" + `
}
`

func TestConfigulatorRunsTheGenerator(t *testing.T) {
	t.Parallel()
	files := goModOfTheRepository(t)
	files["c.go"] = configulatorType
	got := fill(t, files, "<!-- docgen:configulator pkg=. type=Config env=APP_ -->\n<!-- /docgen:configulator -->\n")
	wantIn(t, got, "`port`", "`8080`", "APP_PORT", "port to listen on", "`host`")
	wantNotIn(t, got, "# ")
}

func TestConfigulatorErrorOfTheType(t *testing.T) {
	t.Parallel()
	files := goModOfTheRepository(t)
	files["c.go"] = configulatorType
	files["docs/p.md"] = "<!-- docgen:configulator pkg=. type=Nope -->\n<!-- /docgen:configulator -->\n"
	root := writeRepo(t, files)
	rc, log := docgen(t, root, "update")
	if rc != 1 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
	wantIn(t, log, "Nope")
	wantNotIn(t, log, "go get -tool", "add the generator")
}

func TestNewFilesAreReadable(t *testing.T) {
	t.Parallel()
	root := writeRepo(t, map[string]string{
		"go.mod":      "module example.com/w\n\ngo 1.24.0\n",
		"p/p.go":      collidingMachines,
		"p/p_test.go": "package p\n\nimport \"testing\"\n\nfunc TestM(t *testing.T) { (&Ab{}).Go() }\n",
		"docs/p.md":   "<!-- docgen:states pkg=p type=Ab -->\n<!-- /docgen:states -->\n",
	})
	probe := filepath.Join(root, "probe")
	if err := os.WriteFile(probe, nil, 0o666); err != nil { //nolint:gosec // reads the umask
		t.Fatal(err)
	}
	info, err := os.Stat(probe)
	if err != nil {
		t.Fatal(err)
	}
	umask := 0o666 &^ info.Mode().Perm()
	if rc, log := docgen(t, root, "update"); rc != 0 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
	for name, want := range map[string]os.FileMode{"docs/states": 0o755 &^ umask, "docs/states/p-ab.svg": 0o644 &^ umask} {
		info, err := os.Stat(filepath.Join(root, filepath.FromSlash(name)))
		if err != nil {
			t.Fatal(err)
		}
		if got := info.Mode().Perm(); got != want {
			t.Errorf("%s: mode %o, want %o", name, got, want)
		}
	}
}

func TestPathsOutsideTheRepository(t *testing.T) {
	t.Parallel()
	for name, c := range map[string]struct{ marker, want string }{
		"code":     {"<!-- docgen:code file=../secret.txt -->\n<!-- /docgen:code -->\n", "file=../secret.txt is outside the repository"},
		"nested":   {"<!-- docgen:code file=docs/../../secret.txt -->\n<!-- /docgen:code -->\n", "is outside the repository"},
		"gomod":    {"<!-- docgen:gomod field=module file=../go.mod --><!-- /docgen:gomod -->\n", "file=../go.mod is outside the repository"},
		"example":  {"<!-- docgen:output cmd=../sub -->\n<!-- /docgen:output -->\n", "cmd=../sub is outside the repository"},
		"absolute": {"<!-- docgen:code file=/etc/hostname -->\n<!-- /docgen:code -->\n", "is outside the repository"},
		"flags":    {"<!-- docgen:flags cmd=../tool -->\n<!-- /docgen:flags -->\n", "cmd=../tool is outside the repository"},
		"doc":      {"<!-- docgen:doc pkg=../tool -->\n<!-- /docgen:doc -->\n", "pkg=../tool is outside the repository"},
		"symlink":  {"<!-- docgen:code file=link.txt -->\n<!-- /docgen:code -->\n", "file=link.txt is outside the repository"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			parent := t.TempDir()
			writeTree(t, parent, map[string]string{
				"secret.txt":      "secret\n",
				"go.mod":          "module example.com/outside\n",
				"repo/go.mod":     "module example.com/w\n\ngo 1.24.0\n",
				"repo/sub/x.go":   "package main\n\nfunc main() { println(\"ran\") }\n",
				"repo/docs/p.md":  c.marker,
				"repo/examples/k": "x\n",
				"tool/main.go":    "// Command tool is outside.\npackage main\n\nfunc main() {}\n",
			})
			root := filepath.Join(parent, "repo")
			if err := os.Symlink(filepath.Join(parent, "secret.txt"), filepath.Join(root, "link.txt")); err != nil {
				t.Fatal(err)
			}
			rc, log := docgen(t, root, "update")
			if rc != 1 {
				t.Fatalf("rc %d, log:\n%s", rc, log)
			}
			wantIn(t, log, c.want)
			if got := page(t, root, "p.md"); got != c.marker {
				t.Errorf("the page changed:\n%s", got)
			}
		})
	}
}

func TestConfigFileKeysAndExtensions(t *testing.T) {
	t.Parallel()
	root := writeRepo(t, map[string]string{"alt/p.md": "Text.\n", "other.yml": "docs: alt\n", "bad.yml": "docs: alt\ndosc: alt\n", ".docgen.yaml": "dosc: alt\n", "empty.yml": ""})
	if rc, log := docgen(t, root, "check"); rc != 1 || !strings.Contains(log, "dosc") {
		t.Errorf("an unknown key: rc %d, log:\n%s", rc, log)
	}
	if rc, log := docgen(t, root, "check", "-config", filepath.Join(root, "bad.yml")); rc != 1 || !strings.Contains(log, "dosc") {
		t.Errorf("an unknown key in a .yml file: rc %d, log:\n%s", rc, log)
	}
	if rc, log := docgen(t, root, "check", "-config", filepath.Join(root, "other.yml")); rc != 0 {
		t.Errorf("a .yml file: rc %d, log:\n%s", rc, log)
	}
	if rc, log := docgen(t, root, "check", "-config", filepath.Join(root, "empty.yml"), "alt"); rc != 0 {
		t.Errorf("an empty file: rc %d, log:\n%s", rc, log)
	}
}

func TestConfigFieldEmbedded(t *testing.T) {
	t.Parallel()
	got := fill(t, map[string]string{
		"p/p.go": `package p

import "net/url"

// Inner is a part.
type Inner struct{}

// Gen is generic.
type Gen[T any] struct{}

// C is a config.
type C struct {
	// Inner holds the part.
	Inner
	// URL is embedded through a pointer of another package.
	*url.URL
	Gen[int] // Gen holds the generic part.
}
`,
	}, "<!-- docgen:config pkg=p type=C -->\n<!-- /docgen:config -->\n")
	wantIn(t, got, "Inner holds the part.", "URL is embedded through a pointer of another package.", "Gen holds the generic part.")
}

func TestDocOfAnUndocumentedName(t *testing.T) {
	t.Parallel()
	for name, c := range map[string]struct{ marker, want string }{
		"func":    {"name=F", "no doc comment for F in example.com/w/p"},
		"type":    {"name=T", "no doc comment for T in example.com/w/p"},
		"method":  {"name=T.M", "no doc comment for T.M in example.com/w/p"},
		"const":   {"name=C", "no doc comment for C in example.com/w/p"},
		"package": {"", "no doc comment for package example.com/w/p"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := writeRepo(t, map[string]string{
				"go.mod":    "module example.com/w\n\ngo 1.24.0\n",
				"p/p.go":    "package p\n\nconst C = 1\n\nfunc F() {}\n\ntype T int\n\nfunc (T) M() {}\n",
				"docs/p.md": "<!-- docgen:doc pkg=p " + c.marker + " -->\n<!-- /docgen:doc -->\n",
			})
			rc, log := docgen(t, root, "check")
			if rc != 1 {
				t.Fatalf("rc %d, log:\n%s", rc, log)
			}
			wantIn(t, log, c.want)
		})
	}
}

func TestConfigDefaultWithBacktick(t *testing.T) {
	t.Parallel()
	got := fill(t, map[string]string{
		"p/p.go": "package p\n\n// C is a config.\ntype C struct {\n\t// S is a string.\n\tS string\n}\n\n// Use applies the default.\nfunc Use(c C) string {\n\tif c.S == \"\" {\n\t\tc.S = \"a`b\"\n\t}\n\treturn c.S\n}\n",
	}, "<!-- docgen:config pkg=p type=C -->\n<!-- /docgen:config -->\n")
	wantIn(t, got, "| ``\"a`b\"`` |")
}
