package gen

import (
	"context"
	"fmt"
	"strings"
)

// runTarget builds the example or the binary name of the package crate, runs it
// with args in the directory of the Cargo workspace, and returns its standard
// output. kind is example or bin.
func (g *Generator) runTarget(ctx context.Context, crate, kind, name string, args []string) (string, error) {
	ws, pkg, err := g.cargoPackage(ctx, crate)
	if err != nil {
		return "", err
	}
	if _, err := targetName(pkg, kind, name); err != nil {
		return "", err
	}
	exe, err := g.cargoBuild(ctx, ws, crate, kind, name)
	if err != nil {
		return "", err
	}
	fmt.Fprintf(g.log, "docgen: running %s %s of %s\n", kind, name, crate)
	out, errOut, err := g.rust.runner.Exec(ctx, ws.Root, exe, args...)
	if err != nil {
		return "", fmt.Errorf("%s %s of %s: %w\n%s", kind, name, crate, err, strings.TrimSpace(errOut))
	}
	return out, nil
}
