package marker

import (
	"testing"
)

func TestWrapNeutralizesMarkerText(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct{ in, want string }{
		"open":     {"A " + testOpen + " b.", "A &lt;!-- docgen:const pkg=a name=B --> b."},
		"close":    {"A " + testClose + " b.", "A &lt;!-- /docgen:const --> b."},
		"escaped":  {`A \<!-- /docgen:doc --> b.`, "A &lt;!-- /docgen:doc --> b."},
		"tight":    {"A <!--docgen:const--> b.", "A &lt;!--docgen:const--> b."},
		"case":     {"A <!-- DocGen:Const --> b.", "A &lt;!-- DocGen:Const --> b."},
		"region":   {"A <!-- docgen:end --> b.", "A &lt;!-- docgen:end --> b."},
		"table":    {"| a | " + testClose + " |", "| a | &lt;!-- /docgen:const --> |"},
		"other":    {"A <!-- note --> b.", "A <!-- note --> b."},
		"fence":    {"```text\n" + testOpen + "\n```", "```text\n" + testOpen + "\n```"},
		"tilde":    {"~~~\n" + testClose + "\n~~~", "~~~\n" + testClose + "\n~~~"},
		"indented": {"Text.\n\n\t" + testOpen + "\n\nText.", "Text.\n\n\t" + testOpen + "\n\nText."},
		"span":     {"| `" + testClose + "` |", "| `" + testClose + "` |"},
		"mixed":    {"```\n" + testOpen + "\n```\n\n" + testOpen, "```\n" + testOpen + "\n```\n\n&lt;!-- docgen:const pkg=a name=B -->"},
	} {
		m := Marker{Kind: "doc"}
		if got := m.Wrap(tc.in); got != "\n"+tc.want+"\n" {
			t.Errorf("%s: got %q, want %q", name, got, tc.want)
		}
	}
	inline := Marker{Kind: "const", inline: true}
	if got, want := inline.Wrap("`"+testClose+"`"), "`"+testClose+"`"; got != want {
		t.Errorf("inline span: got %q, want %q", got, want)
	}
	if got, want := inline.Wrap(`a`+testClose), "a&lt;!-- /docgen:const -->"; got != want {
		t.Errorf("inline text: got %q, want %q", got, want)
	}
}

func TestWrapReparses(t *testing.T) {
	t.Parallel()
	for name, content := range map[string]string{
		"open":     "A " + testOpen + " b.",
		"close":    "A <!-- /docgen:doc --> b.",
		"escaped":  `A \<!-- /docgen:doc --> b.`,
		"code":     "Text.\n\n\t<!-- /docgen:doc -->\n\t" + testOpen + "\n\nText.",
		"fence":    "```\n<!-- /docgen:doc -->\n```",
		"tick":     "a ` <!-- /docgen:doc --> b",
		"comment":  "a <!-- b",
		"unclosed": "\t```go\n\n<!-- /docgen:doc -->",
		"span":     "| `<!-- /docgen:doc -->` |\n\n\t" + testOpen,
		"moved":    "a ``` b\n" + testOpen + "``` " + testOpen + " ```",
	} {
		for _, prefix := range []string{"", "- item\n  ", "> "} {
			first := prefix + "<!-- docgen:doc pkg=a -->\n<!-- /docgen:doc -->\n"
			ms, err := parse(first)
			if err != nil {
				t.Fatal(err)
			}
			body := ms[0].Wrap(content)
			second := first[:ms[0].Start] + body + first[ms[0].End:]
			ms, err = parse(second + "\n" + testPair + "\n")
			if err != nil {
				t.Errorf("%s: %q: %v\n%s", name, prefix, err, second)
				continue
			}
			if len(ms) != 2 || ms[0].Body != body || ms[0].Wrap(content) != body {
				t.Errorf("%s: %q: got %d markers, body %q, want %q", name, prefix, len(ms), ms[0].Body, body)
			}
		}
	}
}
