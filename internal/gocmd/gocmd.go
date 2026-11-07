// Package gocmd runs the go command.
package gocmd

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// The limits of one go command or built program.
const (
	// ToolTimeout limits a build or a run of a tool.
	ToolTimeout = 5 * time.Minute
	// ExampleTimeout limits a run of an example or of the Examples of a package.
	ExampleTimeout = 10 * time.Minute
	// RecordTimeout limits a run of the tests of a package with the state hook.
	RecordTimeout = 20 * time.Minute
	// WaitDelay limits the wait for a process after its context ends.
	WaitDelay = 5 * time.Second
)

// goEnv returns the environment of the process with the jobs and tags of a run
// added to its GOFLAGS, as flagsEnv does.
func goEnv(jobs int, tags []string, extra ...string) []string {
	return flagsEnv(os.Getenv("GOFLAGS"), jobs, tags, extra...)
}

// flagsEnv returns the environment of the process with GOFLAGS set to flags and
// the jobs and tags of a run, and with no GOTMPDIR, followed by extra. jobs 0
// adds no -p. The tags join a -tags of flags, because the last -tags replaces the
// ones before it. The rest of the environment does not change, so GOPROXY and
// GOWORK have the same effect as for any go command.
func flagsEnv(flags string, jobs int, tags []string, extra ...string) []string {
	add := func(flag string) {
		flags = strings.TrimSpace(flags + " " + flag)
	}
	if jobs > 0 {
		add("-p=" + strconv.Itoa(jobs))
	}
	if len(tags) > 0 {
		add("-tags=" + strings.Join(append(goflagsTags(flags), tags...), ","))
	}
	env := []string{}
	for _, e := range os.Environ() {
		k, _, _ := strings.Cut(e, "=")
		if k != "GOFLAGS" && k != "GOTMPDIR" {
			env = append(env, e)
		}
	}
	env = append(env, "GOFLAGS="+flags)
	return append(env, extra...)
}

// goflagsFields splits a GOFLAGS value as the go command does: at white space,
// with quotes around a field that holds some.
func goflagsFields(s string) []string {
	const space = " \t\r\n"
	var fields []string
	for {
		s = strings.TrimLeft(s, space)
		if s == "" {
			return fields
		}
		if q := s[0]; q == '"' || q == '\'' {
			if end := strings.IndexByte(s[1:], q); end >= 0 {
				fields = append(fields, s[1:end+1])
				s = s[end+2:]
				continue
			}
		}
		end := strings.IndexAny(s, space)
		if end < 0 {
			end = len(s)
		}
		fields = append(fields, s[:end])
		s = s[end:]
	}
}

// goflagsTags returns the build tags of the last -tags of the GOFLAGS value s.
func goflagsTags(s string) []string {
	var tags []string
	for _, f := range goflagsFields(s) {
		v, ok := strings.CutPrefix(strings.TrimPrefix(f, "-"), "-tags=")
		if !ok {
			v, ok = strings.CutPrefix(f, "-tags=")
		}
		if ok {
			tags = strings.FieldsFunc(v, func(r rune) bool { return r == ',' || unicode.IsSpace(r) })
		}
	}
	return tags
}

// Run runs go with args in dir and returns its standard output and standard
// error. When ctx ends, Run sends an interrupt to the process group of the go
// command, and kills a process that is alive after WaitDelay. When the output
// stays open after go exits, Run returns after at most WaitDelay with no error.
func Run(ctx context.Context, dir string, env []string, args ...string) (string, string, error) {
	cmd := exec.CommandContext(ctx, "go", args...)
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
