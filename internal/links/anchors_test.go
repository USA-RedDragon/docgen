package links

import (
	"testing"
)

func TestAnchors(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		text string
		want []string
		not  []string
	}{
		{"words", "# Hello World\n", []string{"hello-world"}, []string{"Hello-World", "hello"}},
		{"punctuation", "## What's new, v1.2?\n", []string{"whats-new-v12"}, nil},
		{"hyphens stay", "# a - b\n", []string{"a---b"}, []string{"a-b"}},
		{"underscore stays", "# snake_case\n", []string{"snake_case"}, nil},
		{"closing hashes", "## Title ##\n", []string{"title"}, nil},
		{"code span", "## The `Run` method\n", []string{"the-run-method"}, nil},
		{"code span with a tag", "## Use `<T>`\n", []string{"use-t"}, nil},
		{"emphasis", "## *Very* **big** _word_\n", []string{"very-big-word"}, nil},
		{"link", "## See [the guide](a.md)\n", []string{"see-the-guide"}, nil},
		{"image", "## ![a](b.png) Logo\n", []string{"-logo"}, []string{"logo"}},
		{"entity", "## A &amp; B\n", []string{"a--b"}, nil},
		{"unicode", "## Überblick\n", []string{"überblick"}, nil},
		{"marker", "## Go <!-- docgen:gomod field=go -->`1.24`<!-- /docgen:gomod -->\n", []string{"go-124"}, nil},
		{"setext", "Title One\n=========\n\nTwo\n---\n", []string{"title-one", "two"}, nil},
		{"duplicates", "# A\n\n# A\n\n# A\n", []string{"a", "a-1", "a-2"}, []string{"a-3"}},
		{"fence", "```\n# Not\n```\n", nil, []string{"not"}},
		{"indented code", "Text.\n\n    # Not\n", nil, []string{"not"}},
		{"not a heading", "#Not\n", nil, []string{"not"}},
		{"comment after the hash", "#<!-- c --> Not\n", nil, []string{"not"}},
		{"id", "<div class=\"x\" id=\"here\"></div>\n", []string{"here"}, nil},
		{"id single", "<span id='here'></span>\n", []string{"here"}, nil},
		{"a name", "<a name=\"here\"></a>\n", []string{"here"}, nil},
		{"id in code", "`<a id=\"not\">`\n", nil, []string{"not"}},
		{"meta name", "<meta name=\"not\">\n", nil, []string{"not"}},
		{"number of a heading", "# Dup\n\n# Dup\n\n# Dup-1\n", []string{"dup", "dup-1", "dup-1-1"}, nil},
		{"leading comment", "# <!-- c --> Text\n", []string{"-text"}, []string{"text"}},
		{"underscores in emphasis", "# _a_b_\n", []string{"a_b"}, []string{"_a_b_"}},
		{"two emphasis runs", "# _a_ _b_\n", []string{"a-b"}, nil},
		{"open underscore", "# _a\n", []string{"_a"}, nil},
		{"backtick in a code span", "# `` a`b ``\n", []string{"ab"}, nil},
		{"multi-line setext", "Multi-line\nsetext\n===\n", []string{"multi-linesetext"}, nil},
		{"break after a list item", "- item\n---\n", nil, []string{"item"}},
		{"setext in a list item", "- item\n  ---\n", []string{"item"}, nil},
		{"html heading", "<h3>Html Title</h3>\n", []string{"html-title"}, nil},
		{"html heading with attributes", "<h2 class=\"x\">Html <em>Title</em></h2>\n", []string{"html-title"}, nil},
		{"html heading in code", "`<h3>Not</h3>`\n", nil, []string{"not"}},
		{"html heading after a heading", "# Same\n\n<h2>Same</h2>\n", []string{"same", "same-1"}, nil},
		{"heading in a list", "- # In list\n", []string{"in-list"}, nil},
		{"heading in an ordered list", "1. ## In list\n", []string{"in-list"}, nil},
		{"heading in a quote", "> # In quote\n", []string{"in-quote"}, nil},
		{"letter number", "# \u2167 word\n", []string{"\u2177-word"}, nil},
		{"byte order mark", "\ufeff# Title\n", []string{"title"}, nil},
		{"id in a block", "<div id=\"here\">\n</div>\n", []string{"here"}, nil},
		{"heading in pre", "<pre>\n# Not\n</pre>\n", nil, []string{"not"}},
		{"code after a heading", "# T\n    # Not\n", []string{"t"}, []string{"not"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := Anchors(tc.text)
			for _, w := range tc.want {
				if !got[w] {
					t.Errorf("no anchor %q in %v", w, got)
				}
			}
			for _, n := range tc.not {
				if got[n] {
					t.Errorf("anchor %q in %v", n, got)
				}
			}
		})
	}
}
