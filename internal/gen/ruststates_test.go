package gen

import (
	"strings"
	"testing"
)

func TestRustStates(t *testing.T) {
	t.Parallel()
	page := "<!-- docgen:states crate=states-lamp type=Lamp -->\n<!-- /docgen:states -->\n\n" +
		"<!-- docgen:states crate=states-door type=Door run=open_close diagram=door.svg -->\n<!-- /docgen:states -->\n\n" +
		"<!-- docgen:states crate=states-odd type=Worker -->\n<!-- /docgen:states -->\n"
	root, err := rustRunRoot(t, page)
	if err != nil {
		t.Fatal(err)
	}
	got := read(t, root, "docs/p.md")
	lamp := "<!-- docgen:states crate=states-lamp type=Lamp -->\n" +
		"States of `Lamp`, from `Lamp::state`:\n\n" +
		"| State | Meaning | Seen in tests |\n|---|---|---|\n" +
		"| `LampState::Off` | Gives no light. | yes, first state |\n" +
		"| `LampState::On` | Gives light until the wait ends. | yes |\n" +
		"| `LampState::Broken` | Never gives light again. | yes |\n\n" +
		"Transitions seen in tests:\n\n| From | Method call | To |\n|---|---|---|\n" +
		"| Off | `start` | On |\n| Off | `smash` | Broken |\n| On | `stop`, `tick` | Off |\n\n" +
		"![Transitions of Lamp](states/states-lamp-lamp.svg)\n\n" +
		"Not recorded (returns a borrow): name_mut.\n<!-- /docgen:states -->\n"
	door := "States of `Door`, from its variant:\n\n" +
		"| State | Meaning | Seen in tests |\n|---|---|---|\n" +
		"| `Door::Closed` | Shut and not locked. | yes |\n" +
		"| `Door::Open` | Open. | yes |\n" +
		"| `Door::Locked { .. }` | Shut and locked with a code. | no |\n\n" +
		"Transitions seen in tests:\n\n| From | Method call | To |\n|---|---|---|\n" +
		"| Closed | `open` | Open |\n| Open | `close` | Closed |\n\n" +
		"![Transitions of Door](door.svg)\n"
	worker := "| `Mode::Idle` | Idle. | yes, first state |\n" +
		"| `Mode::Busy` | Busy. | yes, first state |\n"
	for _, want := range []string{lamp, door, worker,
		"| Idle | `start` | Busy |\n| Busy | `stop` | Idle |\n",
		"Not recorded (async): stop_later.\n\nNot recorded (does not build with the hook): start_view.\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("page lacks:\n%s\npage:\n%s", want, got)
		}
	}
	for _, svg := range []string{"docs/states/states-lamp-lamp.svg", "docs/door.svg", "docs/states/states-odd-worker.svg"} {
		data := read(t, root, svg)
		if !strings.HasPrefix(data, "<svg ") {
			t.Errorf("%s: %.60q", svg, data)
		}
	}
	if svg := read(t, root, "docs/states/states-lamp-lamp.svg"); !strings.Contains(svg, ">stop, tick</text>") || !strings.Contains(svg, `stroke-width="3"`) {
		t.Errorf("lamp diagram:\n%s", svg)
	}
}

func TestRustStatesEvents(t *testing.T) {
	t.Parallel()
	root, err := rustRunRoot(t, "<!-- docgen:states crate=states-door type=Door run=events -->\n<!-- /docgen:states -->\n")
	if err != nil {
		t.Fatal(err)
	}
	want := "| Closed | `on(Push)` | Open |\n| Closed | `on(Lock)` | Locked |\n| Open | `on(Pull)` | Closed |\n| Locked | `on(Unlock)` | Closed |\n"
	if got := read(t, root, "docs/p.md"); !strings.Contains(got, want) {
		t.Errorf("page lacks:\n%s\npage:\n%s", want, got)
	}
}

func TestRustStatesErrors(t *testing.T) {
	t.Parallel()
	cases := []struct{ tag, want string }{
		{"crate=states-odd type=Twin", "struct Twin has no method state(&self) and 2 fields whose type is an enum of crate states-odd (a, b)"},
		{"crate=states-odd type=Count", "Count::state is not a fn state(&self) that returns an enum"},
		{"crate=states-odd type=Boxed", "Boxed is generic"},
		{"crate=states-odd type=Still", "Still has no pub method with &mut self"},
		{"crate=states-odd type=Mode::Idle", "crate states-odd has no item Mode::Idle"},
		{"crate=states-odd type=Worker features=fail", "the tests of crate states-odd failed with the state hook:\nfailures:"},
		{"crate=states-odd type=Worker features=forbid", "crate states-odd forbids a lint that the state hook allows in the code that it adds, and a forbid admits no allow, so no method with the hook builds:\n" +
			`  - #![cfg_attr(feature = "forbid", forbid(missing_docs))] at crates/states-odd/src/lib.rs:2 forbids missing_docs` + "\n" +
			"Use deny in place of forbid, such as #![deny(missing_docs)]"},
		{"crate=states-door type=Door run=nomatch", "the tests of crate states-door recorded no transition of Door"},
		{"crate=states-door type=Door features=nope", "nope"},
		{"crate=nope type=Door", "nope"},
	}
	var page strings.Builder
	for _, c := range cases {
		page.WriteString("<!-- docgen:states " + c.tag + " -->\n<!-- /docgen:states -->\n")
	}
	_, err := rustRunRoot(t, page.String())
	if err == nil {
		t.Fatal("no error")
	}
	for _, c := range cases {
		if !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: error lacks %q\n%v", c.tag, c.want, err)
		}
	}
	if strings.Contains(err.Error(), "the worker fails") && !strings.Contains(err.Error(), "panicked at crates/states-odd/src/lib.rs") {
		t.Errorf("the failure has no place:\n%v", err)
	}
}

func TestRustStatesNoCrate(t *testing.T) {
	t.Parallel()
	_, err := rustRunRoot(t, "<!-- docgen:states crate=nope type=Door -->\n<!-- /docgen:states -->\n")
	if err == nil || !strings.Contains(err.Error(), "nope") {
		t.Errorf("error %v", err)
	}
}

func TestRustStatesBind(t *testing.T) {
	t.Parallel()
	for tag, want := range map[string]string{
		"crate=states-door type=Door run=-x":            "run=-x starts with -",
		"crate=states-door type=Door diagram=d.png":     "diagram=d.png does not end in .svg",
		"crate=states-door type=Door diagram=/d.svg":    "is not relative to the page",
		"crate=states-door type=Door pkg=x":             "has both crate= and pkg=",
		"crate=states-door":                             "type",
		"crate=states-door type=Door allfeatures=maybe": "allfeatures= is not true or false",
	} {
		_, err := rustRunRoot(t, "<!-- docgen:states "+tag+" -->\n<!-- /docgen:states -->\n")
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: error %v, want %q", tag, err, want)
		}
	}
}
