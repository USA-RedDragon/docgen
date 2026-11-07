package cli

import (
	"path/filepath"
	"testing"
)

const sampleConst = "<!-- docgen:const pkg=. name=V --><!-- /docgen:const -->\n"

func sampleCode() map[string]string {
	return map[string]string{
		"go.mod":    "module example.com/s\n\ngo 1.24.0\n",
		"s.go":      "package s\n\n// V is a value.\nconst V = 7\n",
		"docs/a.md": sampleConst,
	}
}

func TestSampleGoLinkOutOfTheRepository(t *testing.T) {
	t.Parallel()
	files := sampleCode()
	root := outerRepo(t, nestedMarker("root=testdata/sample page=docs/a.md"), files)
	outside := t.TempDir()
	writeTree(t, outside, map[string]string{"x.go": "package s\n"})
	link(t, filepath.Join(outside, "x.go"), filepath.Join(root, "testdata/sample/x.go"))
	rc, log := docgen(t, root, "update")
	if rc != 1 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
	wantIn(t, log, "x.go is a symbolic link outside root=testdata/sample")
}

func TestSampleWithNoGoMod(t *testing.T) {
	t.Parallel()
	root := outerRepo(t, nestedMarker("root=testdata/sample page=docs/a.md"), map[string]string{
		"s.go":      "package s\n\n// V is a value.\nconst V = 7\n",
		"docs/a.md": sampleConst,
	})
	rc, log := docgen(t, root, "update")
	if rc != 1 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
	wantIn(t, log, "module= of the docgen:docgen marker")
	wantNotIn(t, log, "-module")
}

func TestSampleConfigOfIgnoredSettings(t *testing.T) {
	t.Parallel()
	for _, text := range []string{"depth: 99\n", "jobs: -1\n", "jobs: 100000\n", "tags: \"a!b\"\n", "depth: many\n", "module: nowhere\n"} {
		files := sampleCode()
		files[".docgen.yaml"] = text
		root := outerRepo(t, nestedMarker("root=testdata/sample page=docs/a.md"), files)
		if rc, log := docgen(t, root, "update"); rc != 0 {
			t.Errorf("%q: rc %d, log:\n%s", text, rc, log)
		}
	}
}

func TestSampleConfigOfReadSettings(t *testing.T) {
	t.Parallel()
	for text, want := range map[string]string{
		"print: nope\n": "print",
		"nokey: 1\n":    "nokey",
	} {
		files := sampleCode()
		files[".docgen.yaml"] = text
		root := outerRepo(t, nestedMarker("root=testdata/sample page=docs/a.md"), files)
		rc, log := docgen(t, root, "update")
		if rc != 1 {
			t.Fatalf("%q: rc %d, log:\n%s", text, rc, log)
		}
		wantIn(t, log, want)
	}
}

//nolint:paralleltest // sets the environment
func TestSampleWithGoworkOutside(t *testing.T) {
	work := filepath.Join(t.TempDir(), "go.work")
	writeTree(t, filepath.Dir(work), map[string]string{"go.work": "go 1.24.0\n"})
	t.Setenv("GOWORK", work)
	root := outerRepo(t, nestedMarker("root=testdata/sample page=docs/a.md"), sampleCode())
	if rc, log := docgen(t, root, "update"); rc != 0 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
	wantIn(t, readPage(t, root, "docs/p.md"), "`7`")
}

//nolint:paralleltest // sets the environment
func TestSampleWithGoworkInside(t *testing.T) {
	files := sampleCode()
	files["go.work"] = "go 1.24.0\n\nuse .\n"
	root := outerRepo(t, nestedMarker("root=testdata/sample page=docs/a.md"), files)
	t.Setenv("GOWORK", filepath.Join(root, "testdata/sample/go.work"))
	if rc, log := docgen(t, root, "update"); rc != 0 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
	wantIn(t, readPage(t, root, "docs/p.md"), "`7`")
}
