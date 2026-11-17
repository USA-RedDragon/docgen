package rustsrc

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/USA-RedDragon/docgen/internal/cargo"
	"github.com/USA-RedDragon/docgen/internal/excerpt"
)

// SymbolCrate is the symbol of the inner documentation of a file, the lines
// that start with //!. It needs raw=true.
const SymbolCrate = "crate"

// maxListed is the most names that the error of a missing symbol lists.
const maxListed = 20

// Symbols holds the items of one Rust file, as the helper reads the file alone:
// with no module tree and no crate.
type Symbols struct {
	File string `json:"file"`
	// Error is the parse error of the file, or "".
	Error string `json:"error"`
	// CrateDoc holds the first and last line of each inner doc attribute of the
	// file.
	CrateDoc [][2]int `json:"crate_doc"`
	Symbols  []Symbol `json:"symbols"`
}

// A Symbol is an item that symbol= can name.
type Symbol struct {
	// Kind is fn, struct, enum, union, trait, type, const, static, macro, mod
	// or impl.
	Kind string `json:"kind"`
	// Name is the first of Names, the spelling that errors print.
	Name string `json:"name"`
	// Names holds each spelling that selects the symbol.
	Names []string `json:"names"`
	// Line and EndLine are the first and last line of the item, from its
	// first attribute or doc comment, counted from 1.
	Line    int `json:"line"`
	EndLine int `json:"end_line"`
	// Docs holds the first and last line of each doc comment of the item.
	Docs [][2]int `json:"docs"`
	Cfg  []string `json:"cfg"`
}

// ReadSymbols runs the helper bin on each file with r.Exec, and returns one
// Symbols for each file, in order.
func ReadSymbols(ctx context.Context, r *cargo.Runner, bin string, files []string) ([]*Symbols, error) {
	out, errOut, err := r.Exec(ctx, filepath.Dir(bin), bin, append([]string{"--symbols"}, files...)...)
	if err != nil {
		return nil, fmt.Errorf("%s: %w\n%s", helperName, err, strings.TrimSpace(errOut))
	}
	var syms []*Symbols
	if err := json.Unmarshal([]byte(out), &syms); err != nil {
		return nil, fmt.Errorf("%s output: %w", helperName, err)
	}
	if len(syms) != len(files) {
		return nil, fmt.Errorf("%s output has %d files, not %d", helperName, len(syms), len(files))
	}
	return syms, nil
}

// Excerpt returns the whole lines of src, the text of the file that s holds,
// for the symbol sym. The lines start at the first attribute of the item and
// end with its last token, and a doc comment of the item is not in them unless
// raw is true. The lines of an item in an impl, a trait or an inline module
// lose the indent that they all share. With raw true, SymbolCrate is the lines
// of the inner doc comments of the file.
func (s *Symbols) Excerpt(src []byte, sym string, raw bool) (string, error) {
	name := filepath.Base(s.File)
	if s.Error != "" {
		return "", errors.New(s.Error)
	}
	lines := strings.Split(string(src), "\n")
	if sym == SymbolCrate {
		return s.crateDoc(lines, name, raw)
	}
	var found []Symbol
	want := stripSpace(sym)
	for _, c := range s.Symbols {
		if slices.ContainsFunc(c.Names, func(n string) bool { return stripSpace(n) == want }) {
			found = append(found, c)
		}
	}
	switch len(found) {
	case 0:
		return "", s.missing(sym, name)
	case 1:
		return cut(lines, found[0], raw)
	}
	var where []string
	distinct := map[string]bool{}
	for _, c := range found {
		distinct[c.Name] = true
		w := fmt.Sprintf("%s at line %d", c.Name, c.Line)
		if c.Kind != "impl" {
			w = c.Kind + " " + w
		}
		if len(c.Cfg) > 0 {
			w += " with cfg(" + strings.Join(c.Cfg, ") and cfg(") + ")"
		}
		where = append(where, w)
	}
	hint := "they have the same name, so symbol= cannot tell them apart; use region= or lines="
	if len(distinct) == len(found) {
		hint = "use one of the names above as symbol="
	}
	return "", fmt.Errorf("symbol=%q is more than one item in %s: %s; %s", sym, name, strings.Join(where, ", "), hint)
}

func (s *Symbols) crateDoc(lines []string, name string, raw bool) (string, error) {
	if !raw {
		return "", fmt.Errorf("symbol=%s is the //! documentation of %s and needs raw=true", SymbolCrate, name)
	}
	var out []string
	for _, r := range s.CrateDoc {
		if r[0] < 1 || r[1] > len(lines) || r[0] > r[1] {
			return "", fmt.Errorf("the //! lines %d-%d are outside %s", r[0], r[1], name)
		}
		out = append(out, lines[r[0]-1:r[1]]...)
	}
	if len(out) == 0 {
		return "", fmt.Errorf("%s has no //! documentation", name)
	}
	return strings.Join(out, "\n"), nil
}

// cut returns the lines of c, with no doc comment lines unless raw is true.
func cut(lines []string, c Symbol, raw bool) (string, error) {
	if c.Line < 1 || c.EndLine > len(lines) || c.Line > c.EndLine {
		return "", fmt.Errorf("%s is at lines %d-%d, outside the file", c.Name, c.Line, c.EndLine)
	}
	var out []string
	for n := c.Line; n <= c.EndLine; n++ {
		if !raw && inRanges(c.Docs, n) {
			continue
		}
		out = append(out, lines[n-1])
	}
	return excerpt.Dedent(out), nil
}

func inRanges(ranges [][2]int, n int) bool {
	return slices.ContainsFunc(ranges, func(r [2]int) bool { return r[0] <= n && n <= r[1] })
}

// missing returns the error for a symbol that no item has.
func (s *Symbols) missing(sym, name string) error {
	var names []string
	for _, c := range s.Symbols {
		if !slices.Contains(names, c.Name) {
			names = append(names, c.Name)
		}
	}
	if len(names) == 0 {
		return fmt.Errorf("no item %s in %s: the file has no items", sym, name)
	}
	slices.Sort(names)
	more := ""
	if len(names) > maxListed {
		more = fmt.Sprintf(" and %d more", len(names)-maxListed)
		names = names[:maxListed]
	}
	return fmt.Errorf("no item %s in %s; the symbols are %s%s", sym, name, strings.Join(names, ", "), more)
}

func stripSpace(s string) string {
	return strings.Join(strings.Fields(s), "")
}
