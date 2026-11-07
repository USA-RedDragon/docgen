package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

const envExample = "package main\n\nimport (\n\t\"fmt\"\n\t\"os\"\n)\n\nfunc main() {\n\tfmt.Println(\"[\" + os.Getenv(\"GOFLAGS\") + \"]\", \"[\" + os.Getenv(\"GOMAXPROCS\") + \"]\")\n}\n"

func TestJobs(t *testing.T) {
	t.Parallel()
	for name, c := range map[string]struct {
		args []string
		want string
	}{
		"default": {nil, ""},
		"flag":    {[]string{"-jobs", "3"}, "-p=3"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := writeRepo(t, map[string]string{
				"go.mod":               "module example.com/w\n\ngo 1.24.0\n",
				"examples/env/main.go": envExample,
				"docs/p.md":            "<!-- docgen:output cmd=examples/env -->\n<!-- /docgen:output -->\n",
			})
			if rc, log := docgen(t, root, append([]string{"update"}, c.args...)...); rc != 0 {
				t.Fatalf("rc %d, log:\n%s", rc, log)
			}
			wantIn(t, page(t, root, "p.md"), "["+c.want+"] ["+os.Getenv("GOMAXPROCS")+"]")
		})
	}
}

func TestJobsNegative(t *testing.T) {
	t.Parallel()
	root := writeRepo(t, map[string]string{"docs/p.md": "Text.\n"})
	if rc, log := docgen(t, root, "check", "-jobs", "-1"); rc != 1 || !strings.Contains(log, "jobs") {
		t.Errorf("rc %d, log:\n%s", rc, log)
	}
}

func parallelRepo(t *testing.T) string {
	t.Helper()
	files := map[string]string{"go.mod": "module example.com/w\n\ngo 1.24.0\n"}
	var b strings.Builder
	for i := range 6 {
		n := strconv.Itoa(i)
		files["examples/e"+n+"/main.go"] = "package main\n\nimport \"fmt\"\n\nfunc main() { fmt.Println(\"out " + n + "\") }\n"
		files["p"+n+"/p.go"] = "package p" + n + "\n\n// S is a state.\ntype S int\n\n// The states.\nconst (\n\tOff S = iota\n\tOn\n)\n\n// M is a machine.\ntype M struct{ s S }\n\n// State returns the state.\nfunc (m *M) State() S { return m.s }\n\n// Go moves it.\nfunc (m *M) Go() { m.s = On }\n"
		files["p"+n+"/p_test.go"] = "package p" + n + "\n\nimport (\n\t\"fmt\"\n\t\"testing\"\n)\n\nfunc TestM(t *testing.T) { (&M{}).Go() }\n\nfunc ExampleM() {\n\tfmt.Println(\"ex " + n + "\")\n\t// Output: ex " + n + "\n}\n"
		files["cmd/c"+n+"/main.go"] = "package main\n\nimport \"flag\"\n\nfunc main() { flag.Int(\"n" + n + "\", 1, \"a flag\"); flag.Parse() }\n"
		fmt.Fprintf(&b, "<!-- docgen:output cmd=examples/e%s -->\n<!-- /docgen:output -->\n\n<!-- docgen:output cmd=examples/none%s -->\n<!-- /docgen:output -->\n\n<!-- docgen:states pkg=p%s type=M -->\n<!-- /docgen:states -->\n\n<!-- docgen:output test=p%s.ExampleM -->\n<!-- /docgen:output -->\n\n<!-- docgen:flags cmd=cmd/c%s -->\n<!-- /docgen:flags -->\n\n", n, n, n, n, n)
	}
	files["docs/p.md"] = b.String()
	return writeRepo(t, files)
}

var progressLine = regexp.MustCompile(`(?m)^docgen: (running|recording) .*\n`)

func TestJobsKeepTheOutput(t *testing.T) {
	t.Parallel()
	root := parallelRepo(t)
	_, want := docgen(t, root, "check", "-jobs", "1")
	want = progressLine.ReplaceAllString(want, "")
	wantIn(t, want, "stale: docs/p.md:1: ", "no Go command in examples/none0")
	for _, jobs := range []string{"2", "16", "16"} {
		_, got := docgen(t, root, "check", "-jobs", jobs)
		if got = progressLine.ReplaceAllString(got, ""); got != want {
			t.Errorf("-jobs %s:\n%s\nwant:\n%s", jobs, got, want)
		}
	}
}

func TestLogLinesAreWhole(t *testing.T) {
	t.Parallel()
	root := parallelRepo(t)
	_, log := docgen(t, root, "check", "-jobs", "16")
	for l := range strings.Lines(log) {
		if strings.Count(l, "docgen: ") > 1 && !strings.Contains(l, ": docgen:") {
			t.Errorf("a torn line: %q", l)
		}
	}
}

const taggedTree = "//go:build special\n\n// Package p has a tag.\npackage p\n\n// N is behind the tag.\nconst N = 1\n"

func taggedRepo(t *testing.T) string {
	t.Helper()
	return writeRepo(t, map[string]string{
		"go.mod":          "module example.com/w\n\ngo 1.24.0\n",
		"p/p.go":          taggedTree,
		"p/other.go":      "package p\n",
		"tools/x/go.mod":  "module example.com/x\n\ngo 1.24.0\n",
		"tools/x/main.go": "//go:build special\n\n// Command x has a tag.\npackage main\n\nfunc main() {}\n",
		"docs/p.md":       "<!-- docgen:const pkg=p name=N --><!-- /docgen:const -->\n\n<!-- docgen:doc pkg=tools/x -->\n<!-- /docgen:doc -->\n",
	})
}

func TestTags(t *testing.T) {
	t.Parallel()
	root := taggedRepo(t)
	if rc, log := docgen(t, root, "update"); rc != 1 || !strings.Contains(log, "N is not a constant") {
		t.Fatalf("no tags: rc %d, log:\n%s", rc, log)
	}
	if rc, log := docgen(t, root, "update", "-tags", "other, special"); rc != 0 {
		t.Fatalf("-tags: rc %d, log:\n%s", rc, log)
	}
	wantIn(t, page(t, root, "p.md"), "`1`", "Command x has a tag.")
	if rc, log := docgen(t, root, "check", "-tags", "special"); rc != 0 {
		t.Fatalf("check: rc %d, log:\n%s", rc, log)
	}
}

func TestTagsInTheConfigFile(t *testing.T) {
	t.Parallel()
	root := taggedRepo(t)
	writeTree(t, root, map[string]string{".docgen.yaml": "tags: special\n"})
	if rc, log := docgen(t, root, "update"); rc != 0 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
}

//nolint:paralleltest // sets the environment
func TestTagsInTheEnvironment(t *testing.T) {
	t.Setenv("DOCGEN_TAGS", "special")
	root := taggedRepo(t)
	if rc, log := docgen(t, root, "update"); rc != 0 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
}

func TestDefaultScan(t *testing.T) {
	t.Parallel()
	root := pageRepo(t, map[string]string{
		"README.md":         goodPair + "\n",
		"sub/x.md":          goodPair + "\n",
		"testdata/a.md":     badPair + "\n",
		"vendor/b.md":       badPair + "\n",
		"node_modules/c.md": badPair + "\n",
		".hidden/d.md":      badPair + "\n",
		"_draft/e.md":       badPair + "\n",
	})
	rc, log := docgen(t, root, "check")
	if rc != 1 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
	wantIn(t, log, "docgen: 2 stale", "stale: README.md:1:", "stale: sub/x.md:1:")
	if rc, log := docgen(t, root, "update"); rc != 0 {
		t.Fatalf("update: rc %d, log:\n%s", rc, log)
	}
	wantIn(t, readPage(t, root, "README.md"), "`1`")
	wantIn(t, readPage(t, root, "sub/x.md"), "`1`")
}

func TestScanSkipsWhatGitIgnores(t *testing.T) {
	t.Parallel()
	root := gitRepo(t, map[string]string{
		".gitignore":    "out/\nskip.md\n",
		"go.mod":        pageModule,
		"p.go":          "package p\n\n// N is a number.\nconst N = 1\n",
		"docs/p.md":     goodPair + "\n",
		"out/a.md":      badPair + "\n",
		"skip.md":       badPair + "\n",
		"docs/out/b.md": badPair + "\n",
	})
	for _, args := range [][]string{{"update"}, {"update", "docs"}} {
		if rc, log := docgen(t, root, args...); rc != 0 {
			t.Fatalf("%q: rc %d, log:\n%s", args, rc, log)
		}
	}
	wantIn(t, readPage(t, root, "docs/p.md"), "`1`")
}

func TestNarrowedScan(t *testing.T) {
	t.Parallel()
	for name, c := range map[string]struct {
		config string
		args   []string
	}{
		"setting":  {"docs: sub\n", nil},
		"argument": {"", []string{"sub"}},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := pageRepo(t, map[string]string{
				".docgen.yaml": c.config,
				"README.md":    badPair + "\n",
				"sub/x.md":     goodPair + "\n",
			})
			if rc, log := docgen(t, root, append([]string{"update"}, c.args...)...); rc != 0 {
				t.Fatalf("rc %d, log:\n%s", rc, log)
			}
			wantIn(t, readPage(t, root, "sub/x.md"), "`1`")
			if got := readPage(t, root, "README.md"); got != badPair+"\n" {
				t.Errorf("README.md changed:\n%s", got)
			}
		})
	}
}

func TestFilesSettingIsGone(t *testing.T) {
	t.Parallel()
	root := pageRepo(t, map[string]string{".docgen.yaml": "files: [README.md]\n"})
	rc, log := docgen(t, root, "check")
	if rc != 1 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
	wantIn(t, log, `unknown field "files"`)
}

func TestDiagramOfTheReadme(t *testing.T) {
	t.Parallel()
	root := writeRepo(t, map[string]string{
		"go.mod":      "module example.com/w\n\ngo 1.24.0\n",
		"p/p.go":      collidingMachines,
		"p/p_test.go": "package p\n\nimport \"testing\"\n\nfunc TestM(t *testing.T) { (&Ab{}).Go() }\n",
		"README.md":   "<!-- docgen:states pkg=p type=Ab -->\n<!-- /docgen:states -->\n",
		"docs/p.md":   "Text.\n",
	})
	rc, log, diff := docgenDiff(t, root, "check", "-diff")
	if rc != 1 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
	wantIn(t, log, "stale: states/p-ab.svg (from README.md:1:")
	wantIn(t, diff, "--- README.md\n+++ README.md\n", "--- states/p-ab.svg\n+++ states/p-ab.svg\n@@ -0,0 +1,")
	if rc, log := docgen(t, root, "update"); rc != 0 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
	if _, err := os.Stat(filepath.Join(root, "states/p-ab.svg")); err != nil {
		t.Error(err)
	}
}

func TestDiff(t *testing.T) {
	t.Parallel()
	root := pageRepo(t, map[string]string{
		"docs/a.md": "one\ntwo\nthree\nfour\n" + goodPair + "\nfive\nsix\nseven\neight\n",
		"docs/b.md": goodPair + "\n",
	})
	rc, log, diff := docgenDiff(t, root, "check", "-diff")
	if rc != 1 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
	wantLog := `stale: docs/a.md:5: docgen:const pkg=. name=N
stale: docs/b.md:1: docgen:const pkg=. name=N
docgen: 2 stale; run docgen update and commit the result
`
	if log != wantLog {
		t.Errorf("log:\n%s\nwant:\n%s", log, wantLog)
	}
	want := `--- docs/a.md
+++ docs/a.md
@@ -2,7 +2,7 @@
 two
 three
 four
-<!-- docgen:const pkg=. name=N --><!-- /docgen:const -->
+<!-- docgen:const pkg=. name=N -->` + "`1`" + `<!-- /docgen:const -->
 five
 six
 seven
--- docs/b.md
+++ docs/b.md
@@ -1 +1 @@
-<!-- docgen:const pkg=. name=N --><!-- /docgen:const -->
+<!-- docgen:const pkg=. name=N -->` + "`1`" + `<!-- /docgen:const -->
`
	if diff != want {
		t.Errorf("diff:\n%s\nwant:\n%s", diff, want)
	}
}

func TestDiffIsOnlyForCheck(t *testing.T) {
	t.Parallel()
	text := goodPair + "\n"
	root := pageRepo(t, map[string]string{"docs/a.md": text})
	if rc, log := docgen(t, root, "update", "-diff"); rc != 2 || !strings.Contains(log, "-diff") {
		t.Errorf("rc %d, log:\n%s", rc, log)
	}
	if got := readPage(t, root, "docs/a.md"); got != text {
		t.Errorf("the page changed:\n%s", got)
	}
}

func TestEscapesOfQuotedArguments(t *testing.T) {
	t.Parallel()
	body := func(kind, args string) string {
		return "<!-- docgen:" + kind + " " + args + " -->\n<!-- /docgen:" + kind + " -->\n"
	}
	root := pageRepo(t, map[string]string{"docs/p.md": body("output", `exec="echo \"hi\""`) +
		body("text", `text="printf \"a\\nb\"\nnext"`) +
		body("output", `exec="printf '%s' 'a\b'"`)})
	if rc, log := docgen(t, root, "update"); rc != 0 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
	wantIn(t, readPage(t, root, "docs/p.md"), "\nhi\n", "\nprintf \"a\\nb\"\nnext\n", "\na\\b\n")
}

func TestCodeRaw(t *testing.T) {
	t.Parallel()
	const src = "// Package m is a sample.\npackage m\n\n// docgen:region r\n\n// F is a func.\nfunc F() {}\n\n// docgen:end\n"
	for _, c := range []struct{ args, want string }{
		{"symbol=F raw=true", "// F is a func.\nfunc F() {}"},
		{"symbol=package raw=true", "// Package m is a sample.\npackage m"},
		{"lines=4-9 raw=true", "// docgen:region r\n\n// F is a func.\nfunc F() {}\n\n// docgen:end"},
		{"raw=true", strings.TrimSuffix(src, "\n")},
		{"symbol=F", "func F() {}"},
	} {
		root := t.TempDir()
		page := "<!-- docgen:code file=m.go " + c.args + " -->\n<!-- /docgen:code -->\n\n<!-- docgen:code file=m.go region=r -->\n<!-- /docgen:code -->\n"
		writeTree(t, root, map[string]string{
			".git/HEAD": "ref: refs/heads/main\n",
			"go.mod":    "module example.com/m\n\ngo 1.21\n",
			"m.go":      src,
			"page.md":   page,
		})
		if rc, log := docgen(t, root, "update"); rc != 0 {
			t.Fatalf("%s: rc %d, log:\n%s", c.args, rc, log)
		}
		got, err := os.ReadFile(filepath.Join(root, "page.md"))
		if err != nil {
			t.Fatal(err)
		}
		if want := "```go\n" + c.want + "\n```\n"; !strings.Contains(string(got), want) {
			t.Errorf("%s: page:\n%s\nwant:\n%s", c.args, got, want)
		}
		if rc, log := docgen(t, root, "check"); rc != 0 {
			t.Errorf("%s: check: rc %d, log:\n%s", c.args, rc, log)
		}
	}
}
