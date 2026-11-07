//go:build unix

package gocmd

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func writeProgram(t *testing.T, source string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/p\n\ngo 1.24.0\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

// waitPID returns the PID in the file, after the program writes it.
func waitPID(t *testing.T, file string) int {
	t.Helper()
	for deadline := time.Now().Add(2 * time.Minute); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		b, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		if pid, err := strconv.Atoi(strings.TrimSpace(string(b))); err == nil {
			return pid
		}
	}
	t.Fatal("the program did not start")
	return 0
}

func waitGone(t *testing.T, pid int) {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		if errors.Is(syscall.Kill(pid, 0), syscall.ESRCH) {
			return
		}
	}
	t.Errorf("process %d is alive", pid)
}

const holdProgram = `package main

import (
	"os"
	"os/signal"
	"strconv"
	"time"
)

func main() {
	IGNORE
	if err := os.WriteFile(os.Args[1]+".tmp", []byte(strconv.Itoa(os.Getpid())), 0o600); err != nil {
		panic(err)
	}
	if err := os.Rename(os.Args[1]+".tmp", os.Args[1]); err != nil {
		panic(err)
	}
	_ = signal.Ignored
	time.Sleep(time.Hour)
}
`

func TestRunCancelStopsProgram(t *testing.T) {
	t.Parallel()
	for name, ignore := range map[string]string{
		"exits on interrupt": "",
		"ignores interrupt":  "signal.Ignore(os.Interrupt)",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			dir := writeProgram(t, strings.Replace(holdProgram, "IGNORE", ignore, 1))
			file := filepath.Join(t.TempDir(), "pid")
			env := goEnv(2, nil, "GOTMPDIR="+t.TempDir())
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			done := make(chan error, 1)
			go func() {
				_, _, err := Run(ctx, dir, env, "run", ".", file)
				done <- err
			}()
			pid := waitPID(t, file)
			start := time.Now()
			cancel()
			select {
			case err := <-done:
				if err == nil {
					t.Error("Run returned no error")
				}
			case <-time.After(WaitDelay + 20*time.Second):
				t.Fatal("Run did not return")
			}
			t.Logf("Run returned after %v", time.Since(start))
			waitGone(t, pid)
		})
	}
}

func TestRunHeldOutput(t *testing.T) {
	t.Parallel()
	dir := writeProgram(t, `package main

import (
	"fmt"
	"os"
	"os/exec"
)

func main() {
	cmd := exec.Command("sleep", "60")
	cmd.Stdout = os.Stdout
	if err := cmd.Start(); err != nil {
		panic(err)
	}
	fmt.Println(cmd.Process.Pid)
}
`)
	start := time.Now()
	out, errOut, err := Run(t.Context(), dir, goEnv(2, nil), "run", ".")
	if err != nil {
		t.Fatalf("Run: %v\n%s", err, errOut)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(out))
	if err != nil {
		t.Fatalf("output %q: %v", out, err)
	}
	if err := syscall.Kill(pid, syscall.SIGKILL); err != nil {
		t.Error(err)
	}
	if d := time.Since(start); d > 50*time.Second {
		t.Errorf("Run took %v", d)
	}
}
