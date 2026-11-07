package cli

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

const sampleMod = "module example.com/s\n\ngo 1.24.0\n"

const sampleGoMod = "<!-- docgen:gomod field=module --><!-- /docgen:gomod -->\n"

const sampleFilled = "<!-- docgen:gomod field=module -->`example.com/s`<!-- /docgen:gomod -->\n"

// nestedMarker returns a docgen:docgen marker with the arguments args.
func nestedMarker(args string) string {
	return "<!-- docgen:docgen " + args + " -->\n<!-- /docgen:docgen -->\n"
}

// outerRepo writes a repository with the page text at docs/p.md, and the sample
// files below sample/. It returns the root.
func outerRepo(t *testing.T, text string, sample map[string]string) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{"go.mod": "module example.com/w\n\ngo 1.24.0\n", "docs/p.md": text}
	for name, body := range sample {
		files["testdata/sample/"+name] = body
	}
	writeTree(t, root, files)
	return root
}

func snapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil || !d.Type().IsRegular() {
			return err
		}
		b, err := os.ReadFile(p)
		out[p] = string(b)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestDocgenMarkerBody(t *testing.T) {
	t.Parallel()
	root := outerRepo(t, "# Page\n\n"+nestedMarker("root=testdata/sample page=docs/a.md")+"\nEnd.\n", map[string]string{
		"go.mod":    sampleMod,
		"docs/a.md": "Module:\n\n" + sampleGoMod + "\n```text\na fence\n```\n",
	})
	sample := snapshot(t, filepath.Join(root, "testdata"))
	if rc, log := docgen(t, root, "check"); rc != 1 || !strings.Contains(log, "stale: docs/p.md:3: docgen:docgen root=testdata/sample page=docs/a.md") {
		t.Fatalf("check: rc %d, log:\n%s", rc, log)
	}
	if rc, log := docgen(t, root, "update"); rc != 0 {
		t.Fatalf("update: rc %d, log:\n%s", rc, log)
	}
	want := "# Page\n\n<!-- docgen:docgen root=testdata/sample page=docs/a.md -->\n" +
		"#### Before\n\n````\nModule:\n\n" + sampleGoMod + "\n```text\na fence\n```\n````\n\n" +
		"#### After\n\n````\nModule:\n\n" + sampleFilled + "\n```text\na fence\n```\n````\n" +
		"<!-- /docgen:docgen -->\n\nEnd.\n"
	if got := page(t, root, "p.md"); got != want {
		t.Errorf("page:\n%s\nwant:\n%s", got, want)
	}
	if rc, log := docgen(t, root, "check"); rc != 0 {
		t.Errorf("check after update: rc %d, log:\n%s", rc, log)
	}
	after := snapshot(t, filepath.Join(root, "testdata"))
	if fmt.Sprint(after) != fmt.Sprint(sample) {
		t.Errorf("the sample changed:\n%v\nwant:\n%v", after, sample)
	}
}

func TestDocgenMarkerCRLFPage(t *testing.T) {
	t.Parallel()
	root := outerRepo(t, nestedMarker("root=testdata/sample page=docs/a.md"), map[string]string{
		"go.mod":    sampleMod,
		"docs/a.md": "Text\r\n",
	})
	if rc, log := docgen(t, root, "update"); rc != 0 {
		t.Fatalf("update: rc %d, log:\n%s", rc, log)
	}
	if got := page(t, root, "p.md"); strings.Contains(got, "\r") {
		t.Errorf("page holds a carriage return: %q", got)
	}
}

func TestDocgenMarkerPathErrors(t *testing.T) {
	t.Parallel()
	for name, c := range map[string]struct{ args, want string }{
		"root leaves":       {"root=.. page=a.md", "root=.. is outside the repository"},
		"root is absolute":  {"root=/etc page=a.md", "root=/etc is outside the repository"},
		"root is a link":    {"root=out page=a.md", "root=out is outside the repository"},
		"root is missing":   {"root=gone page=a.md", "root=gone is not a directory"},
		"root is a file":    {"root=go.mod page=a.md", "root=go.mod is not a directory"},
		"page leaves":       {"root=testdata/sample page=../a.md", "page=../a.md is outside root=testdata/sample"},
		"page is absolute":  {"root=testdata/sample page=/etc/passwd", "page=/etc/passwd is outside root=testdata/sample"},
		"page is a link":    {"root=testdata/sample page=link.md", "page=link.md is outside root=testdata/sample"},
		"page is missing":   {"root=testdata/sample page=gone.md", "no page gone.md in root=testdata/sample"},
		"page is a dir":     {"root=testdata/sample page=docs", "no page docs in root=testdata/sample"},
		"page in .git":      {"root=testdata/sample page=.git/a.md", "no page .git/a.md in root=testdata/sample"},
		"module leaves":     {"root=testdata/sample page=docs/a.md module=..", "module=.. is outside root=testdata/sample"},
		"module is a link":  {"root=testdata/sample page=docs/a.md module=modlink", "module=modlink is outside root=testdata/sample"},
		"module is missing": {"root=testdata/sample page=docs/a.md module=gone", "module directory testdata/sample/gone does not exist"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			text := nestedMarker(c.args)
			root := outerRepo(t, text, map[string]string{"go.mod": sampleMod, "docs/a.md": "A\n", ".git/a.md": "A\n"})
			outside := t.TempDir()
			writeTree(t, outside, map[string]string{"a.md": "A\n", "go.mod": sampleMod})
			if err := os.Symlink(outside, filepath.Join(root, "out")); err != nil {
				t.Fatal(err)
			}
			sample := filepath.Join(root, "testdata/sample")
			if c.args == "root=testdata/sample page=link.md" {
				if err := os.Symlink(filepath.Join(outside, "a.md"), filepath.Join(sample, "link.md")); err != nil {
					t.Fatal(err)
				}
			}
			if strings.Contains(c.args, "modlink") {
				if err := os.Symlink(outside, filepath.Join(sample, "modlink")); err != nil {
					t.Fatal(err)
				}
			}
			rc, log := docgen(t, root, "update")
			if rc != 1 || !strings.Contains(log, c.want) {
				t.Errorf("rc %d, log:\n%s", rc, log)
			}
			if !strings.Contains(log, "docs/p.md:1: docgen:docgen "+c.args+": ") {
				t.Errorf("the error does not name the marker:\n%s", log)
			}
			if got := page(t, root, "p.md"); got != text {
				t.Errorf("page changed: %q", got)
			}
		})
	}
}

func TestDocgenCopyLeavesOut(t *testing.T) {
	t.Parallel()
	for name, c := range map[string]struct {
		file string
		ok   bool
	}{
		"git directory":         {".git/config", false},
		"git in a subdir":       {"sub/.git/config", false},
		"artifacts":             {"artifacts/x.txt", true},
		"artifacts in a subdir": {"sub/artifacts/x.txt", true},
		"other":                 {"sub/x.txt", true},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := outerRepo(t, nestedMarker("root=testdata/sample page=docs/a.md"), map[string]string{
				"go.mod":    sampleMod,
				c.file:      "x\n",
				"docs/a.md": "<!-- docgen:code file=" + c.file + " --><!-- /docgen:code -->\n",
			})
			rc, log := docgen(t, root, "update")
			if c.ok && rc != 0 {
				t.Errorf("rc %d, log:\n%s", rc, log)
			}
			if !c.ok && (rc != 1 || !strings.Contains(log, "no such file")) {
				t.Errorf("rc %d, log:\n%s", rc, log)
			}
		})
	}
}

func TestDocgenCopyLinks(t *testing.T) {
	t.Parallel()
	text := nestedMarker("root=testdata/sample page=docs/a.md")
	sample := map[string]string{
		"go.mod":     sampleMod,
		"real/x.txt": "linked\n",
		"docs/a.md":  "<!-- docgen:code file=docs/rel.txt --><!-- /docgen:code -->\n<!-- docgen:code file=abs.txt --><!-- /docgen:code -->\n",
	}
	root := outerRepo(t, text, sample)
	base := filepath.Join(root, "testdata/sample")
	if err := os.Symlink("../real/x.txt", filepath.Join(base, "docs/rel.txt")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(base, "real/x.txt"), filepath.Join(base, "abs.txt")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("gone", filepath.Join(base, "dangling")); err != nil {
		t.Fatal(err)
	}
	if rc, log := docgen(t, root, "update"); rc != 0 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
	if got := page(t, root, "p.md"); strings.Count(got, "linked\n") != 2 {
		t.Errorf("page:\n%s", got)
	}

	outside := filepath.Join(t.TempDir(), "secret.txt")
	if err := os.WriteFile(outside, []byte("secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(base, "leak.txt")); err != nil {
		t.Fatal(err)
	}
	rc, log := docgen(t, root, "update")
	if rc != 1 || !strings.Contains(log, "leak.txt is a symbolic link outside root=testdata/sample") {
		t.Errorf("rc %d, log:\n%s", rc, log)
	}
	if strings.Contains(log, outside) {
		t.Errorf("the log holds the target of the link:\n%s", log)
	}
}

func TestDocgenCopyOfReadOnlyTree(t *testing.T) {
	t.Parallel()
	root := outerRepo(t, nestedMarker("root=testdata/sample page=docs/a.md"), map[string]string{
		"go.mod":    sampleMod,
		"docs/a.md": sampleGoMod,
	})
	docs := filepath.Join(root, "testdata/sample/docs")
	if err := os.Chmod(filepath.Join(docs, "a.md"), 0o400); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(docs, 0o500); err != nil { //nolint:gosec // a directory with no write bit
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(docs, 0o700) }) //nolint:gosec // for the removal of t.TempDir
	if rc, log := docgen(t, root, "update"); rc != 0 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
	if got := page(t, root, "p.md"); !strings.Contains(got, sampleFilled) {
		t.Errorf("page:\n%s", got)
	}
}

func TestDocgenNestedFailure(t *testing.T) {
	t.Parallel()
	text := "# Page\n\n" + nestedMarker("root=testdata/sample page=docs/a.md")
	root := outerRepo(t, text, map[string]string{
		"go.mod":                sampleMod,
		"docs/a.md":             "<!-- docgen:output cmd=examples/boom --><!-- /docgen:output -->\n",
		"examples/boom/main.go": "package main\n\nimport \"os\"\n\nfunc main() {\n\tcwd, _ := os.Getwd()\n\tprintln(\"failed in \" + cwd)\n\tos.Exit(3)\n}\n",
	})
	rc, log := docgen(t, root, "update")
	if rc != 1 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
	for _, want := range []string{
		"docs/p.md:3: docgen:docgen root=testdata/sample page=docs/a.md: docs/a.md:1: docgen:output cmd=examples/boom: ",
		"failed in testdata/sample\n",
		"docgen: running examples/boom",
	} {
		if !strings.Contains(log, want) {
			t.Errorf("the log does not hold %q:\n%s", want, log)
		}
	}
	if got := page(t, root, "p.md"); got != text {
		t.Errorf("page changed: %q", got)
	}
}

func TestDocgenNestedRunIsSilent(t *testing.T) {
	t.Parallel()
	root := outerRepo(t, nestedMarker("root=testdata/sample page=docs/a.md"), map[string]string{
		"go.mod":              sampleMod,
		"docs/a.md":           "<!-- docgen:output cmd=examples/hi --><!-- /docgen:output -->\n<!-- docgen:code file=x.txt lines=1-1 -->\n<!-- /docgen:code -->\n",
		"x.txt":               "x\n",
		"examples/hi/main.go": "package main\n\nfunc main() {\n\tprintln(\"hi\")\n}\n",
	})
	rc, log := docgen(t, root, "update")
	if rc != 0 || log != "" {
		t.Errorf("rc %d, log:\n%s", rc, log)
	}
}

// counting returns the files of a sample whose example adds a line to file on
// each run.
func counting(file string) map[string]string {
	return map[string]string{
		"go.mod":    sampleMod,
		"docs/a.md": "<!-- docgen:output cmd=examples/count --><!-- /docgen:output -->\n",
		"docs/b.md": "Other\n",
		"examples/count/main.go": fmt.Sprintf(`package main

import "os"

func main() {
	f, err := os.OpenFile(%q, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	f.WriteString("run\n")
}
`, file),
	}
}

func runsIn(t *testing.T, file string) int {
	t.Helper()
	b, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	return bytes.Count(b, []byte("run\n"))
}

func TestDocgenMarkersUseOneRun(t *testing.T) {
	t.Parallel()
	file := filepath.Join(t.TempDir(), "runs")
	var text strings.Builder
	for i := range 16 {
		pg := "docs/a.md"
		if i%2 == 1 {
			pg = "docs/b.md"
		}
		root := "testdata/sample"
		if i%4 == 3 {
			root = "./testdata/sample/"
		}
		text.WriteString(nestedMarker("root=" + root + " page=" + pg))
		text.WriteString("\n")
	}
	root := outerRepo(t, text.String(), counting(file))
	if rc, log := docgen(t, root, "update", "-jobs", "4"); rc != 0 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
	if got := runsIn(t, file); got != 1 {
		t.Errorf("%d runs, want 1", got)
	}
}

func TestDocgenRootAndModuleSelectTheRun(t *testing.T) {
	t.Parallel()
	file := filepath.Join(t.TempDir(), "runs")
	files := counting(file)
	sample := map[string]string{}
	for name, body := range files {
		sample[name] = body
		sample["sub/"+name] = body
	}
	text := nestedMarker("root=testdata/sample page=docs/a.md") + "\n" +
		nestedMarker("root=testdata/sample page=docs/a.md module=.") + "\n" +
		nestedMarker("root=testdata/sample page=docs/a.md module=sub") + "\n" +
		nestedMarker("root=testdata/sample/sub page=docs/b.md")
	root := outerRepo(t, text, sample)
	if rc, log := docgen(t, root, "update"); rc != 0 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
	if got := runsIn(t, file); got != 3 {
		t.Errorf("%d runs, want 3", got)
	}
}

func TestDocgenDepth(t *testing.T) {
	t.Parallel()
	inner := map[string]string{
		"docs/a.md":    nestedMarker("root=in page=docs/b.md"),
		"in/docs/b.md": sampleGoMod,
		"in/go.mod":    sampleMod,
		"go.mod":       sampleMod,
	}
	marker := nestedMarker("root=testdata/sample page=docs/a.md")
	for name, c := range map[string]struct {
		yaml string
		rc   int
		want string
	}{
		"default stops the second level": {"", 1, "docs/p.md:1: docgen:docgen root=testdata/sample page=docs/a.md: docs/a.md:1: docgen:docgen root=in page=docs/b.md: the marker is at level 2, above the depth limit 1"},
		"depth 1":                        {"depth: 1\n", 1, "above the depth limit 1"},
		"depth 2":                        {"depth: 2\n", 0, ""},
		"depth 8":                        {"depth: 8\n", 0, ""},
		"depth 0":                        {"depth: 0\n", 1, "docs/p.md:1: docgen:docgen root=testdata/sample page=docs/a.md: the marker is at level 1, above the depth limit 0"},
		"depth 9":                        {"depth: 9\n", 1, "depth is 9; it must be at most 8"},
		"negative":                       {"depth: -1\n", 1, "depth is -1; it must be 0 or more"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := outerRepo(t, marker, inner)
			if c.yaml != "" {
				writeTree(t, root, map[string]string{".docgen.yaml": c.yaml})
			}
			rc, log := docgen(t, root, "update")
			if rc != c.rc || !strings.Contains(log, c.want) {
				t.Errorf("rc %d, log:\n%s", rc, log)
			}
			if c.rc == 0 && !strings.Contains(page(t, root, "p.md"), "`example.com/s`") {
				t.Errorf("page:\n%s", page(t, root, "p.md"))
			}
		})
	}
}

func TestDocgenSampleConfigCannotRaiseDepth(t *testing.T) {
	t.Parallel()
	for name, c := range map[string]struct {
		yaml string
		want string
	}{
		"valid value":   {"depth: 8\n", "above the depth limit 1"},
		"invalid value": {"depth: 9\n", "above the depth limit 1"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := outerRepo(t, nestedMarker("root=testdata/sample page=docs/a.md"), map[string]string{
				".docgen.yaml": c.yaml,
				"go.mod":       sampleMod,
				"docs/a.md":    nestedMarker("root=in page=docs/b.md"),
				"in/go.mod":    sampleMod,
				"in/docs/b.md": sampleGoMod,
			})
			if rc, log := docgen(t, root, "update"); rc != 1 || !strings.Contains(log, c.want) {
				t.Errorf("rc %d, log:\n%s", rc, log)
			}
		})
	}
}

func TestDocgenDepthLimitsCycles(t *testing.T) {
	t.Parallel()
	root := outerRepo(t, nestedMarker("root=. page=docs/p.md"), nil)
	writeTree(t, root, map[string]string{".docgen.yaml": "depth: 8\n"})
	rc, log := docgen(t, root, "update")
	if rc != 1 || !strings.Contains(log, "the marker is at level 9, above the depth limit 8") {
		t.Errorf("rc %d, log:\n%s", rc, log)
	}
}

// TestDocgenDepthVariable sets a variable, so it does not run in parallel.
//
//nolint:paralleltest // t.Setenv
func TestDocgenDepthVariable(t *testing.T) {
	root := outerRepo(t, nestedMarker("root=testdata/sample page=docs/a.md"), map[string]string{"go.mod": sampleMod, "docs/a.md": sampleGoMod})
	t.Setenv("DOCGEN_DEPTH", "0")
	if rc, log := docgen(t, root, "update"); rc != 1 || !strings.Contains(log, "above the depth limit 0") {
		t.Errorf("rc %d, log:\n%s", rc, log)
	}
}

func TestDepthHasNoFlag(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeTree(t, root, map[string]string{"go.mod": sampleMod})
	if rc, _ := docgen(t, root, "check", "-depth", "2"); rc != 2 {
		t.Errorf("rc %d, want 2", rc)
	}
}

func TestDocgenSampleSettings(t *testing.T) {
	t.Parallel()
	root := outerRepo(t, nestedMarker("root=testdata/sample page=content/a.md"), map[string]string{
		".docgen.yaml": "docs: content\n",
		"go.mod":       sampleMod,
		"content/a.md": sampleGoMod,
		"other.md":     "<!-- docgen:nope -->\n",
	})
	if rc, log := docgen(t, root, "update"); rc != 0 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
	if got := page(t, root, "p.md"); !strings.Contains(got, sampleFilled) {
		t.Errorf("page:\n%s", got)
	}
}

func TestDocgenPageOutsideTheDocsDirectory(t *testing.T) {
	t.Parallel()
	for name, files := range map[string]map[string]string{
		"no docs directory": {"go.mod": sampleMod, "README.md": sampleGoMod},
		"docs directory":    {"go.mod": sampleMod, "README.md": sampleGoMod, "docs/a.md": "A\n"},
		"narrowed scan":     {".docgen.yaml": "docs: docs\n", "go.mod": sampleMod, "README.md": sampleGoMod, "docs/a.md": "A\n"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := outerRepo(t, nestedMarker("root=testdata/sample page=README.md"), files)
			if rc, log := docgen(t, root, "update"); rc != 0 {
				t.Fatalf("rc %d, log:\n%s", rc, log)
			}
			if got := page(t, root, "p.md"); !strings.Contains(got, sampleFilled) {
				t.Errorf("page:\n%s", got)
			}
		})
	}
}

func TestDocgenSampleDocsStayInTheCopy(t *testing.T) {
	t.Parallel()
	for name, docs := range map[string]func(root string) string{
		"relative": func(string) string { return "../.." },
		"absolute": func(root string) string { return filepath.Join(root, "docs") },
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			text := nestedMarker("root=testdata/sample page=a.md")
			root := outerRepo(t, text, map[string]string{"go.mod": sampleMod, "a.md": "A\n"})
			other := filepath.Join(root, "docs/other.md")
			if err := os.WriteFile(other, []byte(sampleGoMod), 0o600); err != nil {
				t.Fatal(err)
			}
			writeTree(t, root, map[string]string{"testdata/sample/.docgen.yaml": "docs: " + docs(root) + "\n"})
			rc, log := docgen(t, root, "update")
			if rc != 1 || !strings.Contains(log, "is outside root=testdata/sample") {
				t.Errorf("rc %d, log:\n%s", rc, log)
			}
			if got := page(t, root, "other.md"); got != sampleGoMod {
				t.Errorf("the nested run wrote outside the copy: %q", got)
			}
		})
	}
}

// TestDocgenVariablesStayOutside sets variables, so it does not run in parallel.
//
//nolint:paralleltest // t.Setenv
func TestDocgenVariablesStayOutside(t *testing.T) {
	root := outerRepo(t, nestedMarker("root=testdata/sample page=docs/a.md"), map[string]string{"go.mod": sampleMod, "docs/a.md": sampleGoMod})
	if err := os.Rename(filepath.Join(root, "docs"), filepath.Join(root, "pages")); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DOCGEN_DOCS", "pages")
	if rc, log := docgen(t, root, "update"); rc != 0 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
	if got, err := os.ReadFile(filepath.Join(root, "pages/p.md")); err != nil || !strings.Contains(string(got), sampleFilled) {
		t.Errorf("page: %v\n%s", err, got)
	}
}

func TestDocgenSessionSettings(t *testing.T) {
	t.Parallel()
	sample := map[string]string{
		".docgen.yaml":         "tags: other\n",
		"go.mod":               sampleMod,
		"docs/a.md":            "<!-- docgen:output cmd=examples/tagged --><!-- /docgen:output -->\n",
		"examples/tagged/a.go": "//go:build outer && !other\n\npackage main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"outer tag\")\n}\n",
		"examples/tagged/b.go": "//go:build !outer || other\n\npackage main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"other tags\")\n}\n",
	}
	text := nestedMarker("root=testdata/sample page=docs/a.md")
	for _, args := range [][]string{{"-tags", "outer"}, {"-tags", "outer", "-jobs", "1"}, {"-tags", "outer", "-jobs", "2"}} {
		root := outerRepo(t, text, sample)
		if rc, log := docgen(t, root, append([]string{"update"}, args...)...); rc != 0 {
			t.Fatalf("%q: rc %d, log:\n%s", args, rc, log)
		}
		if got := page(t, root, "p.md"); !strings.Contains(got, "outer tag") {
			t.Errorf("%q: page:\n%s", args, got)
		}
	}
}

func TestDocgenNoLimitDeadlock(t *testing.T) {
	t.Parallel()
	file := filepath.Join(t.TempDir(), "runs")
	sample := map[string]string{}
	for _, dir := range []string{"a", "b", "c"} {
		for name, body := range counting(file) {
			sample[dir+"/"+name] = body
		}
	}
	text := nestedMarker("root=testdata/sample/a page=docs/a.md") + "\n" + nestedMarker("root=testdata/sample/b page=docs/a.md") + "\n" + nestedMarker("root=testdata/sample/c page=docs/a.md")
	root := outerRepo(t, text, sample)
	if rc, log := docgen(t, root, "update", "-jobs", "1"); rc != 0 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
	if got := runsIn(t, file); got != 3 {
		t.Errorf("%d runs, want 3", got)
	}
}

func TestDocgenNoTempPathInTheOutput(t *testing.T) {
	t.Parallel()
	root := outerRepo(t, nestedMarker("root=testdata/sample page=docs/a.md"), map[string]string{
		"go.mod":    sampleMod,
		"docs/a.md": "<!-- docgen:output cmd=examples/where --><!-- /docgen:output -->\n",
		"examples/where/main.go": `package main

import (
	"fmt"
	"os"
)

func main() {
	cwd, _ := os.Getwd()
	fmt.Println("dir", cwd)
	fmt.Println("program", os.Args[0])
}
`,
	})
	if rc, log := docgen(t, root, "update"); rc != 0 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
	got := page(t, root, "p.md")
	if !strings.Contains(got, "dir testdata/sample\n") {
		t.Errorf("page:\n%s", got)
	}
	if regexp.MustCompile(`/docgen/run-|go-build\d`).MatchString(got) {
		t.Errorf("the page holds a temporary path:\n%s", got)
	}
	if rc, log := docgen(t, root, "check"); rc != 0 {
		t.Errorf("check: rc %d, log:\n%s", rc, log)
	}
}

// TestDocgenCacheInsideRoot sets a variable, so it does not run in parallel.
//
//nolint:paralleltest // t.Setenv
func TestDocgenCacheInsideRoot(t *testing.T) {
	root := outerRepo(t, nestedMarker("root=testdata/sample page=docs/a.md"), map[string]string{"go.mod": sampleMod, "docs/a.md": sampleGoMod})
	t.Setenv("XDG_CACHE_HOME", filepath.Join(root, "testdata/sample/cache"))
	if rc, log := docgen(t, root, "update"); rc != 0 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
	if es, err := os.ReadDir(filepath.Join(root, "testdata/sample/cache/docgen")); err != nil && !errors.Is(err, os.ErrNotExist) || len(es) != 0 {
		t.Errorf("run directories after the run: %v, %v", es, err)
	}
}

func TestFixtureImagesMatchTheGolden(t *testing.T) {
	t.Parallel()
	for _, f := range []string{"machine-lamp.svg", "machine-counter.svg", "machine-pair-sidestate.svg", "door-open.svg"} {
		got, err := os.ReadFile(filepath.Join("testdata/repo/docs/machines/states", f))
		if err != nil {
			t.Fatal(err)
		}
		want, err := os.ReadFile(filepath.Join("testdata/want/states", f))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("testdata/repo/docs/machines/states/%s differs from testdata/want/states", f)
		}
	}
}
