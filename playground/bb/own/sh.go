// Package own holds the commands of bb that u-root does not have in a form that builds for js/wasm.
package own

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"mvdan.cc/sh/v3/expand"
	"mvdan.cc/sh/v3/interp"
	"mvdan.cc/sh/v3/syntax"
)

func runner(params []string) (*interp.Runner, error) {
	opts := []interp.RunnerOption{
		interp.StdIO(os.Stdin, os.Stdout, os.Stderr),
		interp.Env(expand.ListEnviron(os.Environ()...)),
		interp.ExecHandlers(func(next interp.ExecHandlerFunc) interp.ExecHandlerFunc { return next }),
	}
	if params != nil {
		opts = append(opts, interp.Params(append([]string{"--"}, params...)...))
	}
	return interp.New(opts...)
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	var es interp.ExitStatus
	if errors.As(err, &es) {
		return int(es)
	}
	fmt.Fprintln(os.Stderr, err)
	return 1
}

func runSource(name, src string, params []string, opts []string) int {
	f, err := syntax.NewParser().Parse(strings.NewReader(src), name)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	r, err := runner(params)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	for _, o := range opts {
		if err := interp.Params(o)(r); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 2
		}
	}
	return exitCode(r.Run(context.Background(), f))
}

// Sh is a POSIX shell: sh [-eux] -c command [name [args...]], sh [-eux] file [args...], or a script on stdin.
func Sh() {
	args := os.Args[1:]
	var opts []string
	cmd, hasCmd := "", false
	for len(args) > 0 && strings.HasPrefix(args[0], "-") && args[0] != "-" && args[0] != "--" {
		a := args[0]
		args = args[1:]
		for _, c := range a[1:] {
			switch c {
			case 'c':
				hasCmd = true
			case 'e', 'u', 'x', 'f', 'n':
				opts = append(opts, "-"+string(c))
			default:
				fmt.Fprintf(os.Stderr, "sh: unknown option -%c\n", c)
				os.Exit(2)
			}
		}
	}
	if len(args) > 0 && args[0] == "--" {
		args = args[1:]
	}
	if hasCmd {
		if len(args) == 0 {
			fmt.Fprintln(os.Stderr, "sh: -c requires an argument")
			os.Exit(2)
		}
		cmd, args = args[0], args[1:]
		if len(args) > 0 {
			args = args[1:]
		}
		os.Exit(runSource("", cmd, args, opts))
	}
	if len(args) > 0 {
		b, err := os.ReadFile(args[0])
		if err != nil {
			fmt.Fprintln(os.Stderr, "sh:", err)
			os.Exit(127)
		}
		os.Exit(runSource(args[0], string(b), args[1:], opts))
	}
	b, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintln(os.Stderr, "sh:", err)
		os.Exit(2)
	}
	os.Exit(runSource("", string(b), nil, opts))
}

// Builtin returns a command that runs the shell builtin name with the arguments of the process.
func Builtin(name string) func() {
	return func() {
		os.Exit(runSource("", name+` "$@"`, os.Args[1:], nil))
	}
}
