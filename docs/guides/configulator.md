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
