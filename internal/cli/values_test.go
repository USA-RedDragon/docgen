package cli

import (
	"go/build"
	"os/exec"
	"strings"
	"testing"
)

// otherLevel is a package with an exported alias of an unexported type and a
// complex constant, each with a String method.
const otherLevel = `package o

type level int

func (level) String() string { return "lv" }

// Level is an alias of an unexported type.
type Level = level

// Info is a level.
const Info Level = 1

// Wave is a complex type.
type Wave complex128

func (Wave) String() string { return "wave" }

// Peak is a complex constant.
const Peak Wave = 1i
`

const plainStringer = `
// T has a String method.
type T int

func (T) String() string { return "the-t" }

// V is a T.
const V T = 1
`

func TestUnusableImportOfAValue(t *testing.T) {
	t.Parallel()
	for name, c := range map[string]struct{ field, zero, value string }{
		"unexported type": {"o.Level", "0", "o.Info"},
		"complex":         {"any", "nil", "o.Peak"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := fill(t, map[string]string{
				"o/o.go": otherLevel,
				"p/p.go": "package p\n\nimport \"example.com/w/o\"\n" + plainStringer + "\n// Cfg holds settings.\ntype Cfg struct {\n\tL " + c.field + "\n}\n\n// New applies the default.\nfunc New(c Cfg) Cfg {\n\tif c.L == " + c.zero + " {\n\t\tc.L = " + c.value + "\n\t}\n\treturn c\n}\n",
			}, "<!-- docgen:config pkg=p type=Cfg -->\n<!-- /docgen:config -->\n\n<!-- docgen:const pkg=p name=V --><!-- /docgen:const -->\n")
			wantIn(t, got, "| `L` | `"+c.field+"` | `"+c.value+"`", "`the-t`")
		})
	}
}

func TestInvalidPrintLeavesOtherValues(t *testing.T) {
	t.Parallel()
	root := writeRepo(t, map[string]string{
		"go.mod": pageModule,
		"o/o.go": "package o\n\n// Mode has a String method.\ntype Mode int\n\nfunc (Mode) String() string { return \"m\" }\n",
		"p/p.go": "package p\n\nimport \"example.com/p/o\"\n" + plainStringer + "\n// Cfg holds settings.\ntype Cfg struct {\n\tM o.Mode\n}\n\n// New applies the default.\nfunc New(c Cfg) Cfg {\n\tif c.M == 0 {\n\t\tc.M = 1\n\t}\n\treturn c\n}\n",
		"docs/p.md": "<!-- docgen:config pkg=p type=Cfg print=bad -->\n<!-- /docgen:config -->\n\n" +
			"<!-- docgen:const pkg=p name=V --><!-- /docgen:const -->\n",
	})
	rc, log := docgen(t, root, "update")
	if rc != 1 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
	wantIn(t, log, "print=bad is not go")
	wantNotIn(t, log, "not used")
}

func TestTestMainWithNoRun(t *testing.T) {
	t.Parallel()
	root := writeRepo(t, map[string]string{
		"go.mod":      pageModule,
		"p/p.go":      "package p\n" + plainStringer,
		"p/p_test.go": "package p\n\nimport (\n\t\"os\"\n\t\"testing\"\n)\n\nfunc TestMain(*testing.M) { os.Exit(0) }\n",
		"docs/p.md":   "<!-- docgen:const pkg=p name=V --><!-- /docgen:const -->\n",
	})
	rc, log := docgen(t, root, "update")
	if rc != 1 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
	wantIn(t, log, "TestMain", "m.Run")
	wantNotIn(t, log, "values.txt", "values-")
}

func constPages(names ...string) string {
	var b strings.Builder
	for _, n := range names {
		b.WriteString(n + " is <!-- docgen:const pkg=p name=" + n + " --><!-- /docgen:const -->.\n\n")
	}
	return b.String()
}

func TestConstEmptyAndBlankLine(t *testing.T) {
	t.Parallel()
	got := fill(t, map[string]string{
		"p/p.go": "package p\n\nconst Empty = \"\"\n\nconst Two = \"a\\n\\nb\"\n",
	}, constPages("Empty", "Two"))
	wantIn(t, got, "name=Empty -->`\"\"`<!--", "name=Two -->`a  b`<!--")
}

func TestConstTextLikeABadVerb(t *testing.T) {
	t.Parallel()
	got := fill(t, map[string]string{
		"p/p.go": "package p\n\nconst Odd = \"%!v(x)\"\n\n// T has a String method.\ntype T int\n\nfunc (T) String() string { return \"%!v(y)\" }\n\n// V is a T.\nconst V T = 1\n",
	}, constPages("Odd", "V")+"<!-- docgen:const pkg=p name=Odd print=raw --><!-- /docgen:const -->\n")
	wantIn(t, got, "name=Odd -->`%!v(x)`<!--", "name=V -->`%!v(y)`<!--", "name=Odd print=raw -->`%!v(x)`<!--")
}

func TestConstPanicIsAnError(t *testing.T) {
	t.Parallel()
	root := writeRepo(t, map[string]string{
		"go.mod":    pageModule,
		"p/p.go":    "package p\n\n// T has a String method.\ntype T int\n\nfunc (T) String() string { panic(\"boom\") }\n\n// V is a T.\nconst V T = 1\n",
		"docs/p.md": constPages("V"),
	})
	rc, log := docgen(t, root, "update")
	if rc != 1 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
	wantIn(t, log, "the value has no text", "boom")
}

func TestConstOutOfRangeVerbs(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ source, verb, want string }{
		{"const F = 1e400", "%g", "print=%g is not valid for the value"},
		{"const F = 1e400", "%e", "print=%e is not valid for the value"},
		{"const F = 1e-400", "%g", "print=%g is not valid for the value"},
		{"const F = 1 << 100", "%T", "print=%T is not valid for the value"},
		{"const F = 1 << 100", "%s", "print=%s is not valid for the value"},
	} {
		root := writeRepo(t, map[string]string{
			"go.mod":    pageModule,
			"p/p.go":    "package p\n\n" + c.source + "\n",
			"docs/p.md": "<!-- docgen:const pkg=p name=F print=" + c.verb + " --><!-- /docgen:const -->\n",
		})
		rc, log := docgen(t, root, "update")
		if rc != 1 {
			t.Fatalf("%s %s: rc %d, log:\n%s", c.source, c.verb, rc, log)
		}
		wantIn(t, log, c.want)
	}
}

func TestPrintNameInThePackageOfTheMarker(t *testing.T) {
	t.Parallel()
	got := fill(t, map[string]string{
		"o/o.go": "package o\n\n// Level is a level.\ntype Level int\n\n// Low is a level.\nconst Low Level = 1\n",
		"p/p.go": "package p\n\nimport (\n\t\"time\"\n\n\t\"example.com/w/o\"\n)\n\nconst D = 1500 * time.Millisecond\n\nconst Sec = time.Second\n\nvar Var = o.Low\n\nvar Long = 60 * time.Second\n",
	}, "<!-- docgen:const pkg=p name=D print=name --><!-- /docgen:const -->\n\n"+
		"<!-- docgen:const pkg=p name=Sec print=name --><!-- /docgen:const -->\n\n"+
		"<!-- docgen:const pkg=p name=Var print=name --><!-- /docgen:const -->\n\n"+
		"<!-- docgen:const pkg=p name=Long print=name --><!-- /docgen:const -->\n")
	wantIn(t, got, "name=D print=name -->`D`<!--", "name=Sec print=name -->`Sec`<!--", "name=Var print=name -->`o.Low`<!--", "name=Long print=name -->`time.Minute`<!--")
}

func TestConfigPrintNameOfALiteral(t *testing.T) {
	t.Parallel()
	got := fill(t, map[string]string{
		"p/p.go": "package p\n\n// Mode is a mode.\ntype Mode int\n\n// Two names of one value.\nconst (\n\tA Mode = 1\n\tB Mode = 1\n\tC Mode = 2\n)\n\n// Cfg holds settings.\ntype Cfg struct {\n\tM Mode\n\tN Mode\n}\n\n// New applies the defaults.\nfunc New(c Cfg) Cfg {\n\tif c.M == 0 {\n\t\tc.M = 1\n\t}\n\tif c.N == 0 {\n\t\tc.N = 2\n\t}\n\treturn c\n}\n",
	}, "<!-- docgen:config pkg=p type=Cfg print=name -->\n<!-- /docgen:config -->\n")
	wantIn(t, got, "| `M` | `Mode` | `1` |", "| `N` | `Mode` | `2` |")
}

func TestCgoFieldType(t *testing.T) {
	t.Parallel()
	if !build.Default.CgoEnabled {
		t.Skip("cgo is off")
	}
	if _, err := exec.LookPath("gcc"); err != nil {
		t.Skip("no C compiler")
	}
	got := fill(t, map[string]string{
		"p/p.go": "// Package p uses C.\npackage p\n\n// int one(void) { return 1; }\nimport \"C\"\n\n// Cfg holds settings.\ntype Cfg struct {\n\t// N is a C int.\n\tN C.int\n\t// S is a C string.\n\tS *C.char\n}\n",
	}, "<!-- docgen:config pkg=p type=Cfg -->\n<!-- /docgen:config -->\n")
	wantIn(t, got, "| `N` | `C.int` |", "| `S` | `*C.char` |")
	wantNotIn(t, got, "_Ctype_")
}

//nolint:paralleltest // sets the environment
func TestNoGoCommand(t *testing.T) {
	root := writeRepo(t, map[string]string{
		"go.mod":    pageModule,
		"p/p.go":    "package p\n\nconst V = 7\n",
		"docs/p.md": constPages("V"),
	})
	t.Setenv("PATH", t.TempDir())
	rc, log := docgen(t, root, "update")
	if rc != 1 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
	wantIn(t, log, "the go command is not on PATH")
}
