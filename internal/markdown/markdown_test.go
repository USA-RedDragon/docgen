package markdown

import (
	"strings"
	"testing"
)

func TestFence(t *testing.T) {
	t.Parallel()
	cases := map[string]struct{ lang, in, want string }{
		"plain":             {"go", "x := 1", "```go\nx := 1\n```"},
		"no language":       {"", "x", "```\nx\n```"},
		"trailing newlines": {"text", "x\n\n\n", "```text\nx\n```"},
		"leading newline":   {"text", "\nx", "```text\n\nx\n```"},
		"empty":             {"text", "", "```text\n\n```"},
		"one backtick":      {"text", "a ` b", "```text\na ` b\n```"},
		"two backticks":     {"text", "a `` b", "```text\na `` b\n```"},
		"inner fence":       {"md", "```go\nx\n```", "````md\n```go\nx\n```\n````"},
		"longer inner":      {"md", "`````\nx\n`````", "``````md\n`````\nx\n`````\n``````"},
		"mid-line run":      {"md", "a ``` b", "````md\na ``` b\n````"},
		"mixed runs":        {"md", "```\n`````", "``````md\n```\n`````\n``````"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := Fence(c.lang, c.in); got != c.want {
				t.Errorf("Fence(%q, %q) = %q, want %q", c.lang, c.in, got, c.want)
			}
		})
	}
}

// TestFenceCloses checks the CommonMark rule for each body: no line of the body
// closes the fence, because each run of backticks in it is shorter.
func TestFenceCloses(t *testing.T) {
	t.Parallel()
	for _, body := range []string{"```", "````\n```", "x\n`````````\ny"} {
		lines := strings.Split(Fence("text", body), "\n")
		open := strings.TrimSuffix(lines[0], "text")
		if lines[len(lines)-1] != open {
			t.Errorf("close %q does not match open %q", lines[len(lines)-1], open)
		}
		for _, l := range lines[1 : len(lines)-1] {
			if strings.Contains(l, open) {
				t.Errorf("body line %q holds the fence %q", l, open)
			}
		}
	}
}

func TestCell(t *testing.T) {
	t.Parallel()
	cases := map[string]struct{ in, want string }{
		"plain":       {"text", "text"},
		"empty":       {"", ""},
		"pipe":        {"a|b", `a\|b`},
		"pipes":       {"a | b | c", `a \| b \| c`},
		"newlines":    {"one\ntwo\n\nthree", "one two three"},
		"tabs":        {"  a\t\tb  ", "a b"},
		"only spaces": {" \n\t ", ""},
		"pipe at end": {"a\n|\nb|", `a \| b\|`},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := Cell(c.in); got != c.want {
				t.Errorf("Cell(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestPkgLink(t *testing.T) {
	t.Parallel()
	got := PkgLink("example.com/m/client", "Client.Close")
	if want := "https://pkg.go.dev/example.com/m/client#Client.Close"; got != want {
		t.Errorf("PkgLink = %q, want %q", got, want)
	}
}

func TestCodeSpan(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{
		"a":      "`a`",
		"a`b":    "``a`b``",
		"a``b`c": "```a``b`c```",
		"`":      "`` ` ``",
		"`a":     "`` `a ``",
		"a`":     "`` a` ``",
		" a ":    "`  a  `",
		"  ":     "`  `",
		" a":     "` a`",
	} {
		if got := CodeSpan(in); got != want {
			t.Errorf("CodeSpan(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCodeSpanOfEmptyAndBlankLines(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{
		"":           "`\"\"`",
		"a\n\nb":     "`a  b`",
		"a\nb":       "`a b`",
		"\n":         "` `",
		"a\r\n\r\nb": "`a  b`",
	} {
		if got := CodeSpan(in); got != want {
			t.Errorf("CodeSpan(%q) = %q, want %q", in, got, want)
		}
	}
}
