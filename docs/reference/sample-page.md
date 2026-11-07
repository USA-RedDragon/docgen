# Sample page

A page with most marker kinds: its empty markers, the same page after `docgen update -module v2`, and how that page renders. It is the test fixture of docgen, so the tests compare this output with the result of each change.

<!-- site:large -->
<!-- site:editor -->
<!-- docgen:code file=internal/cli/testdata/repo/v2/machine/machine.go -->
```go
// Package machine is a small state machine for the docgen tests.
//
// # Rates
//
// A Lamp blinks at these rates:
//   - slow, every [DefaultWait]
//   - fast, every period
//
// # Other
//
// Not copied.
package machine

import (
	"errors"
	"time"

	"github.com/USA-RedDragon/docgen/v2"
)

// DefaultWait is the wait of a Lamp that Config does not set.
const DefaultWait = 250 * time.Millisecond

// MaxTries is the largest Config.Tries.
const MaxTries = 3

var (
	// ErrBusy is returned by Start while the Lamp is on.
	ErrBusy = errors.New("machine: busy")
	// ErrOff is returned by Stop while the Lamp is off.
	ErrOff = errors.New("machine: off")
)

// Config configures NewLamp.
type Config struct {
	// Wait is how long the Lamp stays on. 0 means DefaultWait.
	Wait time.Duration
	// Tries is the number of blinks. 0 means 1, and it is at most MaxTries.
	Tries int
	// Name is a label.
	Name string
}

// LampState is the state of a Lamp.
type LampState uint8

// The LampState values.
const (
	// LampOff gives no light.
	LampOff LampState = iota
	// LampOn gives light.
	LampOn
	// LampBroken never gives light again.
	LampBroken
)

// Lamp turns on and off.
type Lamp struct {
	state LampState
	wait  time.Duration
	tries int
	until time.Duration
}

// NewLamp returns a Lamp. It returns a *docgen.ConfigError for a Tries
// above MaxTries.
func NewLamp(cfg Config) (*Lamp, error) {
	if cfg.Tries > MaxTries {
		return nil, &docgen.ConfigError{Field: "Tries"}
	}
	l := &Lamp{wait: cfg.Wait, tries: cfg.Tries}
	if l.wait == 0 {
		l.wait = DefaultWait
	}
	if cfg.Tries == 0 {
		l.tries = 1
	}
	return l, nil
}

// State returns the state of the Lamp.
func (l *Lamp) State() LampState { return l.state }

// Start turns the Lamp on. It returns ErrBusy while the Lamp is on.
func (l *Lamp) Start(now time.Duration) error {
	if l.state == LampOn {
		return ErrBusy
	}
	if l.state == LampOff {
		l.state, l.until = LampOn, now+l.wait
	}
	return nil
}

// Stop turns the Lamp off. It returns ErrOff while the Lamp is off.
func (l *Lamp) Stop() error {
	if l.state != LampOn {
		return ErrOff
	}
	l.off()
	return nil
}

// Tick turns the Lamp off at the end of its wait.
func (l *Lamp) Tick(now time.Duration) {
	if l.state == LampOn && now >= l.until {
		l.off()
	}
}

// Break breaks the Lamp.
func (l *Lamp) Break() { l.state = LampBroken }

func (l *Lamp) off() { l.state = LampOff }

// Counter counts and has no State method.
type Counter struct{ n int }

// Add adds one.
func (c *Counter) Add() { c.n++ }

// Clear sets the count to 0.
func (c *Counter) Clear() { c.n = 0 }

// Len returns the count.
func (c *Counter) Len() int { return c.n }

const period = 30 * time.Millisecond

var hold = 4 * period

// Side is one side of a Pair.
type Side uint8

// The Side values.
const (
	// SideA is the first side.
	SideA Side = iota + 1
	// SideB is the second side.
	SideB
)

// SideState is the state of one side of a Pair.
type SideState uint8

// The SideState values.
const (
	// SideDown is down.
	SideDown SideState = iota
	// SideUp is up.
	SideUp
)

// Pair has two sides.
type Pair struct{ up [3]bool }

// Raise raises side s.
func (p *Pair) Raise(s Side) {
	if s == SideA || s == SideB {
		p.up[s] = true
	}
}

// SideState returns the state of side s.
func (p *Pair) SideState(s Side) SideState {
	if s != SideA && s != SideB || !p.up[s] {
		return SideDown
	}
	return SideUp
}
```
<!-- /docgen:code -->
<!-- docgen:docgen root=internal/cli/testdata/repo module=v2 page=docs/machines/machine-lamp.md -->
#### Before

```
# Lamp

This page covers every marker kind.

<!-- docgen:config pkg=machine type=Config -->
<!-- /docgen:config -->

The default wait is <!-- docgen:const pkg=machine name=DefaultWait --><!-- /docgen:const --> and at most <!-- docgen:const pkg=machine name=MaxTries --><!-- /docgen:const --> tries.

<!-- docgen:states pkg=machine type=Lamp -->
<!-- /docgen:states -->

<!-- docgen:states pkg=machine type=Counter -->
<!-- /docgen:states -->

<!-- docgen:errors pkg=machine type=Lamp -->
<!-- /docgen:errors -->

<!-- docgen:states pkg=machine type=SideState -->
<!-- /docgen:states -->

The period is <!-- docgen:const pkg=machine name=period --><!-- /docgen:const --> and the hold <!-- docgen:const pkg=machine name=hold --><!-- /docgen:const -->.

<!-- docgen:doc pkg=machine section=Rates -->
<!-- /docgen:doc -->

<!-- docgen:doc pkg=machine name=Lamp.Start -->
<!-- /docgen:doc -->

<!-- docgen:code file=v2/machine/machine.go lines=96-102 -->
<!-- /docgen:code -->

<!-- docgen:output test=machine.ExampleLamp -->
<!-- /docgen:output -->

<!-- docgen:output cmd=v2/examples/hello -->
<!-- /docgen:output -->

<!-- docgen:code file=v2/machine/example_test.go symbol=ExampleLamp -->
<!-- /docgen:code -->

<!-- docgen:code file=v2/machine/machine.go symbol=Lamp.Tick -->
<!-- /docgen:code -->

<!-- docgen:code file=v2/machine/machine.go symbol=SideState -->
<!-- /docgen:code -->

<!-- docgen:code file=v2/machine/machine.go symbol=SideB -->
<!-- /docgen:code -->

It needs Go <!-- docgen:gomod field=go --><!-- /docgen:gomod --> or later.

<!-- docgen:flags cmd=cmd/tool -->
<!-- /docgen:flags -->

<!-- docgen:doc pkg=cmd/tool section=Flags -->
<!-- /docgen:doc -->

<!-- docgen:code file=v2/examples/hello/main.go -->
<!-- /docgen:code -->
```

#### After

````
# Lamp

This page covers every marker kind.

<!-- docgen:config pkg=machine type=Config -->
| Field | Type | Default | Description |
|---|---|---|---|
| `Wait` | `time.Duration` | `DefaultWait` (250ms) | Wait is how long the Lamp stays on. 0 means DefaultWait. |
| `Tries` | `int` | `1` | Tries is the number of blinks. 0 means 1, and it is at most MaxTries. |
| `Name` | `string` | zero value | Name is a label. |
<!-- /docgen:config -->

The default wait is <!-- docgen:const pkg=machine name=DefaultWait -->`250ms`<!-- /docgen:const --> and at most <!-- docgen:const pkg=machine name=MaxTries -->`3`<!-- /docgen:const --> tries.

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

<!-- docgen:states pkg=machine type=Counter -->
[`Counter`](https://pkg.go.dev/github.com/USA-RedDragon/docgen/v2/machine#Counter) has no State method. The state in this table is the result of `Len()`. States seen in tests:

| State | Seen in tests |
|---|---|
| Len() 0 | yes, first state |
| Len() > 0 | yes |

Transitions seen in tests:

| From | Method call | To |
|---|---|---|
| Len() 0 | `Add` | Len() > 0 |
| Len() > 0 | `Clear` | Len() 0 |

![Transitions of Counter](states/machine-counter.svg)
<!-- /docgen:states -->

<!-- docgen:errors pkg=machine type=Lamp -->
| Error | Returned by | Meaning |
|---|---|---|
| [`docgen.ConfigError`](https://pkg.go.dev/github.com/USA-RedDragon/docgen/v2#ConfigError) | `NewLamp` | ConfigError is the error of a constructor for a field with a value out of range. |
| [`machine.ErrBusy`](https://pkg.go.dev/github.com/USA-RedDragon/docgen/v2/machine#ErrBusy) | `Lamp.Start` | ErrBusy is returned by Start while the Lamp is on. |
| [`machine.ErrOff`](https://pkg.go.dev/github.com/USA-RedDragon/docgen/v2/machine#ErrOff) | `Lamp.Stop` | ErrOff is returned by Stop while the Lamp is off. |
<!-- /docgen:errors -->

<!-- docgen:states pkg=machine type=SideState -->
States of [`Pair`](https://pkg.go.dev/github.com/USA-RedDragon/docgen/v2/machine#Pair), from [`Pair.SideState`](https://pkg.go.dev/github.com/USA-RedDragon/docgen/v2/machine#Pair.SideState) for each `Side`:

| State | Meaning | Seen in tests |
|---|---|---|
| [`SideDown`](https://pkg.go.dev/github.com/USA-RedDragon/docgen/v2/machine#SideDown) | SideDown is down. | yes, first state |
| [`SideUp`](https://pkg.go.dev/github.com/USA-RedDragon/docgen/v2/machine#SideUp) | SideUp is up. | yes |

Transitions seen in tests:

| From | Method call | To |
|---|---|---|
| SideDown | `Raise` | SideUp |

![Transitions of Pair](states/machine-pair-sidestate.svg)
<!-- /docgen:states -->

The period is <!-- docgen:const pkg=machine name=period -->`30ms`<!-- /docgen:const --> and the hold <!-- docgen:const pkg=machine name=hold -->`120ms`<!-- /docgen:const -->.

<!-- docgen:doc pkg=machine section=Rates -->
A Lamp blinks at these rates:

  - slow, every [DefaultWait](https://pkg.go.dev/github.com/USA-RedDragon/docgen/v2/machine#DefaultWait)
  - fast, every period
<!-- /docgen:doc -->

<!-- docgen:doc pkg=machine name=Lamp.Start -->
Start turns the Lamp on. It returns ErrBusy while the Lamp is on.
<!-- /docgen:doc -->

<!-- docgen:code file=v2/machine/machine.go lines=96-102 -->
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

<!-- docgen:output test=machine.ExampleLamp -->
```text
true
```
<!-- /docgen:output -->

<!-- docgen:output cmd=v2/examples/hello -->
```text
hello from the fixture
```
<!-- /docgen:output -->

<!-- docgen:code file=v2/machine/example_test.go symbol=ExampleLamp -->
```go
func ExampleLamp() {
	l, _ := machine.NewLamp(machine.Config{})
	_ = l.Start(0)
	fmt.Println(l.State() == machine.LampOn)
}
```
<!-- /docgen:code -->

<!-- docgen:code file=v2/machine/machine.go symbol=Lamp.Tick -->
```go
func (l *Lamp) Tick(now time.Duration) {
	if l.state == LampOn && now >= l.until {
		l.off()
	}
}
```
<!-- /docgen:code -->

<!-- docgen:code file=v2/machine/machine.go symbol=SideState -->
```go
type SideState uint8
```
<!-- /docgen:code -->

<!-- docgen:code file=v2/machine/machine.go symbol=SideB -->
```go
const (
	// SideA is the first side.
	SideA Side = iota + 1
	// SideB is the second side.
	SideB
)
```
<!-- /docgen:code -->

It needs Go <!-- docgen:gomod field=go -->`1.24.0`<!-- /docgen:gomod --> or later.

<!-- docgen:flags cmd=cmd/tool -->
```text
Usage of tool:
  -n int
    	how many lines to print (default 3)
```
<!-- /docgen:flags -->

<!-- docgen:doc pkg=cmd/tool section=Flags -->
It takes one flag.
<!-- /docgen:doc -->

<!-- docgen:code file=v2/examples/hello/main.go -->
```go
// Command hello prints a greeting.
package main

import "fmt"

func main() {
	fmt.Println("hello from the fixture")
}
```
<!-- /docgen:code -->
````
<!-- /docgen:docgen -->
<!-- /site:editor -->
