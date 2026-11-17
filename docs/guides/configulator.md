# Document a configulator config

If your program loads its settings with [configulator](https://github.com/USA-RedDragon/configulator), add a reference table of each key, with its type, default, environment variable, flag and description.

<!-- site:editor -->
<!-- docgen:code file=internal/cli/testdata/settings/config.go symbol=Config -->
```go
type Config struct {
	Name   string `name:"name" default:"lamp" description:"name of the program"`
	Server Server `name:"server"`
}
```
<!-- /docgen:code -->
<!-- docgen:code file=internal/cli/testdata/settings/config.go symbol=Server -->
```go
type Server struct {
	Port int `name:"port" default:"8080" description:"port of the server"`
}
```
<!-- /docgen:code -->
<!-- docgen:docgen root=internal/cli/testdata/settings page=docs/guide-configulator.md -->
#### Before

```
<!-- docgen:configulator pkg=. type=Config env=APP_ envsep=__ flagsep=- -->
<!-- /docgen:configulator -->
```

#### After

```
<!-- docgen:configulator pkg=. type=Config env=APP_ envsep=__ flagsep=- -->
| Key           | Type    | Default | Environment        | Flag            | Description         |
|---------------|---------|---------|--------------------|-----------------|---------------------|
| `name`        | string  | `lamp`  | `APP_NAME`         | `--name`        | name of the program |
| `server.port` | integer | `8080`  | `APP_SERVER__PORT` | `--server-port` | port of the server  |
<!-- /docgen:configulator -->
```
<!-- /docgen:docgen -->
<!-- /site:editor -->

For all options, see [`docgen:configulator`](../reference/markers.md#docgenconfigulator).

The table comes from the configulator generator, which also writes the code that loads the settings, so the two cannot disagree. A change to a tag fails `docgen check` until you run `docgen update`.

## Your module must require the generator

A module that generates its config already does. If yours does not:

```sh
go get -tool github.com/USA-RedDragon/configulator/v2/cmd/configulator
```

## Match your program

Give the same values that you give to `WithEnvironmentVariables` and to the flag binding:

| Argument | Meaning | Default |
|---|---|---|
| `env=` | The environment variable prefix, such as `MYAPP_`. | none |
| `envsep=` | The environment variable separator. | `_` |
| `flagsep=` | The flag separator. | `.` |

## Rust

For a Rust program that loads its settings with [configulator-rs](https://github.com/USA-RedDragon/configulator-rs), give the package name with `crate=` and the path of the config struct with `type=`:

<!-- site:editor -->
<!-- docgen:code file=internal/cli/testdata/rust/crates/core-a/src/config.rs -->
```rust
//! The configuration of the app.

use configulator::Config;

/// The configuration of the app.
#[derive(Config, Debug)]
pub struct AppConfig {
    #[configulator(name = "log-level", default = "info", description = "log level: debug, info, warn or error")]
    pub log_level: String,

    #[configulator(name = "jobs", short = 'j', default = "4", description = "jobs that run at once")]
    pub jobs: usize,

    #[configulator(name = "http", nested)]
    pub http: Http,

    #[configulator(name = "token", required, secret, description = "API token")]
    pub token: String,
}

/// The listener of the app.
#[derive(Config, Debug)]
pub struct Http {
    #[configulator(name = "host", default = "[::]", description = "listen address")]
    pub host: String,

    #[configulator(name = "port", env = "LISTEN_PORT", default = "8080", description = "listen port")]
    pub port: u16,
}
```
<!-- /docgen:code -->
<!-- docgen:docgen root=internal/cli/testdata/rust page=docs/guide-configulator.md -->
#### Before

```
<!-- docgen:configulator crate=core-a type=config::AppConfig env=APP_ -->
<!-- /docgen:configulator -->
```

#### After

```
<!-- docgen:configulator crate=core-a type=config::AppConfig env=APP_ -->
| Key         | Type    | Default | Environment            | Flag           | Description                           |
|-------------|---------|---------|------------------------|----------------|---------------------------------------|
| `log-level` | string  | `info`  | `APP_LOG_LEVEL`        | `--log-level`  | log level: debug, info, warn or error |
| `jobs`      | integer | `4`     | `APP_JOBS`             | `-j`, `--jobs` | jobs that run at once                 |
| `http.host` | string  | `[::]`  | `APP_HTTP_HOST`        | `--http.host`  | listen address                        |
| `http.port` | integer | `8080`  | `APP_HTTP_LISTEN_PORT` | `--http.port`  | listen port                           |
| `token`     | string  |         | `APP_TOKEN`            | `--token`      | API token (required) (secret)         |
<!-- /docgen:configulator -->
```
<!-- /docgen:docgen -->
<!-- /site:editor -->

The table is the one that `configulator --markdown` prints. docgen gets it from configulator-rs in a test on a copy of the crate, so it comes from the configulator-rs version of `Cargo.lock`, and you install nothing.
