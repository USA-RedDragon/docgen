//go:build unix

package cli

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/USA-RedDragon/docgen/internal/gocmd"
)

const childArgs = "DOCGEN_TEST_CHILD_ARGS"

// TestInterruptChild is the docgen process of TestInterrupt.
func TestInterruptChild(t *testing.T) {
	t.Parallel()
	args := os.Getenv(childArgs)
	if args == "" {
		t.Skip("runs only as a child of TestInterrupt")
	}
	os.Exit(Main(strings.Fields(args), os.Stderr))
}

const holdMachine = `package p

// S is a state.
type S int

// The states.
const (
	Off S = iota
	On
)

// Lamp is a machine.
type Lamp struct{ s S }

// State returns the state.
func (l *Lamp) State() S { return l.s }

// Start turns the lamp on.
func (l *Lamp) Start() { l.s = On }
`

// holdCode writes the PID of the process to the file and blocks.
func holdCode(file string) string {
	return fmt.Sprintf(`os.WriteFile(%[1]q+".tmp", []byte(strconv.Itoa(os.Getpid())), 0o600)
	os.Rename(%[1]q+".tmp", %[1]q)
	time.Sleep(time.Hour)`, file)
}

func holdMain(file string) string {
	return "package main\n\nimport (\n\t\"os\"\n\t\"strconv\"\n\t\"time\"\n)\n\nfunc main() {\n\t" + holdCode(file) + "\n}\n"
}

// holdParent starts a child that blocks, writes the PID of the child to the
// file and blocks.
func holdParent(file string) string {
	return fmt.Sprintf(`package main

import (
	"os"
	"os/exec"
	"strconv"
	"time"
)

func main() {
	c := exec.Command("sleep", "300")
	if err := c.Start(); err != nil {
		panic(err)
	}
	os.WriteFile(%[1]q+".tmp", []byte(strconv.Itoa(c.Process.Pid)), 0o600)
	os.Rename(%[1]q+".tmp", %[1]q)
	time.Sleep(time.Hour)
}
`, file)
}

func holdTest(file string) string {
	return "package p\n\nimport (\n\t\"os\"\n\t\"strconv\"\n\t\"testing\"\n\t\"time\"\n)\n\nfunc TestHold(t *testing.T) {\n\t" + holdCode(file) + "\n}\n"
}

func holdExample(file string) string {
	return "package p\n\nimport (\n\t\"os\"\n\t\"strconv\"\n\t\"time\"\n)\n\nfunc ExampleLamp() {\n\t" + holdCode(file) + "\n\t// Output: x\n}\n"
}

// longWait is the upper bound of a wait on a process of a test. It is long, so
// that a test under load does not fail, and a wait that meets its condition ends
// at once.
const longWait = 10 * time.Minute

// waitForFile returns the PID in the file once the file exists. An end of
// docgen before that is an error.
func waitForFile(t *testing.T, file string, done <-chan error) int {
	t.Helper()
	tick := time.NewTicker(10 * time.Millisecond)
	defer tick.Stop()
	limit := time.After(longWait)
	for {
		if b, err := os.ReadFile(file); err == nil {
			pid, err := strconv.Atoi(string(b))
			if err != nil {
				t.Fatal(err)
			}
			return pid
		}
		select {
		case err := <-done:
			t.Fatalf("docgen ended before the signal: %v", err)
		case <-limit:
			t.Fatal("the program did not start")
		case <-tick.C:
		}
	}
}

// waitForExit waits for the process pid to end. It reports false when the
// process is alive after longWait.
func waitForExit(pid int) bool {
	for limit := time.Now().Add(longWait); !errors.Is(syscall.Kill(pid, 0), syscall.ESRCH); time.Sleep(10 * time.Millisecond) {
		if time.Now().After(limit) {
			return false
		}
	}
	return true
}

func goCache(t *testing.T) string {
	t.Helper()
	out, errOut, err := gocmd.Run(t.Context(), ".", os.Environ(), "env", "GOCACHE")
	if err != nil {
		t.Fatalf("go env: %v\n%s", err, errOut)
	}
	return strings.TrimSpace(out)
}

func hasPrefix(prefix string) func(string) bool {
	return func(s string) bool { return strings.HasPrefix(s, prefix) }
}

// runDir returns the one temporary directory of a run in work.
func runDir(t *testing.T, work string) string {
	t.Helper()
	names := dirNames(t, work)
	if len(names) != 1 || !strings.HasPrefix(names[0], "run-") {
		t.Fatalf("directories during the run: %q, want one run directory", names)
	}
	return filepath.Join(work, names[0])
}

// holdCC is a C compiler that blocks. The go command gets its version, then
// runs it during the build of a cgo package.
func holdCC(file string) string {
	return fmt.Sprintf(`#!/bin/sh
case "$*" in
*-###*) echo 'gcc version 1'; exit 0 ;;
*--version*) echo 'gcc 1'; exit 0 ;;
esac
printf %%s $$ > %[1]q.tmp
mv %[1]q.tmp %[1]q
exec sleep 300
`, file)
}

// TestInterruptCompile interrupts docgen while a go build compiles, a moment
// when the go command has no handler for the signal and does not remove its
// work directory. That directory must be in the run directory and go with it.
func TestInterruptCompile(t *testing.T) {
	t.Parallel()
	gocache := goCache(t)
	root, cache, tmp := t.TempDir(), t.TempDir(), t.TempDir()
	hold := filepath.Join(t.TempDir(), "pid")
	cc := filepath.Join(t.TempDir(), "cc")
	if err := os.WriteFile(cc, []byte(holdCC(hold)), 0o700); err != nil { //nolint:gosec // the stub must be executable
		t.Fatal(err)
	}
	text := "<!-- docgen:usage cmd=cmd/hold --><!-- /docgen:usage -->\n"
	writeTree(t, root, map[string]string{
		"go.mod":           "module example.com/w\n\ngo 1.24.0\n",
		"cmd/hold/main.go": "package main\n\n// int f(void);\nimport \"C\"\n\nfunc main() {}\n",
		"docs/p.md":        text,
	})
	cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestInterruptChild$") //nolint:gosec // the test binary
	cmd.Dir = root
	cmd.Env = append(os.Environ(), childArgs+"=update", "XDG_CACHE_HOME="+cache, "GOCACHE="+gocache, "TMPDIR="+tmp, "CC="+cc, "CGO_ENABLED=1")
	var log strings.Builder
	cmd.Stdout, cmd.Stderr = &log, &log
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	waitForFile(t, hold, done)
	work := filepath.Join(cache, "docgen")
	if names := dirNames(t, runDir(t, work)); !slices.ContainsFunc(names, hasPrefix("go-build")) {
		t.Errorf("run directory during the build: %q", names)
	}
	if err := cmd.Process.Signal(syscall.SIGINT); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		var ee *exec.ExitError
		if !errors.As(err, &ee) || ee.ExitCode() != 1 {
			t.Errorf("exit: %v, want code 1\n%s", err, log.String())
		}
	case <-time.After(longWait):
		t.Fatalf("docgen did not stop:\n%s", log.String())
	}
	if names := dirNames(t, work); len(names) != 0 {
		t.Errorf("work directories after the run: %q", names)
	}
	if names := dirNames(t, tmp); slices.ContainsFunc(names, hasPrefix("go-build")) {
		t.Errorf("build directory of the go command in TMPDIR: %q", names)
	}
	if got := page(t, root, "p.md"); got != text {
		t.Errorf("page changed: %q", got)
	}
}

func TestInterrupt(t *testing.T) {
	t.Parallel()
	gocache := goCache(t)
	const filled = "<!-- docgen:gomod field=module --><!-- /docgen:gomod -->\n"
	for _, c := range []struct {
		name   string
		sig    syscall.Signal
		work   string
		marker string
		files  func(hold string) map[string]string
	}{
		{
			name:   "states",
			sig:    syscall.SIGINT,
			work:   "overlay-",
			marker: "<!-- docgen:states pkg=p type=Lamp --><!-- /docgen:states -->\n",
			files: func(hold string) map[string]string {
				return map[string]string{"p/p.go": holdMachine, "p/p_test.go": holdTest(hold)}
			},
		},
		{
			name:   "states on SIGTERM",
			sig:    syscall.SIGTERM,
			work:   "overlay-",
			marker: "<!-- docgen:states pkg=p type=Lamp --><!-- /docgen:states -->\n",
			files: func(hold string) map[string]string {
				return map[string]string{"p/p.go": holdMachine, "p/p_test.go": holdTest(hold)}
			},
		},
		{
			name:   "usage",
			sig:    syscall.SIGINT,
			work:   "usage-",
			marker: "<!-- docgen:usage cmd=cmd/hold --><!-- /docgen:usage -->\n",
			files: func(hold string) map[string]string {
				return map[string]string{"cmd/hold/main.go": holdMain(hold)}
			},
		},
		{
			name:   "usage, child of the program",
			sig:    syscall.SIGINT,
			work:   "usage-",
			marker: "<!-- docgen:usage cmd=cmd/hold --><!-- /docgen:usage -->\n",
			files: func(hold string) map[string]string {
				return map[string]string{"cmd/hold/main.go": holdParent(hold)}
			},
		},
		{
			name:   "example",
			sig:    syscall.SIGINT,
			marker: "<!-- docgen:output cmd=examples/hold --><!-- /docgen:output -->\n",
			files: func(hold string) map[string]string {
				return map[string]string{"examples/hold/main.go": holdMain(hold)}
			},
		},
		{
			name:   "test output",
			sig:    syscall.SIGINT,
			marker: "<!-- docgen:output test=p.ExampleLamp --><!-- /docgen:output -->\n",
			files: func(hold string) map[string]string {
				return map[string]string{"p/p.go": holdMachine, "p/p_test.go": holdExample(hold)}
			},
		},
		{
			name:   "nested run",
			sig:    syscall.SIGINT,
			work:   "sample-",
			marker: nestedMarker("root=testdata/sample page=docs/a.md"),
			files: func(hold string) map[string]string {
				return map[string]string{
					"testdata/sample/go.mod":                sampleMod,
					"testdata/sample/docs/a.md":             "<!-- docgen:output cmd=examples/hold --><!-- /docgen:output -->\n",
					"testdata/sample/examples/hold/main.go": holdMain(hold),
				}
			},
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			root, cache, tmp := t.TempDir(), t.TempDir(), t.TempDir()
			hold := filepath.Join(t.TempDir(), "pid")
			text := "# Page\n\n" + filled + c.marker
			files := c.files(hold)
			files["go.mod"] = "module example.com/w\n\ngo 1.24.0\n"
			files["docs/a.md"] = filled
			files["docs/p.md"] = text
			writeTree(t, root, files)

			cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestInterruptChild$") //nolint:gosec // the test binary
			cmd.Dir = root
			cmd.Env = append(os.Environ(), childArgs+"=update", "XDG_CACHE_HOME="+cache, "GOCACHE="+gocache, "TMPDIR="+tmp)
			var log strings.Builder
			cmd.Stdout, cmd.Stderr = &log, &log
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			done := make(chan error, 1)
			go func() { done <- cmd.Wait() }()
			pid := waitForFile(t, hold, done)
			gone := false
			t.Cleanup(func() {
				if !gone {
					_ = syscall.Kill(pid, syscall.SIGKILL)
				}
			})
			work := filepath.Join(cache, "docgen")
			if c.work != "" {
				if names := dirNames(t, runDir(t, work)); !slices.ContainsFunc(names, hasPrefix(c.work)) {
					t.Errorf("work directories during the run: %q", names)
				}
			}
			if err := cmd.Process.Signal(c.sig); err != nil {
				t.Fatal(err)
			}
			select {
			case err := <-done:
				var ee *exec.ExitError
				if !errors.As(err, &ee) || ee.ExitCode() != 1 {
					t.Errorf("exit: %v, want code 1\n%s", err, log.String())
				}
			case <-time.After(longWait):
				t.Fatalf("docgen did not stop:\n%s", log.String())
			}
			if !strings.Contains(log.String(), "signal received") {
				t.Errorf("no cause in the log:\n%s", log.String())
			}
			if strings.Contains(log.String(), "stale:") {
				t.Errorf("stale lines after an interrupt:\n%s", log.String())
			}
			if names := dirNames(t, work); len(names) != 0 {
				t.Errorf("work directories after the run: %q", names)
			}
			if got := page(t, root, "p.md"); got != text {
				t.Errorf("page changed: %q", got)
			}
			if got := page(t, root, "a.md"); got != filled {
				t.Errorf("other page changed: %q", got)
			}
			if names := dirNames(t, filepath.Join(root, "docs/states")); len(names) != 0 {
				t.Errorf("diagrams written: %q", names)
			}
			if gone = waitForExit(pid); !gone {
				t.Fatalf("process %d is alive", pid)
			}
			if names := dirNames(t, tmp); len(names) != 0 {
				t.Errorf("temporary files of the go command: %q", names)
			}
		})
	}
}
