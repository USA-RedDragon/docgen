# Show a doc comment

Put a doc comment in a page, so that you write the text once, in the code.

<!-- site:editor -->
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
<!-- docgen:docgen root=internal/cli/testdata/repo module=v2 page=docs/machines/guide-doc.md -->
#### Before

```
# Start

<!-- docgen:doc pkg=machine name=Lamp.Start -->
<!-- /docgen:doc -->
```

#### After

```
# Start

<!-- docgen:doc pkg=machine name=Lamp.Start -->
Start turns the Lamp on. It returns ErrBusy while the Lamp is on.
<!-- /docgen:doc -->
```
<!-- /docgen:docgen -->
<!-- /site:editor -->

For the package doc or one section of it, leave out `name=` or add `section=`:

<!-- site:editor -->
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
<!-- docgen:code file=internal/cli/testdata/repo/v2/machine/machine.go symbol=package raw=true -->
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
```
<!-- /docgen:code -->
<!-- docgen:docgen root=internal/cli/testdata/repo module=v2 page=docs/machines/guide-doc-package.md -->
#### Before

```
<!-- docgen:doc pkg=. -->
<!-- /docgen:doc -->

<!-- docgen:doc pkg=machine section=Rates -->
<!-- /docgen:doc -->
```

#### After

```
<!-- docgen:doc pkg=. -->
Package docgen is the fixture root package of the docgen tests.
<!-- /docgen:doc -->

<!-- docgen:doc pkg=machine section=Rates -->
A Lamp blinks at these rates:

  - slow, every [DefaultWait](https://pkg.go.dev/github.com/USA-RedDragon/docgen/v2/machine#DefaultWait)
  - fast, every period
<!-- /docgen:doc -->
```
<!-- /docgen:docgen -->
<!-- /site:editor -->

Text in backticks in a doc comment is a code span. For all options, see [`docgen:doc`](../reference/markers.md#docgendoc).
