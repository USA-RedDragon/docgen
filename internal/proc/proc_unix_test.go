//go:build unix

package proc

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestRunOutput(t *testing.T) {
	t.Parallel()
	out, errOut, err := Run(t.Context(), t.TempDir(), nil, "sh", "-c", "echo out; echo err >&2")
	if err != nil {
		t.Fatal(err)
	}
	if out != "out\n" || errOut != "err\n" {
		t.Errorf("out %q, err %q", out, errOut)
	}
}

func TestRunCancelStopsGroup(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	type result struct {
		out string
		err error
	}
	done := make(chan result, 1)
	go func() {
		out, _, err := Run(ctx, t.TempDir(), nil, "sh", "-c", "sleep 60 & echo $!; wait")
		done <- result{out, err}
	}()
	time.Sleep(500 * time.Millisecond)
	cancel()
	select {
	case r := <-done:
		if r.err == nil {
			t.Error("Run returned no error")
		}
		pid, err := strconv.Atoi(strings.TrimSpace(r.out))
		if err != nil {
			t.Fatalf("output %q: %v", r.out, err)
		}
		for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
			if errors.Is(syscall.Kill(pid, 0), syscall.ESRCH) {
				return
			}
		}
		t.Errorf("process %d is alive", pid)
	case <-time.After(WaitDelay + 20*time.Second):
		t.Fatal("Run did not return")
	}
}
