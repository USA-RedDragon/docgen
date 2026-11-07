//go:build !unix

package gocmd

import "os/exec"

func isolate(*exec.Cmd) {}

func killGroup(*exec.Cmd) {}
