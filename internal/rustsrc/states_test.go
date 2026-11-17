package rustsrc

import (
	"slices"
	"strings"
	"testing"
)

func TestShapeOf(t *testing.T) {
	t.Parallel()
	m := &Machine{Type: &Item{Name: "Lamp"}}
	for ret, want := range map[string]int{
		"Self":                               shapePlain,
		"Lamp":                               shapePlain,
		"Option<Self>":                       shapeOption,
		"std::option::Option<Lamp>":          shapeOption,
		"Result<Self, Error>":                shapeResult,
		"io::Result<Self>":                   shapeResult,
		"Result < Self , Self >":             shapeResult,
		"":                                   shapeNone,
		"u8":                                 shapeNone,
		"Option<u8>":                         shapeNone,
		"Vec<Self>":                          shapeNone,
		"Result<(Self, u8), Error>":          shapeNone,
		"core::result::Result<Lamp, String>": shapeResult,
	} {
		if got := m.shapeOf(ret); got != want {
			t.Errorf("shapeOf(%q) = %d, want %d", ret, got, want)
		}
	}
}

func TestApplyEdits(t *testing.T) {
	t.Parallel()
	text := "fn a() { x }\n// end"
	got := applyEdits(text, []edit{
		{offset: len(text), text: " impl A {}"},
		{offset: 8, text: " pre"},
		{offset: 11, text: "post "},
		{offset: len(text), text: "\nmod m {}\n"},
	})
	want := "fn a() { pre x post }\n// end\n impl A {}\nmod m {}\n"
	if got != want {
		t.Errorf("applyEdits:\n%q\nwant:\n%q", got, want)
	}
	if got := applyEdits("x\n", []edit{{offset: 2, text: " y"}}); got != "x\n y" {
		t.Errorf("applyEdits with a newline at the end: %q", got)
	}
}

func TestVariantFn(t *testing.T) {
	t.Parallel()
	e := &Item{Name: "Door", Variants: []Variant{{Name: "Closed"}, {Name: "Locked", Style: "named"}, {Name: "Ajar", Cfg: []string{`feature = "ajar"`}}}}
	got := variantFn(e)
	for _, want := range []string{
		"impl Door {",
		`Self::Closed { .. } => "Closed",`,
		`Self::Locked { .. } => "Locked",`,
		`#[cfg(feature = "ajar")] Self::Ajar { .. } => "Ajar",`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("variantFn lacks %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "\n") {
		t.Errorf("variantFn has a newline: %q", got)
	}
}

func hookNames(m *Machine) []string {
	out := make([]string, 0, len(m.hooks))
	for _, h := range m.hooks {
		out = append(out, h.name)
	}
	return out
}

func TestNewMachine(t *testing.T) {
	t.Parallel()
	crates := readFixture(t, "crates/states-lamp/src/lib.rs", "crates/states-door/src/lib.rs", "crates/states-odd/src/lib.rs")
	lamp, door, odd := crates[0], crates[1], crates[2]

	m, err := NewMachine(lamp, "states-lamp", "Lamp")
	if err != nil {
		t.Fatal(err)
	}
	if m.From != StateMethod || m.Via != "state" || m.EnumPath != "LampState" {
		t.Errorf("Lamp: from %d via %q enum %q", m.From, m.Via, m.EnumPath)
	}
	if got := hookNames(m); !slices.Equal(got, []string{"new", "start", "stop", "tick", "smash"}) {
		t.Errorf("Lamp hooks: %v", got)
	}
	if got := m.Skipped[SkipBorrow]; !slices.Equal(got, []string{"name_mut"}) {
		t.Errorf("Lamp skipped: %v", m.Skipped)
	}

	m, err = NewMachine(door, "states-door", "crate::Door")
	if err != nil {
		t.Fatal(err)
	}
	if m.From != StateSelf || m.Path != "Door" || m.Calls() != 3 {
		t.Errorf("Door: from %d path %q calls %d", m.From, m.Path, m.Calls())
	}
	if h := m.hooks[0]; h.name != "on" || !slices.Equal(h.labels, []string{"event"}) || h.shape != shapePlain {
		t.Errorf("Door::on hook: %+v", h)
	}

	m, err = NewMachine(odd, "states-odd", "Worker")
	if err != nil {
		t.Fatal(err)
	}
	if m.From != StateField || m.Via != "mode" {
		t.Errorf("Worker: from %d via %q", m.From, m.Via)
	}
	if got := hookNames(m); !slices.Equal(got, []string{"new", "busy", "start", "start_view", "stop"}) {
		t.Errorf("Worker hooks: %v", got)
	}
	if got := m.Skipped[SkipAsync]; !slices.Equal(got, []string{"stop_later"}) {
		t.Errorf("Worker skipped: %v", m.Skipped)
	}

	for name, want := range map[string]string{
		"Twin":  "struct Twin has no method state(&self) and 2 fields whose type is an enum of crate states-odd (a, b)",
		"Count": `Count::state is not a fn state(&self) that returns an enum of crate states-odd with no generics; it returns "u8"`,
		"Boxed": "Boxed is generic",
		"Still": "Still has no pub method with &mut self, or with self that returns Still, to record",
		"View":  "View is generic",
		"tests": "tests is a module, not an enum or a struct",
		"Nope":  "crate states-odd has no item Nope",
	} {
		if _, err := NewMachine(odd, "states-odd", name); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: error %v, want %q", name, err, want)
		}
	}
}

func TestResolve(t *testing.T) {
	t.Parallel()
	c := &Crate{Modules: []*Module{
		{Path: "", Items: []*Item{{Kind: KindEnum, Name: "Top"}}, Uses: []Use{{Name: "Inner", Path: []string{"a", "Inner"}}}},
		{Path: "a", Items: []*Item{{Kind: KindEnum, Name: "Inner"}}, Uses: []Use{{Name: "Top", Path: []string{"super", "Top"}}}},
	}}
	c.index()
	a := c.Module("a")
	for p, want := range map[string]string{
		"Top":           "Top",
		"crate::Top":    "Top",
		"super::Top":    "Top",
		"Inner":         "a::Inner",
		"self::Inner":   "a::Inner",
		"crate::Inner":  "a::Inner",
		"Vec<Top>":      "",
		"&Top":          "",
		"std::cmp::Ord": "",
	} {
		f, ok := c.Resolve(a, p)
		if got := map[bool]string{true: f.Path}[ok]; got != want {
			t.Errorf("Resolve(a, %q) = %q, want %q", p, got, want)
		}
	}
}
