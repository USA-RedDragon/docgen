package cli

import (
	"maps"
	"strings"
	"testing"
)

const referencesCode = `// Package p is a package.
//
// # Full
//
// The text.
//
// # Hollow
//
// # Last
//
// The end.
package p

import "errors"

// Limit is a limit.
const Limit = 3

// Bare has a doc comment in the tests of docgen:const only.
const Bare = 4

// Dynamic has no constant value.
var Dynamic = len(errors.New("x").Error())

// Conf is a config.
type Conf struct {
	// Size is a size.
	Size int
}

// Hidden has no exported field.
type Hidden struct {
	size int
}

// Number is not a struct.
type Number int

// Do does the work.
//
// # Notes
//
// # Other
//
// More text.
func (Conf) Do() {}

func NoDoc() {}

// M is a machine.
type M struct{ on bool }

// On reports the state.
func (m *M) On() bool { return m.on }

// Set moves the machine.
func (m *M) Set() { m.on = true }
`

const referencesTest = `package p

import (
	"fmt"
	"testing"
)

func TestM(t *testing.T) { (&M{}).Set() }

func Example() {
	fmt.Println("x")
	// Output: x
}

func ExampleNoDoc() {
	fmt.Println("x")
}

func ExampleNoDoc_empty() {
	// Output:
}
`

func referencesRepo() map[string]string {
	return map[string]string{
		"go.mod":             "module example.com/p\n\ngo 1.24.0\n",
		"p/p.go":             referencesCode,
		"p/p_test.go":        referencesTest,
		"data/a.txt":         "one\n\nthree\n",
		"data/empty.txt":     "",
		"data/blank.txt":     "\n\n",
		"data/marks.txt":     "# docgen:region m\n# docgen:end\n",
		"data/r.txt":         "x\n# docgen:region empty\n# docgen:end\n# docgen:region blank\n\n  \n# docgen:end\n# docgen:region open\ny\n",
		"data/tail.txt":      "x # docgen:region tail\ny\nz # docgen:end\n",
		"data/c.yaml":        "jobs:\n  test:\n    steps:\n      - name: a\n",
		"data/c.c":           "#define ONE 1\n",
		"data/bad.mod":       "not a go.mod line\n",
		"docs/shown.md":      "```text\n# docgen:region fenced\nx\n# docgen:end\n```\n\n`docgen:region spanned`\n",
		"sample/docs/s.md":   "Text.\n",
		"sample/docs/nil.md": "",
		"sample/plain/f.txt": "x\n",
	}
}

func inlineKind(kind string) bool {
	return kind == "const" || kind == "gomod"
}

func referencePair(tag string) string {
	kind, _, _ := strings.Cut(tag, " ")
	sep := "\n"
	if inlineKind(kind) {
		sep = ""
	}
	return "<!-- docgen:" + tag + " -->" + sep + "<!-- /docgen:" + kind + " -->\n"
}

// TestDanglingReferences holds one row for each argument of a marker that refers
// to something. Each row has the marker and the text of its error. A row with
// parse true fails when the page parses, so its error names the kind of the
// marker and not all of its arguments.
func TestDanglingReferences(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		tag   string
		want  string
		parse bool
	}{
		{tag: "code file=nope.txt", want: "no such file"},
		{tag: `code file=""`, want: "file= is required", parse: true},
		{tag: "code file=data", want: "is a directory"},
		{tag: "code file=../nope.txt", want: "is outside the repository"},
		{tag: "code file=data/empty.txt", want: "data/empty.txt holds no text"},
		{tag: "code file=data/blank.txt", want: "data/blank.txt holds no text"},
		{tag: "code file=data/marks.txt", want: "data/marks.txt holds no text"},

		{tag: "code file=p/p.go symbol=Nope", want: "no func, type, const or var Nope"},
		{tag: "code file=p/p.go symbol=Conf.Nope", want: "no func, type, const or var Conf.Nope"},
		{tag: `code file=p/p.go symbol=""`, want: "symbol= is empty", parse: true},
		{tag: "code file=data/c.yaml symbol=jobs.nope", want: "no key jobs.nope"},
		{tag: "code file=data/c.yaml symbol=jobs.test.steps[b]", want: "no items"},
		{tag: "code file=data/c.c symbol=TWO", want: "no #define or function TWO"},
		{tag: "code file=data/a.txt symbol=One", want: "applies to a Go, YAML or C file"},

		{tag: "code file=data/r.txt region=nope", want: "no docgen:region nope"},
		{tag: `code file=data/r.txt region=""`, want: "region= is empty", parse: true},
		{tag: "code file=data/r.txt region=empty", want: "region empty is empty"},
		{tag: "code file=data/r.txt region=blank", want: "region blank is empty"},
		{tag: "code file=data/r.txt region=open", want: "region open has no docgen:end"},
		{tag: "code file=data/tail.txt region=tail", want: "no docgen:region tail"},
		{tag: "code file=docs/shown.md region=fenced", want: "no docgen:region fenced"},
		{tag: "code file=docs/shown.md region=spanned", want: "no docgen:region spanned"},

		{tag: "code file=data/a.txt lines=2-9", want: "data/a.txt has 3 lines, the range ends at 9"},
		{tag: "code file=data/a.txt lines=3-4", want: "data/a.txt has 3 lines, the range ends at 4"},
		{tag: "code file=data/a.txt lines=4-4", want: "data/a.txt has 3 lines, the range ends at 4"},
		{tag: "code file=data/empty.txt lines=1-1", want: "data/empty.txt has 0 lines, the range ends at 1"},
		{tag: `code file=data/a.txt lines=""`, want: "lines= is empty", parse: true},
		{tag: "code file=data/a.txt lines=2-2", want: "lines 2-2 of data/a.txt hold no text"},
		{tag: "code file=data/a.txt lines=2-1", want: "is not a range"},
		{tag: "code file=data/marks.txt lines=1-2", want: "hold only region lines"},

		{tag: "gomod field=nope", want: "field=nope is not go, toolchain or module"},
		{tag: `gomod field=""`, want: "field= is required", parse: true},
		{tag: "gomod field=toolchain", want: "has no toolchain line"},
		{tag: "gomod field=go file=nope/go.mod", want: "no such file"},
		{tag: `gomod field=go file=""`, want: "file= is empty", parse: true},
		{tag: "gomod field=go file=data/bad.mod", want: "bad.mod"},
		{tag: "gomod field=go file=../go.mod", want: "is outside the repository"},

		{tag: "const pkg=nope name=Limit", want: "no package nope"},
		{tag: `const pkg="" name=Limit`, want: "pkg= is required", parse: true},
		{tag: "const pkg=p name=Nope", want: "Nope is not a constant"},
		{tag: `const pkg=p name=""`, want: "name= is required", parse: true},
		{tag: "const pkg=p name=Conf", want: "Conf is not a constant"},
		{tag: "const pkg=p name=Dynamic", want: "is not a constant expression"},

		{tag: "config pkg=nope type=Conf", want: "no package nope"},
		{tag: "config pkg=p type=Nope", want: "Nope is not a type"},
		{tag: `config pkg=p type=""`, want: "type= is required", parse: true},
		{tag: "config pkg=p type=Number", want: "Number is not a struct"},
		{tag: "config pkg=p type=Hidden", want: "Hidden has no exported field"},

		{tag: "configulator pkg=nope type=Conf", want: "no package nope"},
		{tag: `configulator pkg=p type=""`, want: "type= is required", parse: true},

		{tag: "errors pkg=nope type=Conf", want: "no package nope"},
		{tag: "errors pkg=p type=Nope", want: "Nope is not a type"},
		{tag: `errors pkg=p type=""`, want: "type= is required", parse: true},

		{tag: "states pkg=nope type=M", want: "no package nope"},
		{tag: "states pkg=p type=Nope", want: "Nope"},
		{tag: `states pkg=p type=""`, want: "type= is required", parse: true},
		{tag: "states pkg=p type=M run=^TestNope$", want: "recorded no transition"},
		{tag: `states pkg=p type=M diagram=""`, want: "diagram= is empty", parse: true},

		{tag: "doc pkg=nope", want: "no package nope"},
		{tag: `doc pkg=""`, want: "pkg= is required", parse: true},
		{tag: "doc pkg=p name=Nope", want: "no doc for Nope"},
		{tag: "doc pkg=p name=Conf.Nope", want: "no doc for Conf.Nope"},
		{tag: `doc pkg=p name=""`, want: "name= is empty", parse: true},
		{tag: "doc pkg=p name=NoDoc", want: "no doc comment for NoDoc"},
		{tag: "doc pkg=p section=Nope", want: "no heading # Nope"},
		{tag: `doc pkg=p section=""`, want: "section= is empty", parse: true},
		{tag: "doc pkg=p section=Hollow", want: "heading # Hollow in the doc of package example.com/p/p has no text"},
		{tag: "doc pkg=p name=Conf.Do section=Notes", want: "heading # Notes in the doc of Conf.Do in example.com/p/p has no text"},

		{tag: "output cmd=nope", want: "no Go command in nope"},
		{tag: `output cmd=""`, want: "cmd= is empty", parse: true},
		{tag: "output cmd=data", want: "no Go command in data"},
		{tag: "output test=p.ExampleNope", want: "no func ExampleNope"},
		{tag: "output test=nope.Example", want: "no package nope"},
		{tag: `output test=""`, want: "test= is empty", parse: true},
		{tag: "output test=p.ExampleNoDoc", want: "has no // Output: comment"},
		{tag: "output test=p.ExampleNoDoc_empty", want: "func ExampleNoDoc_empty has an empty // Output: comment"},
		{tag: `output exec=""`, want: "exec= is empty", parse: true},
		{tag: "output exec=false", want: "exit status 1"},

		{tag: "flags cmd=nope", want: "no Go command in nope"},
		{tag: `flags cmd=""`, want: "cmd= is required", parse: true},
		{tag: "flags cmd=data", want: "no Go command in data"},

		{tag: "docgen root=nope page=docs/s.md", want: "root=nope is not a directory"},
		{tag: `docgen root="" page=docs/s.md`, want: "root= is required", parse: true},
		{tag: "docgen root=data/a.txt page=docs/s.md", want: "is not a directory"},
		{tag: "docgen root=../nope page=docs/s.md", want: "is outside the repository"},
		{tag: "docgen root=sample page=docs/nope.md", want: "no page docs/nope.md in root=sample"},
		{tag: "docgen root=sample page=docs", want: "no page docs in root=sample"},
		{tag: `docgen root=sample page=""`, want: "page= is required", parse: true},
		{tag: "docgen root=sample page=../go.mod", want: "page=../go.mod is outside root=sample"},
		{tag: "docgen root=sample page=docs/nil.md", want: "page docs/nil.md of root=sample holds no text"},
		{tag: "docgen root=sample page=docs/s.md module=nope", want: "does not exist"},
		{tag: `docgen root=sample page=docs/s.md module=""`, want: "module= is empty", parse: true},
		{tag: "docgen root=sample page=docs/s.md module=plain/f.txt", want: "is not a directory"},
		{tag: "docgen root=sample page=docs/s.md module=../p", want: "module=../p is outside root=sample"},
	} {
		t.Run(tc.tag, func(t *testing.T) {
			t.Parallel()
			files := referencesRepo()
			text := "# T\n\n" + referencePair(tc.tag)
			files["docs/p.md"] = text
			root := writeRepo(t, files)
			rc, log := docgen(t, root, "check")
			if rc != 1 {
				t.Fatalf("rc %d, log:\n%s", rc, log)
			}
			wantIn(t, log, tc.want)
			kind, _, _ := strings.Cut(tc.tag, " ")
			if tc.parse {
				wantIn(t, log, "docs/p.md: line 3: docgen:"+kind)
			} else {
				wantIn(t, log, "docs/p.md:3: docgen:"+tc.tag+": ")
			}
			wantNotIn(t, log, "stale:")
			if got := readPage(t, root, "docs/p.md"); got != text {
				t.Errorf("the page changed:\n%s", got)
			}
		})
	}
}

func TestConfigulatorTypeThatDoesNotExist(t *testing.T) {
	t.Parallel()
	files := goModOfTheRepository(t)
	files["c.go"] = configulatorType
	files["docs/p.md"] = "# T\n\n" + referencePair("configulator pkg=. type=Nope")
	rc, log := docgen(t, writeRepo(t, files), "check")
	if rc != 1 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
	wantIn(t, log, "docs/p.md:3: docgen:configulator pkg=. type=Nope: ", "Nope")
}

func TestDanglingReferencesFailUpdate(t *testing.T) {
	t.Parallel()
	files := referencesRepo()
	maps.Copy(files, map[string]string{"docs/p.md": referencePair("doc pkg=p section=Hollow")})
	root := writeRepo(t, files)
	if rc, log := docgen(t, root, "update"); rc != 1 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
}

func TestReferencesThatExist(t *testing.T) {
	t.Parallel()
	got := fill(t, map[string]string{
		"p/p.go":        referencesCode,
		"p/p_test.go":   referencesTest,
		"data/a.txt":    "one\n\nthree\n",
		"data/r.txt":    "# docgen:region used\ntext\n# docgen:end\n",
		"data/crlf.txt": "one\r\ntwo\r\n",
		"data/no.txt":   "one\ntwo",
	}, referencePair("code file=data/a.txt lines=1-3")+
		referencePair("code file=data/crlf.txt lines=2-2")+
		referencePair("code file=data/no.txt lines=2-2")+
		referencePair("code file=data/r.txt region=used")+
		referencePair("doc pkg=p section=Full")+
		referencePair("doc pkg=p section=Last")+
		referencePair("doc pkg=p name=Conf.Do section=Other")+
		referencePair("output test=p.Example")+
		referencePair("output exec=true"))
	wantIn(t, got, "one\n\nthree", "two", "text", "The text.", "The end.", "More text.")
}
