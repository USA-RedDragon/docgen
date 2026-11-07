package cli

import (
	"os"
	"path/filepath"

	"github.com/USA-RedDragon/docgen/internal/gomod"
)

// findRoot returns the nearest directory at or above dir with a .docgen.yaml
// file or a .git entry, or dir when there is none.
func findRoot(dir string) string {
	root, ok := gomod.Up(dir, func(d string) bool {
		for _, name := range []string{configFile, ".git"} {
			if _, err := os.Stat(filepath.Join(d, name)); err == nil {
				return true
			}
		}
		return false
	})
	if !ok {
		return dir
	}
	return root
}

func abs(root, p string) string {
	if filepath.IsAbs(p) {
		return filepath.Clean(p)
	}
	return filepath.Join(root, p)
}
