//go:build unix

package gocmd

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
)

// isolate starts cmd in a process group of its own and makes the end of its
// context interrupt that group. A signal to the go command alone does not stop
// the program that it runs, and the output pipes of that program stay open.
func isolate(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return signalGroup(cmd, syscall.SIGINT) }
}

// killGroup kills the processes that remain in the group of cmd.
func killGroup(cmd *exec.Cmd) {
	if cmd.Process != nil {
		_ = signalGroup(cmd, syscall.SIGKILL)
	}
}

func signalGroup(cmd *exec.Cmd, sig syscall.Signal) error {
	err := syscall.Kill(-cmd.Process.Pid, sig)
	if errors.Is(err, syscall.ESRCH) {
		return os.ErrProcessDone
	}
	return err
}
