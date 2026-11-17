# Show a constant

Put the value of a constant in a sentence, so that the sentence stays true.

<!-- site:editor -->
<!-- docgen:code file=internal/cli/testdata/repo/v2/machine/machine.go symbol=DefaultWait -->
```go
const DefaultWait = 250 * time.Millisecond
```
<!-- /docgen:code -->
<!-- docgen:docgen root=internal/cli/testdata/repo module=v2 page=docs/machines/guide-const.md -->
#### Before

```
The default wait is <!-- docgen:const pkg=machine name=DefaultWait --><!-- /docgen:const -->.
```

#### After

```
The default wait is <!-- docgen:const pkg=machine name=DefaultWait -->`250ms`<!-- /docgen:const -->.
```
<!-- /docgen:docgen -->
<!-- /site:editor -->

A value prints as Go prints it, so a `time.Duration` is `250ms`, and a type with a `String` method prints its text.

Use `print=` to choose another form:

<!-- site:editor -->
<!-- docgen:code file=internal/cli/testdata/repo/v2/machine/port.go -->
```go
package machine

import "fmt"

// Port is a network port.
type Port uint16

// String returns the port as text.
func (p Port) String() string { return fmt.Sprintf("port %d", uint16(p)) }

// PortWeb is the port of the web server.
const PortWeb Port = 8080

// Listen is the port of a new Lamp.
var Listen = PortWeb
```
<!-- /docgen:code -->
<!-- docgen:docgen root=internal/cli/testdata/repo module=v2 page=docs/machines/guide-const-print.md -->
#### Before

```
- Default: <!-- docgen:const pkg=machine name=Listen --><!-- /docgen:const -->
- `go`: <!-- docgen:const pkg=machine name=Listen print=go --><!-- /docgen:const -->
- `raw`: <!-- docgen:const pkg=machine name=Listen print=raw --><!-- /docgen:const -->
- `config`: <!-- docgen:const pkg=machine name=Listen print=config --><!-- /docgen:const -->
- `name`: <!-- docgen:const pkg=machine name=Listen print=name --><!-- /docgen:const -->
- `%06d`: <!-- docgen:const pkg=machine name=Listen print=%06d --><!-- /docgen:const -->
```

#### After

```
- Default: <!-- docgen:const pkg=machine name=Listen -->`port 8080`<!-- /docgen:const -->
- `go`: <!-- docgen:const pkg=machine name=Listen print=go -->`port 8080`<!-- /docgen:const -->
- `raw`: <!-- docgen:const pkg=machine name=Listen print=raw -->`8080`<!-- /docgen:const -->
- `config`: <!-- docgen:const pkg=machine name=Listen print=config -->`8080`<!-- /docgen:const -->
- `name`: <!-- docgen:const pkg=machine name=Listen print=name -->`PortWeb`<!-- /docgen:const -->
- `%06d`: <!-- docgen:const pkg=machine name=Listen print=%06d -->`008080`<!-- /docgen:const -->
```
<!-- /docgen:docgen -->
<!-- /site:editor -->

With `code=false`, the value has no backticks, such as 250ms.

For all options, see [`docgen:const`](../reference/markers.md#docgenconst).

## Rust

For a Rust crate, give the package name with `crate=` and the path of the const or static from the crate root with `name=`:

<!-- site:editor -->
<!-- docgen:code file=internal/cli/testdata/rust/crates/core-a/src/limits.rs symbol=MAX_JOBS raw=true -->
```rust
/// The most jobs that run at once.
pub const MAX_JOBS: usize = 8;
```
<!-- /docgen:code -->
<!-- docgen:docgen root=internal/cli/testdata/rust page=docs/guide-const.md -->
#### Before

```
The most jobs at once is <!-- docgen:const crate=core-a name=limits::MAX_JOBS --><!-- /docgen:const -->.
```

#### After

```
The most jobs at once is <!-- docgen:const crate=core-a name=limits::MAX_JOBS -->`8`<!-- /docgen:const -->.
```
<!-- /docgen:docgen -->
<!-- /site:editor -->

docgen runs a test on a copy of the crate to print the value, so a value that the compiler works out, such as `MAX_JOBS / 4`, prints as its result. A value prints with its `Display` impl, or else with its `Debug` impl. Private items work too.

Use `print=` to choose another form:

<!-- site:editor -->
<!-- docgen:code file=internal/cli/testdata/rust/crates/core-a/src/limits.rs symbol=Port raw=true -->
```rust
/// A network port.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct Port(pub u16);
```
<!-- /docgen:code -->
<!-- docgen:code file=internal/cli/testdata/rust/crates/core-a/src/limits.rs symbol="impl fmt::Display for Port" -->
```rust
impl fmt::Display for Port {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "port {}", self.0)
    }
}
```
<!-- /docgen:code -->
<!-- docgen:code file=internal/cli/testdata/rust/crates/core-a/src/limits.rs symbol=PORT_WEB raw=true -->
```rust
/// The port of the web server.
pub const PORT_WEB: Port = Port(8080);
```
<!-- /docgen:code -->
<!-- docgen:code file=internal/cli/testdata/rust/crates/core-a/src/limits.rs symbol=LISTEN raw=true -->
```rust
/// The port that the app listens on.
pub static LISTEN: Port = PORT_WEB;
```
<!-- /docgen:code -->
<!-- docgen:code file=internal/cli/testdata/rust/crates/core-a/src/limits.rs symbol=TIMEOUT raw=true -->
```rust
/// The timeout of one job.
pub const TIMEOUT: Duration = Duration::from_millis(1500);
```
<!-- /docgen:code -->
<!-- docgen:code file=internal/cli/testdata/rust/crates/core-a/src/limits.rs symbol=RETRIES raw=true -->
```rust
/// The tries of a job.
const RETRIES: u32 = 3;
```
<!-- /docgen:code -->
<!-- docgen:docgen root=internal/cli/testdata/rust page=docs/guide-const-print.md -->
#### Before

```
- Default: <!-- docgen:const crate=core-a name=limits::LISTEN --><!-- /docgen:const -->
- `display`: <!-- docgen:const crate=core-a name=limits::LISTEN print=display --><!-- /docgen:const -->
- `debug`: <!-- docgen:const crate=core-a name=limits::LISTEN print=debug --><!-- /docgen:const -->
- `raw`: <!-- docgen:const crate=core-a name=limits::LISTEN print=raw --><!-- /docgen:const -->
- `name`: <!-- docgen:const crate=core-a name=limits::LISTEN print=name --><!-- /docgen:const -->
- `{:x}`: <!-- docgen:const crate=core-a name=limits::MAX_JOBS print={:#06x} --><!-- /docgen:const -->
- `Duration`: <!-- docgen:const crate=core-a name=limits::TIMEOUT --><!-- /docgen:const -->
- Private: <!-- docgen:const crate=core-a name=limits::RETRIES --><!-- /docgen:const -->
```

#### After

```
- Default: <!-- docgen:const crate=core-a name=limits::LISTEN -->`port 8080`<!-- /docgen:const -->
- `display`: <!-- docgen:const crate=core-a name=limits::LISTEN print=display -->`port 8080`<!-- /docgen:const -->
- `debug`: <!-- docgen:const crate=core-a name=limits::LISTEN print=debug -->`Port(8080)`<!-- /docgen:const -->
- `raw`: <!-- docgen:const crate=core-a name=limits::LISTEN print=raw -->`PORT_WEB`<!-- /docgen:const -->
- `name`: <!-- docgen:const crate=core-a name=limits::LISTEN print=name -->`limits::PORT_WEB`<!-- /docgen:const -->
- `{:x}`: <!-- docgen:const crate=core-a name=limits::MAX_JOBS print={:#06x} -->`0x0008`<!-- /docgen:const -->
- `Duration`: <!-- docgen:const crate=core-a name=limits::TIMEOUT -->`1.5s`<!-- /docgen:const -->
- Private: <!-- docgen:const crate=core-a name=limits::RETRIES -->`3`<!-- /docgen:const -->
```
<!-- /docgen:docgen -->
<!-- /site:editor -->

### Feature-gated code

A const behind `#[cfg(feature = "...")]` has a value only in a build with that feature. Set the features of the build with the `features` setting, or with `features=` on the marker:

<!-- site:editor -->
<!-- docgen:code file=internal/cli/testdata/rust/crates/feat-gate/src/lib.rs symbol=BASE -->
```rust
pub const BASE: u32 = 8;
```
<!-- /docgen:code -->
<!-- docgen:code file=internal/cli/testdata/rust/crates/feat-gate/src/lib.rs symbol=BIG -->
```rust
#[cfg(feature = "big")]
pub const BIG: u32 = 1024;
```
<!-- /docgen:code -->
<!-- docgen:code file=internal/cli/testdata/rust/crates/feat-gate/src/lib.rs symbol=extra -->
```rust
#[cfg(feature = "extra")]
pub mod extra {
    /// The level with the `extra` feature.
    pub const LEVEL: u8 = 3;
}
```
<!-- /docgen:code -->
<!-- docgen:docgen root=internal/cli/testdata/rust page=docs/guide-const-features.md -->
#### Before

```
- Always: <!-- docgen:const crate=feat-gate name=BASE --><!-- /docgen:const -->
- With `big`: <!-- docgen:const crate=feat-gate name=BIG features=big --><!-- /docgen:const -->
- With `extra`: <!-- docgen:const crate=feat-gate name=extra::LEVEL features=extra --><!-- /docgen:const -->
```

#### After

```
- Always: <!-- docgen:const crate=feat-gate name=BASE -->`8`<!-- /docgen:const -->
- With `big`: <!-- docgen:const crate=feat-gate name=BIG features=big -->`1024`<!-- /docgen:const -->
- With `extra`: <!-- docgen:const crate=feat-gate name=extra::LEVEL features=extra -->`3`<!-- /docgen:const -->
```
<!-- /docgen:docgen -->
<!-- /site:editor -->

With a feature off, the marker is an error that names the `#[cfg]`, so the page cannot show a value that the build does not have. `features=` replaces the list of the `features` setting for that marker. See [Cargo features](../reference/markers.md#cargo-features).

For the rules of `crate=` and `name=`, see [Rust crates](../reference/markers.md#rust-crates).

## C

For a `#define` in a C file, give the file with `file=` and the name with `name=`. The value is the replacement text as written, and docgen does not evaluate it:

<!-- site:editor -->
<!-- docgen:code file=internal/cli/testdata/repo/native/limits.h -->
```c
#define LAMP_PORT 4001
#define LAMP_FLAGS (1 << 3)
#define LAMP_NAME "lamp" /* shown on the label */

#define LAMP_MASK \
	(LAMP_FLAGS | \
	 0x10)
```
<!-- /docgen:code -->
<!-- docgen:docgen root=internal/cli/testdata/repo module=v2 page=docs/machines/guide-const-c.md -->
#### Before

```
- Port: <!-- docgen:const file=native/limits.h name=LAMP_PORT --><!-- /docgen:const -->
- Flags: <!-- docgen:const file=native/limits.h name=LAMP_FLAGS --><!-- /docgen:const -->
- Name: <!-- docgen:const file=native/limits.h name=LAMP_NAME --><!-- /docgen:const -->
- Mask: <!-- docgen:const file=native/limits.h name=LAMP_MASK code=false --><!-- /docgen:const -->
```

#### After

```
- Port: <!-- docgen:const file=native/limits.h name=LAMP_PORT -->`4001`<!-- /docgen:const -->
- Flags: <!-- docgen:const file=native/limits.h name=LAMP_FLAGS -->`(1 << 3)`<!-- /docgen:const -->
- Name: <!-- docgen:const file=native/limits.h name=LAMP_NAME -->`"lamp"`<!-- /docgen:const -->
- Mask: <!-- docgen:const file=native/limits.h name=LAMP_MASK code=false -->(LAMP_FLAGS | 0x10)<!-- /docgen:const -->
```
<!-- /docgen:docgen -->
<!-- /site:editor -->

A comment is not part of the value, and a `#define` that continues on the next lines is one line of text with a single space for each line ending. With `code=false`, the value has no backticks. A function-like macro, a `#define` with no text, a name with more than one `#define` and `print=` are errors.

For all options, see [`docgen:const`](../reference/markers.md#docgenconst).
