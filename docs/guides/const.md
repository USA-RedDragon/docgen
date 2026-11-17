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
<!-- docgen:code file=internal/cli/testdata/rust/crates/core-a/src/limits.rs lines=6-7 -->
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
<!-- docgen:code file=internal/cli/testdata/rust/crates/core-a/src/limits.rs lines=12-43 -->
```rust
/// The name of the app.
pub const NAME: &str = "core-a";

/// The timeout of one job.
pub const TIMEOUT: Duration = Duration::from_millis(1500);

/// The names of the modes.
pub static MODES: [&str; 2] = ["fast", "slow"];

/// A network port.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct Port(pub u16);

impl fmt::Display for Port {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "port {}", self.0)
    }
}

impl Port {
    /// The port of a plain HTTP server.
    pub const HTTP: Port = Port(80);
}

/// The port of the web server.
pub const PORT_WEB: Port = Port(8080);

/// The port that the app listens on.
pub static LISTEN: Port = PORT_WEB;

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

For the rules of `crate=` and `name=`, see [Rust crates](../reference/markers.md#rust-crates).
