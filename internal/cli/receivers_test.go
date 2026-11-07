package cli

import (
	"strings"
	"testing"
)

const receiverStates = `package p

// S is a state.
type S int

const (
	// Off gives no light.
	Off S = iota
	// On gives light.
	On
)
`

const statesOfM = "<!-- docgen:states pkg=p type=M -->\n<!-- /docgen:states -->\n"

func TestValueReceiverMethods(t *testing.T) {
	t.Parallel()
	got := fill(t, map[string]string{
		"p/p.go":      receiverStates + "\n// M is a machine.\ntype M struct{ s *S }\n\n// State returns the state.\nfunc (m M) State() S { return *m.s }\n\n// Go moves the machine.\nfunc (m M) Go() { *m.s = On }\n\n// Back moves it back.\nfunc (m *M) Back() { *m.s = Off }\n",
		"p/p_test.go": "package p\n\nimport \"testing\"\n\nfunc TestM(t *testing.T) { s := On; m := M{&s}; m.Go(); m.Back() }\n",
	}, statesOfM)
	wantIn(t, got, "| [`On`]", "`Back`", "Not recorded (value receiver): Go.\n")
}

func TestOnlyValueReceiverMethods(t *testing.T) {
	t.Parallel()
	got := fill(t, map[string]string{
		"p/p.go":      receiverStates + "\n// M is a machine.\ntype M struct{ s *S }\n\n// State returns the state.\nfunc (m M) State() S { return *m.s }\n\n// Go moves the machine.\nfunc (m M) Go() { *m.s = On }\n",
		"p/p_test.go": "package p\n\nimport \"testing\"\n\nfunc TestM(t *testing.T) { s := Off; m := M{&s}; m.Go() }\n",
	}, statesOfM)
	wantIn(t, got, "Not recorded (value receiver): Go.\n")
}

func TestNoTransitionWithAllMethodsRecorded(t *testing.T) {
	t.Parallel()
	root := writeRepo(t, map[string]string{
		"go.mod":      pageModule,
		"p/p.go":      receiverStates + "\n// M is a machine.\ntype M struct{ s S }\n\n// State returns the state.\nfunc (m *M) State() S { return m.s }\n\n// Stay moves nothing.\nfunc (m *M) Stay() {}\n",
		"p/p_test.go": "package p\n\nimport \"testing\"\n\nfunc TestM(t *testing.T) { (&M{}).Stay() }\n",
		"docs/p.md":   statesOfM,
	})
	rc, log := docgen(t, root, "update")
	if rc != 1 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
	wantIn(t, log, "recorded no transition of M")
}

func TestPromotedMethods(t *testing.T) {
	t.Parallel()
	got := fill(t, map[string]string{
		"p/p.go":      receiverStates + "\n// Base holds the state.\ntype Base struct{ s S }\n\n// Go moves the state.\nfunc (b *Base) Go() { b.s = On }\n\n// M is a machine.\ntype M struct{ Base }\n\n// State returns the state.\nfunc (m *M) State() S { return m.s }\n\n// Back moves it back.\nfunc (m *M) Back() { m.s = Off }\n",
		"p/p_test.go": "package p\n\nimport \"testing\"\n\nfunc TestM(t *testing.T) { m := &M{}; m.Go(); m.Back() }\n",
	}, statesOfM)
	wantIn(t, got, "Not recorded (promoted): Go.\n")
}

func TestStatesOfAnAlias(t *testing.T) {
	t.Parallel()
	got := fill(t, map[string]string{
		"p/p.go":      receiverStates + "\n// Lamp is a machine.\ntype Lamp struct{ s S }\n\n// Alias is another name of Lamp.\ntype Alias = Lamp\n\n// State returns the state.\nfunc (l *Lamp) State() S { return l.s }\n\n// Go moves the machine.\nfunc (l *Lamp) Go() { l.s = On }\n\n// Back moves it back.\nfunc (l *Alias) Back() { l.s = Off }\n",
		"p/p_test.go": "package p\n\nimport \"testing\"\n\nfunc TestL(t *testing.T) { l := &Lamp{}; l.Go(); l.Back() }\n",
	}, "<!-- docgen:states pkg=p type=Alias -->\n<!-- /docgen:states -->\n")
	wantIn(t, got, "`Go`", "`Back`")
	wantNotIn(t, got, "Not recorded")
}

func TestStatesOfAnAliasOfAnotherPackage(t *testing.T) {
	t.Parallel()
	root := writeRepo(t, map[string]string{
		"go.mod":      pageModule,
		"o/o.go":      "package o\n\n// Lamp is a machine.\ntype Lamp struct{ s int }\n\n// State returns the state.\nfunc (l *Lamp) State() int { return l.s }\n\n// Go moves the machine.\nfunc (l *Lamp) Go() { l.s = 1 }\n",
		"p/p.go":      "package p\n\nimport \"example.com/p/o\"\n\n// Alias is another name of o.Lamp.\ntype Alias = o.Lamp\n",
		"p/p_test.go": "package p\n\nimport \"testing\"\n\nfunc TestL(t *testing.T) { (&Alias{}).Go() }\n",
		"docs/p.md":   "<!-- docgen:states pkg=p type=Alias -->\n<!-- /docgen:states -->\n",
	})
	rc, log := docgen(t, root, "update")
	if rc != 1 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
	if !strings.Contains(log, "Alias is an alias of a type of another package") {
		t.Errorf("log:\n%s", log)
	}
}

func TestStateMeaningOfAConversion(t *testing.T) {
	t.Parallel()
	got := fill(t, map[string]string{
		"p/p.go":      "package p\n\n// State is a state.\ntype State int\n\nconst (\n\t// Idle waits.\n\tIdle = State(0)\n\t// Busy works.\n\tBusy = State(1)\n)\n\n// M is a machine.\ntype M struct{ s State }\n\n// State returns the state.\nfunc (m *M) State() State { return m.s }\n\n// Go moves the machine.\nfunc (m *M) Go() { m.s = Busy }\n",
		"p/p_test.go": "package p\n\nimport \"testing\"\n\nfunc TestM(t *testing.T) { (&M{}).Go() }\n",
	}, statesOfM)
	wantIn(t, got, "Idle waits.", "Busy works.")
}

func TestObserversWithOnlyValueReceiverMethods(t *testing.T) {
	t.Parallel()
	got := fill(t, map[string]string{
		"p/p.go":      "package p\n\n// M is a machine.\ntype M struct{ on *bool }\n\n// On reports the state.\nfunc (m M) On() bool { return *m.on }\n\n// Go moves the machine.\nfunc (m M) Go() { *m.on = true }\n",
		"p/p_test.go": "package p\n\nimport \"testing\"\n\nfunc TestM(t *testing.T) { b := false; m := M{&b}; m.Go() }\n",
	}, statesOfM)
	wantIn(t, got, "Not recorded (value receiver): Go.\n")
}

func TestStateTypeWithAMachineAlias(t *testing.T) {
	t.Parallel()
	got := fill(t, map[string]string{
		"p/p.go":      receiverStates + "\n// Lamp is a machine.\ntype Lamp struct{ s S }\n\n// Alias is another name of Lamp.\ntype Alias = Lamp\n\n// Now returns the state.\nfunc (l *Lamp) Now() S { return l.s }\n\n// Go moves the machine.\nfunc (l *Lamp) Go() { l.s = On }\n",
		"p/p_test.go": "package p\n\nimport \"testing\"\n\nfunc TestL(t *testing.T) { (&Lamp{}).Go() }\n",
	}, "<!-- docgen:states pkg=p type=S -->\n<!-- /docgen:states -->\n")
	wantIn(t, got, "`Go`", "Lamp.Now")
}
