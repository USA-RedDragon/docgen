package excerpt

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/USA-RedDragon/docgen/internal/marker"
)

var regionLine = regexp.MustCompile(`^\s*(?://+|#+|/\*+|\*+|<!--+|--+|;+|%+)?\s*docgen:(?:region (\S+?)|end)\s*(?:\*+/|--+>)?\s*$`)

// A Mark is a line that opens or closes a region: the mark is the whole content
// of the line apart from comment syntax. Line starts at 1, and Name is the name of
// the region, or "" for a docgen:end.
type Mark struct {
	Line int
	Name string
}

// Marks returns the marks of the text of the file at path, which has LF line
// endings. In a Markdown file, a mark in code or in the body of a marker is text.
func Marks(path, text string) []Mark {
	if !strings.Contains(text, "docgen:") {
		return nil
	}
	if filepath.Ext(path) == ".md" {
		text = marker.Prose(text)
	}
	var out []Mark
	for i, l := range strings.Split(text, "\n") {
		if !strings.Contains(l, "docgen:") {
			continue
		}
		if m := regionLine.FindStringSubmatch(l); m != nil {
			out = append(out, Mark{Line: i + 1, Name: m[1]})
		}
	}
	return out
}

// Region returns the lines between the mark that opens the region name in the
// file at path and the mark after it, which must be a docgen:end. The first region
// of that name counts. A region with no text is an error.
func Region(path string, src []byte, name string) (string, error) {
	marks := Marks(path, string(src))
	for i, m := range marks {
		if m.Name != name {
			continue
		}
		if i+1 == len(marks) || marks[i+1].Name != "" {
			return "", fmt.Errorf("region %s has no docgen:end", name)
		}
		lines := strings.Split(string(src), "\n")[m.Line : marks[i+1].Line-1]
		if allBlank(lines) {
			return "", fmt.Errorf("region %s is empty", name)
		}
		return dedent(lines), nil
	}
	return "", fmt.Errorf("no docgen:region %s", name)
}

func allBlank(lines []string) bool {
	for _, l := range lines {
		if strings.TrimSpace(l) != "" {
			return false
		}
	}
	return true
}

// DropRegionLines returns text without the lines that open or close a region:
// the lines where the mark is the whole content apart from comment syntax.
func DropRegionLines(text string) string {
	if !strings.Contains(text, "docgen:") {
		return text
	}
	lines := strings.Split(text, "\n")
	out := lines[:0]
	for _, l := range lines {
		if !regionLine.MatchString(l) {
			out = append(out, l)
		}
	}
	return strings.Join(out, "\n")
}
