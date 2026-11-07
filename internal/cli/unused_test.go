package cli

import (
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const (
	unusedModule = "module example.com/u\n\ngo 1.24.0\n"
	regionText   = "# docgen:region r\ntext\n# docgen:end\n"
)

func unusedRepo(t *testing.T, files map[string]string) string {
	t.Helper()
	all := map[string]string{"go.mod": unusedModule, "docs/p.md": "Text.\n"}
	maps.Copy(all, files)
	return writeRepo(t, all)
}

func regionMarker(file string) string {
	return "<!-- docgen:code file=" + file + " region=r -->\n```sh\ntext\n```\n<!-- /docgen:code -->\n"
}

func TestUnusedRegion(t *testing.T) {
	t.Parallel()
	root := unusedRepo(t, map[string]string{"src/a.sh": "x\n" + regionText})
	rc, log := docgen(t, root, "check")
	if rc != 1 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
	wantIn(t, log, "unused: src/a.sh:2: docgen:region r\n", "docgen: 1 unused; ")
	wantNotIn(t, log, "stale:")
}

func TestUsedRegionIsClean(t *testing.T) {
	t.Parallel()
	root := unusedRepo(t, map[string]string{"src/a.sh": regionText, "docs/p.md": regionMarker("src/a.sh")})
	if rc, log := docgen(t, root, "check"); rc != 0 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
}

func TestUpdateHasNoAuditLines(t *testing.T) {
	t.Parallel()
	root := unusedRepo(t, map[string]string{
		"src/a.sh":            regionText,
		"src/b.sh":            "# docgen:end\n# docgen:region open\n",
		"docs/states/old.svg": "<svg/>\n",
		"docs/p.md":           "[a](nope.md)\n",
	})
	rc, log := docgen(t, root, "update")
	if rc != 0 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
	wantNotIn(t, log, "unused", "broken", "docgen:region", "docgen:end")
}

func TestUnusedLinesAreSorted(t *testing.T) {
	t.Parallel()
	root := unusedRepo(t, map[string]string{
		"z.sh":          regionText,
		"a/b.sh":        "# docgen:region two\nx\n# docgen:end\n# docgen:region one\ny\n# docgen:end\n",
		"tool/go.mod":   "module example.com/tool\n\ngo 1.24.0\n",
		"tool/t.sh":     regionText,
		"docs/m.md":     "<!-- docgen:region page -->\nText.\n<!-- docgen:end -->\n",
		"docs/states/x": "not a diagram",
	})
	rc, log := docgen(t, root, "check")
	if rc != 1 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
	var got []string
	for l := range strings.Lines(log) {
		if strings.HasPrefix(l, "unused: ") {
			got = append(got, strings.TrimSpace(l))
		}
	}
	want := []string{
		"unused: a/b.sh:1: docgen:region two",
		"unused: a/b.sh:4: docgen:region one",
		"unused: docs/m.md:1: docgen:region page",
		"unused: tool/t.sh:1: docgen:region r",
		"unused: z.sh:1: docgen:region r",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("got:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestTextThatIsNotARegionMark(t *testing.T) {
	t.Parallel()
	for name, files := range map[string]map[string]string{
		"fence":        {"docs/a.md": "```sh\n# docgen:region r\nx\n# docgen:end\n```\n"},
		"tilde fence":  {"docs/a.md": "~~~\ndocgen:end\n~~~\n"},
		"code span":    {"docs/a.md": "`docgen:region r`\n\n`docgen:end`\n"},
		"marker body":  {"docs/a.md": "<!-- docgen:code file=x.txt -->\n```\nx\n```\n<!-- /docgen:code -->\n", "x.txt": "x\n"},
		"body as text": {"docs/a.md": "<!-- docgen:doc pkg=. -->\n<!-- docgen:region r -->\nThe doc.\n<!-- docgen:end -->\n<!-- /docgen:doc -->\n", "u.go": "// Package u has text.\npackage u\n"},
		"after code":   {"a.go": "package a\n\nconst A = 1 // docgen:region r\n\nconst B = 2 // docgen:end\n"},
		"in a string":  {"a.go": "package a\n\nconst A = \"docgen:region r\"\n"},
		"in prose":     {"a.sh": "# the docgen:end mark closes a docgen:region\n"},
		"two words":    {"a.sh": "# docgen:region a b\n"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			rc, log := docgen(t, unusedRepo(t, files), "check")
			wantNotIn(t, log, "unused:", "has no docgen:", "is also at line")
			if name != "body as text" && rc != 0 {
				t.Errorf("rc %d, log:\n%s", rc, log)
			}
		})
	}
}

func TestPlacesOutOfTheScan(t *testing.T) {
	t.Parallel()
	big := "# docgen:region big\nx\n# docgen:end\n" + strings.Repeat("x", 1<<20)
	root := unusedRepo(t, map[string]string{
		"a/testdata/r.sh":   regionText,
		"vendor/r.sh":       regionText,
		"node_modules/r.sh": regionText,
		".hidden/r.sh":      regionText,
		"_private/r.sh":     regionText,
		"binary.dat":        "\x00\n" + regionText,
		"big.txt":           big,
	})
	if rc, log := docgen(t, root, "check"); rc != 0 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
}

func TestFileAtTheSizeLimitIsScanned(t *testing.T) {
	t.Parallel()
	text := regionText + strings.Repeat("x", 1<<20-len(regionText))
	root := unusedRepo(t, map[string]string{"limit.txt": text})
	rc, log := docgen(t, root, "check")
	if rc != 1 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
	wantIn(t, log, "unused: limit.txt:1: docgen:region r\n")
}

func TestRegionExcerptThroughALink(t *testing.T) {
	t.Parallel()
	root := unusedRepo(t, map[string]string{"src/a.sh": regionText, "docs/p.md": regionMarker("alias.sh")})
	link(t, "src/a.sh", filepath.Join(root, "alias.sh"))
	if rc, log := docgen(t, root, "check"); rc != 0 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
}

func TestWhatDoesNotUseARegion(t *testing.T) {
	t.Parallel()
	for name, page := range map[string]string{
		"another file": regionMarker("b.sh"),
		"whole file":   "<!-- docgen:code file=a.sh -->\n```\ntext\n```\n<!-- /docgen:code -->\n",
		"marker text":  "~~~\n" + regionMarker("a.sh") + "~~~\n",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := unusedRepo(t, map[string]string{"a.sh": regionText, "b.sh": regionText, "docs/p.md": page})
			rc, log := docgen(t, root, "check")
			if rc != 1 {
				t.Fatalf("rc %d, log:\n%s", rc, log)
			}
			wantIn(t, log, "unused: a.sh:1: docgen:region r\n")
		})
	}
}

func TestNestedRunUsesARegion(t *testing.T) {
	t.Parallel()
	root := unusedRepo(t, map[string]string{
		".docgen.yaml":     "docs: docs\n",
		"sample/go.mod":    unusedModule,
		"sample/r.sh":      regionText,
		"sample/docs/s.md": "<!-- docgen:code file=r.sh region=r -->\n<!-- /docgen:code -->\n",
		"docs/p.md":        "<!-- docgen:docgen root=sample page=docs/s.md -->\n<!-- /docgen:docgen -->\n",
	})
	if rc, log := docgen(t, root, "update"); rc != 0 {
		t.Fatalf("update: rc %d, log:\n%s", rc, log)
	}
	if rc, log := docgen(t, root, "check"); rc != 0 {
		t.Fatalf("check: rc %d, log:\n%s", rc, log)
	}
}

func TestRegionMarksOutOfPlace(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, text, want string }{
		{"no end", "x\n# docgen:region r\ny\n", "a.sh:2: docgen:region r has no docgen:end"},
		{"no region", "x\n\n# docgen:end\n", "a.sh:3: docgen:end has no docgen:region"},
		{"twice", "# docgen:region r\nx\n# docgen:end\n# docgen:region r\ny\n# docgen:end\n", "a.sh:4: docgen:region r is also at line 1"},
		{"nested", "# docgen:region a\n# docgen:region b\nx\n# docgen:end\n", "a.sh:1: docgen:region a has no docgen:end"},
		{"second end", "# docgen:region a\nx\n# docgen:end\n# docgen:end\n", "a.sh:4: docgen:end has no docgen:region"},
		{"line endings", "x\r\n# docgen:end\r\n", "a.sh:2: docgen:end has no docgen:region"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			rc, log := docgen(t, unusedRepo(t, map[string]string{"a.sh": tc.text}), "check")
			if rc != 1 {
				t.Fatalf("rc %d, log:\n%s", rc, log)
			}
			wantIn(t, log, tc.want)
		})
	}
}

func TestRegionMarkErrorInAPage(t *testing.T) {
	t.Parallel()
	rc, log := docgen(t, unusedRepo(t, map[string]string{"notes/n.md": "Text.\n\n<!-- docgen:end -->\n"}), "check")
	if rc != 1 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
	wantIn(t, log, "notes/n.md:3: docgen:end has no docgen:region")
}

func TestNoUnusedLinesWithAPageThatDoesNotParse(t *testing.T) {
	t.Parallel()
	root := unusedRepo(t, map[string]string{
		"a.sh":                regionText,
		"b.sh":                "# docgen:end\n",
		"docs/p.md":           "<!-- docgen:nope -->\n",
		"docs/states/old.svg": "<svg/>\n",
	})
	rc, log := docgen(t, root, "check")
	if rc != 1 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
	wantIn(t, log, "docs/p.md: line 1: ", "b.sh:1: docgen:end has no docgen:region")
	wantNotIn(t, log, "unused:")
}

func TestCheckDiffHoldsNoUnusedLine(t *testing.T) {
	t.Parallel()
	root := unusedRepo(t, map[string]string{"a.sh": regionText})
	rc, log, stdout := docgenDiff(t, root, "check", "-diff")
	if rc != 1 || stdout != "" {
		t.Fatalf("rc %d, stdout %q, log:\n%s", rc, stdout, log)
	}
	wantIn(t, log, "unused: a.sh:1: docgen:region r\n")
}

func TestUnusedDiagram(t *testing.T) {
	t.Parallel()
	for name, page := range map[string]string{"marker": "<!-- docgen:gomod field=go --><!-- /docgen:gomod -->\n", "no marker": "Text.\n"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := unusedRepo(t, map[string]string{
				"docs/g/p.md":              page,
				"docs/g/states/old.svg":    "<svg/>\n",
				"docs/g/states/notes.txt":  "text\n",
				"docs/g/states/sub/in.svg": "<svg/>\n",
				"docs/states":              "a file\n",
				"other/states/away.svg":    "<svg/>\n",
			})
			rc, log := docgen(t, root, "check")
			if rc != 1 {
				t.Fatalf("rc %d, log:\n%s", rc, log)
			}
			wantIn(t, log, "unused: docs/g/states/old.svg: docgen:states diagram\n")
			wantNotIn(t, log, "notes.txt", "in.svg", "away.svg")
		})
	}
}

func TestDiagramOfAMarkerIsUsed(t *testing.T) {
	t.Parallel()
	files := machineRepo("<!-- docgen:states pkg=p type=M -->\n<!-- /docgen:states -->\n")
	files["go.mod"] = unusedModule
	files["docs/states/gone.svg"] = "<svg/>\n"
	root := writeRepo(t, files)
	if rc, log := docgen(t, root, "update"); rc != 0 {
		t.Fatalf("update: rc %d, log:\n%s", rc, log)
	}
	rc, log := docgen(t, root, "check")
	if rc != 1 {
		t.Fatalf("check: rc %d, log:\n%s", rc, log)
	}
	wantIn(t, log, "unused: docs/states/gone.svg: docgen:states diagram\n")
	wantNotIn(t, log, "unused: docs/states/p-m.svg")
	if err := os.Remove(filepath.Join(root, "docs/states/gone.svg")); err != nil {
		t.Fatal(err)
	}
	if rc, log := docgen(t, root, "check"); rc != 0 {
		t.Fatalf("clean check: rc %d, log:\n%s", rc, log)
	}
}

func TestNoUnusedDiagramAfterAMarkerError(t *testing.T) {
	t.Parallel()
	files := machineRepo("<!-- docgen:states pkg=p type=Nope -->\n<!-- /docgen:states -->\n")
	files["go.mod"] = unusedModule
	files["docs/states/p-m.svg"] = "<svg/>\n"
	rc, log := docgen(t, writeRepo(t, files), "check")
	if rc != 1 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
	wantNotIn(t, log, "unused:")
}

func TestUnusedRegionInIgnoredPath(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		".gitignore":   "built/\n",
		"built/out.sh": "# docgen:region copy\nx\n# docgen:end\n",
		"src.sh":       "# docgen:region mine\nx\n# docgen:end\n",
		"docs/p.md":    "text\n",
	})
	if out, err := exec.CommandContext(t.Context(), "git", "-C", root, "init", "-q").CombinedOutput(); err != nil {
		t.Skipf("git init: %v\n%s", err, out)
	}
	rc, log := docgen(t, root, "check")
	if rc != 1 || !strings.Contains(log, "src.sh:1: docgen:region mine") || strings.Contains(log, "built/out.sh") {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
}

func TestRegionOfASample(t *testing.T) {
	t.Parallel()
	const code = "<!-- docgen:code file=src/x.sh region=r -->\n<!-- /docgen:code -->\n"
	root := unusedRepo(t, map[string]string{
		".docgen.yaml":        "docs: docs\n",
		"docs/p.md":           nestedMarker("root=sample page=docs/page.md"),
		"sample/src/x.sh":     regionText,
		"sample/src/y.sh":     "# docgen:region other\nx\n# docgen:end\n",
		"sample/docs/page.md": code,
	})
	if rc, log := docgen(t, root, "update"); rc != 0 {
		t.Fatalf("update: rc %d, log:\n%s", rc, log)
	}
	rc, log := docgen(t, root, "check")
	if rc != 1 {
		t.Fatalf("check: rc %d, log:\n%s", rc, log)
	}
	wantIn(t, log, "unused: sample/src/y.sh:1: docgen:region other")
	wantNotIn(t, log, "sample/src/x.sh")
}

func TestRegionWithNoEndIsListedOnce(t *testing.T) {
	t.Parallel()
	root := unusedRepo(t, map[string]string{"src/a.sh": "# docgen:region open\nx\n"})
	rc, log := docgen(t, root, "check")
	if rc != 1 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
	wantIn(t, log, "src/a.sh:1: docgen:region open has no docgen:end")
	wantNotIn(t, log, "unused:")
}
