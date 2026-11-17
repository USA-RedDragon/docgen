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

## Rust

For a Rust crate, give the package name with `crate=`, and the path of an item or module with `name=`:

<!-- site:editor -->
<!-- docgen:code file=internal/cli/testdata/rust/crates/core-a/src/limits.rs symbol=check_jobs raw=true -->
````rust
/// Checks that jobs is between 1 and [`MAX_JOBS`].
///
/// ```
/// # use core_a::limits::check_jobs;
/// assert_eq!(check_jobs(2), Ok(2));
/// ```
pub fn check_jobs(jobs: usize) -> Result<usize, LimitError> {
    match jobs {
        0 => Err(LimitError::Zero),
        n if n > MAX_JOBS => Err(LimitError::TooLarge(n)),
        n => Ok(n.min(RETRIES as usize * MAX_JOBS)),
    }
}
````
<!-- /docgen:code -->
<!-- docgen:docgen root=internal/cli/testdata/rust page=docs/guide-doc.md -->
#### Before

```
# check_jobs

<!-- docgen:doc crate=core-a name=limits::check_jobs -->
<!-- /docgen:doc -->
```

#### After

````
# check_jobs

<!-- docgen:doc crate=core-a name=limits::check_jobs -->
Checks that jobs is between 1 and `MAX_JOBS`.

```rust
assert_eq!(check_jobs(2), Ok(2));
```
<!-- /docgen:doc -->
````
<!-- /docgen:docgen -->
<!-- /site:editor -->

With no `name=`, the marker shows the crate doc. Headings move down two levels, code blocks with no language are `rust` blocks with no hidden lines, and an intra-doc link is its text:

<!-- site:editor -->
<!-- docgen:code file=internal/cli/testdata/rust/crates/core-a/src/lib.rs symbol=crate raw=true -->
```rust
//! The core of the fixture app.
//!
//! # Limits
//!
//! The app runs at most [`limits::MAX_JOBS`] jobs at once, and each job has a
//! timeout.
//!
//! ## Timeouts
//!
//! A job that runs past its timeout stops.
//!
//! # Other
//!
//! Not copied.
```
<!-- /docgen:code -->
<!-- docgen:docgen root=internal/cli/testdata/rust page=docs/guide-doc-crate.md -->
#### Before

```
<!-- docgen:doc crate=core-a section=Limits -->
<!-- /docgen:doc -->

<!-- docgen:doc crate=core-a name=net -->
<!-- /docgen:doc -->
```

#### After

```
<!-- docgen:doc crate=core-a section=Limits -->
The app runs at most `limits::MAX_JOBS` jobs at once, and each job has a
timeout.

#### Timeouts

A job that runs past its timeout stops.
<!-- /docgen:doc -->

<!-- docgen:doc crate=core-a name=net -->
Networking.
<!-- /docgen:doc -->
```
<!-- /docgen:docgen -->
<!-- /site:editor -->
