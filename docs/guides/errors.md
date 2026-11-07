# Document errors

Add a table of the errors of a type: each error, the functions that return it, and its meaning.

<!-- site:editor -->
<!-- docgen:code file=internal/cli/testdata/repo/v2/machine/machine.go symbol=ErrBusy -->
```go
var (
	// ErrBusy is returned by Start while the Lamp is on.
	ErrBusy = errors.New("machine: busy")
	// ErrOff is returned by Stop while the Lamp is off.
	ErrOff = errors.New("machine: off")
)
```
<!-- /docgen:code -->
<!-- docgen:code file=internal/cli/testdata/repo/v2/machine/machine.go symbol=NewLamp raw=true -->
```go
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
```
<!-- /docgen:code -->
<!-- docgen:code file=internal/cli/testdata/repo/v2/machine/machine.go symbol=Lamp.Start raw=true -->
```go
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
```
<!-- /docgen:code -->
<!-- docgen:code file=internal/cli/testdata/repo/v2/machine/machine.go symbol=Lamp.Stop raw=true -->
```go
// Stop turns the Lamp off. It returns ErrOff while the Lamp is off.
func (l *Lamp) Stop() error {
	if l.state != LampOn {
		return ErrOff
	}
	l.off()
	return nil
}
```
<!-- /docgen:code -->
<!-- docgen:code file=internal/cli/testdata/repo/v2/doc.go -->
```go
// Package docgen is the fixture root package of the docgen tests.
package docgen

// ConfigError is the error of a constructor for a field with a value out
// of range.
type ConfigError struct{ Field string }

func (e *ConfigError) Error() string { return "bad " + e.Field }
```
<!-- /docgen:code -->
<!-- docgen:docgen root=internal/cli/testdata/repo module=v2 page=docs/machines/guide-errors.md -->
#### Before

```
# Errors

<!-- docgen:errors pkg=machine type=Lamp -->
<!-- /docgen:errors -->
```

#### After

```
# Errors

<!-- docgen:errors pkg=machine type=Lamp -->
| Error | Returned by | Meaning |
|---|---|---|
| [`docgen.ConfigError`](https://pkg.go.dev/github.com/USA-RedDragon/docgen/v2#ConfigError) | `NewLamp` | ConfigError is the error of a constructor for a field with a value out of range. |
| [`machine.ErrBusy`](https://pkg.go.dev/github.com/USA-RedDragon/docgen/v2/machine#ErrBusy) | `Lamp.Start` | ErrBusy is returned by Start while the Lamp is on. |
| [`machine.ErrOff`](https://pkg.go.dev/github.com/USA-RedDragon/docgen/v2/machine#ErrOff) | `Lamp.Stop` | ErrOff is returned by Stop while the Lamp is off. |
<!-- /docgen:errors -->
```
<!-- /docgen:docgen -->
<!-- /site:editor -->

Name each error in the doc comment of the function that returns it. The table reads those doc comments, and a name that does not resolve adds no row.

For details, see [`docgen:errors`](../reference/markers.md#docgenerrors).
