// Package excerpt cuts parts out of source files: Go symbols, YAML blocks, C
// definitions, regions and line ranges.
package excerpt

import (
	"bytes"
	"path/filepath"
	"strings"
)

const goExt = ".go"

// Source returns the contents of a file as the functions of this package read
// them, with LF line endings.
func Source(b []byte) []byte {
	return bytes.ReplaceAll(b, []byte("\r\n"), []byte("\n"))
}

// Lang returns the language name of a code block for the file at path, or ""
// when there is none.
func Lang(path string) string {
	switch ext := filepath.Ext(path); ext {
	case ".h":
		return "c"
	case ".yml":
		return "yaml"
	case ".py":
		return "python"
	case goExt, ".c", ".sh", ".yaml", ".json", ".js", ".html", ".css", ".lua", ".toml":
		return ext[1:]
	}
	return ""
}

func dedent(lines []string) string {
	prefix := ""
	first := true
	for _, l := range lines {
		if strings.TrimSpace(l) == "" {
			continue
		}
		ws := l[:len(l)-len(strings.TrimLeft(l, " \t"))]
		if first {
			prefix, first = ws, false
			continue
		}
		for !strings.HasPrefix(ws, prefix) {
			prefix = prefix[:len(prefix)-1]
		}
	}
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = strings.TrimPrefix(l, prefix)
		if strings.TrimSpace(out[i]) == "" {
			out[i] = ""
		}
	}
	return strings.Join(out, "\n")
}
