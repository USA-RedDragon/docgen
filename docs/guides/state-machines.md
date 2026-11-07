# Document a state machine

Add a state table, a transition table and an SVG diagram of a state machine. They come from the calls that your tests make.

<!-- site:editor -->
<!-- docgen:code file=internal/cli/testdata/repo/v2/machine/machine.go symbol=LampState -->
```go
type LampState uint8
```
<!-- /docgen:code -->
<!-- docgen:code file=internal/cli/testdata/repo/v2/machine/machine.go symbol=LampOff -->
```go
const (
	// LampOff gives no light.
	LampOff LampState = iota
	// LampOn gives light.
	LampOn
	// LampBroken never gives light again.
	LampBroken
)
```
<!-- /docgen:code -->
<!-- docgen:code file=internal/cli/testdata/repo/v2/machine/machine.go symbol=Lamp.State -->
```go
func (l *Lamp) State() LampState { return l.state }
```
<!-- /docgen:code -->
<!-- docgen:code file=internal/cli/testdata/repo/v2/machine/machine.go symbol=Lamp.Start -->
```go
func (l *Lamp) Start(now time.Duration) error {
	if l.state == LampOn {
		return ErrBusy
	}
	if l.state == LampOff {
		l.state, l.until = LampOn, now+l.wait
	}
	return nil
}
```
<!-- /docgen:code -->
<!-- docgen:code file=internal/cli/testdata/repo/v2/machine/machine.go symbol=Lamp.Stop -->
```go
func (l *Lamp) Stop() error {
	if l.state != LampOn {
		return ErrOff
	}
	l.off()
	return nil
}
```
<!-- /docgen:code -->
<!-- docgen:code file=internal/cli/testdata/repo/v2/machine/machine.go symbol=Lamp.Tick -->
```go
func (l *Lamp) Tick(now time.Duration) {
	if l.state == LampOn && now >= l.until {
		l.off()
	}
}
```
<!-- /docgen:code -->
<!-- docgen:code file=internal/cli/testdata/repo/v2/machine/machine.go symbol=Lamp.Break -->
```go
func (l *Lamp) Break() { l.state = LampBroken }
```
<!-- /docgen:code -->
<!-- docgen:code file=internal/cli/testdata/repo/v2/machine/machine.go symbol=Lamp.off -->
```go
func (l *Lamp) off() { l.state = LampOff }
```
<!-- /docgen:code -->
<!-- docgen:code file=internal/cli/testdata/repo/v2/machine/machine_test.go symbol=TestLamp -->
```go
func TestLamp(t *testing.T) {
	l, err := NewLamp(Config{})
	if err != nil {
		t.Fatal(err)
	}
	_ = l.Start(0)
	l.Tick(DefaultWait)
	_ = l.Start(0)
	_ = l.Stop()
	l.Break()
}
```
<!-- /docgen:code -->
<!-- docgen:docgen root=internal/cli/testdata/repo module=v2 page=docs/machines/guide-states.md -->
#### Before

```
# The lamp

<!-- docgen:states pkg=machine type=Lamp -->
<!-- /docgen:states -->
```

#### After

```
# The lamp

<!-- docgen:states pkg=machine type=Lamp -->
States of [`Lamp`](https://pkg.go.dev/github.com/USA-RedDragon/docgen/v2/machine#Lamp), from [`Lamp.State`](https://pkg.go.dev/github.com/USA-RedDragon/docgen/v2/machine#Lamp.State):

| State | Meaning | Seen in tests |
|---|---|---|
| [`LampOff`](https://pkg.go.dev/github.com/USA-RedDragon/docgen/v2/machine#LampOff) | LampOff gives no light. | yes, first state |
| [`LampOn`](https://pkg.go.dev/github.com/USA-RedDragon/docgen/v2/machine#LampOn) | LampOn gives light. | yes |
| [`LampBroken`](https://pkg.go.dev/github.com/USA-RedDragon/docgen/v2/machine#LampBroken) | LampBroken never gives light again. | yes |

Transitions seen in tests:

| From | Method call | To |
|---|---|---|
| LampOff | `Start` | LampOn |
| LampOff | `Break` | LampBroken |
| LampOn | `Stop`, `Tick` | LampOff |

![Transitions of Lamp](states/machine-lamp.svg)
<!-- /docgen:states -->
```
<!-- /docgen:docgen -->
<!-- /site:editor -->

For all options, see [`docgen:states`](../reference/markers.md#docgenstates).

Commit the diagram with the page. It goes in `states/<pkg>-<type>.svg` next to the page. For another place, add `diagram=<path>`, relative to the page.

## Requirements

- A type with exported methods that have pointer receivers. Generic types are not supported.
- A way to read the state: a `State` method, or methods that return a `bool` or a `(value, bool)`.
- Tests that drive the machine.

## When a transition is missing

A transition that no test makes is not in the table. Add a test. The state table lists each state that no test reached, so a gap is easy to see.

## Slow tests

Add `run=<regexp>` to limit the tests, as `go test -run` does. It applies only when each `docgen:states` marker of the package has one.

<!-- site:editor -->
<!-- docgen:code file=internal/cli/testdata/repo/v2/door/door.go -->
```go
// Package door is a state machine with its own tests, for the run= argument.
package door

// State is the state of a Door.
type State uint8

// The State values.
const (
	// StateClosed is shut.
	StateClosed State = iota
	// StateOpen is open.
	StateOpen
)

// Door opens and closes.
type Door struct{ state State }

// State returns the state of the Door.
func (d *Door) State() State { return d.state }

// Open opens the Door.
func (d *Door) Open() { d.state = StateOpen }

// Close closes the Door.
func (d *Door) Close() { d.state = StateClosed }
```
<!-- /docgen:code -->
<!-- docgen:code file=internal/cli/testdata/repo/v2/door/door_test.go symbol=TestOpen -->
```go
func TestOpen(t *testing.T) {
	var d Door
	d.Open()
}
```
<!-- /docgen:code -->
<!-- docgen:docgen root=internal/cli/testdata/repo module=v2 page=docs/machines/guide-states-run.md -->
#### Before

```
<!-- docgen:states pkg=door type=Door run=TestOpen diagram=states/door-open.svg -->
<!-- /docgen:states -->
```

#### After

```
<!-- docgen:states pkg=door type=Door run=TestOpen diagram=states/door-open.svg -->
States of [`Door`](https://pkg.go.dev/github.com/USA-RedDragon/docgen/v2/door#Door), from [`Door.State`](https://pkg.go.dev/github.com/USA-RedDragon/docgen/v2/door#Door.State):

| State | Meaning | Seen in tests |
|---|---|---|
| [`StateClosed`](https://pkg.go.dev/github.com/USA-RedDragon/docgen/v2/door#StateClosed) | StateClosed is shut. | yes, first state |
| [`StateOpen`](https://pkg.go.dev/github.com/USA-RedDragon/docgen/v2/door#StateOpen) | StateOpen is open. | yes |

Transitions seen in tests:

| From | Method call | To |
|---|---|---|
| StateClosed | `Open` | StateOpen |

![Transitions of Door](states/door-open.svg)
<!-- /docgen:states -->
```
<!-- /docgen:docgen -->
<!-- /site:editor -->

For the exact rules, see [How docgen works](../concepts/how-it-works.md#states-and-transitions).
