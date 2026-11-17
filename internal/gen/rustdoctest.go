package gen

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/USA-RedDragon/docgen/internal/cargo"
	"github.com/USA-RedDragon/docgen/internal/gomod"
	"github.com/USA-RedDragon/docgen/internal/markdown"
)

// A doctestBatch is the doctests of one crate that the markers of a run show.
// One cargo test runs them all.
type doctestBatch struct {
	crate   string
	items   map[string]bool
	results map[string]docResult
}

// A docResult is the body or the error of one doctest marker.
type docResult struct {
	text string
	err  error
}

// batchOf returns the batch of crate, and adds its run to the plan when the
// batch is new.
func (rp *rustPlan) batchOf(p *plan, crate string) *doctestBatch {
	if b := rp.batches[crate]; b != nil {
		return b
	}
	b := &doctestBatch{crate: crate, items: map[string]bool{}, results: map[string]docResult{}}
	rp.batches[crate] = b
	p.cargoTool("rust-doctest\x00"+crate, func(ctx context.Context) (string, error) {
		b.run(ctx, p.gen)
		return "", nil
	})
	return b
}

func (b *doctestBatch) add(item string) { b.items[item] = true }

// A doctest is one line of the output of cargo test --doc: the file, the path
// of the item as rustdoc names it, the line of the opening fence, and the result.
type doctest struct {
	file   string
	path   string
	line   int
	result string
}

var doctestLine = regexp.MustCompile(`^test (.+?\.rs) - (?:(.*) )?\(line (\d+)\)(?: - compile(?: fail)?)?(?: \.\.\. (.+))?$`)

// parseDoctests returns the doctests in the output of cargo test --doc, which
// has a result on each line, or of cargo test --doc -- --list, which has the
// suffix ": test" and no result.
func parseDoctests(out string) []doctest {
	var tests []doctest
	for line := range strings.SplitSeq(out, "\n") {
		line = strings.TrimSuffix(strings.TrimSpace(line), ": test")
		if !strings.HasPrefix(line, "test ") {
			line = "test " + line
		}
		m := doctestLine.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		n, err := strconv.Atoi(m[3])
		if err != nil {
			continue
		}
		tests = append(tests, doctest{file: m[1], path: m[2], line: n, result: m[4]})
	}
	return tests
}

// itemPath returns the doctest path of the doctest= value item of a crate with
// the library name lib: the path in the crate, with no leading crate or lib
// name, and "" for the docs of the crate itself.
func itemPath(item, lib string) string {
	for _, root := range []string{"crate", lib} {
		if item == root {
			return ""
		}
		if rest, ok := strings.CutPrefix(item, root+"::"); ok {
			return rest
		}
	}
	return item
}

func (b *doctestBatch) fail(err error) {
	for item := range b.items {
		b.results[item] = docResult{err: err}
	}
}

// run runs the doctests of the items of the batch in one cargo test and sets
// the result of each item.
func (b *doctestBatch) run(ctx context.Context, g *Generator) {
	ws, pkg, err := g.cargoPackage(ctx, b.crate)
	if err != nil {
		b.fail(err)
		return
	}
	lib := pkg.Lib()
	if lib == nil {
		b.fail(fmt.Errorf("crate %s has no library, so it has no doctests", b.crate))
		return
	}
	paths := map[string]string{}
	filters := map[string]bool{}
	for item := range b.items {
		paths[item] = itemPath(item, strings.ReplaceAll(lib.Name, "-", "_"))
		filters[" - "+paths[item]+" (line "] = true
	}
	args := append([]string{"test", "--locked", "--doc", "--package", b.crate, "--"}, sortedKeys(filters)...)
	fmt.Fprintf(g.log, "docgen: running %d doctests of %s\n", len(b.items), b.crate)
	out, errOut, runErr := g.rust.runner.Run(ctx, ws.Root, args...)
	if errors.Is(runErr, cargo.ErrNotFound) {
		b.fail(runErr)
		return
	}
	tests := parseDoctests(out)
	var runFail error
	if runErr != nil {
		runFail = cargo.LockedError("cargo test --doc --package "+b.crate, runErr, strings.TrimSpace(out+"\n"+errOut))
		if len(tests) == 0 {
			b.fail(runFail)
			return
		}
	}
	var listed []string
	for item, path := range paths {
		var mine []doctest
		for _, t := range tests {
			if t.path == path {
				mine = append(mine, t)
			}
		}
		if len(mine) == 0 {
			if listed == nil {
				listed = g.listDoctests(ctx, ws, b.crate)
			}
			b.results[item] = docResult{err: noDoctestError(b.crate, item, listed)}
			continue
		}
		b.results[item] = b.show(ws, item, mine, runFail)
	}
}

// show returns the result of one item from its doctests, which all ran.
func (b *doctestBatch) show(ws *cargo.Workspace, item string, tests []doctest, runFail error) docResult {
	slices.SortFunc(tests, func(x, y doctest) int { return x.line - y.line })
	var blocks []string
	for _, t := range tests {
		switch {
		case strings.HasPrefix(t.result, "ignored"):
			return docResult{err: fmt.Errorf("the doctest at %s:%d of %s is ignored, so nothing checks it; remove ignore from its code block", t.file, t.line, item)}
		case t.result != "ok":
			if runFail == nil {
				runFail = fmt.Errorf("the doctest at %s:%d of %s: %s", t.file, t.line, item, t.result)
			}
			return docResult{err: runFail}
		}
		path := filepath.Join(ws.Root, filepath.FromSlash(t.file))
		if !gomod.Inside(ws.Root, path) {
			return docResult{err: fmt.Errorf("the doctest of %s is in %s, outside the Cargo workspace", item, t.file)}
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return docResult{err: err}
		}
		code, err := doctestCode(strings.Split(strings.ReplaceAll(string(src), "\r\n", "\n"), "\n"), t.line)
		if err != nil {
			return docResult{err: fmt.Errorf("doctest of %s at %s:%d: %w", item, t.file, t.line, err)}
		}
		blocks = append(blocks, markdown.Fence("rust", code))
	}
	return docResult{text: strings.Join(blocks, "\n\n")}
}

// listDoctests returns the paths of the doctests of crate, or nil when cargo
// cannot list them.
func (g *Generator) listDoctests(ctx context.Context, ws *cargo.Workspace, crate string) []string {
	out, _, err := g.rust.runner.Run(ctx, ws.Root, "test", "--locked", "--doc", "--package", crate, "--", "--list")
	if err != nil {
		return nil
	}
	seen := map[string]bool{}
	for _, t := range parseDoctests(out) {
		seen[t.path] = true
	}
	var paths []string
	for p := range seen {
		if p == "" {
			p = "crate"
		}
		paths = append(paths, p)
	}
	sort.Strings(paths)
	return paths
}

func noDoctestError(crate, item string, listed []string) error {
	if len(listed) == 0 {
		return fmt.Errorf("crate %s has no doctest for %q", crate, item)
	}
	return fmt.Errorf("crate %s has no doctest for %q; the items with a doctest are: %s", crate, item, strings.Join(listed, ", "))
}

// docText returns a line of a doc comment with its comment syntax removed, and
// false for a line that is not a doc comment line.
func docText(line string) (string, bool) {
	t := strings.TrimLeft(line, " \t")
	for _, prefix := range []string{"///", "//!"} {
		if rest, ok := strings.CutPrefix(t, prefix); ok && !strings.HasPrefix(rest, "/") {
			return strings.TrimPrefix(rest, " "), true
		}
	}
	if rest, ok := strings.CutPrefix(t, "*"); ok && !strings.HasPrefix(rest, "/") {
		return strings.TrimPrefix(rest, " "), true
	}
	for _, prefix := range []string{"/**", "/*!"} {
		if rest, ok := strings.CutPrefix(t, prefix); ok {
			return strings.TrimPrefix(rest, " "), true
		}
	}
	return "", false
}

// fenceOf returns the fence character and length of a Markdown code fence line,
// and 0 for any other line.
func fenceOf(text string) (rune, int) {
	t := strings.TrimLeft(text, " ")
	for _, c := range []string{"`", "~"} {
		if n := len(t) - len(strings.TrimLeft(t, c)); n >= 3 {
			return rune(c[0]), n
		}
	}
	return 0, 0
}

// doctestCode returns the code of the doctest whose opening fence is at line
// (from 1) of the lines of a Rust file, as rustdoc shows it: with the hidden
// lines removed.
func doctestCode(lines []string, line int) (string, error) {
	if line < 1 || line > len(lines) {
		return "", errors.New("the line is outside the file")
	}
	open, ok := docText(lines[line-1])
	if !ok {
		return "", errors.New("the line is not in a /// or //! doc comment; docgen reads only those and /** */ comments")
	}
	fence, width := fenceOf(open)
	if fence == 0 {
		return "", errors.New("the line is not a code fence")
	}
	indent := len(open) - len(strings.TrimLeft(open, " "))
	var code []string
	for _, l := range lines[line:] {
		text, ok := docText(l)
		if !ok {
			break
		}
		if f, n := fenceOf(text); f == fence && n >= width && strings.Trim(strings.TrimSpace(text), string(f)) == "" {
			return visibleCode(code)
		}
		code = append(code, strings.TrimPrefix(text, strings.Repeat(" ", min(indent, len(text)-len(strings.TrimLeft(text, " "))))))
	}
	return "", errors.New("the code fence is not closed")
}

// visibleCode returns the lines of a doctest without the lines that rustdoc
// hides: a line that starts with "# " or is just "#", with ## shown as #.
func visibleCode(lines []string) (string, error) {
	var out []string
	for _, l := range lines {
		t := strings.TrimLeft(l, " \t")
		pad := l[:len(l)-len(t)]
		switch {
		case t == "#" || strings.HasPrefix(t, "# "):
		case strings.HasPrefix(t, "##"):
			out = append(out, pad+t[1:])
		default:
			out = append(out, l)
		}
	}
	code := strings.Join(out, "\n")
	if strings.TrimSpace(code) == "" {
		return "", errors.New("the code block has no visible line")
	}
	return code, nil
}
