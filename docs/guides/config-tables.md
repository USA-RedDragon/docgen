# Document a config struct

Add a table of the fields of a struct. The default of each field comes from the code, so the table cannot drift from it.

<!-- site:editor -->
<!-- docgen:code file=internal/cli/testdata/repo/v2/machine/machine.go symbol=Config -->
```go
type Config struct {
	// Wait is how long the Lamp stays on. 0 means DefaultWait.
	Wait time.Duration
	// Tries is the number of blinks. 0 means 1, and it is at most MaxTries.
	Tries int
	// Name is a label.
	Name string
}
```
<!-- /docgen:code -->
<!-- docgen:code file=internal/cli/testdata/repo/v2/machine/machine.go symbol=NewLamp -->
```go
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
<!-- docgen:docgen root=internal/cli/testdata/repo module=v2 page=docs/machines/guide-config.md -->
#### Before

```
# Settings

<!-- docgen:config pkg=machine type=Config -->
<!-- /docgen:config -->
```

#### After

```
# Settings

<!-- docgen:config pkg=machine type=Config -->
| Field | Type | Default | Description |
|---|---|---|---|
| `Wait` | `time.Duration` | `DefaultWait` (250ms) | Wait is how long the Lamp stays on. 0 means DefaultWait. |
| `Tries` | `int` | `1` | Tries is the number of blinks. 0 means 1, and it is at most MaxTries. |
| `Name` | `string` | zero value | Name is a label. |
<!-- /docgen:config -->
```
<!-- /docgen:docgen -->
<!-- /site:editor -->

To show each default as a user writes it in a config file, use `print=config`. A `time.Duration` is then `5s`, a type with a MarshalText method is the text of that method, and an enum with a String method is its number, such as `1`.

Tip: apply each default in an `if` that compares the field with its zero value, such as `if cfg.Tries == 0 { l.tries = 1 }`. A default that the code computes in another way is not in the table.

## Rust

For a Rust crate, give the package name with `crate=` and the path of a struct from the crate root with `type=`. The struct has named fields and no generics:

<!-- site:editor -->
<!-- docgen:code file=internal/cli/testdata/rust/crates/cfg-app/src/guide.rs symbol=Listener raw=true -->
```rust
/// The settings of a listener.
#[derive(Debug, Deserialize)]
#[serde(rename_all = "kebab-case")]
pub struct Listener {
    /// The host to listen on.
    pub host: String,
    /// The port to listen on.
    pub port: u16,
    /// How long a request can take. 0 means no limit.
    pub request_timeout: Duration,
    /// Extra response headers.
    #[serde(default)]
    pub headers: Vec<String>,
    /// How many times a failed request runs again.
    #[serde(default = "default_retries")]
    pub retries: u32,
}
```
<!-- /docgen:code -->
<!-- docgen:code file=internal/cli/testdata/rust/crates/cfg-app/src/guide.rs symbol=default_retries raw=true -->
```rust
fn default_retries() -> u32 {
    3
}
```
<!-- /docgen:code -->
<!-- docgen:code file=internal/cli/testdata/rust/crates/cfg-app/src/guide.rs symbol="impl Default for Listener" raw=true -->
```rust
impl Default for Listener {
    fn default() -> Self {
        Self {
            host: "localhost".to_string(),
            port: LISTEN_PORT,
            request_timeout: Duration::from_millis(1500),
            headers: vec!["x-trace: on".to_string()],
            retries: 1,
        }
    }
}
```
<!-- /docgen:code -->
<!-- docgen:docgen root=internal/cli/testdata/rust page=docs/guide-config.md -->
#### Before

```
<!-- docgen:config crate=cfg-app type=guide::Listener -->
<!-- /docgen:config -->
```

#### After

```
<!-- docgen:config crate=cfg-app type=guide::Listener -->
| Field | Type | Default | Description |
|---|---|---|---|
| `host` | `String` | `"localhost".to_string()` (localhost) | The host to listen on. |
| `port` | `u16` | `LISTEN_PORT` (8000) | The port to listen on. |
| `request-timeout` | `Duration` | `Duration::from_millis(1500)` (1.5s) | How long a request can take. 0 means no limit. |
| `headers` | `Vec<String>` | `Default::default()` ([]) | Extra response headers. |
| `retries` | `u32` | `default_retries()` (3) | How many times a failed request runs again. |
<!-- /docgen:config -->
```
<!-- /docgen:docgen -->
<!-- /site:editor -->

The Field column holds the name that a config file uses. With serde, that is the `#[serde(rename = "...")]` of the field or the `rename_all` of the struct, as `request-timeout` is above. The Type column is the type as the source writes it.

Each default comes from your code. docgen runs a test on a copy of the crate that calls `Listener::default()` and prints each field. When the source writes a default, as `LISTEN_PORT` is above, the table shows it with its value. A default that has no written form, such as a `#[derive(Default)]`, shows the value alone.

A serde attribute on a field wins over the `Default` of the struct, since that is what serde does for a key that is not in the file: `#[serde(default = "default_retries")]` gives `3` and not the `1` of the impl, and `#[serde(default)]` gives the `Default` of the field type, here an empty list. A field with no default shows "no default", or "not machine-readable; see the description" when its doc says so, as with `0 means`.

A `pub` field is a row. When the struct derives `Deserialize`, so is a private field, and a field with `#[serde(skip)]` is not. A field of a struct type is one row, and its fields are not.

Use `print=` to change the value, with the choices of [`docgen:const`](const.md): `debug` quotes a string, `raw` shows only the source and runs no code, and `name` gives the const that has the value, such as `guide::LISTEN_PORT`.

For all the rules and options, see [`docgen:config`](../reference/markers.md#docgenconfig).
