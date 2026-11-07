package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	pageModule = "module example.com/p\n\ngo 1.24.0\n"
	pageCode   = "// Package p is a package.\npackage p\n\n// N is a number.\nconst N = 1\n"
	goodPair   = "<!-- docgen:const pkg=. name=N --><!-- /docgen:const -->"
	badPair    = "<!-- docgen:const pkg=. name=Nope --><!-- /docgen:const -->"
)

func pageRepo(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	writeTree(t, root, map[string]string{"go.mod": pageModule, "p.go": pageCode})
	writeTree(t, root, files)
	return root
}

func readPage(t *testing.T, root, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func updateTwice(t *testing.T, root, name string) string {
	t.Helper()
	if rc, log := docgen(t, root, "update"); rc != 0 {
		t.Fatalf("first update: rc %d, log:\n%s", rc, log)
	}
	first := readPage(t, root, name)
	if rc, log := docgen(t, root, "check"); rc != 0 {
		t.Fatalf("check after update: rc %d, log:\n%s\npage:\n%s", rc, log, first)
	}
	if rc, log := docgen(t, root, "update"); rc != 0 {
		t.Fatalf("second update: rc %d, log:\n%s", rc, log)
	}
	if second := readPage(t, root, name); second != first {
		t.Fatalf("second update changed the page:\n%s\nwas:\n%s", second, first)
	}
	return first
}

func TestMalformedMarkerFailsCheck(t *testing.T) {
	t.Parallel()
	root := pageRepo(t, map[string]string{
		"docs/p.md": "Text.\n\n<!-- docgen:const pkg=. name=N extra --><!-- /docgen:const -->\n",
	})
	if rc, log := docgen(t, root, "check"); rc != 1 || !strings.Contains(log, "docs/p.md: line 3:") {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
}

func TestRegionMarksInPage(t *testing.T) {
	t.Parallel()
	root := pageRepo(t, map[string]string{
		"docs/a.md": "<!-- docgen:region x -->\nThe text.\n<!-- docgen:end -->\n",
		"docs/b.md": "<!-- docgen:code file=docs/a.md region=x -->\n<!-- /docgen:code -->\n",
	})
	if got := updateTwice(t, root, "docs/b.md"); !strings.Contains(got, "The text.") {
		t.Fatalf("page:\n%s", got)
	}
}

func TestMarkerTextInGeneratedContent(t *testing.T) {
	t.Parallel()
	root := pageRepo(t, map[string]string{
		"p.go": `// Package p shows the marker <!-- docgen:const pkg=. name=N --> and the close <!-- /docgen:doc -->.
//
// In code they stay as they are:
//
//	<!-- docgen:const pkg=. name=N --><!-- /docgen:const -->
//	<!-- /docgen:doc -->
package p

// N is a number.
const N = 1

// Config is a config.
type Config struct {
	// Close holds <!-- /docgen:config --> and <!-- /docgen:doc -->.
	Close string
	// Open holds <!-- docgen:const pkg=. name=N -->.
	Open string
}

// Text is text that holds a close tag.
const Text = "<!-- /docgen:const -->"
`,
		"docs/p.md": "<!-- docgen:doc pkg=. -->\n<!-- /docgen:doc -->\n\n" +
			"<!-- docgen:config pkg=. type=Config -->\n<!-- /docgen:config -->\n\n" +
			"<!-- docgen:const pkg=. name=Text --><!-- /docgen:const -->\n\n" + goodPair + "\n",
	})
	got := updateTwice(t, root, "docs/p.md")
	for _, want := range []string{
		"&lt;!-- docgen:const pkg=. name=N --> and the close &lt;!-- /docgen:doc -->",
		"\t<!-- /docgen:doc -->\n",
		"holds &lt;!-- /docgen:config --> and &lt;!-- /docgen:doc -->",
		"`1`",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("page has no %q:\n%s", want, got)
		}
	}
}

func TestEscapedMarkerTextFromAnOlderRun(t *testing.T) {
	t.Parallel()
	root := pageRepo(t, map[string]string{
		"p.go":      "// Package p holds \\<!-- /docgen:doc --> and <!-- docgen:const pkg=. name=N -->.\npackage p\n",
		"docs/p.md": "<!-- docgen:doc pkg=. -->\nOld \\<!-- /docgen:doc --> and \\<!-- docgen:const pkg=. name=N -->.\n<!-- /docgen:doc -->\n",
	})
	updateTwice(t, root, "docs/p.md")
}

func TestPageAfterInlineTripleBacktick(t *testing.T) {
	t.Parallel()
	root := pageRepo(t, map[string]string{"docs/p.md": "```inline```\n\nN is " + goodPair + ".\n"})
	if got := updateTwice(t, root, "docs/p.md"); !strings.Contains(got, "`1`") {
		t.Fatalf("marker skipped:\n%s", got)
	}
}

func TestCRLFPage(t *testing.T) {
	t.Parallel()
	page := strings.ReplaceAll("# Title\n\nN is "+goodPair+".\n\n"+
		"<!-- docgen:doc pkg=. -->\n<!-- /docgen:doc -->\n\n"+
		"<!-- docgen:code file=go.mod -->\n<!-- /docgen:code -->\n\nEnd.\n", "\n", "\r\n")
	root := pageRepo(t, map[string]string{"docs/p.md": page})
	got := updateTwice(t, root, "docs/p.md")
	if strings.Count(got, "\n") != strings.Count(got, "\r\n") {
		t.Errorf("page has bare line feeds:\n%q", got)
	}
	for _, want := range []string{"`1`", "Package p is a package.\r\n<!-- /docgen:doc -->", "go 1.24.0\r\n```\r\n"} {
		if !strings.Contains(got, want) {
			t.Errorf("page has no %q:\n%q", want, got)
		}
	}
}

func TestMixedLineEndingsKeepProse(t *testing.T) {
	t.Parallel()
	root := pageRepo(t, map[string]string{
		"docs/p.md": "One.\r\nTwo.\nThree.\r\n\r\n<!-- docgen:doc pkg=. -->\r\n<!-- /docgen:doc -->\r\nFour.\nFive.\r\n",
	})
	got := updateTwice(t, root, "docs/p.md")
	if !strings.HasPrefix(got, "One.\r\nTwo.\nThree.\r\n\r\n") || !strings.HasSuffix(got, "<!-- /docgen:doc -->\r\nFour.\nFive.\r\n") {
		t.Fatalf("prose changed:\n%q", got)
	}
}

func TestCRLFSourceInLFPage(t *testing.T) {
	t.Parallel()
	root := pageRepo(t, map[string]string{
		"conf.txt":  "a\r\nb\r\n",
		"docs/p.md": "<!-- docgen:code file=conf.txt -->\n<!-- /docgen:code -->\n",
	})
	if got := updateTwice(t, root, "docs/p.md"); strings.Contains(got, "\r") {
		t.Fatalf("page has a carriage return:\n%q", got)
	}
}

func TestSymlinksUnderDocs(t *testing.T) {
	t.Parallel()
	root := pageRepo(t, map[string]string{
		".docgen.yaml": "docs: docs\n",
		"README.md":    badPair + "\n",
		"docs/p.md":    goodPair + "\n",
		"other/q.md":   badPair + "\n",
	})
	for link, target := range map[string]string{
		"docs/index.md":    "../README.md",
		"docs/dangling.md": "nothing.md",
		"docs/inside.md":   "p.md",
		"docs/outside":     "../other",
	} {
		if err := os.Symlink(target, filepath.Join(root, filepath.FromSlash(link))); err != nil {
			t.Fatal(err)
		}
	}
	if got := updateTwice(t, root, "docs/p.md"); !strings.Contains(got, "`1`") {
		t.Fatalf("page:\n%s", got)
	}
	if got := readPage(t, root, "README.md"); got != badPair+"\n" {
		t.Fatalf("the target of a link changed:\n%s", got)
	}
}

func TestCheckListsStaleAndErrors(t *testing.T) {
	t.Parallel()
	root := pageRepo(t, map[string]string{
		"docs/a.md": goodPair + "\n\n" + badPair + "\n",
		"docs/b.md": goodPair + "\n",
		"docs/c.md": "<!-- docgen:Const pkg=. name=N -->\n",
	})
	rc, log := docgen(t, root, "check")
	if rc != 1 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
	for _, want := range []string{
		"stale: docs/a.md:1: docgen:const pkg=. name=N",
		"stale: docs/b.md:1: docgen:const pkg=. name=N",
		"docs/a.md:3: docgen:const pkg=. name=Nope",
		"docs/c.md: line 1:",
		"2 stale",
	} {
		if !strings.Contains(log, want) {
			t.Errorf("log has no %q:\n%s", want, log)
		}
	}
}

func TestUpdateWritesNothingOnFailure(t *testing.T) {
	t.Parallel()
	machine := `package p

// S is a state.
type S int

const (
	// A is a state.
	A S = iota
	// B is a state.
	B
)

// M is a machine.
type M struct{ s S }

// State returns the state.
func (m *M) State() S { return m.s }

// Go moves the machine.
func (m *M) Go() { m.s = B }
`
	test := "package p\n\nimport \"testing\"\n\nfunc TestGo(t *testing.T) {\n\tvar m M\n\tm.Go()\n}\n"
	empty := "<!-- docgen:config pkg=. type=Nope -->\n<!-- /docgen:config -->\n"
	sameLine := "<!-- docgen:config pkg=. type=Nope --><!-- /docgen:config -->\n"
	kept := "<!-- docgen:config pkg=. type=Nope -->\n\nhand text\n\n\n<!-- /docgen:config -->\n"
	pages := map[string]string{
		"docs/a.md": goodPair + "\n\n" + "<!-- docgen:states pkg=. type=M -->\n<!-- /docgen:states -->\n",
		"docs/b.md": goodPair + "\n\n" + empty + "\n" + sameLine + "\n" + kept,
		"docs/c.md": goodPair + "\n",
	}
	files := map[string]string{"m.go": machine, "m_test.go": test}
	for k, v := range pages {
		files[k] = v
	}
	root := pageRepo(t, files)
	if rc, log := docgen(t, root, "update"); rc != 1 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
	for name, want := range pages {
		if got := readPage(t, root, name); got != want {
			t.Errorf("%s changed:\n%s", name, got)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "docs/states")); !os.IsNotExist(err) {
		t.Errorf("a failed update wrote a generated file: %v", err)
	}
}

func TestUpdateWritesNothingOnParseError(t *testing.T) {
	t.Parallel()
	pages := map[string]string{
		"docs/a.md": goodPair + "\n",
		"docs/b.md": "<!-- /docgen:const -->\n",
	}
	root := pageRepo(t, pages)
	if rc, log := docgen(t, root, "update"); rc != 1 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
	if got := readPage(t, root, "docs/a.md"); got != pages["docs/a.md"] {
		t.Errorf("a.md changed:\n%s", got)
	}
}

func TestClosePrefixKept(t *testing.T) {
	t.Parallel()
	page := "- item\n\n  <!-- docgen:doc pkg=. -->\n  <!-- /docgen:doc -->\n\n" +
		"> quote\n>\n> <!-- docgen:code file=go.mod -->\n> <!-- /docgen:code -->\n> more\n"
	root := pageRepo(t, map[string]string{"docs/p.md": page})
	got := updateTwice(t, root, "docs/p.md")
	for _, want := range []string{
		"\nPackage p is a package.\n  <!-- /docgen:doc -->\n",
		"go 1.24.0\n```\n> <!-- /docgen:code -->\n> more\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("page has no %q:\n%s", want, got)
		}
	}
}
