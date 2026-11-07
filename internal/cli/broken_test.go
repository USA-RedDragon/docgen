package cli

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func brokenLines(log string) []string {
	var out []string
	for l := range strings.Lines(log) {
		if s, ok := strings.CutPrefix(strings.TrimSpace(l), "broken: "); ok {
			out = append(out, s)
		}
	}
	return out
}

func TestLinks(t *testing.T) {
	t.Parallel()
	files := map[string]string{
		"go.mod":            unusedModule,
		"docs/other.md":     "# Other Page\n\n## Same\n\n## Same\n\n<a name=\"named\"></a>\n\n<div id=\"block\"></div>\n",
		"docs/my file.md":   "# Spaced Title\n",
		"docs/a#b.md":       "Text.\n",
		"docs/img/logo.png": "x",
		"docs/sub/inner.md": "Text.\n",
		"src/code.go":       "package src\n",
		"docs/outer.txt":    "x",
	}
	for _, tc := range []struct {
		name string
		page string
		want []string
	}{
		{name: "file", page: "[a](other.md)\n"},
		{name: "missing file", page: "\n[a](nope.md)\n", want: []string{"docs/p.md:2: nope.md"}},
		{name: "image", page: "![a](img/logo.png)\n"},
		{name: "missing image", page: "![a](img/nope.png)\n", want: []string{"docs/p.md:1: img/nope.png"}},
		{name: "directory", page: "[a](sub)\n\n[b](sub/)\n\n[c](./)\n"},
		{name: "missing directory", page: "[a](nope/)\n", want: []string{"docs/p.md:1: nope/"}},
		{name: "file with a slash", page: "[a](other.md/)\n", want: []string{"docs/p.md:1: other.md/"}},
		{name: "parent", page: "[a](../src/code.go)\n"},
		{name: "file as a directory", page: "[a](other.md/x)\n", want: []string{"docs/p.md:1: other.md/x"}},
		{name: "root path", page: "[a](/src/code.go)\n"},
		{name: "missing root path", page: "[a](/nope.go)\n", want: []string{"docs/p.md:1: /nope.go"}},
		{name: "title", page: "[a](other.md \"Title\")\n\n[b](nope.md \"Title\")\n", want: []string{"docs/p.md:3: nope.md"}},
		{name: "escaped bracket", page: "\\[a](nope.md)\n\n[b\\]c](other.md)\n"},
		{name: "image in a table cell", page: "| a | b |\n|---|---|\n| ![c](img/nope.png) | ![d](img/logo.png) |\n", want: []string{"docs/p.md:3: img/nope.png"}},
		{name: "encoded path", page: "[a](my%20file.md)\n\n[b](a%23b.md)\n"},
		{name: "encoded path and anchor", page: "[a](my%20file.md#spaced-title)\n"},
		{name: "missing encoded path", page: "[a](no%20file.md)\n", want: []string{"docs/p.md:1: no%20file.md"}},
		{name: "angle path", page: "[a](<my file.md>)\n"},
		{name: "fence", page: "```md\n[a](nope.md)\n```\n"},
		{name: "code span", page: "`[a](nope.md)`\n"},
		{name: "indented code", page: "Text.\n\n    [a](nope.md)\n"},
		{name: "comment", page: "<!-- [a](nope.md) -->\n"},
		{name: "marker body", page: "<!-- docgen:code file=src/code.go -->\n```go\npackage src\n```\n\n[a](nope.md)\n<!-- /docgen:code -->\n"},
		{name: "definition", page: "[a][b]\n\n[b]: other.md\n"},
		{name: "broken definition", page: "[a][b]\n\n[b]: nope.md\n", want: []string{"docs/p.md:3: nope.md"}},
		{name: "broken shortcut", page: "[b]\n\n[b]: nope.md \"T\"\n", want: []string{"docs/p.md:3: nope.md"}},
		{name: "undefined label", page: "[a][nope]\n", want: []string{"docs/p.md:1: [nope]"}},
		{name: "empty target", page: "[a]()\n", want: []string{"docs/p.md:1: ()"}},
		{name: "leaves the root", page: "[a](../../../outside.md)\n", want: []string{"docs/p.md:1: ../../../outside.md"}},
		{name: "leaves the root to a file", page: "[a](../../go.mod)\n", want: []string{"docs/p.md:1: ../../go.mod"}},
		{name: "external", page: "[a](https://example.invalid/nope)\n\n[b](mailto:a@example.invalid)\n\n[c](//example.invalid/x)\n\n[d]: http://example.invalid\n"},
		{name: "anchor", page: "# Top Heading\n\n[a](#top-heading)\n"},
		{name: "missing anchor", page: "# Top\n\n[a](#nope)\n", want: []string{"docs/p.md:3: #nope"}},
		{name: "case of an anchor", page: "# Top\n\n[a](#Top)\n", want: []string{"docs/p.md:3: #Top"}},
		{name: "anchor of another page", page: "[a](other.md#other-page)\n"},
		{name: "missing anchor of another page", page: "[a](other.md#nope)\n", want: []string{"docs/p.md:1: other.md#nope"}},
		{name: "duplicate headings", page: "[a](other.md#same)\n\n[b](other.md#same-1)\n\n[c](other.md#same-2)\n", want: []string{"docs/p.md:5: other.md#same-2"}},
		{name: "id", page: "[a](other.md#block)\n"},
		{name: "a name", page: "[a](other.md#named)\n"},
		{name: "own id", page: "<span id=\"here\"></span>\n\n[a](#here)\n"},
		{name: "heading in a fence", page: "```\n# Not\n```\n\n[a](#not)\n", want: []string{"docs/p.md:5: #not"}},
		{name: "anchor of a file", page: "[a](../src/code.go#L1)\n"},
		{name: "anchor of a page out of the scan", page: "[a](sub/inner.md#nope)\n", want: []string{"docs/p.md:1: sub/inner.md#nope"}},
		{name: "query", page: "[a](other.md?plain=1#same)\n"},
		{name: "sorted", page: "[a](z.md)\n\n[b](a.md)\n\n[c]: m.md\n", want: []string{"docs/p.md:1: z.md", "docs/p.md:3: a.md", "docs/p.md:5: m.md"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root := writeRepo(t, files)
			writeTree(t, root, map[string]string{"docs/p.md": tc.page})
			rc, log := docgen(t, root, "check")
			if got := brokenLines(log); !slices.Equal(got, tc.want) {
				t.Errorf("broken lines %q, want %q, log:\n%s", got, tc.want, log)
			}
			if tc.want != nil && rc != 1 {
				t.Errorf("rc %d, want 1", rc)
			}
			if tc.want == nil && tc.name != "marker body" && rc != 0 {
				t.Errorf("rc %d, log:\n%s", rc, log)
			}
		})
	}
}

func TestLinkThroughASymlinkOutsideTheRepository(t *testing.T) {
	t.Parallel()
	outside := t.TempDir()
	writeTree(t, outside, map[string]string{"x.md": "Text.\n"})
	root := unusedRepo(t, map[string]string{"docs/p.md": "[a](away.md)\n"})
	link(t, filepath.Join(outside, "x.md"), filepath.Join(root, "docs/away.md"))
	rc, log := docgen(t, root, "check")
	if rc != 1 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
	wantIn(t, log, "broken: docs/p.md:1: away.md\n", "docgen: 1 broken; ")
}

func TestLinksOfTheReadme(t *testing.T) {
	t.Parallel()
	root := unusedRepo(t, map[string]string{"README.md": "[a](nope.md)\n"})
	rc, log := docgen(t, root, "check")
	if rc != 1 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
	wantIn(t, log, "broken: README.md:1: nope.md\n")
}

func TestStaleUnusedAndBrokenTogether(t *testing.T) {
	t.Parallel()
	root := unusedRepo(t, map[string]string{
		"a.sh":      regionText,
		"docs/p.md": "<!-- docgen:gomod field=go --><!-- /docgen:gomod -->\n\n[a](nope.md)\n",
	})
	rc, log := docgen(t, root, "check")
	if rc != 1 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
	wantIn(t, log, "stale: docs/p.md:1: ", "unused: a.sh:1: docgen:region r\n", "broken: docs/p.md:3: nope.md\n")
}

func TestLinkToADiagramOfThisRun(t *testing.T) {
	t.Parallel()
	page := "<!-- docgen:states pkg=p type=M -->\n<!-- /docgen:states -->\n\n[a](states/p-m.svg)\n"
	files := machineRepo(page)
	files["go.mod"] = unusedModule
	root := writeRepo(t, files)
	rc, log := docgen(t, root, "check")
	if rc != 1 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
	wantIn(t, log, "stale: ")
	wantNotIn(t, log, "broken:")
	if _, err := os.Stat(filepath.Join(root, "docs/states/p-m.svg")); err == nil {
		t.Error("check wrote the diagram")
	}
}

func TestLinksOfAStaleBody(t *testing.T) {
	t.Parallel()
	const pkg = "// Package p has text.\npackage p\n"
	root := unusedRepo(t, map[string]string{
		"p/p.go":    pkg,
		"docs/p.md": "<!-- docgen:doc pkg=p -->\nOld [link](gone.md).\n<!-- /docgen:doc -->\n",
	})
	rc, log := docgen(t, root, "check")
	if rc != 1 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
	wantIn(t, log, "stale: docs/p.md:1:")
	wantNotIn(t, log, "broken:")
}

func TestAnchorOfAStaleBody(t *testing.T) {
	t.Parallel()
	root := unusedRepo(t, map[string]string{
		"p/p.go":    "// Package p has text.\n//\n// # New Section\n//\n// Text.\npackage p\n",
		"docs/p.md": "[a](#new-section)\n\n<!-- docgen:doc pkg=p -->\n<!-- /docgen:doc -->\n",
	})
	rc, log := docgen(t, root, "check")
	if rc != 1 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
	wantNotIn(t, log, "broken:")
}
