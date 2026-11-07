package godoc

import (
	"go/doc/comment"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/USA-RedDragon/docgen/internal/gocmd"
	"github.com/USA-RedDragon/docgen/internal/gomod"
	"github.com/USA-RedDragon/docgen/internal/load"
)

const lampSource = `// Package lamp drives a lamp.
//
// See [Lamp], [fmt.Println] and [the manual].
//
// # Wiring
//
// Connect the lamp to a supply.
//
// A second paragraph.
//
// # Limits
//
// The most is [MaxWatts].
//
// # Empty
//
// # Last
//
// The end.
//
// [the manual]: https://example.com/manual
package lamp

import "fmt"

// Group doc of the limits.
const (
	// MaxWatts is the most power.
	MaxWatts = 60
	MinWatts = 5 // MinWatts is the least power.
	Spare    = 1
	// Low and High are the two levels.
	Low, High = 1, 2
)

// hold is the time of a press.
const hold = 3

// Solo is alone in its declaration.
const Solo = 7

// Group doc of the vars.
var (
	// Supply is the voltage.
	Supply = 12
	Mains  = 230 // Mains is the wall voltage.
	Other  = 1
)

// A Mode is a state of a Lamp.
type Mode int

// Group doc of the modes.
const (
	// Off gives no light.
	Off Mode = iota
	On       // On gives light.
	Dim
)

// Fallback is the mode after a fault.
var Fallback Mode = Off

// A Lamp is one lamp.
type Lamp struct{ mode Mode }

// New returns a Lamp.
//
// # Errors
//
// None.
func New() *Lamp { return &Lamp{} }

// Press changes the mode of the lamp.
func (l *Lamp) Press() { l.mode = On }

// Standalone prints.
func Standalone() { fmt.Println() }

func Bare() {}
`

func loadLamp(t *testing.T) *load.Package {
	t.Helper()
	root := t.TempDir()
	for name, text := range map[string]string{
		"go.mod":       "module example.com/w\n\ngo 1.24.0\n",
		"lamp/lamp.go": lampSource,
	} {
		p := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	m, err := gomod.Open(root, root)
	if err != nil {
		t.Fatal(err)
	}
	session := &gocmd.Session{}
	defer session.Close()
	set, err := load.Packages(t.Context(), m, session, map[string]bool{"example.com/w/lamp": true})
	if err != nil {
		t.Fatal(err)
	}
	pi, err := set.Package("lamp")
	if err != nil {
		t.Fatal(err)
	}
	return pi
}

func TestMarkdownNames(t *testing.T) {
	t.Parallel()
	pi := loadLamp(t)
	for name, want := range map[string]string{
		"MaxWatts": "MaxWatts is the most power.",
		"MinWatts": "MinWatts is the least power.",
		"Spare":    "Group doc of the limits.",
		"Low":      "Low and High are the two levels.",
		"High":     "Low and High are the two levels.",
		"hold":     "hold is the time of a press.",
		"Solo":     "Solo is alone in its declaration.",
		"Supply":   "Supply is the voltage.",
		"Mains":    "Mains is the wall voltage.",
		"Other":    "Group doc of the vars.",
		"Off":      "Off gives no light.",
		"On":       "On gives light.",
		"Dim":      "Group doc of the modes.",
		"Fallback": "Fallback is the mode after a fault.",

		"Lamp":       "A Lamp is one lamp.",
		"Mode":       "A Mode is a state of a Lamp.",
		"Lamp.Press": "Press changes the mode of the lamp.",
		"Standalone": "Standalone prints.",
	} {
		got, err := Markdown(pi, name, "")
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
}

func TestMarkdownUndocumented(t *testing.T) {
	t.Parallel()
	pi := loadLamp(t)
	if _, err := Markdown(pi, "Bare", ""); err == nil || !strings.Contains(err.Error(), "no doc comment for Bare in") {
		t.Errorf("err = %v", err)
	}
}

func TestMarkdownConstructor(t *testing.T) {
	t.Parallel()
	pi := loadLamp(t)
	got, err := Markdown(pi, "New", "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(got, "New returns a Lamp.\n\n### Errors") {
		t.Errorf("New = %q", got)
	}
}

func TestMarkdownPackage(t *testing.T) {
	t.Parallel()
	pi := loadLamp(t)
	got, err := Markdown(pi, "", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"Package lamp drives a lamp.",
		"[Lamp](https://pkg.go.dev/example.com/w/lamp#Lamp)",
		"[fmt.Println](https://pkg.go.dev/fmt#Println)",
		"[the manual](https://example.com/manual)",
		"### Wiring",
		"### Limits",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("no %q in:\n%s", want, got)
		}
	}
	if strings.HasSuffix(got, "\n") {
		t.Errorf("trailing newline in %q", got)
	}
}

func TestMarkdownSection(t *testing.T) {
	t.Parallel()
	pi := loadLamp(t)
	for _, c := range []struct{ name, section, want string }{
		{"", "Wiring", "Connect the lamp to a supply.\n\nA second paragraph."},
		{"", "Limits", "The most is [MaxWatts](https://pkg.go.dev/example.com/w/lamp#MaxWatts)."},
		{"", "Last", "The end."},
		{"New", "Errors", "None."},
	} {
		got, err := Markdown(pi, c.name, c.section)
		if err != nil {
			t.Errorf("%q section %q: %v", c.name, c.section, err)
			continue
		}
		if got != c.want {
			t.Errorf("%q section %q = %q, want %q", c.name, c.section, got, c.want)
		}
	}
}

func TestMarkdownErrors(t *testing.T) {
	t.Parallel()
	pi := loadLamp(t)
	for _, c := range []struct{ name, section, want string }{
		{"Missing", "", "no doc for Missing in example.com/w/lamp"},
		{"Lamp.Missing", "", "no doc for Lamp.Missing in example.com/w/lamp"},
		{"Missing.Press", "", "no doc for Missing.Press in example.com/w/lamp"},
		{"", "Nope", "no heading # Nope in the doc of package example.com/w/lamp"},
		{"New", "Nope", "no heading # Nope in the doc of New in example.com/w/lamp"},
		{"", "Empty", "heading # Empty in the doc of package example.com/w/lamp has no text"},
	} {
		got, err := Markdown(pi, c.name, c.section)
		if err == nil || err.Error() != c.want {
			t.Errorf("%q section %q = %q, %v, want error %q", c.name, c.section, got, err, c.want)
		}
	}
}

func TestMarkdownRootPackage(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dir := filepath.Join(root, "tool")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/tool\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	src := "// Command tool runs.\npackage main\n\n// Group doc.\nconst (\n\t// Rate is the rate.\n\tRate = 1\n\tSize = 2\n)\n\nfunc main() {}\n"
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	m, err := gomod.Open(root, root)
	if err != nil {
		t.Fatal(err)
	}
	pi, err := load.RootPackage(m, nil, "tool")
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{"": "Command tool runs.", "Rate": "Rate is the rate.", "Size": "Group doc."} {
		got, err := Markdown(pi, name, "")
		if err != nil {
			t.Errorf("%q: %v", name, err)
			continue
		}
		if got != want {
			t.Errorf("%q = %q, want %q", name, got, want)
		}
	}
}

func TestInlineText(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		in   comment.Text
		want string
	}{
		{"plain", comment.Plain("a"), "a"},
		{"italic", comment.Italic("b"), "b"},
		{"link", &comment.Link{Text: []comment.Text{comment.Plain("c"), comment.Italic("d")}, URL: "https://example.com"}, "cd"},
		{"doc link", &comment.DocLink{Text: []comment.Text{comment.Plain("e")}, Name: "E"}, "e"},
		{"nested", &comment.Link{Text: []comment.Text{&comment.DocLink{Text: []comment.Text{comment.Plain("f")}}}}, "f"},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			var b strings.Builder
			inlineText(&b, c.in)
			if b.String() != c.want {
				t.Errorf("text = %q, want %q", b.String(), c.want)
			}
		})
	}
}

func TestHeadingText(t *testing.T) {
	t.Parallel()
	h := &comment.Heading{Text: []comment.Text{comment.Plain("Build "), comment.Italic("tags")}}
	if got := headingText(h); got != "Build tags" {
		t.Errorf("headingText = %q", got)
	}
}

func TestCodeSpans(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{
		"a \\`b=\\` c":                  "a `b=` c",
		"\\`\\<dir>\\` and \\`x\\_y\\`": "`<dir>` and `x_y`",
		"no spans \\* here":             "no spans \\* here",
		"one \\` only":                  "one \\` only",
		"\t\\`code block\\`":            "\t\\`code block\\`",
	} {
		if got := codeSpans(in); got != want {
			t.Errorf("codeSpans(%q) = %q, want %q", in, got, want)
		}
	}
}
