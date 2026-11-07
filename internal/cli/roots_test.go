package cli

import (
	"os"
	"path/filepath"
	"testing"
)

const rootBadMarker = "<!-- docgen:Const pkg=. -->\n"

func TestRootPagesStayOutOfTheOuterRun(t *testing.T) {
	t.Parallel()
	sample := map[string]string{
		"samples/a/.docgen.yaml":            "docs: docs\n",
		"samples/a/go.mod":                  sampleMod,
		"samples/a/docs/a.md":               sampleGoMod,
		"samples/a/notes/bad.md":            rootBadMarker,
		"samples/a/notes/stale.md":          sampleGoMod,
		"samples/a/notes/link.md":           "[x](missing.md)\n",
		"samples/a/notes/region.md":         "<!-- docgen:region r -->\nText.\n<!-- docgen:end -->\n",
		"samples/a/notes/unpaired.md":       "<!-- docgen:end -->\n",
		"samples/a/notes/states/unused.svg": "<svg/>\n",
		"docs/p.md":                         nestedMarker("root=samples/a page=docs/a.md"),
		"go.mod":                            unusedModule,
	}
	root := writeRepo(t, sample)
	if rc, log := docgen(t, root, "update"); rc != 0 {
		t.Fatalf("update: rc %d, log:\n%s", rc, log)
	}
	if rc, log := docgen(t, root, "check"); rc != 0 {
		t.Fatalf("check: rc %d, log:\n%s", rc, log)
	}
	if got := readPage(t, root, "samples/a/notes/stale.md"); got != sampleGoMod {
		t.Errorf("the outer run filled a page of the root:\n%s", got)
	}
	wantIn(t, readPage(t, root, "docs/p.md"), sampleFilled)
}

func TestRootPageErrorIsAnErrorOfTheNestedRun(t *testing.T) {
	t.Parallel()
	root := writeRepo(t, map[string]string{
		"go.mod":              unusedModule,
		"docs/p.md":           nestedMarker("root=samples/a page=a.md"),
		"samples/a/go.mod":    sampleMod,
		"samples/a/a.md":      sampleGoMod,
		"samples/a/bad/b.md":  rootBadMarker,
		"samples/a/bad/c.txt": "text\n",
	})
	rc, log := docgen(t, root, "update")
	if rc != 1 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
	wantIn(t, log, "docs/p.md:1: docgen:docgen root=samples/a page=a.md: bad/b.md: line 1: malformed marker comment")
	wantNotIn(t, log, "samples/a/bad/b.md")
}

func TestRootThatHoldsItsPageKeepsItsError(t *testing.T) {
	t.Parallel()
	for _, marker := range []string{"root=. page=docs/p.md", "root=docs page=p.md"} {
		root := writeRepo(t, map[string]string{"go.mod": unusedModule, "docs/p.md": nestedMarker(marker), "docs/q.md": rootBadMarker})
		rc, log := docgen(t, root, "update")
		if rc != 1 {
			t.Fatalf("%s: rc %d, log:\n%s", marker, rc, log)
		}
		wantIn(t, log, "docs/q.md: line 1: malformed marker comment")
	}
	root := writeRepo(t, map[string]string{"go.mod": unusedModule, "docs/p.md": nestedMarker("root=. page=docs/p.md")})
	rc, log := docgen(t, root, "update")
	if rc != 1 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
	wantIn(t, log, "docs/p.md:1: docgen:docgen root=. page=docs/p.md: docs/p.md:1: docgen:docgen root=. page=docs/p.md: the marker is at level 2, above the depth limit 1")
}

func TestRootThatHoldsAnotherRoot(t *testing.T) {
	t.Parallel()
	root := writeRepo(t, map[string]string{
		"go.mod": unusedModule,
		"docs/p.md": nestedMarker("root=samples page=a/a.md") +
			nestedMarker("root=samples/a page=a.md"),
		"samples/.docgen.yaml":   "docs: top\n",
		"samples/go.mod":         sampleMod,
		"samples/top/t.md":       "Text.\n",
		"samples/bad.md":         rootBadMarker,
		"samples/a/.docgen.yaml": "docs: in\n",
		"samples/a/go.mod":       sampleMod,
		"samples/a/a.md":         sampleGoMod,
		"samples/a/in/i.md":      "Text.\n",
		"samples/a/bad.md":       rootBadMarker,
	})
	if rc, log := docgen(t, root, "update"); rc != 0 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
	if rc, log := docgen(t, root, "check"); rc != 0 {
		t.Fatalf("check: rc %d, log:\n%s", rc, log)
	}
}

func TestRootOfALeftOutPageDoesNotCount(t *testing.T) {
	t.Parallel()
	root := writeRepo(t, map[string]string{
		"go.mod":              sampleMod,
		"deep/er/p.md":        nestedMarker("root=s page=z.md"),
		"other/q.md":          sampleGoMod,
		"s/.docgen.yaml":      "docs: sub\n",
		"s/go.mod":            sampleMod,
		"s/sub/x.md":          "Text.\n",
		"s/z.md":              "Text.\n",
		"s/y.md":              nestedMarker("root=other page=q.md"),
		"s/other/q.md":        "Text.\n",
		"s/deep/er/bad.md":    rootBadMarker,
		"s/deep/er/stale.txt": "text\n",
	})
	if rc, log := docgen(t, root, "update"); rc != 0 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
	wantIn(t, readPage(t, root, "other/q.md"), sampleFilled)
}

func TestRootsOfTwoPagesThatHoldEachOther(t *testing.T) {
	t.Parallel()
	root := writeRepo(t, map[string]string{
		"go.mod": unusedModule,
		"a/p.md": nestedMarker("root=b page=q.md"),
		"b/q.md": nestedMarker("root=a page=p.md"),
	})
	rc, log := docgen(t, root, "update")
	if rc != 1 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
	wantIn(t, log, "a/p.md:1: docgen:docgen root=b page=q.md:")
	wantNotIn(t, log, "b/q.md:1:")
}

func TestRootPageOfAMarkerStaysInTheNestedRun(t *testing.T) {
	t.Parallel()
	root := writeRepo(t, map[string]string{
		".docgen.yaml": "depth: 2\n",
		"go.mod":       unusedModule,
		"docs/p.md":    nestedMarker("root=s page=in/b.md"),
		"s/go.mod":     sampleMod,
		"s/a.md":       nestedMarker("root=in page=b.md"),
		"s/in/go.mod":  "module example.com/in\n\ngo 1.24.0\n",
		"s/in/b.md":    sampleGoMod,
	})
	if rc, log := docgen(t, root, "update"); rc != 0 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
	wantIn(t, readPage(t, root, "docs/p.md"), sampleFilled)
}

func TestOuterPageExcerptsAFileUnderARoot(t *testing.T) {
	t.Parallel()
	text := nestedMarker("root=s page=a.md") +
		"<!-- docgen:code file=s/run.sh region=r -->\n<!-- /docgen:code -->\n" +
		"<!-- docgen:code file=s/a.md -->\n<!-- /docgen:code -->\n" +
		"<!-- docgen:code file=s/notes.md region=n -->\n<!-- /docgen:code -->\n"
	root := writeRepo(t, map[string]string{
		"go.mod":     unusedModule,
		"docs/p.md":  text,
		"s/go.mod":   sampleMod,
		"s/a.md":     sampleGoMod,
		"s/notes.md": "<!-- docgen:region n -->\nNote.\n<!-- docgen:end -->\n",
		"s/run.sh":   regionText,
	})
	if rc, log := docgen(t, root, "update"); rc != 0 {
		t.Fatalf("update: rc %d, log:\n%s", rc, log)
	}
	if rc, log := docgen(t, root, "check"); rc != 0 {
		t.Fatalf("check: rc %d, log:\n%s", rc, log)
	}
	wantIn(t, readPage(t, root, "docs/p.md"), "\ntext\n", "\nNote.\n", "<!-- docgen:gomod field=module --><!-- /docgen:gomod -->")
}

func TestRegionsOfARoot(t *testing.T) {
	t.Parallel()
	root := unusedRepo(t, map[string]string{
		"docs/p.md":           nestedMarker("root=sample page=docs/page.md"),
		"sample/go.mod":       sampleMod,
		"sample/docs/page.md": regionMarker("src/used.sh"),
		"sample/src/used.sh":  regionText,
		"sample/src/idle.sh":  regionText,
	})
	rc, log := docgen(t, root, "update")
	if rc != 0 {
		t.Fatalf("update: rc %d, log:\n%s", rc, log)
	}
	rc, log = docgen(t, root, "check")
	if rc != 1 {
		t.Fatalf("check: rc %d, log:\n%s", rc, log)
	}
	wantIn(t, log, "unused: sample/src/idle.sh:1: docgen:region r\n")
	wantNotIn(t, log, "used.sh")
}

func TestRootThroughASymbolicLink(t *testing.T) {
	t.Parallel()
	root := writeRepo(t, map[string]string{
		"go.mod":            unusedModule,
		"docs/p.md":         nestedMarker("root=alias page=a.md"),
		"real/go.mod":       sampleMod,
		"real/a.md":         sampleGoMod,
		"real/.docgen.yaml": "docs: in\n",
		"real/in/i.md":      "Text.\n",
		"real/x/bad.md":     rootBadMarker,
	})
	if err := os.Symlink("real", filepath.Join(root, "alias")); err != nil {
		t.Fatal(err)
	}
	if rc, log := docgen(t, root, "update"); rc != 0 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
}
