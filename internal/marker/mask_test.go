package marker

import (
	"strings"
	"testing"
)

func TestMarkerInFence(t *testing.T) {
	t.Parallel()
	text := "a <!-- docgen:gomod field=go --><!-- /docgen:gomod -->\n\n```text\n<!-- docgen:config pkg=a type=B -->\n<!-- /docgen:config -->\n```\n\n<!-- docgen:const pkg=a name=C --><!-- /docgen:const -->\n"
	ms, err := parse(text)
	if err != nil {
		t.Fatal(err)
	}
	if len(ms) != 2 || ms[0].Kind != "gomod" || ms[1].Kind != "const" {
		t.Fatalf("got %d markers, want only the two outside the fence", len(ms))
	}
}

func TestCodeSpans(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		text string
		want int
	}{
		"span":           {"Write `" + testOpen + "` here.\n", 0},
		"pair":           {"Write `" + testPair + "` here.\n", 0},
		"double":         {"Write `` ` " + testOpen + " `` here.\n", 0},
		"close":          {"Write `" + testClose + "` here.\n", 0},
		"two lines":      {"Write `a\nb " + testOpen + " c` here.\n", 0},
		"block comment":  {"A ` tick.\n" + testOpen + "`1`" + testClose + "\n", 1},
		"then marker":    {"Write `" + testOpen + "` and " + testPair + ".\n", 1},
		"escaped":        {"\\`" + testPair + "\\`\n", 1},
		"unclosed":       {"A ` tick and " + testPair + ".\n", 1},
		"other length":   {"A `` tick " + testPair + " ` here.\n", 1},
		"two paragraphs": {"A ` tick.\n\n" + testPair + " b ` c.\n", 1},
		"list items":     {"- a ` b\n- " + testPair + " ` c\n", 1},
		"table rows":     {"| a ` b |\n| " + testPair + " ` c |\n", 1},
		"comment first":  {"<!-- docgen:doc pkg=a section=\"a`b\" -->\n<!-- /docgen:doc --> ` c\n", 1},
		"body span":      {testOpen + "`" + testClose + "`" + testClose + "\n", 1},
		"escaped opener": {"\\" + testPair + "\n", 0},
	} {
		text := tc.text
		if name == "escaped opener" {
			text = "\\" + testOpen + " \\" + testClose + "\n"
		}
		ms, err := parse(text)
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if len(ms) != tc.want {
			t.Errorf("%s: got %d markers, want %d", name, len(ms), tc.want)
		}
	}
}

func TestFences(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		text string
		want int
	}{
		"quote":            {"> ```\n> " + testPair + "\n> ```\n", 0},
		"quote no space":   {">```\n>" + testPair + "\n>```\n", 0},
		"nested quote":     {"> > ~~~\n> > " + testPair + "\n> > ~~~\n", 0},
		"quote then page":  {"> ```\n> " + testPair + "\n> ```\n\n" + testPair + "\n", 1},
		"quote ends fence": {"> ```\n> code\n\n" + testPair + "\n", 1},
		"quote in fence":   {"```\n> ```\n" + testPair + "\n```\n", 0},
		"list item":        {"- ```\n  " + testPair + "\n  ```\n\n" + testPair + "\n", 1},
		"inline triple":    {"```inline```\n\n" + testPair + "\n", 1},
		"inline info tick": {"``` a`b\n\n" + testPair + "\n", 1},
		"inline quote":     {"> ```inline```\n\n" + testPair + "\n", 1},
		"tilde info tick":  {"~~~ a`b\n" + testPair + "\n~~~\n", 0},
		"indented outside": {"    " + testPair + "\n", 1},
	} {
		ms, err := parse(tc.text)
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if len(ms) != tc.want {
			t.Errorf("%s: got %d markers, want %d", name, len(ms), tc.want)
		}
	}
}

func TestUnbalancedFenceInIndentedBody(t *testing.T) {
	t.Parallel()
	text := "<!-- docgen:doc pkg=a -->\nText.\n\n\t```go\n\n<!-- /docgen:doc -->\n\n" + testPair + "\n"
	ms, err := parse(text)
	if err != nil {
		t.Fatal(err)
	}
	if len(ms) != 2 {
		t.Fatalf("got %d markers, want 2", len(ms))
	}
}

func TestProseAndMaskCode(t *testing.T) {
	t.Parallel()
	text := "a\n```\nfence\n```\n`span`\n<!-- docgen:doc pkg=p -->\nbody\n\n    indented\n<!-- /docgen:doc -->\n<!-- docgen:nope\n"
	prose, masked := Prose(text), MaskCode(text)
	if len(prose) != len(text) || len(masked) != len(text) {
		t.Fatalf("lengths %d, %d, want %d", len(prose), len(masked), len(text))
	}
	for _, s := range []string{prose, masked} {
		for _, code := range []string{"fence", "span", "indented"} {
			if strings.Contains(s, code) {
				t.Errorf("%q in %q", code, s)
			}
		}
	}
	if strings.Contains(prose, "body") {
		t.Errorf("a body in %q", prose)
	}
	if !strings.Contains(masked, "body") {
		t.Errorf("no body in %q", masked)
	}
}
