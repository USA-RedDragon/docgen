# Show code

Put code from your repository in a page, so that the page changes when the code changes.

## A symbol

Prefer `symbol=`, because the excerpt stays correct when the code moves in the file.

<!-- site:editor -->
<!-- docgen:code file=internal/cli/testdata/repo/v2/machine/machine.go symbol=Lamp.Start -->
```go
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
<!-- docgen:docgen root=internal/cli/testdata/repo module=v2 page=docs/machines/guide-code.md -->
#### Before

```
# Start a lamp

<!-- docgen:code file=v2/machine/machine.go symbol=Lamp.Start -->
<!-- /docgen:code -->
```

#### After

````
# Start a lamp

<!-- docgen:code file=v2/machine/machine.go symbol=Lamp.Start -->
```go
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
````
<!-- /docgen:docgen -->
<!-- /site:editor -->

It also works in [YAML](../reference/markers.md#symbol-in-a-yaml-file) and [C](../reference/markers.md#symbol-in-a-c-file) files:

<!-- site:editor -->
<!-- docgen:code file=internal/cli/testdata/repo/ci/workflow.yaml -->
```yaml
jobs:
  test:
    env:
      GOFLAGS: -mod=readonly
    steps:
      - name: Get the code
        uses: actions/checkout@v6
      - name: Run the tests
        run: go test ./...
      - name: Check the docs
        run: docgen check
```
<!-- /docgen:code -->
<!-- docgen:docgen root=internal/cli/testdata/repo module=v2 page=docs/machines/guide-code-yaml.md -->
#### Before

```
<!-- docgen:code file=ci/workflow.yaml symbol=jobs.test.env -->
<!-- /docgen:code -->

<!-- docgen:code file=ci/workflow.yaml symbol="jobs.test.steps[Get the code:Run the tests]" -->
<!-- /docgen:code -->

<!-- docgen:code file=ci/workflow.yaml symbol="jobs.test.steps[Check the docs]" -->
<!-- /docgen:code -->
```

#### After

````
<!-- docgen:code file=ci/workflow.yaml symbol=jobs.test.env -->
```yaml
env:
  GOFLAGS: -mod=readonly
```
<!-- /docgen:code -->

<!-- docgen:code file=ci/workflow.yaml symbol="jobs.test.steps[Get the code:Run the tests]" -->
```yaml
- name: Get the code
  uses: actions/checkout@v6
- name: Run the tests
  run: go test ./...
```
<!-- /docgen:code -->

<!-- docgen:code file=ci/workflow.yaml symbol="jobs.test.steps[Check the docs]" -->
```yaml
- name: Check the docs
  run: docgen check
```
<!-- /docgen:code -->
````
<!-- /docgen:docgen -->
<!-- /site:editor -->

## A region, a line range or a whole file

Mark a region in the file itself, in the comment syntax of the file. A region with no marker fails `check`.

<!-- site:editor -->
<!-- docgen:code file=internal/cli/testdata/repo/scripts/build.sh raw=true -->
```sh
#!/bin/sh
set -eu
# docgen:region setup
go mod download
go build ./...
# docgen:end
go test ./...
```
<!-- /docgen:code -->
<!-- docgen:docgen root=internal/cli/testdata/repo module=v2 page=docs/machines/guide-code-file.md -->
#### Before

```
<!-- docgen:code file=scripts/build.sh region=setup -->
<!-- /docgen:code -->

<!-- docgen:code file=scripts/build.sh lines=1-2 -->
<!-- /docgen:code -->

<!-- docgen:code file=scripts/build.sh -->
<!-- /docgen:code -->
```

#### After

````
<!-- docgen:code file=scripts/build.sh region=setup -->
```sh
go mod download
go build ./...
```
<!-- /docgen:code -->

<!-- docgen:code file=scripts/build.sh lines=1-2 -->
```sh
#!/bin/sh
set -eu
```
<!-- /docgen:code -->

<!-- docgen:code file=scripts/build.sh -->
```sh
#!/bin/sh
set -eu
go mod download
go build ./...
go test ./...
```
<!-- /docgen:code -->
````
<!-- /docgen:docgen -->
<!-- /site:editor -->

`lines=` is the last choice, because it fails when the file changes under the excerpt. The error holds the new range when the same text moved.

## Doc comments and region marks

Other excerpts leave out the doc comment of a symbol and the region marks. Add `raw=true` to keep the text of the file. With it, `symbol=package` in a Go file is the package clause and its doc comment.

<!-- site:editor -->
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
<!-- docgen:code file=internal/cli/testdata/repo/scripts/build.sh raw=true -->
```sh
#!/bin/sh
set -eu
# docgen:region setup
go mod download
go build ./...
# docgen:end
go test ./...
```
<!-- /docgen:code -->
<!-- docgen:docgen root=internal/cli/testdata/repo module=v2 page=docs/machines/guide-code-raw.md -->
#### Before

```
# Keep the comments

<!-- docgen:code file=v2/machine/machine.go symbol=Lamp.Start raw=true -->
<!-- /docgen:code -->

<!-- docgen:code file=v2/machine/machine.go symbol=package raw=true -->
<!-- /docgen:code -->

<!-- docgen:code file=scripts/build.sh raw=true -->
<!-- /docgen:code -->
```

#### After

````
# Keep the comments

<!-- docgen:code file=v2/machine/machine.go symbol=Lamp.Start raw=true -->
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

<!-- docgen:code file=v2/machine/machine.go symbol=package raw=true -->
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

<!-- docgen:code file=scripts/build.sh raw=true -->
```sh
#!/bin/sh
set -eu
# docgen:region setup
go mod download
go build ./...
# docgen:end
go test ./...
```
<!-- /docgen:code -->
````
<!-- /docgen:docgen -->
<!-- /site:editor -->

For all options, see [`docgen:code`](../reference/markers.md#docgencode).
