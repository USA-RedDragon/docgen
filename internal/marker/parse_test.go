package marker

import (
	"strings"
	"testing"
)

func TestMarkerErrors(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ text, want string }{
		{"<!-- docgen:nope a=b -->\n<!-- /docgen:nope -->", "unknown marker kind"},
		{"<!-- docgen:config pkg=x -->\n<!-- /docgen:config -->", "type= is required"},
		{"<!-- docgen:output cmd=examples/a test=b.C -->\n<!-- /docgen:output -->", "exactly one of"},
		{"<!-- docgen:code file=a lines=1-2 -->\n", "has no <!-- /docgen:code -->"},
		{"<!-- docgen:const pkg=x name=Y extra=1 --><!-- /docgen:const -->", "has no argument extra="},
	} {
		if _, err := parse(tc.text); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%q: got %v, want %q", tc.text, err, tc.want)
		}
	}
}

func TestLookupCalls(t *testing.T) {
	t.Parallel()
	var kinds []string
	ms, err := Parse("```\n"+testPair+"\n```\n"+testPair+"\n<!-- docgen:region a -->\n<!-- docgen:doc pkg=a -->\n<!-- /docgen:doc -->\n<!-- docgen:end -->\n", func(kind string) Binder {
		kinds = append(kinds, kind)
		return testLookup(kind)
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(kinds) != 2 || kinds[0] != "const" || kinds[1] != "doc" || len(ms) != 2 {
		t.Errorf("lookups %q for %d markers", kinds, len(ms))
	}
}

func TestMarkerInBody(t *testing.T) {
	t.Parallel()
	for name, body := range map[string]string{
		"fence":    "```text\n<!-- docgen:const pkg=a name=B --><!-- /docgen:const -->\n<!-- /docgen:code -->\n```",
		"tilde":    "~~~text\n<!-- docgen:const pkg=a name=B -->\n<!-- /docgen:code -->\n~~~",
		"long":     "````md\n```\n<!-- docgen:const pkg=a name=B -->\n<!-- /docgen:code -->\n```\n````",
		"tab":      "Text.\n\n\t<!-- docgen:const pkg=a name=B --><!-- /docgen:const -->\n\t<!-- /docgen:code -->\n\nText.",
		"spaces":   "Text.\n\n    <!-- docgen:const pkg=a name=B -->\n    <!-- /docgen:code -->",
		"infoline": "```go title=\"x\"\n```not a close\n<!-- docgen:const pkg=a name=B -->\n```",
	} {
		text := "<!-- docgen:code file=x -->\n" + body + "\n<!-- /docgen:code -->\n\n<!-- docgen:gomod field=go --><!-- /docgen:gomod -->\n"
		ms, err := parse(text)
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if len(ms) != 2 || ms[0].Kind != "code" || ms[1].Kind != "gomod" {
			t.Errorf("%s: got %d markers, want the code and the gomod marker", name, len(ms))
			continue
		}
		if want := "\n" + body + "\n"; ms[0].Body != want {
			t.Errorf("%s: body %q, want %q", name, ms[0].Body, want)
		}
	}
}

func TestIndentedMarker(t *testing.T) {
	t.Parallel()
	text := "- item\n\n    <!-- docgen:code file=x -->\n    <!-- /docgen:code -->\n"
	ms, err := parse(text)
	if err != nil {
		t.Fatal(err)
	}
	if len(ms) != 1 || !strings.HasPrefix(ms[0].Body, "\n") {
		t.Fatalf("got %d markers: %+v", len(ms), ms)
	}
}

func TestMalformedMarkers(t *testing.T) {
	t.Parallel()
	for _, bad := range []string{
		`<!-- docgen:const pkg=a name=Nope extra -->`,
		`<!-- docgen:const pkg=a name="Nope"x -->`,
		`<!-- docgen:doc pkg=a section='Build tags' -->`,
		`<!-- docgen:doc pkg=a section=Build tags -->`,
		`<!-- docgen:const pkg=a Name=Nope -->`,
		`<!--docgen:const pkg=a name=Nope-->`,
		`<!--  docgen:const pkg=a name=Nope -->`,
		`<!--- docgen:const pkg=a name=Nope -->`,
		`<!-- docgen:Const pkg=a name=Nope -->`,
		`<!-- DOCGEN:const pkg=a name=Nope -->`,
		`<!-- docgen :const pkg=a name=Nope -->`,
		`<!-- /docgen:const -->`,
		`<!-- /docgen:Const -->`,
		`<!--/docgen:const-->`,
		`<!-- docgen:region -->`,
		`<!-- docgen:region a b -->`,
		`<!-- docgen:Region a -->`,
		`<!-- docgen:End -->`,
		`<!-- docgen:end a -->`,
		`<!-- docgen:doc pkg=a section="x\" -->`,
		`<!-- docgen:doc pkg=a section="x\"y -->`,
	} {
		for name, text := range map[string]string{
			"alone":  "# Title\n\n" + bad + "\n",
			"after":  testPair + "\n\n" + bad + "\n",
			"before": "\n\n" + bad + "\n\n" + testPair + "\n",
		} {
			_, err := parse(text)
			if err == nil || !strings.Contains(err.Error(), "line 3:") {
				t.Errorf("%s: %q: got %v, want an error at line 3", name, bad, err)
			}
		}
	}
}

func TestEscapedQuoteInATag(t *testing.T) {
	t.Parallel()
	open := `<!-- docgen:doc pkg=a section="say \"hi\" twice" -->`
	text := "Text.\n\n" + open + "\nbody\n<!-- /docgen:doc -->\n"
	ms, err := parse(text)
	if err != nil {
		t.Fatal(err)
	}
	if len(ms) != 1 || ms[0].Body != "\nbody\n" {
		t.Fatalf("markers: %+v", ms)
	}
	if want := `docgen:doc pkg=a section="say \"hi\" twice"`; ms[0].String() != want {
		t.Errorf("String = %q, want %q", ms[0].String(), want)
	}
	if got := Prose(text); strings.Contains(got, "body") {
		t.Errorf("Prose kept the body: %q", got)
	}
}

func TestMalformedMarkerInCode(t *testing.T) {
	t.Parallel()
	bad := "<!-- docgen:Const pkg=a -->"
	for name, text := range map[string]string{
		"fence": "```text\n" + bad + "\n```\n",
		"quote": "> ```text\n> " + bad + "\n> ```\n",
		"span":  "Write `" + bad + "` here.\n",
		"body":  "<!-- docgen:doc pkg=a -->\n" + bad + "\n<!-- /docgen:doc -->\n",
	} {
		if _, err := parse(text); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestRegionMarks(t *testing.T) {
	t.Parallel()
	for name, text := range map[string]string{
		"both":     "<!-- docgen:region x -->\nText.\n<!-- docgen:end -->\n",
		"tight":    "<!--docgen:region x -->\nText.\n<!--docgen:end-->\n",
		"marker":   "<!-- docgen:region x -->\n" + testPair + "\n<!-- docgen:end -->\n",
		"in body":  "<!-- docgen:doc pkg=a -->\n<!-- docgen:region x -->\nText.\n<!-- docgen:end -->\n<!-- /docgen:doc -->\n",
		"argument": "<!-- docgen:region a=b -->\nText.\n<!-- docgen:end -->\n",
	} {
		ms, err := parse(text)
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if want := strings.Count(text, "<!-- /docgen:"); name != "marker" && len(ms) != want {
			t.Errorf("%s: got %d markers, want %d", name, len(ms), want)
		}
	}
}

func TestClosePrefix(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct{ open, prefix string }{
		"list":   {"- item\n  ", "  "},
		"tab":    {"- item\n\t", "\t"},
		"quote":  {"> ", "> "},
		"nested": {"> > ", "> > "},
		"indent": {"- item\n\n    ", "    "},
	} {
		text := tc.open + "<!-- docgen:code file=x -->\nold\n" + tc.prefix + "<!-- /docgen:code -->\nafter\n"
		ms, err := parse(text)
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		m := ms[0]
		if m.Body != "\nold\n" {
			t.Errorf("%s: body %q", name, m.Body)
		}
		if got := text[m.End:]; got != tc.prefix+"<!-- /docgen:code -->\nafter\n" {
			t.Errorf("%s: text after the body %q", name, got)
		}
	}
}
