package cli

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

const leak = "leaked-value"

// docgenDiff runs docgen and returns the exit code, the standard error and the
// standard output.
func docgenDiff(t *testing.T, dir string, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	rc := runTo(t.Context(), dir, args, &stdout, &stderr)
	return rc, stderr.String(), stdout.String()
}

const outsidePkg = "// Package x is " + leak + ".\npackage x\n\n" +
	"// Secret is " + leak + ".\nconst Secret = \"" + leak + "\"\n\n" +
	"// C is " + leak + ".\ntype C struct {\n\t// A is " + leak + ".\n\tA int\n}\n\n" +
	"// S is " + leak + ".\ntype S int\n\n// T is " + leak + ".\ntype T struct{ s S }\n\n" +
	"// State returns the state.\nfunc (t *T) State() S { return t.s }\n\n// Go moves it.\nfunc (t *T) Go() { t.s = 1 }\n"

// ranMain returns the source of a program that makes the file ran.
func ranMain(ran string) string {
	return "package main\n\nimport \"os\"\n\nfunc main() {\n\tif err := os.WriteFile(" + strconv.Quote(ran) + ", nil, 0o600); err != nil {\n\t\tpanic(err)\n\t}\n\tprintln(\"" + leak + "\")\n}\n"
}

// outsideRepo writes files in repo and outside below a new directory, and
// returns the two directories. Tests link the first to the second.
func outsideRepo(t *testing.T, files, outsideFiles map[string]string) (string, string) {
	t.Helper()
	parent, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	root, outside := filepath.Join(parent, "repo"), filepath.Join(parent, "outside")
	if files["go.mod"] == "" {
		files["go.mod"] = pageModule
	}
	writeTree(t, root, files)
	writeTree(t, outside, outsideFiles)
	return root, outside
}

func link(t *testing.T, target, name string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, name); err != nil {
		t.Fatal(err)
	}
}

func wantRefused(t *testing.T, root, want string) {
	t.Helper()
	const page = "docs/p.md"
	before := readPage(t, root, page)
	for _, cmd := range []string{"check", "update"} {
		rc, log := docgen(t, root, cmd)
		if rc != 1 {
			t.Fatalf("%s: rc %d, log:\n%s", cmd, rc, log)
		}
		wantIn(t, log, want)
		wantNotIn(t, log, leak)
	}
	if got := readPage(t, root, page); got != before {
		t.Errorf("the page changed:\n%s", got)
	}
}

func TestPkgThroughASymlink(t *testing.T) {
	t.Parallel()
	for name, marker := range map[string]string{
		"const":        "<!-- docgen:const pkg=pkglink name=Secret --><!-- /docgen:const -->",
		"doc":          "<!-- docgen:doc pkg=pkglink -->\n<!-- /docgen:doc -->",
		"doc name":     "<!-- docgen:doc pkg=pkglink name=Secret -->\n<!-- /docgen:doc -->",
		"config":       "<!-- docgen:config pkg=pkglink type=C -->\n<!-- /docgen:config -->",
		"errors":       "<!-- docgen:errors pkg=pkglink type=C -->\n<!-- /docgen:errors -->",
		"states":       "<!-- docgen:states pkg=pkglink type=T -->\n<!-- /docgen:states -->",
		"configulator": "<!-- docgen:configulator pkg=pkglink type=C -->\n<!-- /docgen:configulator -->",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root, outside := outsideRepo(t, map[string]string{"docs/p.md": marker + "\n"},
				map[string]string{"pkgx/x.go": outsidePkg})
			link(t, filepath.Join(outside, "pkgx"), filepath.Join(root, "pkglink"))
			wantRefused(t, root, "pkg=pkglink is outside the repository")
		})
	}
}

func TestPkgInTheRepositoryThroughASymlink(t *testing.T) {
	t.Parallel()
	root := pageRepo(t, map[string]string{
		"real/x.go": "package x\n\nconst N = 5\n",
		"docs/p.md": "<!-- docgen:const pkg=pkglink name=N --><!-- /docgen:const -->\n",
	})
	link(t, "real", filepath.Join(root, "pkglink"))
	wantIn(t, updateTwice(t, root, "docs/p.md"), "`5`")
}

func TestFileLinkInAPackage(t *testing.T) {
	t.Parallel()
	for name, c := range map[string]struct {
		files map[string]string
		dir   string
		arg   string
	}{
		"module": {map[string]string{
			"p/p.go":    "package x\n",
			"docs/p.md": "<!-- docgen:const pkg=p name=Secret --><!-- /docgen:const -->\n",
		}, "p", "pkg=p"},
		"module doc": {map[string]string{
			"p/p.go":    "package x\n",
			"docs/p.md": "<!-- docgen:doc pkg=p -->\n<!-- /docgen:doc -->\n",
		}, "p", "pkg=p"},
		"other module": {map[string]string{
			"tool/go.mod": "module example.com/tool\n\ngo 1.24.0\n",
			"tool/t.go":   "package x\n",
			"docs/p.md":   "<!-- docgen:doc pkg=tool -->\n<!-- /docgen:doc -->\n",
		}, "tool", "pkg=tool"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root, outside := outsideRepo(t, c.files, map[string]string{"x.go": outsidePkg})
			link(t, filepath.Join(outside, "x.go"), filepath.Join(root, c.dir, "leak.go"))
			wantRefused(t, root, c.arg+" is outside the repository")
		})
	}
}

func TestFileLinkInTheRepository(t *testing.T) {
	t.Parallel()
	root := pageRepo(t, map[string]string{
		"src/k.txt":   "// Package t is kept.\npackage t\n",
		"tool/go.mod": "module example.com/tool\n\ngo 1.24.0\n",
		"docs/p.md":   "<!-- docgen:doc pkg=tool -->\n<!-- /docgen:doc -->\n",
	})
	link(t, "../src/k.txt", filepath.Join(root, "tool/k.go"))
	wantIn(t, updateTwice(t, root, "docs/p.md"), "Package t is kept.")
}

func TestExampleThroughASymlink(t *testing.T) {
	t.Parallel()
	const page = "<!-- docgen:output cmd=examples/exlink -->\n<!-- /docgen:output -->\n"
	for name, setup := range map[string]func(t *testing.T, root, outside string){
		"directory": func(t *testing.T, root, outside string) {
			t.Helper()
			link(t, filepath.Join(outside, "ex"), filepath.Join(root, "examples/exlink"))
		},
		"parent": func(t *testing.T, root, outside string) {
			t.Helper()
			if err := os.Mkdir(filepath.Join(outside, "examples"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Rename(filepath.Join(outside, "ex"), filepath.Join(outside, "examples/exlink")); err != nil {
				t.Fatal(err)
			}
			link(t, filepath.Join(outside, "examples"), filepath.Join(root, "examples"))
		},
		"file": func(t *testing.T, root, outside string) {
			t.Helper()
			link(t, filepath.Join(outside, "ex/main.go"), filepath.Join(root, "examples/exlink/main.go"))
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root, outside := outsideRepo(t, map[string]string{"docs/p.md": page}, nil)
			ran := filepath.Join(outside, "ran")
			writeTree(t, outside, map[string]string{"ex/main.go": ranMain(ran)})
			setup(t, root, outside)
			wantRefused(t, root, "cmd=examples/exlink is outside the repository")
			if _, err := os.Stat(ran); err == nil {
				t.Error("the program outside the repository ran")
			}
		})
	}
}

func TestExampleInTheRepositoryThroughASymlink(t *testing.T) {
	t.Parallel()
	root := pageRepo(t, map[string]string{
		"demo/main.go": "package main\n\nimport \"fmt\"\n\nfunc main() { fmt.Println(\"kept\") }\n",
		"docs/p.md":    "<!-- docgen:output cmd=examples/exlink -->\n<!-- /docgen:output -->\n",
	})
	link(t, "../demo", filepath.Join(root, "examples/exlink"))
	wantIn(t, updateTwice(t, root, "docs/p.md"), "kept")
}

func TestTestThroughASymlink(t *testing.T) {
	t.Parallel()
	root, outside := outsideRepo(t, map[string]string{
		"docs/p.md": "<!-- docgen:output test=pkglink.Example -->\n<!-- /docgen:output -->\n",
	}, nil)
	ran := filepath.Join(outside, "ran")
	writeTree(t, outside, map[string]string{
		"pkgx/x.go": "package x\n",
		"pkgx/x_test.go": "package x\n\nimport (\n\t\"fmt\"\n\t\"os\"\n)\n\nfunc Example() {\n\t_ = os.WriteFile(" + strconv.Quote(ran) +
			", nil, 0o600)\n\tfmt.Println(\"" + leak + "\")\n\t// Output: " + leak + "\n}\n",
	})
	link(t, filepath.Join(outside, "pkgx"), filepath.Join(root, "pkglink"))
	wantRefused(t, root, "pkg=pkglink is outside the repository")
	if _, err := os.Stat(ran); err == nil {
		t.Error("the Example outside the repository ran")
	}
}

func TestCommandWithAFileLink(t *testing.T) {
	t.Parallel()
	root, outside := outsideRepo(t, map[string]string{
		"docs/p.md": "<!-- docgen:usage cmd=cmd/x -->\n<!-- /docgen:usage -->\n",
	}, map[string]string{"main.go": "package main\n\nfunc main() { println(\"usage: " + leak + "\") }\n"})
	link(t, filepath.Join(outside, "main.go"), filepath.Join(root, "cmd/x/main.go"))
	wantRefused(t, root, "cmd=cmd/x is outside the repository")
}

func TestImportedPackageThroughASymlink(t *testing.T) {
	t.Parallel()
	root, outside := outsideRepo(t, map[string]string{
		"p/p.go": "// Package p is a package.\npackage p\n\nimport \"example.com/p/pkglink\"\n\n" +
			"// T is a type.\ntype T struct{}\n\n// Do returns x.ErrX.\nfunc (T) Do() error { return x.ErrX }\n",
		"docs/p.md": "<!-- docgen:errors pkg=p type=T -->\n<!-- /docgen:errors -->\n",
	}, map[string]string{
		"pkgx/x.go": "package x\n\nimport \"errors\"\n\n// ErrX is " + leak + ".\nvar ErrX = errors.New(\"x\")\n",
	})
	link(t, filepath.Join(outside, "pkgx"), filepath.Join(root, "pkglink"))
	docgen(t, root, "update")
	wantNotIn(t, readPage(t, root, "docs/p.md"), leak)
}

const machineFiles = "package p\n\n// S is a state.\ntype S int\n\nconst (\n\t// Off is off.\n\tOff S = iota\n\t// On is on.\n\tOn\n)\n\n" +
	"// M is a machine.\ntype M struct{ s S }\n\n// State returns the state.\nfunc (m *M) State() S { return m.s }\n\n// Go moves it.\nfunc (m *M) Go() { m.s = On }\n"

const machineTest = "package p\n\nimport \"testing\"\n\nfunc TestM(t *testing.T) { (&M{}).Go() }\n"

const statesMarker = "<!-- docgen:states pkg=p type=M -->\n<!-- /docgen:states -->\n"

func machineRepo(page string) map[string]string {
	return map[string]string{"p/p.go": machineFiles, "p/p_test.go": machineTest, "docs/p.md": page}
}

func TestDiagramThroughASymlink(t *testing.T) {
	t.Parallel()
	const victim = leak + "\n"
	for name, setup := range map[string]func(t *testing.T, root, outside string){
		"directory": func(t *testing.T, root, outside string) {
			t.Helper()
			link(t, outside, filepath.Join(root, "docs/states"))
		},
		"file": func(t *testing.T, root, outside string) {
			t.Helper()
			link(t, filepath.Join(outside, "p-m.svg"), filepath.Join(root, "docs/states/p-m.svg"))
		},
		"dangling file": func(t *testing.T, root, outside string) {
			t.Helper()
			if err := os.Remove(filepath.Join(outside, "p-m.svg")); err != nil {
				t.Fatal(err)
			}
			link(t, filepath.Join(outside, "p-m.svg"), filepath.Join(root, "docs/states/p-m.svg"))
		},
		"dangling directory": func(t *testing.T, root, outside string) {
			t.Helper()
			link(t, filepath.Join(outside, "new"), filepath.Join(root, "docs/states"))
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root, outside := outsideRepo(t, machineRepo(statesMarker), map[string]string{"p-m.svg": victim})
			setup(t, root, outside)
			rc, log, diff := docgenDiff(t, root, "check", "-diff")
			if rc != 1 {
				t.Fatalf("rc %d, log:\n%s", rc, log)
			}
			wantIn(t, log, "docs/states/p-m.svg is outside the repository")
			wantNotIn(t, log+diff, leak)
			if rc, log := docgen(t, root, "update"); rc != 1 {
				t.Fatalf("rc %d, log:\n%s", rc, log)
			}
			if b, err := os.ReadFile(filepath.Join(outside, "p-m.svg")); err == nil && string(b) != victim {
				t.Errorf("the file outside the repository changed:\n%s", b)
			}
			if _, err := os.Stat(filepath.Join(outside, "new")); err == nil {
				t.Error("update made a directory outside the repository")
			}
			if strings.Contains(readPage(t, root, "docs/p.md"), "Transitions") {
				t.Error("the page changed")
			}
		})
	}
}

func TestDiagramThroughALinkInTheRepository(t *testing.T) {
	t.Parallel()
	root := pageRepo(t, machineRepo(statesMarker))
	if err := os.Mkdir(filepath.Join(root, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	link(t, "../assets", filepath.Join(root, "docs/states"))
	updateTwice(t, root, "docs/p.md")
	if _, err := os.Stat(filepath.Join(root, "assets/p-m.svg")); err != nil {
		t.Error(err)
	}
}

func TestDiagramOfDocsOutsideTheRepository(t *testing.T) {
	t.Parallel()
	root, outside := outsideRepo(t, map[string]string{"p/p.go": machineFiles, "p/p_test.go": machineTest},
		map[string]string{"docs/p.md": statesMarker, "docs/assets/keep": "", "other/keep": ""})
	docs := filepath.Join(outside, "docs")
	if rc, log := docgen(t, root, "update", docs); rc != 0 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
	if _, err := os.Stat(filepath.Join(docs, "states/p-m.svg")); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(docs, "states"), filepath.Join(docs, "assets/states")); err != nil {
		t.Fatal(err)
	}
	link(t, "assets/states", filepath.Join(docs, "states"))
	if rc, log := docgen(t, root, "check", docs); rc != 0 {
		t.Fatalf("a link in the docs directory: rc %d, log:\n%s", rc, log)
	}
	if err := os.Remove(filepath.Join(docs, "states")); err != nil {
		t.Fatal(err)
	}
	link(t, filepath.Join(outside, "other"), filepath.Join(docs, "states"))
	rc, log := docgen(t, root, "update", docs)
	if rc != 1 {
		t.Fatalf("a link out of the docs directory: rc %d, log:\n%s", rc, log)
	}
	wantIn(t, log, "is outside the docs directory")
	if _, err := os.Stat(filepath.Join(outside, "other/p-m.svg")); err == nil {
		t.Error("update wrote through the link")
	}
}

func TestExcerptOfADiagram(t *testing.T) {
	t.Parallel()
	files := machineRepo(statesMarker)
	files["docs/states/p-m.svg"] = "<svg>old</svg>\n"
	files["docs/b.md"] = "<!-- docgen:code file=docs/states/p-m.svg -->\n```text\n<svg>old</svg>\n```\n<!-- /docgen:code -->\n"
	root := pageRepo(t, files)
	if rc, log := docgen(t, root, "update"); rc != 0 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
	if rc, log := docgen(t, root, "check"); rc != 0 {
		t.Fatalf("check after update: rc %d, log:\n%s", rc, log)
	}
	wantNotIn(t, readPage(t, root, "docs/b.md"), "old")
}

func TestExcerptOfANewDiagram(t *testing.T) {
	t.Parallel()
	files := machineRepo(statesMarker)
	files["docs/b.md"] = "<!-- docgen:code file=docs/states/p-m.svg -->\n<!-- /docgen:code -->\n"
	wantIn(t, updateTwice(t, pageRepo(t, files), "docs/b.md"), "<svg")
}

func chainRepo(n int) map[string]string {
	files := map[string]string{"docs/p1.md": "<!-- docgen:gomod field=module --><!-- /docgen:gomod -->\n"}
	for i := 2; i <= n; i++ {
		files[fmt.Sprintf("docs/p%d.md", i)] = fmt.Sprintf("<!-- docgen:code file=docs/p%d.md -->\n<!-- /docgen:code -->\n", i-1)
	}
	return files
}

func TestChainLimit(t *testing.T) {
	t.Parallel()
	root := pageRepo(t, chainRepo(maxChain))
	if rc, log := docgen(t, root, "update"); rc != 0 {
		t.Fatalf("a chain of %d: rc %d, log:\n%s", maxChain, rc, log)
	}
	if rc, log := docgen(t, root, "check"); rc != 0 {
		t.Fatalf("check after update: rc %d, log:\n%s", rc, log)
	}
	root = pageRepo(t, chainRepo(maxChain+1))
	rc, log := docgen(t, root, "update")
	if rc != 1 {
		t.Fatalf("a chain of %d: rc %d, log:\n%s", maxChain+1, rc, log)
	}
	wantIn(t, log, "do not converge after 8 render passes")
}

const maxChain = 8

// goShim writes a go command that records each run in a new directory and then
// runs the real one. It holds each run for a moment, so runs that can overlap
// do. The directory holds a line in peaks for each run, with the number of runs
// that were alive, and a line in flags with its GOFLAGS.
func goShim(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\nd=" + dir + "\nf=$(mktemp \"$d/live.XXXXXX\")\nls \"$d\" | grep -c '^live' >>\"$d/peaks\"\necho \"$GOFLAGS\" >>\"$d/flags\"\nsleep 0.1\n" +
		goBin + " \"$@\"\nrc=$?\nrm -f \"$f\"\nexit $rc\n"
	if err := os.WriteFile(filepath.Join(dir, "go"), []byte(script), 0o700); err != nil { //nolint:gosec // a program
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return dir
}

func goPeak(t *testing.T, shim string) int {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(shim, "peaks"))
	if err != nil {
		t.Fatal(err)
	}
	peak := 0
	for f := range strings.FieldsSeq(string(b)) {
		n, err := strconv.Atoi(f)
		if err != nil {
			t.Fatal(err)
		}
		peak = max(peak, n)
	}
	return peak
}

//nolint:paralleltest // t.Setenv
func TestGoCommandLimit(t *testing.T) {
	files := map[string]string{
		"p/p.go":    machineFiles,
		"docs/p.md": "<!-- docgen:const pkg=p name=On --><!-- /docgen:const -->\n",
	}
	var page strings.Builder
	for i := range 16 {
		name := fmt.Sprintf("e%d", i)
		files["examples/"+name+"/main.go"] = "package main\n\nfunc main() {}\n"
		fmt.Fprintf(&page, "<!-- docgen:output cmd=examples/%s -->\n<!-- /docgen:output -->\n", name)
	}
	files["docs/q.md"] = page.String()
	root := pageRepo(t, files)
	shim := goShim(t)
	if rc, log := docgen(t, root, "update", "-jobs", "32"); rc != 0 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
	if peak := goPeak(t, shim); peak > 8 || peak < 4 {
		t.Errorf("%d go commands ran at once, want at most 8 and more than a few", peak)
	}
}

//nolint:paralleltest // t.Setenv
func TestJobsZeroLeavesParallelismToGo(t *testing.T) {
	root := pageRepo(t, map[string]string{
		"p/p.go":    "package p\n\nconst N = 1\n",
		"docs/p.md": "<!-- docgen:const pkg=p name=N --><!-- /docgen:const -->\n",
	})
	shim := goShim(t)
	t.Setenv("GOFLAGS", "-p=2")
	if rc, log := docgen(t, root, "update"); rc != 0 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
	b, err := os.ReadFile(filepath.Join(shim, "flags"))
	if err != nil {
		t.Fatal(err)
	}
	for line := range strings.Lines(string(b)) {
		if line != "-p=2\n" {
			t.Errorf("GOFLAGS = %q, want -p=2", line)
		}
	}
}

func TestJobsLimit(t *testing.T) {
	t.Parallel()
	root := pageRepo(t, map[string]string{"docs/p.md": "text\n"})
	if rc, log := docgen(t, root, "check", "-jobs", "1024"); rc != 0 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
	rc, log := docgen(t, root, "check", "-jobs", "20000000")
	if rc != 1 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
	wantIn(t, log, "jobs is 20000000; it must be at most 1024")
}

func TestJobsInTheFile(t *testing.T) {
	t.Parallel()
	for value, want := range map[string]string{
		"1.5":  "is not an integer",
		"2.0":  "is not an integer",
		"1e3":  "is not an integer",
		"true": "is not an integer",
		`"x"`:  "is not an integer",
		"-1":   "jobs",
		"4":    "",
		"0x10": "",
		"0":    "",
	} {
		t.Run(value, func(t *testing.T) {
			t.Parallel()
			root := pageRepo(t, map[string]string{".docgen.yaml": "jobs: " + value + "\n", "docs/p.md": "text\n"})
			rc, log := docgen(t, root, "check")
			if want == "" {
				if rc != 0 {
					t.Fatalf("rc %d, log:\n%s", rc, log)
				}
				return
			}
			if rc != 1 {
				t.Fatalf("rc %d, log:\n%s", rc, log)
			}
			wantIn(t, log, want)
		})
	}
}

func TestNoDocsDirectory(t *testing.T) {
	t.Parallel()
	const page = "<!-- docgen:gomod field=module --><!-- /docgen:gomod -->\n"
	root := pageRepo(t, map[string]string{"README.md": page})
	wantIn(t, updateTwice(t, root, "README.md"), "`example.com/p`")
}

func TestMissingDocsDirectoryErrors(t *testing.T) {
	t.Parallel()
	for name, c := range map[string]struct {
		config string
		args   []string
	}{
		"setting":  {"docs: gone\n", nil},
		"argument": {"", []string{"docs"}},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := pageRepo(t, map[string]string{".docgen.yaml": c.config, "README.md": "text\n"})
			if rc, log := docgen(t, root, append([]string{"check"}, c.args...)...); rc != 1 {
				t.Fatalf("rc %d, log:\n%s", rc, log)
			}
		})
	}
}

func TestBadTags(t *testing.T) {
	t.Parallel()
	root := pageRepo(t, map[string]string{"docs/p.md": "text\n"})
	for _, tag := range []string{`alpha"x`, "a-b", "!a", "a,b$"} {
		rc, log := docgen(t, root, "check", "-tags", tag)
		if rc != 1 {
			t.Fatalf("%s: rc %d, log:\n%s", tag, rc, log)
		}
		wantIn(t, log, "tag", "letters, digits")
	}
	if rc, log := docgen(t, root, "check", "-tags", "a_1.b,go1.24 linux"); rc != 0 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
}

//nolint:paralleltest // t.Setenv
func TestTagsOfGoflagsAndConfig(t *testing.T) {
	root := pageRepo(t, map[string]string{
		"p/a.go":    "//go:build one\n\npackage p\n\nconst A = 1\n",
		"p/b.go":    "//go:build two\n\npackage p\n\nconst B = 2\n",
		"p/c.go":    "package p\n",
		"docs/p.md": "<!-- docgen:const pkg=p name=A --><!-- /docgen:const --> <!-- docgen:const pkg=p name=B --><!-- /docgen:const -->\n",
	})
	t.Setenv("GOFLAGS", "-tags=one")
	if rc, log := docgen(t, root, "update", "-tags", "two"); rc != 0 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
	wantIn(t, readPage(t, root, "docs/p.md"), "`1`", "`2`")
}

func TestConfiguratorLogOfTheRootPackage(t *testing.T) {
	t.Parallel()
	root := pageRepo(t, map[string]string{
		"p.go":      "package w\n\n// Nope is a type.\ntype Nope struct{}\n",
		"docs/p.md": "<!-- docgen:configulator pkg=. type=Nope -->\n<!-- /docgen:configulator -->\n",
	})
	_, log := docgen(t, root, "check")
	wantIn(t, log, "docgen: running configulator on Nope\n")
	if regexp.MustCompile(`on \.+Nope`).MatchString(log) {
		t.Errorf("log:\n%s", log)
	}
}

func TestDiffOnStandardOutputIsAPatch(t *testing.T) {
	t.Parallel()
	patch, err := exec.LookPath("patch")
	if err != nil {
		t.Skip(err)
	}
	root := pageRepo(t, machineRepo("# Title\n\n"+statesMarker+"\nText.\n"))
	rc, log, diff := docgenDiff(t, root, "check", "-diff")
	if rc != 1 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
	wantNotIn(t, diff, "stale:", "recording")
	wantNotIn(t, log, "--- docs/p.md")
	cmd := exec.CommandContext(t.Context(), patch, "-p0", "--batch")
	cmd.Dir = root
	cmd.Stdin = strings.NewReader(diff)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("patch: %v\n%s", err, out)
	}
	if rc, log := docgen(t, root, "check"); rc != 0 {
		t.Fatalf("check after patch: rc %d, log:\n%s", rc, log)
	}
}

func TestDiffWriteError(t *testing.T) {
	t.Parallel()
	root := pageRepo(t, map[string]string{"docs/a.md": goodPair + "\n"})
	var stderr bytes.Buffer
	if rc := runTo(t.Context(), root, []string{"check", "-diff"}, failingWriter{}, &stderr); rc != 1 {
		t.Fatalf("rc %d, log:\n%s", rc, stderr.String())
	}
	wantIn(t, stderr.String(), "no space")
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("no space") }

const constSource = `package p

import (
	"example.com/p/q"
	"time"
)

// State has a String method.
type State int

const (
	Off State = iota
	On
)

func (s State) String() string { return [...]string{"off", "on"}[s] }

// Plain has no method.
type Plain int

const (
	PlainA Plain = iota
	PlainB
)

// Err is an error.
type Err string

func (e Err) Error() string { return "err: " + string(e) }

const ErrGone Err = "gone"

// G has a GoString method.
type G int

func (G) GoString() string { return "the-g" }

const GV G = 1

const hidden Err = "secret"

const Text = "no quotes"

const Million = 1e6

const Half float64 = 0.5

const Big = 1 << 100

const Wait = 1500 * time.Millisecond

const Foreign = q.Up

var Current = On

var count int = 3

const Same1 Plain = 9

const Same2 Plain = 9

const Lost Plain = 8

const Other = PlainB + 5

func (s State) Unused() {}
`

const constForeign = "package q\n\n// Dir is a direction.\ntype Dir int\n\nconst Up Dir = 1\n\nfunc (d Dir) String() string { return \"up!\" }\n"

func constPage(args ...string) string {
	var b strings.Builder
	for _, a := range args {
		fmt.Fprintf(&b, "<!-- docgen:const pkg=p %s --><!-- /docgen:const -->\n", a)
	}
	return b.String()
}

func TestConstPrinting(t *testing.T) {
	t.Parallel()
	for args, want := range map[string]string{
		"name=On":                 "`on`",
		"name=On print=%d":        "`1`",
		"name=On print=name":      "`On`",
		"name=On print=%q":        "`\"on\"`",
		"name=On print=%T":        "`p.State`",
		"name=PlainB":             "`1`",
		"name=PlainB print=name":  "`PlainB`",
		"name=PlainB print=%#x":   "`0x1`",
		"name=PlainB print=%T":    "`p.Plain`",
		"name=ErrGone":            "`err: gone`",
		"name=ErrGone print=%q":   "`\"err: gone\"`",
		"name=GV":                 "`1`",
		"name=GV print=%#v":       "`the-g`",
		"name=hidden":             "`err: secret`",
		"name=Text":               "`no quotes`",
		"name=Text print=%q":      "`\"no quotes\"`",
		"name=Million":            "`1e+06`",
		"name=Half":               "`0.5`",
		"name=Big":                "`1267650600228229401496703205376`",
		"name=Wait":               "`1.5s`",
		"name=Wait print=%d":      "`1500000000`",
		"name=Foreign":            "`up!`",
		"name=Foreign print=%d":   "`1`",
		"name=Foreign print=name": "`Foreign`",
		"name=Current":            "`on`",
		"name=count":              "`3`",
	} {
		t.Run(args, func(t *testing.T) {
			t.Parallel()
			root := pageRepo(t, map[string]string{
				"p/p.go": constSource, "q/q.go": constForeign,
				"docs/p.md": constPage(args),
			})
			wantIn(t, updateTwice(t, root, "docs/p.md"), "-->"+want+"<!--")
		})
	}
}

func TestConstBatchRunsOncePerPackage(t *testing.T) {
	t.Parallel()
	root := pageRepo(t, map[string]string{
		"p/p.go": constSource, "q/q.go": constForeign,
		"docs/p.md": constPage("name=On", "name=On print=%d", "name=ErrGone", "name=hidden"),
	})
	rc, log := docgen(t, root, "update")
	if rc != 0 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
	if n := strings.Count(log, "docgen: printing"); n != 1 {
		t.Errorf("%d runs, log:\n%s", n, log)
	}
	wantIn(t, log, "docgen: printing 4 values of p\n")
}

func TestConstNeedsNoRunForStaticValues(t *testing.T) {
	t.Parallel()
	root := pageRepo(t, map[string]string{
		"p/p.go": constSource, "q/q.go": constForeign,
		"docs/p.md": constPage("name=Text", "name=Wait", "name=PlainB", "name=Big", "name=On print=name"),
	})
	rc, log := docgen(t, root, "update")
	if rc != 0 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
	wantNotIn(t, log, "docgen: printing")
}

func TestConstErrors(t *testing.T) {
	t.Parallel()
	for args, want := range map[string]string{
		"name=Same1 print=name": "several constants of type example.com/p/p.Plain have the value 9: Same1, Same2",
		"name=Lost print=%z":    "print=%z is not valid for the value: %!z(int=8)",
		"name=On print=%z":      "print=%z is not valid for the value: %!z(p.State=1)",
		"name=On print=nope":    "print=nope is not go, raw, name or a fmt verb",
		`name=On print="%d %d"`: "is not go, raw, name or a fmt verb",
		"name=Text print=name":  "no constant of type string has the value",
	} {
		t.Run(args, func(t *testing.T) {
			t.Parallel()
			src := constSource
			if strings.Contains(args, "Text") {
				src = strings.ReplaceAll(src, "const Text = \"no quotes\"", "var Text = \"no quotes\"")
			}
			root := pageRepo(t, map[string]string{"p/p.go": src, "q/q.go": constForeign, "docs/p.md": constPage(args)})
			rc, log := docgen(t, root, "check")
			if rc != 1 {
				t.Fatalf("rc %d, log:\n%s", rc, log)
			}
			wantIn(t, log, want)
		})
	}
}

func TestConstNameWithNoConstant(t *testing.T) {
	t.Parallel()
	root := pageRepo(t, map[string]string{
		"p/p.go":    "package p\n\n// S is a state.\ntype S int\n\nvar V S = 4\n",
		"docs/p.md": constPage("name=V print=name"),
	})
	rc, log := docgen(t, root, "check")
	if rc != 1 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
	wantIn(t, log, "no constant of type example.com/p/p.S has the value 4")
}

func TestConstWithBrokenTests(t *testing.T) {
	t.Parallel()
	root := pageRepo(t, map[string]string{
		"p/p.go":      "package p\n\ntype S int\n\nconst A S = 1\n\nconst N = 5\n\nfunc (S) String() string { return \"s\" }\n",
		"p/p_test.go": "package p\n\nfunc broken() { undefined() }\n",
		"docs/p.md":   constPage("name=A"),
		"docs/q.md":   constPage("name=N"),
	})
	rc, log := docgen(t, root, "check")
	if rc != 1 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
	wantIn(t, log, "docs/p.md:1: docgen:const pkg=p name=A: printing the values of p", "stale: docs/q.md:1:")
	wantNotIn(t, log, "docs/q.md:1: docgen:const pkg=p name=N:")
}

func TestConstPanicInString(t *testing.T) {
	t.Parallel()
	root := pageRepo(t, map[string]string{
		"p/p.go":    "package p\n\ntype S int\n\nconst A S = 1\n\nfunc (S) String() string { panic(\"boom\") }\n",
		"docs/p.md": constPage("name=A"),
	})
	rc, log := docgen(t, root, "check")
	if rc != 1 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
	wantIn(t, log, "the value has no text", "boom")
}

func TestConstNamesThatClashWithTheTestFile(t *testing.T) {
	t.Parallel()
	root := pageRepo(t, map[string]string{
		"p/p.go":    "package p\n\ntype S int\n\nconst fmt S = 1\n\nconst os = 2\n\nconst testing = 3\n\nfunc (S) String() string { return \"s\" }\n",
		"docs/p.md": constPage("name=fmt"),
	})
	wantIn(t, updateTwice(t, root, "docs/p.md"), "`s`")
}

func TestConstOfAMainPackageWithTestMain(t *testing.T) {
	t.Parallel()
	root := pageRepo(t, map[string]string{
		"p/p.go":      "package main\n\ntype S int\n\nconst A S = 1\n\nfunc (S) String() string { return \"s\" }\n\nfunc main() {}\n",
		"p/p_test.go": "package main\n\nimport (\n\t\"os\"\n\t\"testing\"\n)\n\nfunc TestMain(m *testing.M) { os.Exit(m.Run()) }\n",
		"docs/p.md":   constPage("name=A"),
	})
	wantIn(t, updateTwice(t, root, "docs/p.md"), "`s`")
}

func TestConfigDefaultsPrintLikeConst(t *testing.T) {
	t.Parallel()
	const src = `package p

import (
	"example.com/p/q"
	"time"
)

// Mode is a mode.
type Mode int

const Fast Mode = 2

func (m Mode) String() string { return "fast" }

const DefaultWait = 250 * time.Millisecond

const DefaultName = "lamp"

// Config is the config.
type Config struct {
	Wait time.Duration
	Kind Mode
	Dir  q.Dir
	Name string
	Lit  string
	Num  Mode
}

// Apply sets the defaults.
func (c *Config) Apply() {
	if c.Wait == 0 {
		c.Wait = DefaultWait
	}
	if c.Kind == 0 {
		c.Kind = Fast
	}
	if c.Dir == 0 {
		c.Dir = q.Up
	}
	if c.Name == "" {
		c.Name = DefaultName
	}
	if c.Lit == "" {
		c.Lit = "x"
	}
	if c.Num == 0 {
		c.Num = 2
	}
}
`
	root := pageRepo(t, map[string]string{
		"p/p.go": src, "q/q.go": constForeign,
		"docs/p.md": "<!-- docgen:config pkg=p type=Config -->\n<!-- /docgen:config -->\n\n" + constPage("name=Fast", "name=DefaultWait"),
	})
	got := updateTwice(t, root, "docs/p.md")
	wantIn(t, got, "`DefaultWait` (250ms)", "`Fast` (fast)", "`q.Up` (up!)", "`DefaultName` (lamp)", "| `\"x\"` |", "`2` (fast)", "-->`fast`<!--", "-->`250ms`<!--")
}

func TestDocsDirectoryLeavesTheRepository(t *testing.T) {
	t.Parallel()
	const text = "<!-- docgen:gomod field=module --><!-- /docgen:gomod -->\n"
	root, outside := outsideRepo(t, map[string]string{"keep": ""}, map[string]string{"p.md": text})
	link(t, outside, filepath.Join(root, "docs"))
	for _, args := range [][]string{{"check", "docs"}, {"update", "docs"}} {
		rc, log := docgen(t, root, args...)
		if rc != 1 {
			t.Fatalf("%q: rc %d, log:\n%s", args, rc, log)
		}
		wantIn(t, log, "docs is outside the repository")
	}
	if rc, log := docgen(t, root, "update"); rc != 0 {
		t.Fatalf("a scan of the root: rc %d, log:\n%s", rc, log)
	}
	if b, err := os.ReadFile(filepath.Join(outside, "p.md")); err != nil || string(b) != text {
		t.Errorf("the page outside the repository changed: %q, %v", b, err)
	}
}

func TestDocsDirectoryLinkInsideTheRepository(t *testing.T) {
	t.Parallel()
	root, _ := outsideRepo(t, map[string]string{"real/p.md": "<!-- docgen:gomod field=module --><!-- /docgen:gomod -->\n"}, map[string]string{"x": ""})
	link(t, "real", filepath.Join(root, "docs"))
	if rc, log := docgen(t, root, "update"); rc != 0 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
}
