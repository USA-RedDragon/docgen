// Package proc runs a tool such as go or cargo in a process group of its own.
package proc

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"time"
)

// WaitDelay limits the wait for a process after its context ends.
const WaitDelay = 5 * time.Second

// Run runs the tool name with args in dir and returns its standard output and
// standard error. When ctx ends, Run sends an interrupt to the process group of
// the tool, and kills a process that is alive after WaitDelay. When the output
// stays open after the tool exits, Run returns after at most WaitDelay with no
// error.
func Run(ctx context.Context, dir string, env []string, name string, args ...string) (string, string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Env = env
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	err := RunGroup(ctx, cmd)
	return out.String(), errb.String(), err
}

// RunGroup runs cmd in a process group of its own, with the rules of Run. cmd
// comes from exec.CommandContext with ctx.
func RunGroup(ctx context.Context, cmd *exec.Cmd) error {
	cmd.WaitDelay = WaitDelay
	isolate(cmd)
	err := cmd.Run()
	if ctx.Err() != nil {
		killGroup(cmd)
	} else if errors.Is(err, exec.ErrWaitDelay) {
		err = nil
	}
	return err
}
