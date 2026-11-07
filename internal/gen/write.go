package gen

import (
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/USA-RedDragon/docgen/internal/diff"
)

// diffOf returns the diff of each of files, in path order.
func (g *Generator) diffOf(files []outFile) string {
	sorted := slices.Clone(files)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].path < sorted[j].path })
	var b strings.Builder
	for i, f := range sorted {
		if i > 0 && sorted[i-1].path == f.path {
			continue
		}
		rel, _ := filepath.Rel(g.mod.Root, f.path)
		b.WriteString(diff.Unified(filepath.ToSlash(rel), f.old, f.data))
	}
	return b.String()
}

func writeFiles(files []outFile) error {
	for _, f := range files {
		if err := os.MkdirAll(filepath.Dir(f.path), 0o755); err != nil { //nolint:gosec // published with the pages
			return err
		}
		if err := os.WriteFile(f.path, f.data, 0o644); err != nil { //nolint:gosec // published with the pages
			return err
		}
	}
	return nil
}
