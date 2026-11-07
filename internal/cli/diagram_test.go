package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func statesPage(args string) string {
	return "<!-- docgen:states pkg=p type=M " + args + " -->\n<!-- /docgen:states -->\n"
}

func TestDiagramPathMustBeSVG(t *testing.T) {
	t.Parallel()
	const other = "Hand-written page.\n"
	files := machineRepo(statesPage("diagram=other.md"))
	files["go.mod"] = pageModule
	files["docs/other.md"] = other
	root := writeRepo(t, files)
	rc, log := docgen(t, root, "update")
	if rc != 1 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
	wantIn(t, log, "diagram=other.md does not end in .svg")
	if got := readPage(t, root, "docs/other.md"); got != other {
		t.Errorf("the page changed:\n%s", got)
	}
}

func TestDiagramOfItsOwnPage(t *testing.T) {
	t.Parallel()
	text := statesPage("diagram=p.md")
	files := machineRepo(text)
	files["go.mod"] = pageModule
	root := writeRepo(t, files)
	if rc, log := docgen(t, root, "update"); rc != 1 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
	if got := readPage(t, root, "docs/p.md"); got != text {
		t.Errorf("the page changed:\n%s", got)
	}
}

func TestDiagramIsAScannedPage(t *testing.T) {
	t.Parallel()
	const other = "Hand-written page.\n"
	files := machineRepo(statesPage("diagram=x.svg"))
	files["go.mod"] = pageModule
	files["docs/other.md"] = other
	root := writeRepo(t, files)
	link(t, "other.md", filepath.Join(root, "docs/x.svg"))
	for _, cmd := range []string{"check", "update"} {
		rc, log := docgen(t, root, cmd)
		if rc != 1 {
			t.Fatalf("%s: rc %d, log:\n%s", cmd, rc, log)
		}
		wantIn(t, log, "docs/x.svg is the page docs/other.md")
	}
	if got := readPage(t, root, "docs/other.md"); got != other {
		t.Errorf("the page changed:\n%s", got)
	}
}

func TestTwoMachinesOnOneDiagramThroughASymlink(t *testing.T) {
	t.Parallel()
	const second = "package p\n\n// N is a machine.\ntype N struct{ s S }\n\n// State returns the state.\nfunc (m *N) State() S { return m.s }\n\n// Go moves the machine.\nfunc (m *N) Go() { m.s = On }\n"
	const secondTest = "package p\n\nimport \"testing\"\n\nfunc TestN(t *testing.T) { (&N{}).Go() }\n"
	files := machineRepo(statesPage("diagram=real/a.svg") + "\n<!-- docgen:states pkg=p type=N diagram=link/a.svg -->\n<!-- /docgen:states -->\n")
	files["go.mod"] = pageModule
	files["p/n.go"] = second
	files["p/n_test.go"] = secondTest
	files["docs/real/keep"] = ""
	root := writeRepo(t, files)
	link(t, "real", filepath.Join(root, "docs/link"))
	rc, log := docgen(t, root, "update")
	if rc != 1 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
	wantIn(t, log, "write the same file")
	if _, err := os.Stat(filepath.Join(root, "docs/real/a.svg")); err == nil {
		t.Error("update wrote the diagram")
	}
}

func TestDiagramPathWithSpaces(t *testing.T) {
	t.Parallel()
	files := machineRepo(statesPage(`diagram="sp ace/a b.svg"`))
	files["go.mod"] = pageModule
	root := writeRepo(t, files)
	if rc, log := docgen(t, root, "update"); rc != 0 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
	wantIn(t, readPage(t, root, "docs/p.md"), "](sp%20ace/a%20b.svg)")
	if _, err := os.Stat(filepath.Join(root, "docs/sp ace/a b.svg")); err != nil {
		t.Error(err)
	}
	if rc, log := docgen(t, root, "check"); rc != 0 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
}
