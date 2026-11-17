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

## Rust

For a Rust crate, give the package name with `crate=` and the path of the machine type with `type=`. The state is the variant of an enum. Here a `state` method returns it:

<!-- site:editor -->
<!-- docgen:code file=internal/cli/testdata/rust/crates/states-lamp/src/lib.rs symbol=LampState -->
```rust
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum LampState {
    /// Gives no light.
    Off,
    /// Gives light until the wait ends.
    On,
    /// Never gives light again.
    Broken,
}
```
<!-- /docgen:code -->
<!-- docgen:code file=internal/cli/testdata/rust/crates/states-lamp/src/lib.rs symbol=Lamp::new -->
```rust
pub fn new(wait: u64) -> Self {
    Lamp {
        state: LampState::Off,
        until: 0,
        wait,
        name: String::from("lamp"),
    }
}
```
<!-- /docgen:code -->
<!-- docgen:code file=internal/cli/testdata/rust/crates/states-lamp/src/lib.rs symbol=Lamp::state -->
```rust
pub fn state(&self) -> LampState {
    self.state
}
```
<!-- /docgen:code -->
<!-- docgen:code file=internal/cli/testdata/rust/crates/states-lamp/src/lib.rs symbol=Lamp::start -->
```rust
pub fn start(&mut self, now: u64) -> Result<(), LampError> {
    if self.state == LampState::On {
        return Err(LampError::Busy);
    }
    if self.state == LampState::Off {
        self.state = LampState::On;
        self.until = now + self.wait;
    }
    Ok(())
}
```
<!-- /docgen:code -->
<!-- docgen:code file=internal/cli/testdata/rust/crates/states-lamp/src/lib.rs symbol=Lamp::name_mut -->
```rust
pub fn name_mut(&mut self) -> &mut String {
    &mut self.name
}
```
<!-- /docgen:code -->
<!-- docgen:code file=internal/cli/testdata/rust/crates/states-lamp/src/lib.rs symbol=tests::lamp -->
```rust
#[test]
fn lamp() {
    let mut l = Lamp::new(3);
    assert_eq!(l.start(0), Ok(()));
    l.tick(3);
    assert_eq!(l.start(0), Ok(()));
    assert_eq!(l.stop(), Ok(()));
    assert_eq!(l.stop(), Err(LampError::Off));
    l.name_mut().push('!');
}
```
<!-- /docgen:code -->
<!-- docgen:docgen root=internal/cli/testdata/rust page=docs/guide-states.md -->
#### Before

```
# The lamp

<!-- docgen:states crate=states-lamp type=Lamp -->
<!-- /docgen:states -->
```

#### After

```
# The lamp

<!-- docgen:states crate=states-lamp type=Lamp -->
States of `Lamp`, from `Lamp::state`:

| State | Meaning | Seen in tests |
|---|---|---|
| `LampState::Off` | Gives no light. | yes, first state |
| `LampState::On` | Gives light until the wait ends. | yes |
| `LampState::Broken` | Never gives light again. | yes |

Transitions seen in tests:

| From | Method call | To |
|---|---|---|
| Off | `start` | On |
| Off | `smash` | Broken |
| On | `stop`, `tick` | Off |

![Transitions of Lamp](states/states-lamp-lamp.svg)

Not recorded (returns a borrow): name_mut.
<!-- /docgen:states -->
```
<!-- /docgen:docgen -->
<!-- /site:editor -->

`new` returns the lamp, so `Off` is its first state. `smash` is called only in an integration test in `tests/`, and it counts, as do doctests. `name_mut` returns a borrow of the lamp, and the state cannot be read while that borrow lives, so the table lists it under Not recorded.

An enum can be the machine itself. A method with `self` that returns `Self` is a transition to the value that it returns, and a parameter whose type is an enum of the crate names its variant in the call:

<!-- site:editor -->
<!-- docgen:code file=internal/cli/testdata/rust/crates/states-door/src/lib.rs symbol=Door -->
```rust
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum Door {
    /// Shut and not locked.
    Closed,
    /// Open.
    Open,
    /// Shut and locked with a code.
    Locked {
        /// The code that unlocks the door.
        code: u16,
    },
}
```
<!-- /docgen:code -->
<!-- docgen:code file=internal/cli/testdata/rust/crates/states-door/src/lib.rs symbol=Door::on -->
```rust
pub fn on(self, event: Event) -> Self {
    match (self, event) {
        (Door::Closed, Event::Push) => Door::Open,
        (Door::Open, Event::Pull) => Door::Closed,
        (Door::Closed, Event::Lock(code)) => Door::Locked { code },
        (Door::Locked { code }, Event::Unlock(c)) if c == code => Door::Closed,
        (door, _) => door,
    }
}
```
<!-- /docgen:code -->
<!-- docgen:code file=internal/cli/testdata/rust/crates/states-door/src/lib.rs symbol=tests::events -->
```rust
#[test]
fn events() {
    let d = Door::Closed.on(Event::Push).on(Event::Pull);
    let d = d.on(Event::Lock(7)).on(Event::Unlock(7));
    assert_eq!(d, Door::Closed);
}
```
<!-- /docgen:code -->
<!-- docgen:docgen root=internal/cli/testdata/rust page=docs/guide-states-enum.md -->
#### Before

```
<!-- docgen:states crate=states-door type=Door run=events diagram=states/door-events.svg -->
<!-- /docgen:states -->
```

#### After

```
<!-- docgen:states crate=states-door type=Door run=events diagram=states/door-events.svg -->
States of `Door`, from its variant:

| State | Meaning | Seen in tests |
|---|---|---|
| `Door::Closed` | Shut and not locked. | yes |
| `Door::Open` | Open. | yes |
| `Door::Locked { .. }` | Shut and locked with a code. | yes |

Transitions seen in tests:

| From | Method call | To |
|---|---|---|
| Closed | `on(Push)` | Open |
| Closed | `on(Lock)` | Locked |
| Open | `on(Pull)` | Closed |
| Locked | `on(Unlock)` | Closed |

![Transitions of Door](states/door-events.svg)
<!-- /docgen:states -->
```
<!-- /docgen:docgen -->
<!-- /site:editor -->

The tests build `Door` from a literal, which no constructor returns, so no state is marked as first. `run=` is a cargo test filter: a test runs when its name holds the text.

### Requirements in Rust

- An enum, or a struct with a `state(&self)` method or one field whose type is an enum of the crate. Generic types are not supported.
- Pub methods with `&mut self`, or with `self` that return `Self`, an `Option<Self>` or a `Result<Self, E>`.
- Tests that drive the machine, in the crate or in its `tests/` directory. docgen adds the hook to a copy of the workspace, so the crate needs no dependency on docgen.

For the exact rules, see [How docgen works](../concepts/how-it-works.md#states-and-transitions).
