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

For `print=` and other options, see [`docgen:config`](../reference/markers.md#docgenconfig).

For a config that uses [configulator](https://github.com/USA-RedDragon/configulator), see [Document a configulator config](configulator.md).

## Get a default into the table

Apply the default with a plain test of the zero value:

```go
wait := c.Wait
if wait == 0 {
	wait = DefaultWait
}
```

The column then holds `DefaultWait` and its value.

For a default that the code computes in another way, state it in the doc comment of the field, for example with "0 means". The column then holds "not machine-readable; see the description".

For the exact rules, see [How docgen works](../concepts/how-it-works.md#defaults).
