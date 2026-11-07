package gen

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/USA-RedDragon/docgen/internal/gocmd"
)

const configulatorCmd = "github.com/USA-RedDragon/configulator/v2/cmd/configulator"

// referenceTable runs the generator with args in the package ip and returns the
// table of the type typ.
func (g *Generator) referenceTable(ctx context.Context, ip, typ string, args []string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, gocmd.ToolTimeout)
	defer cancel()
	short := g.mod.ShortPath(ip)
	name := short + "." + typ
	if short == "." {
		name = typ
	}
	fmt.Fprintf(g.log, "docgen: running configulator on %s\n", name)
	dir := filepath.Join(g.mod.Dir, filepath.FromSlash(short))
	out, errOut, err := g.session.Run(ctx, dir, args...)
	if err != nil {
		if missingGenerator(errOut) {
			return "", fmt.Errorf("go run %s: %w\n%s\nadd the generator to the module: go get -tool %s", configulatorCmd, err, errOut, configulatorCmd)
		}
		return "", fmt.Errorf("go run %s: %w\n%s", configulatorCmd, err, errOut)
	}
	return tableOf(out), nil
}

// missingGenerator reports whether the standard error of the go command holds a
// message for a module with no generator package.
func missingGenerator(errOut string) bool {
	for _, s := range []string{"provides package ", "providing package ", "does not contain package "} {
		if strings.Contains(errOut, s+configulatorCmd) {
			return true
		}
	}
	return false
}

// tableOf removes the heading that the generator prints above the table.
func tableOf(s string) string {
	if i := strings.Index(s, "\n|"); i >= 0 && strings.HasPrefix(s, "#") {
		return s[i+1:]
	}
	return s
}
