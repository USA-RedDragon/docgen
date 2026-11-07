package gen

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/USA-RedDragon/docgen/internal/gocmd"
	"github.com/USA-RedDragon/docgen/internal/gomod"
)

// A programRun names one run of a Go program or a shell command.
type programRun struct {
	cmd     string
	command string
}

func (r programRun) String() string {
	if r.cmd != "" {
		return r.cmd
	}
	return r.command
}

// A programResult is the output of one run.
type programResult struct {
	out string
	err error
}

// runProgram runs the Go program or the shell command of r.
func (g *Generator) runProgram(ctx context.Context, r programRun) programResult {
	fmt.Fprintf(g.log, "docgen: running %s\n", r)
	runCtx, cancel := context.WithTimeout(ctx, gocmd.ExampleTimeout)
	defer cancel()
	if r.cmd != "" {
		return g.runGo(runCtx, r.cmd)
	}
	return g.runShell(runCtx, r.command)
}

// runGo runs the Go program in the directory cmd of the repository, from its
// nearest go.mod.
func (g *Generator) runGo(ctx context.Context, cmd string) programResult {
	dir := g.mod.RootDir(cmd)
	if !gomod.HasGo(dir) {
		return programResult{err: fmt.Errorf("no Go command in %s", cmd)}
	}
	mod, err := gomod.ModuleRoot(dir)
	if err != nil {
		return programResult{err: fmt.Errorf("%s: %w", cmd, err)}
	}
	rel, err := filepath.Rel(mod, dir)
	if err != nil {
		return programResult{err: err}
	}
	out, errOut, err := g.session.Run(ctx, mod, "run", "./"+filepath.ToSlash(rel))
	if err != nil {
		err = fmt.Errorf("go run %s: %w\n%s", cmd, err, errOut)
	}
	return programResult{out: out, err: err}
}

// runShell runs command with sh in the repository root.
func (g *Generator) runShell(ctx context.Context, command string) programResult {
	env, err := g.session.Env(ctx)
	if err != nil {
		return programResult{err: err}
	}
	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	cmd.Dir = g.mod.Root
	cmd.Env = env
	var out, errOut strings.Builder
	cmd.Stdout, cmd.Stderr = &out, &errOut
	if err := gocmd.RunGroup(ctx, cmd); err != nil {
		return programResult{err: fmt.Errorf("%s: %w\n%s", command, err, errOut.String())}
	}
	return programResult{out: out.String()}
}

// runTest runs the Examples with the names in tests of the package ip and
// returns the error of the run.
func (g *Generator) runTest(ctx context.Context, ip string, tests map[string]bool) error {
	names := make([]string, 0, len(tests))
	for _, n := range sortedKeys(tests) {
		names = append(names, regexp.QuoteMeta("Example"+n))
	}
	fmt.Fprintf(g.log, "docgen: running %d examples of %s\n", len(names), g.mod.ShortPath(ip))
	runCtx, cancel := context.WithTimeout(ctx, gocmd.ExampleTimeout)
	defer cancel()
	out, errOut, err := g.session.Run(runCtx, g.mod.Dir, "test", "-count=1", "-run", "^("+strings.Join(names, "|")+")$", ip)
	if err != nil {
		return fmt.Errorf("go test %s: %w\n%s%s", g.mod.ShortPath(ip), err, out, errOut)
	}
	return nil
}
