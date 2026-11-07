package links

import (
	"slices"
	"testing"
)

func targets(text string) []string {
	links := Scan(text)
	if len(links) == 0 {
		return nil
	}
	out := make([]string, 0, len(links))
	for _, l := range links {
		out = append(out, l.Target)
	}
	return out
}

func TestScan(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		text string
		want []string
	}{
		{"inline", "A [b](c.md) d.\n", []string{"c.md"}},
		{"image", "![a](b.png)\n", []string{"b.png"}},
		{"double title", "[a](b.md \"The title\")\n", []string{"b.md"}},
		{"single title", "[a](b.md 'The title')\n", []string{"b.md"}},
		{"paren title", "[a](b.md (The title))\n", []string{"b.md"}},
		{"angle", "[a](<b c.md>)\n", []string{"b c.md"}},
		{"parens", "[a](b(c).md)\n", []string{"b(c).md"}},
		{"escaped paren", "[a](b\\).md)\n", []string{"b).md"}},
		{"entity", "[a](b&amp;c.md)\n", []string{"b&c.md"}},
		{"nested image", "[![a](i.png)](t.md)\n", []string{"i.png", "t.md"}},
		{"code in text", "[`a`](b.md)\n", []string{"b.md"}},
		{"two lines", "[a\nb](c.md)\n", []string{"c.md"}},
		{"table cell", "| a | b |\n|---|---|\n| ![c](d.png) | [e](f.md) |\n", []string{"d.png", "f.md"}},
		{"pipe in a cell", "| [a](b\\|c.md) |\n", []string{"b|c.md"}},
		{"escaped open", "\\[a](b.md)\n", nil},
		{"escaped image", "!\\[a](b.png)\n", nil},
		{"escaped close", "[a\\](b.md)\n", nil},
		{"escaped backslash", "\\\\[a](b.md)\n", []string{"b.md"}},
		{"close in text", "[a\\]b](c.md)\n", []string{"c.md"}},
		{"fence", "```\n[a](b.md)\n```\n", nil},
		{"tilde fence", "~~~md\n[a](b.md)\n~~~\n", nil},
		{"code span", "A `[a](b.md)` c.\n", nil},
		{"indented code", "Text.\n\n    [a](b.md)\n\nText.\n", nil},
		{"indented in a list", "- Item.\n\n    [a](b.md)\n", []string{"b.md"}},
		{"comment", "<!-- [a](b.md) -->\n", nil},
		{"marker body", "<!-- docgen:doc pkg=p -->\n[a](b.md)\n<!-- /docgen:doc -->\n", []string{"b.md"}},
		{"fence in a marker body", "<!-- docgen:doc pkg=p -->\n```\n[a](b.md)\n```\n<!-- /docgen:doc -->\n", nil},
		{"no destination", "[a] (b.md)\n", nil},
		{"space in the target", "[a](b c.md)\n", nil},
		{"blank line", "[a\n\nb](c.md)\n", nil},
		{"definition", "[a][b]\n\n[b]: c.md\n", []string{"c.md"}},
		{"definition with a title", "[b]: c.md \"T\"\n", []string{"c.md"}},
		{"definition in angles", "[b]: <c d.md>\n", []string{"c d.md"}},
		{"collapsed", "[b][]\n\n[b]: c.md\n", []string{"c.md"}},
		{"shortcut", "[b]\n\n[b]: c.md\n", []string{"c.md"}},
		{"label case", "[a][B  C]\n\n[b c]: d.md\n", []string{"d.md"}},
		{"not a definition", "[b]: c d e\n", nil},
		{"in a paragraph", "Text\n[b]: c.md\n", nil},
		{"footnote", "A[^1].\n\n[^1]: The note.\n", nil},
		{"text in brackets", "A [b] c.\n", nil},
		{"undefined", "[a][nope]\n", []string{"[nope]"}},
		{"undefined collapsed", "[foo][]\n", []string{"[foo]"}},
		{"collapsed footnote", "[^1][]\n", nil},
		{"definition on two lines", "[x][a]\n\n[a]:\n    ok.md\n", []string{"ok.md"}},
		{"definition with a title below", "[x][t]\n\n[t]: a.md\n\"title\"\n[u]: b.md\n", []string{"a.md", "b.md"}},
		{"title below in a quote", "> [t]: a.md\n> \"title\"\n", []string{"a.md"}},
		{"definition after a heading", "# T\n[a]: ok.md\n\n[x][a]\n", []string{"ok.md"}},
		{"definition after a setext heading", "T\n=\n[a]: ok.md\n\n[x][a]\n", []string{"ok.md"}},
		{"definition in a quote", "> [a]: ok.md\n\n[x][a]\n", []string{"ok.md"}},
		{"definition in a list item", "[x][a]\n\n- [a]: ok.md\n", []string{"ok.md"}},
		{"pre block", "<pre>\n[a](nope.md)\n\n[b](nope.md)\n</pre>\n\n[c](ok.md)\n", []string{"ok.md"}},
		{"script block", "<script>\nf[a](nope.md)\n</script>\n", nil},
		{"div block", "<div>\n[a](nope.md)\n</div>\n", nil},
		{"after a div block", "<div>\n\n[a](ok.md)\n\n</div>\n", []string{"ok.md"}},
		{"definition in a div block", "<div>\n[a]: nope.md\n</div>\n", nil},
		{"custom tag block", "<foo>\n[a](nope.md)\n</foo>\n", nil},
		{"inline tag in a paragraph", "Text <span>[a](ok.md)</span>\n", []string{"ok.md"}},
		{"tag in a paragraph line", "Text\n<span>\n[a](ok.md)\n", []string{"ok.md"}},
		{"code in a list item", "- Item.\n\n      [a](nope.md)\n", nil},
		{"code in an ordered item", "1. Item.\n\n       [a](nope.md)\n", nil},
		{"text of a nested item", "- A\n\n  - B\n\n    [a](ok.md)\n", []string{"ok.md"}},
		{"code after a heading", "# T\n    [a](nope.md)\n", nil},
		{"code after a setext heading", "T\n=\n    [a](nope.md)\n", nil},
		{"indented paragraph line", "Text\n    [a](ok.md)\n", []string{"ok.md"}},
		{"code span at the start of a line", "`abcd` and [a](ok.md)\n", []string{"ok.md"}},
		{"undefined before a target", "[a][b](ok.md)\n", []string{"ok.md"}},
		{"undefined before a reference", "[a][b][c]\n\n[c]: ok.md\n", []string{"ok.md"}},
		{"escaped bracket in a label", "[x][a\\]b]\n\n[a\\]b]: ok.md\n", []string{"ok.md"}},
		{"full case fold", "[x][\u1e9e]\n\n[SS]: ok.md\n", []string{"ok.md"}},
		{"final sigma", "[x][\u03a3]\n\n[\u03c2]: ok.md\n", []string{"ok.md"}},
		{"ligature", "[x][\ufb01]\n\n[FI]: ok.md\n", []string{"ok.md"}},
		{"link in a link", "[a [b](ok.md) c](nope.md)\n", []string{"ok.md"}},
		{"reference in a link", "[a [b][c] d](nope.md)\n\n[c]: ok.md\n", []string{"ok.md"}},
		{"shortcut in a link", "[a [c] d](nope.md)\n\n[c]: ok.md\n", []string{"ok.md"}},
		{"image in an image", "![a ![b](i.png) c](j.png)\n", []string{"i.png", "j.png"}},
		{"link in an image", "![a [b](ok.md) c](i.png)\n", []string{"ok.md", "i.png"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := targets(tc.text); !slices.Equal(got, tc.want) {
				t.Errorf("Scan = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestScanLines(t *testing.T) {
	t.Parallel()
	got := Scan("# T\n\n[a](b.md)\n\n[c][d]\n\n[d]: e.md\n\n[f\ng](h.md)\n")
	want := []Link{{Line: 7, Target: "e.md"}, {Line: 3, Target: "b.md"}, {Line: 10, Target: "h.md"}}
	if !slices.Equal(got, want) {
		t.Errorf("Scan = %+v, want %+v", got, want)
	}
}

func TestUndefinedReference(t *testing.T) {
	t.Parallel()
	got := Scan("\n[a][nope]\n")
	want := []Link{{Line: 2, Target: "[nope]", Undefined: true}}
	if !slices.Equal(got, want) {
		t.Errorf("Scan = %+v, want %+v", got, want)
	}
}

func TestSplit(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		target, path, fragment string
		local                  bool
	}{
		{"a.md", "a.md", "", true},
		{"a.md#b", "a.md", "b", true},
		{"#b", "", "b", true},
		{"a.md?raw=1#b", "a.md", "b", true},
		{"my%20file.md", "my file.md", "", true},
		{"a%23b.md#c%20d", "a#b.md", "c d", true},
		{"100%.md", "100%.md", "", true},
		{"https://example.com/a", "", "", false},
		{"mailto:a@example.com", "", "", false},
		{"//example.com/a", "", "", false},
	} {
		path, fragment, local := Split(tc.target)
		if path != tc.path || fragment != tc.fragment || local != tc.local {
			t.Errorf("Split(%q) = %q, %q, %v", tc.target, path, fragment, local)
		}
	}
}

func TestDefinitionInAListItemAlone(t *testing.T) {
	t.Parallel()
	if got := Scan("- [a]: nope.md\n"); len(got) != 0 {
		t.Errorf("Scan = %+v, want none", got)
	}
}

func TestDefinitionTargetLine(t *testing.T) {
	t.Parallel()
	got := Scan("[x][a]\n\n[a]:\n    ok.md\n")
	want := []Link{{Line: 4, Target: "ok.md"}}
	if !slices.Equal(got, want) {
		t.Errorf("Scan = %+v, want %+v", got, want)
	}
}

func TestByteOrderMarkDefinition(t *testing.T) {
	t.Parallel()
	got := Scan("\ufeff[a]: ok.md\n\n[x][a]\n")
	want := []Link{{Line: 1, Target: "ok.md"}}
	if !slices.Equal(got, want) {
		t.Errorf("Scan = %+v, want %+v", got, want)
	}
}
