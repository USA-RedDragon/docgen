// Package sitetest tests the scripts of docs/site.
package sitetest

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func copySite(t *testing.T, dst string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.CopyFS(dst, os.DirFS("../../docs/site")); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(dst, "build.sh"), 0o755); err != nil { //nolint:gosec // the script must be executable
		t.Fatal(err)
	}
}

func repo(t *testing.T) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	root = filepath.Join(root, "repo")
	copySite(t, filepath.Join(root, "docs/site"))
	for name, text := range map[string]string{
		"README.md":    "# Widget\n\nIntro.\n",
		"docs/nav.md":  "- [Home](../README.md)\n- [Page](page.md)\n",
		"docs/page.md": "# Page\n\nText.\n",
		"docs/sub/x":   "x\n",
		"go.mod":       "module github.com/acme/widget\n\ngo 1.24.0\n",
	} {
		p := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// guardBin holds a stub rm, written before any test starts because a parallel
// fork can hold a new executable open for writing, which fails its exec.
//
//nolint:gochecknoglobals // TestMain sets it once
var guardBin string

func TestMain(m *testing.M) {
	os.Exit(runTests(m))
}

func runTests(m *testing.M) int {
	rmPath, err := exec.LookPath("rm")
	if err != nil {
		panic(err)
	}
	guardBin, err = os.MkdirTemp("", "docgen-guard-")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(guardBin)
	script := "#!/bin/sh\nfor a in \"$@\"; do\n\tcase $a in\n\t-*) ;;\n\t\"$DOCGEN_RM_GUARD\"/*) ;;\n\t*) echo \"guarded rm: $a\" >&2; exit 97 ;;\n\tesac\ndone\nexec '" + rmPath + "' \"$@\"\n"
	if err := os.WriteFile(filepath.Join(guardBin, "rm"), []byte(script), 0o700); err != nil { //nolint:gosec // the stub must be executable
		panic(err)
	}
	return m.Run()
}

// guardedEnv returns the environment of a run whose rm exits 97 for a path
// outside dir, so that a fault in build.sh cannot delete anything else.
func guardedEnv(dir string) []string {
	return []string{"PATH=" + guardBin + string(os.PathListSeparator) + os.Getenv("PATH"), "DOCGEN_RM_GUARD=" + dir}
}

func build(t *testing.T, root, cwd string, env []string, args ...string) (string, int) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "sh", append([]string{filepath.Join(root, "docs/site/build.sh")}, args...)...)
	cmd.Dir = cwd
	cmd.Env = append(append(append(os.Environ(), "GIT_CEILING_DIRECTORIES="+filepath.Dir(root)), guardedEnv(filepath.Dir(root))...), env...)
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	err := cmd.Run()
	var ee *exec.ExitError
	switch {
	case err == nil:
		return out.String(), 0
	case errors.As(err, &ee):
		return out.String(), ee.ExitCode()
	}
	t.Fatal(err)
	return "", 0
}

func needPandoc(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("pandoc"); err != nil {
		t.Skip("pandoc is not on PATH")
	}
}

func exists(t *testing.T, p string) {
	t.Helper()
	if _, err := os.Stat(p); err != nil {
		t.Error(err)
	}
}

func TestOutputRefusals(t *testing.T) {
	t.Parallel()
	for name, c := range map[string]struct{ cwd, out string }{
		"docs":             {".", "docs"},
		"docs, slash":      {".", "docs/"},
		"root":             {".", "."},
		"root, dotdot":     {"docs", ".."},
		"root, missing":    {".", "gone/.."},
		"parent of root":   {".", ".."},
		"inside docs":      {".", "docs/out"},
		"inside, cwd":      {"docs", "out"},
		"site":             {".", "docs/site"},
		"docs, symlink":    {".", "link"},
		"inside, symlink":  {".", "link/out"},
		"a file":           {".", "README.md"},
		"docs from a cwd":  {"docs/sub", "../../docs"},
		"root, trailing .": {"docs", "../."},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := repo(t)
			if err := os.Symlink("docs", filepath.Join(root, "link")); err != nil {
				t.Fatal(err)
			}
			out, rc := build(t, root, filepath.Join(root, c.cwd), nil, c.out)
			if rc != 2 || !strings.Contains(out, "build.sh: ") {
				t.Errorf("rc %d, want 2, output:\n%s", rc, out)
			}
			for _, f := range []string{"README.md", "docs/page.md", "docs/site/build.sh", "go.mod"} {
				exists(t, filepath.Join(root, f))
			}
		})
	}
}

func TestOutputPaths(t *testing.T) {
	t.Parallel()
	needPandoc(t)
	root := repo(t)
	if out, rc := build(t, root, filepath.Join(root, "docs"), nil); rc != 0 {
		t.Fatalf("default output: rc %d:\n%s", rc, out)
	}
	exists(t, filepath.Join(root, "artifacts/docs-site/index.html"))
	cwd := filepath.Join(filepath.Dir(root), "caller dir")
	if err := os.MkdirAll(cwd, 0o755); err != nil {
		t.Fatal(err)
	}
	if out, rc := build(t, root, cwd, nil, "site out"); rc != 0 {
		t.Fatalf("relative output: rc %d:\n%s", rc, out)
	}
	exists(t, filepath.Join(cwd, "site out/index.html"))
	exists(t, filepath.Join(cwd, "site out/page.html"))
	if _, err := os.Stat(filepath.Join(root, "site out")); err == nil {
		t.Error("the relative output resolved from the repository root")
	}
}

func TestModuleInSubdirectory(t *testing.T) {
	t.Parallel()
	needPandoc(t)
	for name, c := range map[string]struct{ dir, mod, repoURL, api string }{
		"major version": {"v2", "github.com/acme/widget/v2", "https://github.com/acme/widget", "https://pkg.go.dev/github.com/acme/widget/v2"},
		"nested module": {"tools/gen", "github.com/acme/widget/tools/gen", "https://github.com/acme/widget", "https://pkg.go.dev/github.com/acme/widget/tools/gen"},
		"both":          {"tools/gen", "github.com/acme/widget/tools/gen/v3", "https://github.com/acme/widget", "https://pkg.go.dev/github.com/acme/widget/tools/gen/v3"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := repo(t)
			if err := os.Remove(filepath.Join(root, "go.mod")); err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(filepath.Join(root, c.dir), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, c.dir, "go.mod"), []byte("module "+c.mod+"\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			out, rc := build(t, root, root, []string{"DOCS_MODULE=" + c.dir})
			if rc != 0 {
				t.Fatalf("rc %d:\n%s", rc, out)
			}
			page, err := os.ReadFile(filepath.Join(root, "artifacts/docs-site/page.html"))
			if err != nil {
				t.Fatal(err)
			}
			for _, want := range []string{`href="` + c.repoURL + `"`, c.repoURL + "/blob/main/docs/page.md", `href="` + c.api + `"`} {
				if !strings.Contains(string(page), want) {
					t.Errorf("the page does not hold %s", want)
				}
			}
		})
	}
}

func TestRootModuleMajorVersion(t *testing.T) {
	t.Parallel()
	needPandoc(t)
	root := repo(t)
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module github.com/acme/widget/v12\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if out, rc := build(t, root, root, nil); rc != 0 {
		t.Fatalf("rc %d:\n%s", rc, out)
	}
	page, err := os.ReadFile(filepath.Join(root, "artifacts/docs-site/page.html"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`href="https://github.com/acme/widget"`, `href="https://pkg.go.dev/github.com/acme/widget/v12"`} {
		if !strings.Contains(string(page), want) {
			t.Errorf("the page does not hold %s", want)
		}
	}
}

func TestMissingReadme(t *testing.T) {
	t.Parallel()
	needPandoc(t)
	root := repo(t)
	if err := os.Remove(filepath.Join(root, "README.md")); err != nil {
		t.Fatal(err)
	}
	out, rc := build(t, root, root, nil)
	if rc == 0 || !strings.Contains(out, "README.md not found") || strings.Contains(out, "concatenate") {
		t.Errorf("rc %d, output:\n%s", rc, out)
	}
}

func TestNoModule(t *testing.T) {
	t.Parallel()
	needPandoc(t)
	root := repo(t)
	if err := os.Remove(filepath.Join(root, "go.mod")); err != nil {
		t.Fatal(err)
	}
	out, rc := build(t, root, root, nil)
	if rc == 0 || !strings.Contains(out, "no go.mod at the repository root") {
		t.Errorf("rc %d, output:\n%s", rc, out)
	}
}

func TestRefuseUnrelatedDirectory(t *testing.T) {
	t.Parallel()
	root := repo(t)
	keep := filepath.Join(root, "internal", "keep.go")
	if err := os.MkdirAll(filepath.Dir(keep), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keep, []byte("package internal\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if out, rc := build(t, root, root, nil, "internal"); rc != 2 || !strings.Contains(out, "not an earlier build") {
		t.Fatalf("rc %d, output:\n%s", rc, out)
	}
	if _, err := os.Stat(keep); err != nil {
		t.Fatal(err)
	}
}

func TestGuardBlocksOutsidePaths(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cmd := exec.CommandContext(t.Context(), filepath.Join(guardBin, "rm"), "-rf", "/nonexistent/docgen-guard-test")
	cmd.Env = append(os.Environ(), guardedEnv(dir)...)
	var ee *exec.ExitError
	if err := cmd.Run(); !errors.As(err, &ee) || ee.ExitCode() != 97 {
		t.Fatalf("err %v, want exit 97", err)
	}
}

func buildPage(t *testing.T, root string, env []string, name string) string {
	t.Helper()
	if out, rc := build(t, root, root, env); rc != 0 {
		t.Fatalf("rc %d:\n%s", rc, out)
	}
	b, err := os.ReadFile(filepath.Join(root, "artifacts/docs-site", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func writeRepoFile(t *testing.T, root, name, text string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestLinkToAPageThatIsNotBuilt(t *testing.T) {
	t.Parallel()
	needPandoc(t)
	root := repo(t)
	writeRepoFile(t, root, "docs/page.md", "# Page\n\nThe [nav](nav.md), [more](more.md).\n")
	writeRepoFile(t, root, "docs/more.md", "# More\n")
	got := buildPage(t, root, nil, "page.html")
	for _, want := range []string{`href="https://github.com/acme/widget/blob/main/docs/nav.md"`, `href="more.html"`} {
		if !strings.Contains(got, want) {
			t.Errorf("the page does not hold %s:\n%s", want, got)
		}
	}
}

func TestModuleLine(t *testing.T) {
	t.Parallel()
	needPandoc(t)
	for name, text := range map[string]string{
		"comment":       "// module github.com/wrong/one\nmodule github.com/acme/widget\n",
		"quoted":        "module \"github.com/acme/widget\"\n",
		"backquoted":    "module `github.com/acme/widget`\n",
		"line comment":  "module github.com/acme/widget // the module\n",
		"word in a url": "require modulex v1.0.0\n\nmodule github.com/acme/widget\n",
		"block":         "module (\n\tgithub.com/acme/widget\n)\n",
		"crlf":          "module github.com/acme/widget\r\n\r\ngo 1.24.0\r\n",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := repo(t)
			writeRepoFile(t, root, "go.mod", text)
			got := buildPage(t, root, nil, "page.html")
			for _, want := range []string{`href="https://github.com/acme/widget"`, `href="https://pkg.go.dev/github.com/acme/widget"`} {
				if !strings.Contains(got, want) {
					t.Errorf("the page does not hold %s", want)
				}
			}
		})
	}
}

func TestDocsDirForms(t *testing.T) {
	t.Parallel()
	needPandoc(t)
	for name, dir := range map[string]func(root string) string{
		"dot slash": func(string) string { return "./content" },
		"slash":     func(string) string { return "content/" },
		"parent":    func(string) string { return "docs/../content" },
		"absolute":  func(root string) string { return filepath.Join(root, "content") },
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := repo(t)
			writeRepoFile(t, root, "content/nav.md", "- [Home](../README.md)\n- [Page](page.md)\n")
			writeRepoFile(t, root, "content/page.md", "# Page\n\nA [file](../go.mod).\n")
			got := buildPage(t, root, []string{"DOCS_DIR=" + dir(root)}, "page.html")
			if strings.Contains(got, "/./") {
				t.Errorf("a link holds /./:\n%s", got)
			}
			if want := `href="https://github.com/acme/widget/blob/main/content/page.md"`; !strings.Contains(got, want) {
				t.Errorf("the page does not hold %s:\n%s", want, got)
			}
		})
	}
}

func TestDocsDirRefusals(t *testing.T) {
	t.Parallel()
	for name, c := range map[string]struct {
		dir  func(root string) string
		want string
	}{
		"outside, absolute": {filepath.Dir, "outside the repository root"},
		"outside, relative": {func(string) string { return "../elsewhere" }, "outside the repository root"},
		"root":              {func(string) string { return "." }, "below the repository root"},
		"root, parent":      {func(string) string { return "docs/.." }, "below the repository root"},
		"root, absolute":    {func(root string) string { return root }, "below the repository root"},
		"missing":           {func(string) string { return "gone" }, "gone does not exist"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := repo(t)
			old := filepath.Join(root, "artifacts/docs-site/index.html")
			writeRepoFile(t, root, "artifacts/docs-site/index.html", "old\n")
			writeRepoFile(t, root, "artifacts/docs-site/.docgen-site", "")
			out, rc := build(t, root, root, []string{"DOCS_DIR=" + c.dir(root)})
			if rc != 2 || !strings.Contains(out, c.want) {
				t.Errorf("rc %d, output:\n%s", rc, out)
			}
			exists(t, old)
		})
	}
}

func TestSearchEntriesOfSimilarNames(t *testing.T) {
	t.Parallel()
	needPandoc(t)
	root := repo(t)
	writeRepoFile(t, root, "docs/a/b.md", "# Slash\n")
	writeRepoFile(t, root, "docs/a__b.md", "# Underscores\n")
	writeRepoFile(t, root, "docs/a%2Fb.md", "# Percent\n")
	got := buildPage(t, root, nil, "search-index.js")
	for _, want := range []string{"Slash", "Underscores", "Percent"} {
		if !strings.Contains(got, want) {
			t.Errorf("the index does not hold %s:\n%s", want, got)
		}
	}
}

func TestNoModuleMessages(t *testing.T) {
	t.Parallel()
	needPandoc(t)
	for name, c := range map[string]struct {
		text string
		env  []string
		want string
	}{
		"no go.mod":      {"", nil, "build.sh: no go.mod at the repository root; set DOCS_MODULE to the directory of the Go module"},
		"no DOCS_MODULE": {"", []string{"DOCS_MODULE=gone"}, "build.sh: no go.mod in DOCS_MODULE directory gone"},
		"no module line": {"go 1.24.0\n", nil, "build.sh: ./go.mod has no module line"},
		"only in a word": {"// module\n", nil, "build.sh: ./go.mod has no module line"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := repo(t)
			old := filepath.Join(root, "artifacts/docs-site/index.html")
			writeRepoFile(t, root, "artifacts/docs-site/index.html", "old\n")
			writeRepoFile(t, root, "artifacts/docs-site/.docgen-site", "")
			if c.text == "" {
				if err := os.Remove(filepath.Join(root, "go.mod")); err != nil {
					t.Fatal(err)
				}
			} else {
				writeRepoFile(t, root, "go.mod", c.text)
			}
			out, rc := build(t, root, root, c.env)
			if rc == 0 || !strings.Contains(out, c.want) || strings.Contains(out, "stack traceback") {
				t.Errorf("rc %d, output:\n%s", rc, out)
			}
			exists(t, old)
		})
	}
}

func TestExpectedFailureMessages(t *testing.T) {
	t.Parallel()
	needPandoc(t)
	for name, c := range map[string]struct {
		change func(t *testing.T, root string)
		want   string
	}{
		"no nav": {func(t *testing.T, root string) {
			t.Helper()
			if err := os.Remove(filepath.Join(root, "docs/nav.md")); err != nil {
				t.Fatal(err)
			}
		}, "build.sh: docs/nav.md not found"},
		"nav with no list": {func(t *testing.T, root string) {
			t.Helper()
			writeRepoFile(t, root, "docs/nav.md", "Text.\n")
		}, "build.sh: docs/nav.md has no list"},
		"no pages": {func(t *testing.T, root string) {
			t.Helper()
			writeRepoFile(t, root, "docs/nav.md", "- [Elsewhere](https://example.com)\n")
			if err := os.Remove(filepath.Join(root, "docs/page.md")); err != nil {
				t.Fatal(err)
			}
		}, "build.sh: docs has no pages to build"},
		"missing image": {func(t *testing.T, root string) {
			t.Helper()
			writeRepoFile(t, root, "docs/page.md", "# Page\n\n![x](gone.png)\n")
		}, "build.sh: docs/page.md: image gone.png not found"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := repo(t)
			c.change(t, root)
			out, rc := build(t, root, root, nil)
			if rc == 0 || !strings.Contains(out, c.want) || strings.Contains(out, "stack traceback") {
				t.Errorf("rc %d, output:\n%s", rc, out)
			}
		})
	}
}

func TestNoPandocKeepsTheOutput(t *testing.T) {
	t.Parallel()
	root := repo(t)
	bin := t.TempDir()
	for _, tool := range []string{"sh", "git", "dirname", "ls", "mkdir"} {
		p, err := exec.LookPath(tool)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(p, filepath.Join(bin, tool)); err != nil {
			t.Fatal(err)
		}
	}
	writeRepoFile(t, root, "artifacts/docs-site/index.html", "old\n")
	writeRepoFile(t, root, "artifacts/docs-site/.docgen-site", "")
	out, rc := build(t, root, root, []string{"PATH=" + guardBin + string(os.PathListSeparator) + bin})
	if rc != 2 || !strings.Contains(out, "build.sh: pandoc is not on PATH") {
		t.Errorf("rc %d, output:\n%s", rc, out)
	}
	exists(t, filepath.Join(root, "artifacts/docs-site/index.html"))
}

func TestRefuseDirectoryWithoutTheMarkerFile(t *testing.T) {
	t.Parallel()
	for name, file := range map[string]string{"search index": "search-index.js", "page list": ".pages", "other": "notes.txt"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := repo(t)
			writeRepoFile(t, root, "other/"+file, "x\n")
			writeRepoFile(t, root, "other/keep.txt", "keep\n")
			if out, rc := build(t, root, root, nil, "other"); rc != 2 || !strings.Contains(out, "not an earlier build") {
				t.Fatalf("rc %d, output:\n%s", rc, out)
			}
			exists(t, filepath.Join(root, "other/keep.txt"))
		})
	}
}

func TestRebuildOverAnEarlierBuild(t *testing.T) {
	t.Parallel()
	needPandoc(t)
	root := repo(t)
	if out, rc := build(t, root, root, nil, "site"); rc != 0 {
		t.Fatalf("first build: rc %d:\n%s", rc, out)
	}
	exists(t, filepath.Join(root, "site/.docgen-site"))
	writeRepoFile(t, root, "site/old.txt", "old\n")
	if out, rc := build(t, root, root, nil, "site"); rc != 0 {
		t.Fatalf("second build: rc %d:\n%s", rc, out)
	}
	exists(t, filepath.Join(root, "site/.docgen-site"))
	if _, err := os.Stat(filepath.Join(root, "site/old.txt")); err == nil {
		t.Error("the second build kept a file of the earlier output")
	}
}

const docgenBody = "#### Before\n\n```\n# Sample\n```\n\n#### After\n\n````\n# Sample\n\n![The diagram](states/a.svg)\n````\n"

func TestDocgenMarkerIsAnEditorBlock(t *testing.T) {
	t.Parallel()
	needPandoc(t)
	for name, c := range map[string]struct{ open, image string }{
		"plain":         {`<!-- docgen:docgen root=sample page=docs/m/a.md -->`, "sample/docs/m/states/a.svg"},
		"module":        {`<!-- docgen:docgen module=v2 root=sample page=docs/m/a.md -->`, "sample/docs/m/states/a.svg"},
		"quoted":        {`<!-- docgen:docgen root="sample" page="docs/m/a.md" module=v2 -->`, "sample/docs/m/states/a.svg"},
		"page in root":  {`<!-- docgen:docgen root=sample/docs/m page=a.md -->`, "sample/docs/m/states/a.svg"},
		"dot root":      {`<!-- docgen:docgen root=. page=sample/docs/m/a.md -->`, "sample/docs/m/states/a.svg"},
		"relative root": {`<!-- docgen:docgen root=./sample/ page=docs/m/a.md -->`, "sample/docs/m/states/a.svg"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := repo(t)
			writeRepoFile(t, root, "sample/docs/m/states/a.svg", "<svg xmlns=\"http://www.w3.org/2000/svg\"/>\n")
			writeRepoFile(t, root, "docs/page.md", "# Page\n\n"+c.open+"\n"+docgenBody+"<!-- /docgen:docgen -->\n\nEnd.\n")
			got := buildPage(t, root, nil, "page.html")
			block := editorBlock(t, got)
			if n := strings.Count(block, `editor-file"`); n != 1 {
				t.Errorf("%d files, want 1:\n%s", n, block)
			}
			contains(t, block, `data-name="Before"`, `data-name="After"`, `editor-preview"`, `src="`+c.image+`"`)
			contains(t, got, `<script type="module" src="editor.js"></script>`)
			exists(t, filepath.Join(root, "artifacts/docs-site", c.image))
			exists(t, filepath.Join(root, "artifacts/docs-site/editor.js"))
			if strings.Contains(got, `href="#before"`) {
				t.Errorf("a version name is in the table of contents:\n%s", got)
			}
		})
	}
}

func TestDocgenMarkerImageMissing(t *testing.T) {
	t.Parallel()
	needPandoc(t)
	root := repo(t)
	writeRepoFile(t, root, "docs/page.md", "# Page\n\n<!-- docgen:docgen root=sample page=docs/m/a.md -->\n"+docgenBody+"<!-- /docgen:docgen -->\n")
	out, rc := build(t, root, root, nil)
	if rc == 0 || !strings.Contains(out, "image") || !strings.Contains(out, "not found") {
		t.Errorf("rc %d, output:\n%s", rc, out)
	}
}

func TestLargeBlock(t *testing.T) {
	t.Parallel()
	needPandoc(t)
	const svg = "<svg xmlns=\"http://www.w3.org/2000/svg\"/>\n"
	marker := "<!-- docgen:docgen root=sample page=docs/m/a.md -->\n" + docgenBody + "<!-- /docgen:docgen -->\n"
	for name, text := range map[string]string{
		"line before":  "<!-- site:large -->\n" + marker,
		"blank line":   "<!-- site:large -->\n\n" + marker,
		"editor block": "<!-- site:large -->\n<!-- site:editor -->\n" + marker + "<!-- /site:editor -->\n",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := repo(t)
			writeRepoFile(t, root, "sample/docs/m/states/a.svg", svg)
			writeRepoFile(t, root, "docs/page.md", "# Page\n\n"+text)
			contains(t, buildPage(t, root, nil, "page.html"), `<div class="editor large"`)
		})
	}
}

func TestLargeMarkAppliesToTheNextBlockOnly(t *testing.T) {
	t.Parallel()
	needPandoc(t)
	root := repo(t)
	writeRepoFile(t, root, "sample/docs/m/states/a.svg", "<svg xmlns=\"http://www.w3.org/2000/svg\"/>\n")
	writeRepoFile(t, root, "docs/page.md", "# Page\n\n<!-- site:large -->\n\nText.\n\n<!-- docgen:docgen root=sample page=docs/m/a.md -->\n"+docgenBody+"<!-- /docgen:docgen -->\n")
	got := buildPage(t, root, nil, "page.html")
	if strings.Contains(got, `class="editor large"`) {
		t.Errorf("the block is large:\n%s", got)
	}
	if strings.Contains(got, "site:large") {
		t.Errorf("the mark is in the page:\n%s", got)
	}
}

func TestHeadingsAreInTheNavigation(t *testing.T) {
	t.Parallel()
	needPandoc(t)
	root := repo(t)
	writeRepoFile(t, root, "docs/nav.md", "- [Home](../README.md)\n- [Page](page.md)\n- [Other](other.md)\n")
	writeRepoFile(t, root, "docs/page.md", "# Page\n\n## One\n\n### Two\n\n#### Four\n")
	writeRepoFile(t, root, "docs/other.md", "# Other\n\nText.\n")
	writeRepoFile(t, root, "docs/extra.md", "# Extra\n\n## A\n\n## B\n")
	got := buildPage(t, root, nil, "page.html")
	_, item, ok := strings.Cut(got, `<li class="here"><a href="page.html" class="on" aria-current="page">Page</a><details class="toc"><summary aria-label="On this page" title="On this page"></summary>`)
	if !ok {
		t.Fatalf("no disclosure under the item of the page:\n%s", got)
	}
	item, _, _ = strings.Cut(item, "</details>")
	contains(t, item, `<li class="l2"><a href="#one">One</a></li>`, `<li class="l3"><a href="#two">Two</a></li>`)
	if strings.Contains(item, "four") {
		t.Errorf("a level 4 heading is in the list:\n%s", item)
	}
	for _, s := range []string{"onpage", "toc-inline", `<details class="toc" open`} {
		if strings.Contains(got, s) {
			t.Errorf("the page holds %s", s)
		}
	}
	if other := buildPage(t, root, nil, "other.html"); strings.Contains(other, `<details class="toc"`) {
		t.Errorf("a page with no headings has a disclosure:\n%s", other)
	}
	contains(t, buildPage(t, root, nil, "extra.html"), `<span>On this page</span>`, `href="#a"`)
}

func TestDocgenMarkerInCodeIsText(t *testing.T) {
	t.Parallel()
	needPandoc(t)
	root := repo(t)
	writeRepoFile(t, root, "docs/page.md", "# Page\n\n```text\n<!-- docgen:docgen root=sample page=a.md -->\n<!-- /docgen:docgen -->\n```\n")
	if got := buildPage(t, root, nil, "page.html"); strings.Contains(got, `class="editor`) {
		t.Errorf("an editor block from text in code:\n%s", got)
	}
}

const editorCode = "<!-- docgen:code file=sample/v2/m.go symbol=Wait -->\n```go\nconst Wait = 1\n```\n<!-- /docgen:code -->\n"

const editorPage = "<!-- docgen:docgen root=sample module=v2 page=docs/m/a.md -->\n" + docgenBody + "<!-- /docgen:docgen -->\n"

// editorRepo returns a repository with docs/page.md of the given text and a
// sample directory for the markers of an editor block.
func editorRepo(t *testing.T, text string) string {
	t.Helper()
	root := repo(t)
	writeRepoFile(t, root, "sample/docs/m/states/a.svg", "<svg xmlns=\"http://www.w3.org/2000/svg\"/>\n")
	writeRepoFile(t, root, "sample/v2/m.go", "package m\n\nconst Wait = 1\n")
	writeRepoFile(t, root, "docs/page.md", text)
	return root
}

// editorBlock returns the part of the page from the editor block to the end of
// the article.
func editorBlock(t *testing.T, page string) string {
	t.Helper()
	_, rest, ok := strings.Cut(page, `<div class="editor"`)
	if !ok {
		t.Fatalf("the page holds no editor block:\n%s", page)
	}
	block, _, ok := strings.Cut(rest, `<p class="source">`)
	if !ok {
		t.Fatalf("the page has no end of the article:\n%s", page)
	}
	return block
}

func contains(t *testing.T, got string, want ...string) {
	t.Helper()
	for _, w := range want {
		if !strings.Contains(got, w) {
			t.Errorf("no %s in:\n%s", w, got)
		}
	}
}

func TestEditorBlockHTML(t *testing.T) {
	t.Parallel()
	needPandoc(t)
	for name, body := range map[string]string{
		"code first": editorCode + editorPage,
		"page first": editorPage + editorCode,
		"blank":      editorCode + "\n" + editorPage + "\n",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := editorRepo(t, "# Page\n\nIntro.\n\n<!-- site:editor -->\n"+body+"<!-- /site:editor -->\n\nEnd.\n")
			got := buildPage(t, root, nil, "page.html")
			block := editorBlock(t, got)
			contains(t, block, ` data-root="sample"`, `data-path="v2/m.go"`, `data-line="3"`, `data-path="docs/m/a.md"`, `data-line="1"`, `data-name="Before"`, `data-name="After"`, `editor-preview"`)
			if strings.Contains(block, `data-path="sample/`) {
				t.Errorf("a path holds the root:\n%s", block)
			}
			contains(t, got, `<script type="module" src="editor.js"></script>`, `src="sample/docs/m/states/a.svg"`)
			if strings.Contains(got, "site:editor") || strings.Contains(got, `href="#before"`) {
				t.Errorf("a mark or a title is left in the page:\n%s", got)
			}
			exists(t, filepath.Join(root, "artifacts/docs-site/editor.js"))
			exists(t, filepath.Join(root, "artifacts/docs-site/codemirror.js"))
			if _, err := os.Stat(filepath.Join(root, "artifacts/docs-site/.editor")); err == nil {
				t.Error("the build left the file .editor")
			}
		})
	}
}

func TestEditorBlockHoldsContentWithNoJavaScript(t *testing.T) {
	t.Parallel()
	needPandoc(t)
	root := editorRepo(t, "# Page\n\n<!-- site:editor -->\n"+editorCode+editorPage+"<!-- /site:editor -->\n")
	block := editorBlock(t, buildPage(t, root, nil, "page.html"))
	contains(t, block,
		`<code>v2/m.go</code></h4>`, `<code>docs/m/a.md</code></h4>`,
		`>Before</h5>`, `>After</h5>`, `>Preview</h5>`,
		`<code class="sourceCode go">`, `Wait <span class="op">=</span>`,
		`# Sample`, `<h1 class="sample">Sample</h1>`, `<img src="sample/docs/m/states/a.svg"`)
	for _, hidden := range []string{" hidden", "<button", "<script", "<style"} {
		if strings.Contains(block, hidden) {
			t.Errorf("the block holds %s, and depends on a script or a style:\n%s", hidden, block)
		}
	}
}

func TestEditorLineNumbers(t *testing.T) {
	t.Parallel()
	needPandoc(t)
	for name, c := range map[string]struct{ file, code, want string }{
		"one line":        {"package m\n\nconst Wait = 1\n", "const Wait = 1", `data-line="3"`},
		"first line":      {"const Wait = 1\nvar x int\n", "const Wait = 1", `data-line="1"`},
		"last, no ending": {"package m\nconst Wait = 1", "const Wait = 1", `data-line="2"`},
		"crlf":            {"package m\r\n\r\nconst Wait = 1\r\n", "const Wait = 1", `data-line="3"`},
		"lines and tabs":  {"package m\n\nfunc F() {\n\treturn\n}\n", "func F() {\n\treturn\n}", `data-line="3"`},
		"tab after text":  {"package m\n\nvar a\tint\n", "var a   int", `data-line="3"`},
		"twice":           {"const Wait = 1\n\nconst Wait = 1\n", "const Wait = 1", ""},
		"part of a line":  {"// const Wait = 1\n", "const Wait = 1", ""},
		"longer line":     {"const Wait = 10\n", "const Wait = 1", ""},
		"not in the file": {"package m\n", "const Wait = 1", ""},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := editorRepo(t, "# Page\n\n<!-- site:editor -->\n<!-- docgen:code file=sample/v2/m.go -->\n```go\n"+c.code+"\n```\n<!-- /docgen:code -->\n<!-- /site:editor -->\n")
			writeRepoFile(t, root, "sample/v2/m.go", c.file)
			block := editorBlock(t, buildPage(t, root, nil, "page.html"))
			if c.want == "" && strings.Contains(block, "data-line") {
				t.Errorf("a line number with no true line:\n%s", block)
			}
			if !strings.Contains(block, c.want) {
				t.Errorf("no %s in:\n%s", c.want, block)
			}
		})
	}
	t.Run("no such file", func(t *testing.T) {
		t.Parallel()
		root := editorRepo(t, "# Page\n\n<!-- site:editor -->\n<!-- docgen:code file=sample/gone.go -->\n```go\nx\n```\n<!-- /docgen:code -->\n<!-- /site:editor -->\n")
		if block := editorBlock(t, buildPage(t, root, nil, "page.html")); strings.Contains(block, "data-line") {
			t.Errorf("a line number with no file:\n%s", block)
		}
	})
}

func TestEditorPaths(t *testing.T) {
	t.Parallel()
	needPandoc(t)
	for name, c := range map[string]struct{ open, code, want, root string }{
		"dot root":      {`root=. page=sample/docs/m/a.md`, "sample/v2/m.go", `data-path="sample/v2/m.go"`, ``},
		"relative root": {`root=./sample/ page=docs/m/a.md`, "sample/v2/m.go", `data-path="v2/m.go"`, ` data-root="sample"`},
		"quoted":        {`root="sample" page="docs/m/a.md"`, "sample/v2/m.go", `data-path="v2/m.go"`, ` data-root="sample"`},
		"deep root":     {`root=sample/docs page=m/a.md`, "sample/docs/m/states/a.svg", `data-path="m/states/a.svg"`, ` data-root="docs"`},
		"file outside":  {`root=sample/docs page=m/a.md`, "sample/v2/m.go", `data-path="sample/v2/m.go"`, ` data-root="docs"`},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			code := "<!-- docgen:code file=" + c.code + " -->\n```\nx\n```\n<!-- /docgen:code -->\n"
			root := editorRepo(t, "# Page\n\n<!-- site:editor -->\n"+code+"<!-- docgen:docgen "+c.open+" -->\n"+docgenBody+"<!-- /docgen:docgen -->\n<!-- /site:editor -->\n")
			contains(t, editorBlock(t, buildPage(t, root, nil, "page.html")), c.want, c.root)
		})
	}
}

func TestEditorBlockRefusals(t *testing.T) {
	t.Parallel()
	needPandoc(t)
	for name, c := range map[string]struct{ text, want string }{
		"no close":     {"<!-- site:editor -->\n" + editorCode, "docs/page.md: site:editor has no <!-- /site:editor -->"},
		"no files":     {"<!-- site:editor -->\n<!-- /site:editor -->\n", "docs/page.md: site:editor holds no files"},
		"text":         {"<!-- site:editor -->\n" + editorCode + "\nText.\n\n<!-- /site:editor -->\n", "docs/page.md: site:editor can hold docgen:code and docgen:docgen markers only"},
		"empty code":   {"<!-- site:editor -->\n<!-- docgen:code file=a.go -->\n<!-- /docgen:code -->\n<!-- /site:editor -->\n", "docgen:code marker with no file= or no code"},
		"no file":      {"<!-- site:editor -->\n<!-- docgen:code symbol=A -->\n```\nx\n```\n<!-- /docgen:code -->\n<!-- /site:editor -->\n", "docgen:code marker with no file= or no code"},
		"empty docgen": {"<!-- site:editor -->\n<!-- docgen:docgen root=sample page=a.md -->\n<!-- /docgen:docgen -->\n<!-- /site:editor -->\n", "docgen:docgen marker with no page= or no code"},
		"empty plain":  {"<!-- docgen:docgen root=sample page=a.md -->\n<!-- /docgen:docgen -->\n", "docs/page.md: has a docgen:docgen marker with no page= or no code"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := editorRepo(t, "# Page\n\n"+c.text)
			out, rc := build(t, root, root, nil)
			if rc == 0 || !strings.Contains(out, c.want) || strings.Contains(out, "stack traceback") {
				t.Errorf("rc %d, output:\n%s", rc, out)
			}
		})
	}
}

func TestDocgenMarkerOutsideAnEditorBlock(t *testing.T) {
	t.Parallel()
	needPandoc(t)
	root := editorRepo(t, "# Page\n\n<!-- site:editor -->\n"+editorCode+"<!-- /site:editor -->\n\nText.\n\n"+strings.ReplaceAll(editorPage, "module=v2 ", ""))
	got := buildPage(t, root, nil, "page.html")
	if n := strings.Count(got, `class="editor"`); n != 2 {
		t.Errorf("%d editor blocks, want 2:\n%s", n, got)
	}
}

func TestTabCodeIsGone(t *testing.T) {
	t.Parallel()
	needPandoc(t)
	root := editorRepo(t, "# Page\n\n"+editorPage)
	got := buildPage(t, root, nil, "page.html")
	for _, s := range []string{`role="tab"`, "tab-panel", "tab-bar", `".tabs"`} {
		if strings.Contains(got, s) {
			t.Errorf("the page holds %s:\n%s", s, got)
		}
	}
}

const excerptFile = "package m\n\nconst A = 1\n\nconst B = 2\nconst C = 3\n\nfunc Example() {\n\t// Output:\n\t// x\n}\n"

func codeMarker(args, code string) string {
	return "<!-- docgen:code file=sample/v2/m.go " + args + " -->\n```go\n" + code + "\n```\n<!-- /docgen:code -->\n"
}

func TestExcerptsOfOneFile(t *testing.T) {
	t.Parallel()
	needPandoc(t)
	text := "# Page\n\n<!-- site:editor -->\n" + codeMarker("symbol=C", "const C = 3") + editorPage + codeMarker("symbol=A", "const A = 1") + codeMarker("symbol=B", "const B = 2") + "<!-- /site:editor -->\n"
	root := editorRepo(t, text)
	writeRepoFile(t, root, "sample/v2/m.go", excerptFile)
	block := editorBlock(t, buildPage(t, root, nil, "page.html"))
	if n := strings.Count(block, `data-path="v2/m.go"`); n != 1 {
		t.Errorf("%d files of v2/m.go, want 1:\n%s", n, block)
	}
	a := strings.Index(block, `data-line="3"`)
	b := strings.Index(block, `data-line="5"`)
	c := strings.Index(block, `data-line="6"`)
	if a < 0 || a > b || b > c {
		t.Errorf("the excerpts are not in file order, at %d, %d, %d:\n%s", a, b, c, block)
	}
	if code, page := strings.Index(block, `data-path="v2/m.go"`), strings.Index(block, `data-path="docs/m/a.md"`); code > page {
		t.Errorf("the file moved after the page:\n%s", block)
	}
}

func TestExcerptsWithNoLineNumberKeepFileOrder(t *testing.T) {
	t.Parallel()
	needPandoc(t)
	text := "# Page\n\n<!-- site:editor -->\n" + codeMarker("symbol=Example", "func Example() {\n}") + codeMarker("symbol=A", "const A = 1") + "<!-- /site:editor -->\n"
	root := editorRepo(t, text)
	writeRepoFile(t, root, "sample/v2/m.go", excerptFile)
	block := editorBlock(t, buildPage(t, root, nil, "page.html"))
	line := strings.Index(block, `data-line="3"`)
	example := strings.Index(block, "Example")
	if line < 0 || example < line {
		t.Errorf("the excerpt at line 3 is not first:\n%s", block)
	}
	if n := strings.Count(block, "data-line"); n != 1 {
		t.Errorf("%d line numbers, want 1:\n%s", n, block)
	}
}

func TestOverlappingExcerptsAreRefused(t *testing.T) {
	t.Parallel()
	needPandoc(t)
	for name, second := range map[string]string{
		"same":     codeMarker("symbol=B", "const B = 2"),
		"overlaps": codeMarker("lines=5-6", "const B = 2\nconst C = 3"),
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := editorRepo(t, "# Page\n\n<!-- site:editor -->\n"+codeMarker("symbol=B", "const B = 2")+second+"<!-- /site:editor -->\n")
			writeRepoFile(t, root, "sample/v2/m.go", excerptFile)
			out, rc := build(t, root, root, nil)
			if rc == 0 || !strings.Contains(out, "docs/page.md: site:editor has excerpts of sample/v2/m.go that overlap") {
				t.Errorf("rc %d, output:\n%s", rc, out)
			}
		})
	}
}

func TestSearchTextLeavesOutExcerpts(t *testing.T) {
	t.Parallel()
	needPandoc(t)
	root := editorRepo(t, "# Page\n\nProse.\n\n<!-- site:editor -->\n"+editorCode+editorPage+"<!-- /site:editor -->\n")
	buildPage(t, root, nil, "page.html")
	index, err := os.ReadFile(filepath.Join(root, "artifacts/docs-site/search-index.js"))
	if err != nil {
		t.Fatal(err)
	}
	contains(t, string(index), "Prose.", "Sample")
	if strings.Contains(string(index), "const Wait") {
		t.Errorf("the search text holds an excerpt:\n%s", index)
	}
}

func TestNoEditorBlockNoBundle(t *testing.T) {
	t.Parallel()
	needPandoc(t)
	for name, text := range map[string]string{
		"plain":   "# Page\n\nText.\n",
		"in code": "# Page\n\n```text\n<!-- site:editor -->\n<!-- /site:editor -->\n```\n",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := editorRepo(t, text)
			got := buildPage(t, root, nil, "page.html")
			for _, s := range []string{"editor.js", "--vs-bg", `class="editor"`} {
				if strings.Contains(got, s) {
					t.Errorf("the page holds %s:\n%s", s, got)
				}
			}
			for _, f := range []string{"editor.js", "codemirror.js", ".editor"} {
				if _, err := os.Stat(filepath.Join(root, "artifacts/docs-site", f)); err == nil {
					t.Errorf("the site holds %s", f)
				}
			}
		})
	}
}

func TestEditorScriptPathFromASubdirectory(t *testing.T) {
	t.Parallel()
	needPandoc(t)
	root := editorRepo(t, "# Page\n\nText.\n")
	writeRepoFile(t, root, "docs/nav.md", "- [Home](../README.md)\n- [Page](page.md)\n- [Deep](sub/deep.md)\n")
	writeRepoFile(t, root, "docs/sub/deep.md", "# Deep\n\n<!-- site:editor -->\n"+editorCode+"<!-- /site:editor -->\n")
	contains(t, buildPage(t, root, nil, "sub/deep.html"), `<script type="module" src="../editor.js"></script>`)
	exists(t, filepath.Join(root, "artifacts/docs-site/codemirror.js"))
}

func TestConstGuideEditorBlock(t *testing.T) {
	t.Parallel()
	needPandoc(t)
	text, err := os.ReadFile("../../docs/guides/const.md")
	if err != nil {
		t.Fatal(err)
	}
	const file = "internal/cli/testdata/repo/v2/machine/machine.go"
	src, err := os.ReadFile(filepath.Join("../..", file))
	if err != nil {
		t.Fatal(err)
	}
	before, _, ok := strings.Cut(string(src), "const DefaultWait = ")
	if !ok {
		t.Fatalf("%s has no DefaultWait", file)
	}
	root := repo(t)
	writeRepoFile(t, root, "docs/page.md", string(text))
	writeRepoFile(t, root, file, string(src))
	got := buildPage(t, root, nil, "page.html")
	block := editorBlock(t, got)
	line := strconv.Itoa(strings.Count(before, "\n") + 1)
	contains(t, block, `data-root="repo"`, `data-path="v2/machine/machine.go"`, `data-line="`+line+`"`, `data-path="docs/machines/guide-const.md"`)
	if n := strings.Count(got, `class="editor"`); n != 2 {
		t.Errorf("%d editor blocks, want 2", n)
	}
}

// generatedSrc returns the data-src of the generated file path of block, from the
// page of the site.
func generatedSrc(t *testing.T, block, path string) string {
	t.Helper()
	_, rest, ok := strings.Cut(block, `<div class="editor-generated" data-path="`+path+`" data-src="`)
	if !ok {
		t.Fatalf("no generated file %s in:\n%s", path, block)
	}
	src, rest, _ := strings.Cut(rest, `"`)
	if !strings.HasPrefix(rest, ` data-version="After">`) {
		t.Errorf("the generated file %s is not of the version After:\n%s", path, block)
	}
	return src
}

func TestStatesBoxListsTheDiagram(t *testing.T) {
	t.Parallel()
	needPandoc(t)
	for _, c := range []struct{ page, out string }{
		{"docs/guides/state-machines.md", "guides/state-machines.html"},
		{"docs/reference/markers.md", "reference/markers.html"},
	} {
		t.Run(c.page, func(t *testing.T) {
			t.Parallel()
			text, err := os.ReadFile(filepath.Join("../..", c.page))
			if err != nil {
				t.Fatal(err)
			}
			root := repo(t)
			const states = "internal/cli/testdata/repo/docs/machines/states"
			for _, name := range []string{"machine-lamp.svg", "door-open.svg"} {
				svg, err := os.ReadFile(filepath.Join("../..", states, name))
				if err != nil {
					t.Fatal(err)
				}
				writeRepoFile(t, root, states+"/"+name, string(svg))
			}
			writeRepoFile(t, root, "docs/nav.md", "- [Home](../README.md)\n- [Page]("+strings.TrimPrefix(c.page, "docs/")+")\n")
			writeRepoFile(t, root, c.page, string(text))
			got := buildPage(t, root, nil, c.out)
			for _, name := range []string{"machine-lamp.svg", "door-open.svg"} {
				src := generatedSrc(t, got, "docs/machines/states/"+name)
				if want := "../" + states + "/" + name; src != want {
					t.Errorf("src %s, want %s", src, want)
				}
				exists(t, filepath.Join(root, "artifacts/docs-site", filepath.Dir(c.out), src))
				contains(t, got, `<img src="`+src+`"`)
			}
		})
	}
}

func TestGeneratedFilesAreImagesOfTheLastVersionUnderTheRoot(t *testing.T) {
	t.Parallel()
	needPandoc(t)
	body := "#### Before\n\n```\n![Old](states/old.svg)\n```\n\n#### After\n\n````\n# Sample\n\n" +
		"![A](states/a.svg) ![A again](./states/a.svg) ![Out](../../../out.svg) ![Web](https://example.com/w.svg) ![Up](../b.svg)\n````\n"
	root := editorRepo(t, "# Page\n\n<!-- docgen:docgen root=sample page=docs/m/a.md -->\n"+body+"<!-- /docgen:docgen -->\n")
	writeRepoFile(t, root, "sample/docs/m/states/old.svg", "<svg xmlns=\"http://www.w3.org/2000/svg\"/>\n")
	writeRepoFile(t, root, "sample/docs/b.svg", "<svg xmlns=\"http://www.w3.org/2000/svg\"/>\n")
	writeRepoFile(t, root, "out.svg", "<svg xmlns=\"http://www.w3.org/2000/svg\"/>\n")
	block := editorBlock(t, buildPage(t, root, nil, "page.html"))
	parts := strings.Split(block, `<div class="editor-generated"`)[1:]
	tags := make([]string, 0, len(parts))
	for _, part := range parts {
		tag, _, _ := strings.Cut(part, ">")
		tags = append(tags, tag)
	}
	if len(tags) != 2 {
		t.Errorf("%d generated files, want 2:\n%s", len(tags), block)
	}
	for path, want := range map[string]string{"docs/m/states/a.svg": "sample/docs/m/states/a.svg", "docs/b.svg": "sample/docs/b.svg"} {
		if src := generatedSrc(t, block, path); src != want {
			t.Errorf("src of %s is %s, want %s", path, src, want)
		}
		exists(t, filepath.Join(root, "artifacts/docs-site", want))
	}
	for _, bad := range []string{"old.svg", "out.svg", "example.com"} {
		if tag := strings.Join(tags, "\n"); strings.Contains(tag, bad) {
			t.Errorf("a generated file holds %s:\n%s", bad, tag)
		}
	}
}
