package gen

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/USA-RedDragon/docgen/internal/gocmd"
	"github.com/USA-RedDragon/docgen/internal/gomod"
	"github.com/USA-RedDragon/docgen/internal/markdown"
)

var (
	usageTitle = regexp.MustCompile(`(?m)^Usage of \S*?([^/\s:]+):`)
	usageLine  = regexp.MustCompile(`(?im)^[ \t]*usage\b`)
	flagLine   = regexp.MustCompile(`(?m)^\s*--?[[:alpha:]]`)
	goroutine  = regexp.MustCompile(`(?m)^goroutine \d+ \[`)
	crashTitle = regexp.MustCompile(`(?m)^(?:panic|fatal error): `)
)

// usageOf builds the command in the directory cmdDir and returns its usage as a
// code block.
func (g *Generator) usageOf(ctx context.Context, cmdDir string) (string, error) {
	if err := g.mod.LocalDir("cmd", cmdDir); err != nil {
		return "", err
	}
	dir := g.mod.RootDir(cmdDir)
	if !gomod.HasGo(dir) {
		return "", fmt.Errorf("no Go command in %s", cmdDir)
	}
	mod, err := gomod.ModuleRoot(dir)
	if err != nil {
		return "", fmt.Errorf("%s: %w", cmdDir, err)
	}
	rel, err := filepath.Rel(mod, dir)
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(ctx, gocmd.ToolTimeout)
	defer cancel()
	work, err := g.session.WorkDir("flags-")
	if err != nil {
		return "", err
	}
	defer func() { _ = os.RemoveAll(work) }()
	fmt.Fprintf(g.log, "docgen: running %s -h\n", cmdDir)
	bin, name, err := g.buildCommand(ctx, mod, "./"+filepath.ToSlash(rel), work)
	if err != nil {
		return "", fmt.Errorf("go build %s: %w", cmdDir, err)
	}
	usage, err := g.runHelp(ctx, mod, bin, name)
	if err != nil {
		return "", fmt.Errorf("%s -h: %w", cmdDir, err)
	}
	return markdown.Fence("text", usage), nil
}

// buildCommand builds the package pkg of the module at mod into the directory
// work. It returns the path of the binary and its name, which is the name of the
// same package in go run.
func (g *Generator) buildCommand(ctx context.Context, mod, pkg, work string) (string, string, error) {
	_, errOut, err := g.session.Run(ctx, mod, "build", "-buildvcs=false", "-o", work+string(filepath.Separator), pkg)
	if err != nil {
		return "", "", fmt.Errorf("%w\n%s", err, errOut)
	}
	entries, err := os.ReadDir(work)
	if err != nil {
		return "", "", err
	}
	if len(entries) != 1 {
		return "", "", fmt.Errorf("%d binaries in the output of the build", len(entries))
	}
	file := entries[0].Name()
	return filepath.Join(work, file), strings.TrimSuffix(file, ".exe"), nil
}

// runHelp runs the binary bin with -h, as the program name, and returns what
// it prints, standard error first.
func (g *Generator) runHelp(ctx context.Context, dir, bin, name string) (string, error) {
	env, err := g.session.Env(ctx)
	if err != nil {
		return "", err
	}
	cmd := exec.CommandContext(ctx, bin, "-h")
	cmd.Args[0] = name
	cmd.Dir = dir
	cmd.Env = env
	var out, errOut strings.Builder
	cmd.Stdout, cmd.Stderr = &out, &errOut
	err = gocmd.RunGroup(ctx, cmd)
	usage := strings.ReplaceAll(errOut.String()+out.String(), bin, name)
	var ee *exec.ExitError
	code := 0
	switch {
	case err == nil:
	case !errors.As(err, &ee):
		return "", err
	case !ee.Exited():
		return "", fmt.Errorf("%w\n%s", err, usage)
	default:
		code = ee.ExitCode()
	}
	if goroutine.MatchString(usage) || code == 2 && crashTitle.MatchString(usage) {
		return "", fmt.Errorf("the command crashed:\n%s", usage)
	}
	if code > 2 {
		return "", fmt.Errorf("%w\n%s", err, usage)
	}
	if !usageLine.MatchString(usage) && !flagLine.MatchString(usage) {
		return "", fmt.Errorf("the command printed no usage:\n%s", usage)
	}
	return usageTitle.ReplaceAllString(usage, "Usage of $1:"), nil
}
