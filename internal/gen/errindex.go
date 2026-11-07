package gen

import (
	"fmt"
	"sort"
	"strings"

	"github.com/USA-RedDragon/docgen/internal/load"
	"github.com/USA-RedDragon/docgen/internal/markdown"
	"github.com/USA-RedDragon/docgen/internal/types"
)

type errInfo struct {
	name string
	ip   string
	doc  string
	by   []string
}

// errorIndex returns the errors that can occur in a doc comment of pi, by package
// name and error name: those of pi, of the packages of the module that pi
// imports, and of the root package. When two packages have one name, the key is
// for the first in that order.
func (j job) errorIndex(pi *load.Package) (map[string]*errInfo, error) {
	deps, err := j.loaded.ModuleImports(pi)
	if err != nil {
		return nil, err
	}
	pkgs := append([]*load.Package{pi}, deps...)
	if root := j.loaded.ByPath(j.mod.Path); root != nil && root.Pkg != nil {
		pkgs = append(pkgs, root)
	}
	idx := map[string]*errInfo{}
	for _, pi := range pkgs {
		for _, e := range types.Errors(pi) {
			key := pi.Pkg.Name + "." + e.Name
			if idx[key] == nil {
				idx[key] = &errInfo{name: e.Name, ip: pi.Path, doc: e.Doc}
			}
		}
	}
	return idx, nil
}

// namedErrors returns the errors of idx in the doc comments of the constructors
// and methods of t, by key of idx. Each holds the functions with the error in
// their doc comment.
func namedErrors(pi *load.Package, t types.Type, idx map[string]*errInfo) map[string]*errInfo {
	type fn struct{ name, doc string }
	fns := make([]fn, 0, len(t.Doc.Funcs)+len(t.Doc.Methods))
	for _, f := range t.Doc.Funcs {
		fns = append(fns, fn{f.Name, f.Doc})
	}
	for _, f := range t.Doc.Methods {
		fns = append(fns, fn{t.Name + "." + f.Name, f.Doc})
	}
	found := map[string]*errInfo{}
	for _, f := range fns {
		for _, w := range docWords(f.doc) {
			key := w
			if !strings.Contains(w, ".") {
				key = pi.Pkg.Name + "." + w
			}
			e := idx[key]
			if e == nil {
				continue
			}
			if found[key] == nil {
				found[key] = &errInfo{name: e.name, ip: e.ip, doc: e.doc}
			}
			fe := found[key]
			if len(fe.by) == 0 || fe.by[len(fe.by)-1] != f.name {
				fe.by = append(fe.by, f.name)
			}
		}
	}
	return found
}

func errorTable(found map[string]*errInfo) string {
	keys := make([]string, 0, len(found))
	for k := range found {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	b.WriteString("| Error | Returned by | Meaning |\n|---|---|---|\n")
	for _, k := range keys {
		e := found[k]
		by := make([]string, 0, len(e.by))
		for _, f := range e.by {
			by = append(by, "`"+f+"`")
		}
		fmt.Fprintf(&b, "| [`%s`](%s) | %s | %s |\n", k, markdown.PkgLink(e.ip, e.name), strings.Join(by, ", "), markdown.Cell(firstSentence(e.doc)))
	}
	return b.String()
}

func docWords(d string) []string {
	var out []string
	for _, w := range strings.FieldsFunc(d, func(r rune) bool {
		return r != '.' && r != '_' && (r < '0' || r > '9') && (r < 'a' || r > 'z') && (r < 'A' || r > 'Z')
	}) {
		w = strings.Trim(w, ".")
		if w != "" {
			out = append(out, w)
		}
	}
	return out
}

// firstSentence returns the text of s up to the first period and space that is
// not part of an abbreviation.
func firstSentence(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	for from := 0; ; {
		i := strings.Index(s[from:], ". ")
		if i < 0 {
			return s
		}
		from += i + 1
		if !endsInAbbreviation(s[:from]) {
			return s[:from]
		}
	}
}

// endsInAbbreviation reports whether the last word of s is an abbreviation that
// ends in a period.
func endsInAbbreviation(s string) bool {
	word := s[strings.LastIndexByte(s, ' ')+1:]
	switch strings.ToLower(strings.TrimLeft(word, "([{\"'")) {
	case "e.g.", "i.e.", "etc.", "cf.", "vs.":
		return true
	}
	return false
}
