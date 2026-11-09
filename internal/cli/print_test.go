package cli

import (
	"strings"
	"testing"
)

const printSource = `package p

import (
	"fmt"
	"time"
)

// Port has a String method.
type Port int

func (Port) String() string { panic("String ran") }

// Main is a port.
const Main Port = 4001

// Enum has a Format method.
type Enum uint8

func (Enum) Format(fmt.State, rune) { panic("Format ran") }

// First and Second are values of Enum.
const (
	First Enum = iota
	Second
)

// Kind has an Error method.
type Kind string

func (Kind) Error() string { panic("Error ran") }

// A is a Kind.
const A Kind = "a"

// Mode has a GoString method.
type Mode int

func (Mode) GoString() string { panic("GoString ran") }

// M is a Mode.
const M Mode = 7

// Ratio has a String method.
type Ratio float64

func (Ratio) String() string { panic("String ran") }

// Half is a Ratio.
const Half Ratio = 0.5

// Wait is a duration.
const Wait = 1500 * time.Millisecond

// Plain has no method.
type Plain int

// B is a Plain.
const B Plain = 2

var Current = Main
`

// configSource is a package with a config and its defaults.
const configSource = `package p

import (
	"strconv"
	"time"
)

// Port has a String method.
type Port int

func (p Port) String() string { return "port-" + strconv.Itoa(int(p)) }

// DefaultPort is a port.
const DefaultPort Port = 4001

// Level is a level.
type Level int

// The levels.
const (
	Low Level = iota
	High
)

// DefaultWait is a duration.
const DefaultWait = 1500 * time.Millisecond

// Config holds settings.
type Config struct {
	Listen Port
	Other  Port
	Grade  Level
	Wait   time.Duration
	Name   string
}

// New applies the defaults.
func New(c Config) Config {
	if c.Listen == 0 {
		c.Listen = DefaultPort
	}
	if c.Other == 0 {
		c.Other = 5
	}
	if c.Grade == 0 {
		c.Grade = 1
	}
	if c.Wait == 0 {
		c.Wait = DefaultWait
	}
	if c.Name == "" {
		c.Name = "x"
	}
	return c
}
`

func TestConstPrintRaw(t *testing.T) {
	t.Parallel()
	for args, want := range map[string]string{
		"name=Main print=raw":    "`4001`",
		"name=Second print=raw":  "`1`",
		"name=A print=raw":       "`a`",
		"name=M print=raw":       "`7`",
		"name=Half print=raw":    "`0.5`",
		"name=Wait print=raw":    "`1500000000`",
		"name=B print=raw":       "`2`",
		"name=Current print=raw": "`4001`",
	} {
		t.Run(args, func(t *testing.T) {
			t.Parallel()
			root := pageRepo(t, map[string]string{"p/p.go": printSource, "docs/p.md": constPage(args)})
			rc, log := docgen(t, root, "update")
			if rc != 0 {
				t.Fatalf("rc %d, log:\n%s", rc, log)
			}
			wantNotIn(t, log, "docgen: printing")
			wantIn(t, readPage(t, root, "docs/p.md"), "-->"+want+"<!--")
		})
	}
}

func TestConstPrintGoIsTheDefaultRule(t *testing.T) {
	t.Parallel()
	root := pageRepo(t, map[string]string{
		"p/p.go": constSource, "q/q.go": constForeign,
		"docs/p.md": constPage("name=On", "name=On print=go", "name=Wait", "name=Wait print=go"),
	})
	got := updateTwice(t, root, "docs/p.md")
	if strings.Count(got, "-->`on`<!--") != 2 || strings.Count(got, "-->`1.5s`<!--") != 2 {
		t.Errorf("page:\n%s", got)
	}
}

func TestConfigPrint(t *testing.T) {
	t.Parallel()
	for name, c := range map[string]struct {
		arg  string
		want []string
	}{
		"default": {"", []string{"| `DefaultPort` (port-4001) |", "| `5` (port-5) |", "| `1` |", "| `DefaultWait` (1.5s) |", "| `\"x\"` |"}},
		"go":      {" print=go", []string{"| `DefaultPort` (port-4001) |", "| `5` (port-5) |", "| `1` |", "| `DefaultWait` (1.5s) |", "| `\"x\"` |"}},
		"raw":     {" print=raw", []string{"| `DefaultPort` (4001) |", "| `5` |", "| `1` |", "| `DefaultWait` (1500000000) |", "| `\"x\"` |"}},
		"name":    {" print=name", []string{"| `DefaultPort` |", "| `5` |", "| `1` |", "| `DefaultWait` |", "| `\"x\"` |"}},
		"verb":    {" print=%v", []string{"| `DefaultPort` (port-4001) |", "| `5` (port-5) |", "| `1` |", "| `DefaultWait` (1.5s) |", "| `\"x\"` (x) |"}},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := pageRepo(t, map[string]string{
				"p/p.go":    configSource,
				"docs/p.md": "<!-- docgen:config pkg=p type=Config" + c.arg + " -->\n<!-- /docgen:config -->\n",
			})
			wantIn(t, updateTwice(t, root, "docs/p.md"), c.want...)
		})
	}
}

func TestConfigPrintVerbOfAnInt(t *testing.T) {
	t.Parallel()
	root := pageRepo(t, map[string]string{
		"p/p.go":    "package p\n\nconst D = 255\n\n// C holds settings.\ntype C struct{ A, B int }\n\n// New applies the defaults.\nfunc New(c C) C {\n\tif c.A == 0 {\n\t\tc.A = D\n\t}\n\tif c.B == 0 {\n\t\tc.B = 16\n\t}\n\treturn c\n}\n",
		"docs/p.md": "<!-- docgen:config pkg=p type=C print=%x -->\n<!-- /docgen:config -->\n",
	})
	wantIn(t, updateTwice(t, root, "docs/p.md"), "| `D` (ff) |", "| `16` (10) |")
}

func TestConfigPrintErrors(t *testing.T) {
	t.Parallel()
	for arg, want := range map[string]string{
		"print=nope": "print=nope is not go, raw, config, name or a fmt verb",
		"print=%d":   "print=%d is not valid for the value",
	} {
		t.Run(arg, func(t *testing.T) {
			t.Parallel()
			root := pageRepo(t, map[string]string{
				"p/p.go":    configSource,
				"docs/p.md": "<!-- docgen:config pkg=p type=Config " + arg + " -->\n<!-- /docgen:config -->\n",
			})
			if rc, log := docgen(t, root, "check"); rc != 1 || !strings.Contains(log, want) {
				t.Errorf("rc %d, log:\n%s", rc, log)
			}
		})
	}
}

func TestConfigPrintNameWithTwoConstants(t *testing.T) {
	t.Parallel()
	root := pageRepo(t, map[string]string{
		"p/p.go":    "package p\n\n// L is a level.\ntype L int\n\nconst (\n\tX L = 1\n\tY L = 1\n)\n\n// C holds settings.\ntype C struct{ A L }\n\n// New applies the defaults.\nfunc New(c C) C {\n\tif c.A == 0 {\n\t\tc.A = X\n\t}\n\treturn c\n}\n",
		"docs/p.md": "<!-- docgen:config pkg=p type=C print=name -->\n<!-- /docgen:config -->\n",
	})
	if rc, log := docgen(t, root, "check"); rc != 1 || !strings.Contains(log, "several constants of type") {
		t.Errorf("rc %d, log:\n%s", rc, log)
	}
}

func TestPrintSetting(t *testing.T) {
	t.Parallel()
	for name, c := range map[string]struct {
		yaml, args, want string
		rc               int
	}{
		"default":                 {"", "name=Main", "", 1},
		"go":                      {"print: go\n", "name=Main", "", 1},
		"raw":                     {"print: raw\n", "name=Main", "`4001`", 0},
		"marker print=go":         {"print: raw\n", "name=B print=go", "`2`", 0},
		"marker print=name":       {"print: raw\n", "name=Main print=name", "`Main`", 0},
		"marker verb":             {"print: raw\n", "name=Wait print=%v", "`1.5s`", 0},
		"marker raw over go":      {"print: go\n", "name=Main print=raw", "`4001`", 0},
		"marker print=go on code": {"print: raw\n", "name=Main print=go", "", 1},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := pageRepo(t, map[string]string{"p/p.go": printSource, "docs/p.md": constPage(c.args), ".docgen.yaml": c.yaml})
			rc, log := docgen(t, root, "update")
			if rc != c.rc {
				t.Fatalf("rc %d, log:\n%s", rc, log)
			}
			if rc == 1 {
				wantIn(t, log, "String ran")
				return
			}
			wantIn(t, readPage(t, root, "docs/p.md"), "-->"+c.want+"<!--")
		})
	}
}

func TestPrintSettingValues(t *testing.T) {
	t.Parallel()
	for _, value := range []string{"name", "\"%d\"", "RAW", "Config", "\"\""} {
		root := pageRepo(t, map[string]string{".docgen.yaml": "print: " + value + "\n"})
		if rc, log := docgen(t, root, "check"); rc != 1 || !strings.Contains(log, "it must be go, raw or config") {
			t.Errorf("%s: rc %d, log:\n%s", value, rc, log)
		}
	}
}

func TestPrintHasNoFlag(t *testing.T) {
	t.Parallel()
	root := pageRepo(t, nil)
	if rc, _ := docgen(t, root, "check", "-print", "raw"); rc != 2 {
		t.Errorf("rc %d, want 2", rc)
	}
}

// TestPrintVariable sets a variable, so it does not run in parallel.
//
//nolint:paralleltest // t.Setenv
func TestPrintVariable(t *testing.T) {
	root := pageRepo(t, map[string]string{"p/p.go": printSource, "docs/p.md": constPage("name=Main"), ".docgen.yaml": "print: go\n"})
	t.Setenv("DOCGEN_PRINT", "raw")
	if rc, log := docgen(t, root, "update"); rc != 0 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
	wantIn(t, readPage(t, root, "docs/p.md"), "-->`4001`<!--")
}

func TestPrintRawRunsNoGoTest(t *testing.T) {
	t.Parallel()
	files := map[string]string{
		"p/p.go":      configSource,
		"p/p_test.go": "package p\n\nfunc broken() { undefined() }\n",
		"docs/p.md":   constPage("name=DefaultPort") + "\n<!-- docgen:config pkg=p type=Config -->\n<!-- /docgen:config -->\n",
	}
	root := pageRepo(t, files)
	if rc, log := docgen(t, root, "update"); rc != 1 || !strings.Contains(log, "printing the values of p") {
		t.Fatalf("print: go: rc %d, log:\n%s", rc, log)
	}
	writeTree(t, root, map[string]string{".docgen.yaml": "print: raw\n"})
	rc, log := docgen(t, root, "update")
	if rc != 0 {
		t.Fatalf("print: raw: rc %d, log:\n%s", rc, log)
	}
	wantNotIn(t, log, "printing")
	wantIn(t, readPage(t, root, "docs/p.md"), "-->`4001`<!--", "| `DefaultPort` (4001) |")
}

func TestPrintInANestedRun(t *testing.T) {
	t.Parallel()
	marker := nestedMarker("root=testdata/sample page=docs/a.md")
	sample := map[string]string{
		"go.mod":    sampleMod,
		"p/p.go":    configSource,
		"docs/a.md": constPage("name=DefaultPort"),
	}
	for name, c := range map[string]struct{ outer, inner, want string }{
		"go":        {"", "", "`port-4001`"},
		"sample":    {"", "print: raw\n", "`4001`"},
		"outer":     {"print: raw\n", "", "`4001`"},
		"raw stays": {"print: raw\n", "print: go\n", "`4001`"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := outerRepo(t, marker, sample)
			writeTree(t, root, map[string]string{".docgen.yaml": c.outer, "testdata/sample/.docgen.yaml": c.inner})
			if rc, log := docgen(t, root, "update"); rc != 0 {
				t.Fatalf("rc %d, log:\n%s", rc, log)
			}
			wantIn(t, page(t, root, "p.md"), "-->"+c.want+"<!--")
		})
	}
}

// configTextSource is a package with defaults of a type with a MarshalText
// method, of an enum with a String method and of durations.
const configTextSource = `package p

import (
	"errors"
	"time"
)

// Slot has a String method.
type Slot int

func (s Slot) String() string { return "TS" + string(rune('0'+s)) }

// The slots.
const (
	SlotOne Slot = iota
	SlotTwo
)

// Scene has a String method and a MarshalText method.
type Scene uint8

func (Scene) String() string { return "scene" }

// MarshalText returns the name of the scene.
func (s Scene) MarshalText() ([]byte, error) { return []byte("evening-" + string(rune('0'+s))), nil }

// Evening is a scene.
const Evening Scene = 3

// Mode has a MarshalText method on the pointer.
type Mode string

// MarshalText returns the mode in upper case.
func (m *Mode) MarshalText() ([]byte, error) { return []byte("MODE-" + string(*m)), nil }

// Ticks is a defined type of time.Duration.
type Ticks time.Duration

// Bad has a MarshalText method that returns an error.
type Bad int

// MarshalText returns an error.
func (Bad) MarshalText() ([]byte, error) { return nil, errors.New("no text") }

// Boom has a MarshalText method that panics.
type Boom int

// MarshalText panics.
func (Boom) MarshalText() ([]byte, error) { panic("boom") }

// Config holds settings.
type Config struct {
	Lane  Slot
	View  Scene
	Kind  Mode
	Wait  time.Duration
	Hold  time.Duration
	Tick  Ticks
	Name  string
	Size  int
}

// New applies the defaults.
func New(c Config) Config {
	if c.Lane == 0 {
		c.Lane = SlotTwo
	}
	if c.View == 0 {
		c.View = Evening
	}
	if c.Kind == "" {
		c.Kind = "fast"
	}
	if c.Wait == 0 {
		c.Wait = 5 * time.Second
	}
	if c.Hold == 0 {
		c.Hold = 1500000000
	}
	if c.Tick == 0 {
		c.Tick = Ticks(2 * time.Second)
	}
	if c.Name == "" {
		c.Name = "x"
	}
	if c.Size == 0 {
		c.Size = 0x10
	}
	return c
}

// B is a Bad.
const B Bad = 1

// X is a Boom.
const X Boom = 1
`

func TestConfigPrintConfig(t *testing.T) {
	t.Parallel()
	for name, c := range map[string]struct {
		marker, yaml string
		want         []string
	}{
		"config": {" print=config", "", []string{
			"| `Lane` | `Slot` | `SlotTwo` (1) |",
			"| `View` | `Scene` | `Evening` (evening-3) |",
			"| `Kind` | `Mode` | `\"fast\"` (MODE-fast) |",
			"| `Wait` | `time.Duration` | `5 * time.Second` (5s) |",
			"| `Hold` | `time.Duration` | `1500000000` (1.5s) |",
			"| `Tick` | `Ticks` | `Ticks(2 * time.Second)` (2000000000) |",
			"| `Name` | `string` | `\"x\"` |",
			"| `Size` | `int` | `0x10` |",
		}},
		"setting": {"", "print: config\n", []string{"| `SlotTwo` (1) |", "| `Evening` (evening-3) |", "| `5 * time.Second` (5s) |"}},
		"go":      {" print=go", "print: config\n", []string{"| `SlotTwo` (TS1) |", "| `Evening` (scene) |", "| `5 * time.Second` (5s) |"}},
		"raw":     {" print=raw", "", []string{"| `SlotTwo` (1) |", "| `Evening` (3) |", "| `\"fast\"` |", "| `5 * time.Second` (5000000000) |"}},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := pageRepo(t, map[string]string{
				"p/p.go":       configTextSource,
				".docgen.yaml": c.yaml,
				"docs/p.md":    "<!-- docgen:config pkg=p type=Config" + c.marker + " -->\n<!-- /docgen:config -->\n",
			})
			wantIn(t, updateTwice(t, root, "docs/p.md"), c.want...)
		})
	}
}

func TestConstPrintConfig(t *testing.T) {
	t.Parallel()
	root := pageRepo(t, map[string]string{
		"p/p.go":    configTextSource,
		"docs/p.md": constPage("name=SlotTwo print=config", "name=Evening print=config", "name=Evening print=go"),
	})
	wantIn(t, updateTwice(t, root, "docs/p.md"), "-->`1`<!--", "-->`evening-3`<!--", "-->`scene`<!--")
}

func TestPrintConfigRunsNoTestWithNoMarshalText(t *testing.T) {
	t.Parallel()
	root := pageRepo(t, map[string]string{
		"p/p.go":      configSource,
		"p/p_test.go": "package p\n\nfunc broken() { undefined() }\n",
		"docs/p.md":   "<!-- docgen:config pkg=p type=Config print=config -->\n<!-- /docgen:config -->\n",
	})
	rc, log := docgen(t, root, "update")
	if rc != 0 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
	wantNotIn(t, log, "printing")
	wantIn(t, readPage(t, root, "docs/p.md"), "| `DefaultPort` (4001) |", "| `DefaultWait` (1.5s) |")
}

func TestPrintConfigErrors(t *testing.T) {
	t.Parallel()
	for name, want := range map[string]string{
		"B": "the value has no text: %!config(ERROR=no text)",
		"X": "the value has no text: %!config(PANIC=MarshalText method: boom)",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := pageRepo(t, map[string]string{"p/p.go": configTextSource, "docs/p.md": constPage("name=" + name + " print=config")})
			if rc, log := docgen(t, root, "update"); rc != 1 || !strings.Contains(log, want) {
				t.Errorf("rc %d, log:\n%s", rc, log)
			}
		})
	}
}

func TestFixturePrintConfig(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	copyFixture(t, root)
	if rc, log := docgen(t, root, "update", "-module", "v2"); rc != 0 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
	wantIn(t, readPage(t, root, "docs/dimmer/config-config.md"),
		"| `Mode` | `Mode` | `ModeFade` (1) | Mode is how the level changes. |",
		"| `Scene` | `Scene` | `SceneEvening` (evening) | Scene is the scene at start. |",
		"| `Hold` | `time.Duration` | `5 * time.Second` (5s) | Hold is how long a scene stays. |",
	)
}
