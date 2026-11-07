package excerpt

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/USA-RedDragon/docgen/internal/markdown"
)

func parseRange(s string) (int, int, error) {
	a, b, ok := strings.Cut(s, "-")
	x, err1 := strconv.Atoi(a)
	y, err2 := strconv.Atoi(b)
	if !ok || err1 != nil || err2 != nil || x < 1 || y < x {
		return 0, 0, fmt.Errorf("lines=%s is not a range a-b with 1 <= a <= b", s)
	}
	return x, y, nil
}

// Lines returns the lines of src in the range spec ("a-b") as a code block,
// without the lines that open or close a region. path is the file of src and
// rel is its name in errors. old is the code block on the page, or "" when the
// page has none. An old block that differs from the result is an error, unless
// it is the same range with the region lines. A range of a Go file that does
// not hold whole syntax nodes is an error, and so is a range of region lines
// only.
func Lines(path, rel string, src []byte, spec, old string) (string, error) {
	return lineRange(path, rel, src, spec, old, false)
}

// LinesRaw is Lines with the lines that open or close a region in the result. A
// range of a Go file can start or end at such a line.
func LinesRaw(path, rel string, src []byte, spec, old string) (string, error) {
	return lineRange(path, rel, src, spec, old, true)
}

func lineRange(path, rel string, src []byte, spec, old string, raw bool) (string, error) {
	a, b, err := parseRange(spec)
	if err != nil {
		return "", err
	}
	lines := fileLines(src)
	if b > len(lines) {
		return "", fmt.Errorf("%s has %d lines, the range ends at %d", rel, len(lines), b)
	}
	lang := Lang(path)
	all := lines[a-1 : b]
	kept := withoutRegionLines(all)
	if len(kept) == 0 && !raw {
		return "", fmt.Errorf("lines %d-%d of %s hold only region lines", a, b, rel)
	}
	text := kept
	if raw {
		text = all
	}
	if allBlank(text) {
		return "", fmt.Errorf("lines %d-%d of %s hold no text", a, b, rel)
	}
	want := markdown.Fence(lang, dedent(text))
	if old != "" && old != markdown.Fence(lang, dedent(kept)) && old != markdown.Fence(lang, dedent(all)) {
		if oa, ob, ok := findExcerpt(lines, old, raw); ok {
			return "", fmt.Errorf("range drifted: the excerpt in the page is now at lines=%d-%d of %s", oa, ob, rel)
		}
		return "", fmt.Errorf("lines %d-%d of %s changed; check the range, then empty the marker body and run docgen update", a, b, rel)
	}
	if lang == "go" {
		if err := checkGoRange(path, src, a, b, raw); err != nil {
			return "", err
		}
	}
	return want, nil
}

// fileLines returns the lines of src. The end of the last line starts no line.
func fileLines(src []byte) []string {
	if len(src) == 0 {
		return nil
	}
	return strings.Split(strings.TrimSuffix(string(src), "\n"), "\n")
}

func withoutRegionLines(lines []string) []string {
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		if !regionLine.MatchString(l) {
			out = append(out, l)
		}
	}
	return out
}

// findExcerpt returns the range of lines with the code block body. Unless raw
// is true, a region line inside the range does not count as a line of the
// excerpt.
func findExcerpt(lines []string, body string, raw bool) (int, int, bool) {
	bl := strings.Split(body, "\n")
	if len(bl) < 3 {
		return 0, 0, false
	}
	inner := bl[1 : len(bl)-1]
	n := len(inner)
	var kept []string
	var at []int
	for i, l := range lines {
		if raw || !regionLine.MatchString(l) {
			kept = append(kept, l)
			at = append(at, i+1)
		}
	}
	for i := 0; i+n <= len(kept); i++ {
		if dedent(kept[i:i+n]) == strings.Join(inner, "\n") {
			return at[i], at[i+n-1], true
		}
	}
	return 0, 0, false
}

// checkGoRange fails unless lines a to b hold whole syntax nodes that share
// one parent: one or more declarations, statements, specs or fields, each
// with its doc comment. With raw true, region lines and blank lines at the ends
// of the range do not count.
func checkGoRange(path string, src []byte, a, b int, raw bool) error {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, src, parser.ParseComments)
	if err != nil {
		return err
	}
	line := func(p token.Pos) int { return fset.Position(p).Line }
	starts, ends := map[int][]ast.Node{}, map[int][]ast.Node{}
	var stack []ast.Node
	ast.Inspect(f, func(n ast.Node) bool {
		if n == nil {
			stack = stack[:len(stack)-1]
			return true
		}
		var parent ast.Node
		if len(stack) > 0 {
			parent = stack[len(stack)-1]
		}
		stack = append(stack, n)
		switch x := n.(type) {
		case ast.Decl, ast.Stmt, ast.Spec, *ast.Field, *ast.KeyValueExpr:
			starts[line(n.Pos())] = append(starts[line(n.Pos())], parent)
			ends[line(n.End())] = append(ends[line(n.End())], parent)
			var d *ast.CommentGroup
			switch y := x.(type) {
			case *ast.FuncDecl:
				d = y.Doc
			case *ast.GenDecl:
				d = y.Doc
			case *ast.Field:
				d = y.Doc
			case *ast.ValueSpec:
				d = y.Doc
			case *ast.TypeSpec:
				d = y.Doc
			}
			if d != nil {
				starts[line(d.Pos())] = append(starts[line(d.Pos())], parent)
			}
		}
		return true
	})
	if raw {
		lines := fileLines(src)
		skip := func(l string) bool { return strings.TrimSpace(l) == "" || regionLine.MatchString(l) }
		for a < b && len(starts[a]) == 0 && skip(lines[a-1]) {
			a++
		}
		for a < b && len(ends[b]) == 0 && skip(lines[b-1]) {
			b--
		}
	}
	for _, p := range starts[a] {
		for _, q := range ends[b] {
			if p == q {
				return nil
			}
		}
	}
	return fmt.Errorf("lines %d-%d of %s do not hold whole declarations or statements; the range may have drifted", a, b, filepath.Base(path))
}
