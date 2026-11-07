package gen

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/mod/modfile"
)

// moduleField returns the value of the line field of the go.mod at p.
func moduleField(p, field string) (string, error) {
	b, err := os.ReadFile(p)
	if err != nil {
		return "", err
	}
	f, err := modfile.ParseLax(p, b, nil)
	if err != nil {
		return "", err
	}
	var v string
	switch field {
	case "go":
		if f.Go != nil {
			v = f.Go.Version
		}
	case "toolchain":
		v = toolchainLine(f)
	case "module":
		if f.Module != nil {
			v = f.Module.Mod.Path
		}
	default:
		return "", fmt.Errorf("field=%s is not go, toolchain or module", field)
	}
	if v == "" {
		return "", fmt.Errorf("%s has no %s line", filepath.Base(filepath.Dir(p))+"/go.mod", field)
	}
	return v, nil
}

// toolchainLine returns the value of the toolchain line of f. ParseLax leaves
// f.Toolchain nil, and only the syntax of the file holds the line.
func toolchainLine(f *modfile.File) string {
	for _, s := range f.Syntax.Stmt {
		if l, ok := s.(*modfile.Line); ok && len(l.Token) == 2 && l.Token[0] == "toolchain" {
			return l.Token[1]
		}
	}
	return ""
}
