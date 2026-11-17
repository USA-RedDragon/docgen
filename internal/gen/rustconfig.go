package gen

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/USA-RedDragon/docgen/internal/markdown"
	"github.com/USA-RedDragon/docgen/internal/marker"
	"github.com/USA-RedDragon/docgen/internal/rustsrc"
)

// A rustConfigHandler is a docgen:config marker with crate=: the table of the
// fields of the struct typ of the crate, with the defaults that a test on a
// copy of the crate prints.
type rustConfigHandler struct {
	crate   string
	typ     string
	printAs string
	feat    featureArgs
	value   *rustValue
}

func (h *rustConfigHandler) Bind(a *marker.Args) {
	h.feat.bind(a)
	h.crate = bindCrate(a)
	h.typ = a.Required(argType)
	h.printAs, _ = a.Optional(argPrint)
	a.NonEmpty(argPrint)
	if err := rustsrc.CheckPrint(h.printAs); err != nil {
		a.Fail(err.Error())
	}
}

func (h *rustConfigHandler) plan(p *plan) {
	h.printAs = p.gen.rustPrint(h.printAs)
	h.value = p.rust().value(&rustValue{crate: h.crate, name: h.typ, printAs: h.printAs, config: true, feats: p.gen.features(&h.feat)})
}

func (h *rustConfigHandler) fill(context.Context, job) (string, []outFile, error) {
	if h.value.err != nil {
		return "", nil, h.value.err
	}
	var b strings.Builder
	b.WriteString("| Field | Type | Default | Description |\n|---|---|---|---|\n")
	for _, row := range h.value.rows {
		def, err := row.cell(h.printAs)
		if err != nil {
			return "", nil, fmt.Errorf("the default of %s: %w", row.key, err)
		}
		fmt.Fprintf(&b, "| `%s` | `%s` | %s | %s |\n", markdown.Cell(row.key), markdown.Cell(row.field.Type), markdown.Cell(def), markdown.Cell(row.field.Doc))
	}
	return b.String(), nil, nil
}

// A configRow is one row of the table of a config struct.
type configRow struct {
	field rustsrc.Field
	// key is the name of the field in a config file.
	key string
	// src is the default as the source writes it, and "" when it does not.
	src string
	// has reports whether the field has a default.
	has bool
	// req prints the value of the default, and is nil for a field with no
	// default and for print=raw with a default that the source writes.
	req *rustsrc.Request
}

// configRequests sets the rows of v from its struct, and returns the requests
// that print their defaults.
func (rp *rustPlan) configRequests(v *rustValue) ([]*rustsrc.Request, error) {
	c, f, err := rp.lookup(v.crate, v.name)
	if err != nil {
		return nil, err
	}
	it, err := configItem(f, v.name)
	if err != nil {
		return nil, err
	}
	rows, err := configRows(c.crate, it)
	if err != nil {
		return nil, err
	}
	var reqs []*rustsrc.Request
	for _, row := range rows {
		if row.has && (v.printAs != rustsrc.PrintRaw || row.src == "") {
			req := row.request(f, it, v.printAs)
			if v.printAs == rustsrc.PrintName {
				req.Names = constCandidates(c.crate, row.field.Type, f.Module, nil)
			}
			row.req = req
			reqs = append(reqs, req)
		}
	}
	v.rows = rows
	return reqs, nil
}

// configItem returns the struct of f, or the error of a type that docgen:config
// cannot read: not a struct, a struct with no named fields and a generic
// struct.
func configItem(f rustsrc.Found, name string) (*rustsrc.Item, error) {
	it := f.Item
	switch {
	case it == nil || it.Kind != rustsrc.KindStruct:
		return nil, fmt.Errorf("%s is a %s, not a struct", name, f.Kind())
	case it.Style != "named":
		return nil, fmt.Errorf("%s is a %s struct, which has no named fields", name, it.Style)
	case it.Generics:
		return nil, fmt.Errorf("%s is generic", it.Name)
	}
	return it, nil
}

// configRows returns the rows of the struct it: each field that is pub,
// or each field when the struct derives Deserialize, other than a field that
// serde skips.
func configRows(c *rustsrc.Crate, it *rustsrc.Item) ([]*configRow, error) {
	var rename string
	if it.Serde != nil {
		rename = it.Serde.RenameAll
	}
	all := derives(it, "Deserialize")
	var rows []*configRow
	for _, fl := range it.Fields {
		if fl.Serde != nil && fl.Serde.Skip || fl.Vis != "pub" && !all {
			continue
		}
		key, err := serdeKey(fl, rename)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", it.Name, err)
		}
		row := &configRow{field: fl, key: key}
		row.src, row.has = defaultSource(c, it, fl)
		rows = append(rows, row)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("%s has no row: it has no pub field and does not derive Deserialize", it.Name)
	}
	return rows, nil
}

// defaultSource returns the default of the field fl of it as the source
// writes it, which is "" when the source has no such text, and whether the
// field has a default.
func defaultSource(c *rustsrc.Crate, it *rustsrc.Item, fl rustsrc.Field) (string, bool) {
	switch s := fl.Serde; {
	case s != nil && s.Default != nil && *s.Default == "":
		return "Default::default()", true
	case s != nil && s.Default != nil:
		return *s.Default + "()", true
	}
	if it.Serde != nil && it.Serde.Default != nil && *it.Serde.Default != "" {
		return "", true
	}
	impl, ok := defaultImpl(c, it)
	if !ok && (it.Serde == nil || it.Serde.Default == nil) {
		return "", false
	}
	return impl.Fields[fl.Name], true
}

// defaultImpl returns the impl Default of it, and reports whether it has one:
// a derive, or an impl in a module of the crate for a type with its name.
func defaultImpl(c *rustsrc.Crate, it *rustsrc.Item) (rustsrc.DefaultImpl, bool) {
	for _, m := range c.Modules {
		for _, d := range m.DefaultImpls {
			if d.Type == it.Name {
				return d, true
			}
		}
	}
	return rustsrc.DefaultImpl{}, derives(it, "Default")
}

// request returns the request that prints the default of the row for the
// struct it of f.
func (row *configRow) request(f rustsrc.Found, it *rustsrc.Item, printAs string) *rustsrc.Request {
	member := rustMember(row.field.Name)
	self := "<super::" + it.Name + " as std::default::Default>::default()"
	var expr string
	scope := false
	switch s := row.field.Serde; {
	case s != nil && s.Default != nil && *s.Default == "":
		expr, scope = "<"+row.field.Type+" as std::default::Default>::default()", true
	case s != nil && s.Default != nil:
		expr, scope = scopedPath(*s.Default)+"()", true
	case it.Serde != nil && it.Serde.Default != nil && *it.Serde.Default != "":
		expr, scope = scopedPath(*it.Serde.Default)+"()."+member, true
	default:
		expr = self + "." + member
	}
	cfg := append(f.Cfg(), row.field.Cfg...)
	return &rustsrc.Request{Module: f.Module, Name: f.Path + "." + row.field.Name, Expr: expr, Print: printAs, Cfg: cfg, Scope: scope, Soft: true}
}

// scopedPath returns the path p of a serde attribute, written in the module of
// a struct, as the test module inside it names it.
func scopedPath(p string) string {
	switch {
	case p == "self" || strings.HasPrefix(p, "self::"):
		return "super" + strings.TrimPrefix(p, "self")
	case p == "super" || strings.HasPrefix(p, "super::"):
		return "super::" + p
	}
	return p
}

// cell returns the Default column of the row for the print= value printAs.
func (row *configRow) cell(printAs string) (string, error) {
	if !row.has {
		if hasDefaultPhrase(row.field.Doc) {
			return "not machine-readable; see the description", nil
		}
		return "no default", nil
	}
	value := ""
	if row.req != nil {
		if row.req.Err != nil {
			return "", row.req.Err
		}
		var err error
		if value, err = row.value(printAs); err != nil {
			return "", err
		}
	}
	src := row.src
	switch {
	case src == "" && value == "":
		return markdown.CodeSpan("Default::default()"), nil
	case src == "":
		return markdown.CodeSpan(value), nil
	case value == "" || value == src:
		return markdown.CodeSpan(src), nil
	}
	return markdown.CodeSpan(src) + " (" + value + ")", nil
}

// value returns the printed value of the default of the row, and "" when it
// has none: a type with no Display and no Debug impl, or with print=name no
// const with the value.
func (row *configRow) value(printAs string) (string, error) {
	text := row.req.Text
	switch {
	case printAs == rustsrc.PrintName:
		switch names := strings.Split(text, "\n"); {
		case text == "":
			return "", nil
		case len(names) > 1:
			return "", fmt.Errorf("the consts %s each have the value", strings.Join(names, " and "))
		}
	case text == rustsrc.NoValue:
		return "", nil
	}
	return text, nil
}

// serdeKey returns the name of the field fl in a config file: the serde
// rename of the field, else the name with the rename_all case of its struct,
// else the name.
func serdeKey(fl rustsrc.Field, renameAll string) (string, error) {
	if fl.Serde != nil && fl.Serde.Rename != "" {
		return fl.Serde.Rename, nil
	}
	if renameAll == "" {
		return fl.Name, nil
	}
	words := strings.Split(fl.Name, "_")
	title := func(w string) string {
		if w == "" {
			return w
		}
		return strings.ToUpper(w[:1]) + w[1:]
	}
	var out []string
	switch renameAll {
	case "lowercase":
		return strings.ToLower(fl.Name), nil
	case "UPPERCASE":
		return strings.ToUpper(fl.Name), nil
	case "snake_case":
		return fl.Name, nil
	case "SCREAMING_SNAKE_CASE":
		return strings.ToUpper(fl.Name), nil
	case "kebab-case":
		return strings.ReplaceAll(fl.Name, "_", "-"), nil
	case "SCREAMING-KEBAB-CASE":
		return strings.ToUpper(strings.ReplaceAll(fl.Name, "_", "-")), nil
	case casePascal, caseCamel:
		for i, w := range words {
			if i == 0 && renameAll == caseCamel {
				out = append(out, w)
				continue
			}
			out = append(out, title(w))
		}
		return strings.Join(out, ""), nil
	}
	return "", fmt.Errorf("serde rename_all = %q is not one of %s", renameAll, strings.Join(renameAllCases(), ", "))
}

const (
	casePascal = "PascalCase"
	caseCamel  = "camelCase"
)

// renameAllCases returns the cases of a serde rename_all.
func renameAllCases() []string {
	return []string{"lowercase", "UPPERCASE", casePascal, caseCamel, "snake_case", "SCREAMING_SNAKE_CASE", "kebab-case", "SCREAMING-KEBAB-CASE"}
}

var rustKeyword = regexp.MustCompile(`^(abstract|as|async|await|become|box|break|const|continue|do|dyn|else|enum|extern|false|final|fn|for|gen|if|impl|in|let|loop|macro|match|mod|move|mut|override|priv|pub|ref|return|static|struct|trait|true|try|type|typeof|unsafe|unsized|use|virtual|where|while|yield)$`)

// rustMember returns the field name as an expression names it, with r# for a
// keyword.
func rustMember(name string) string {
	if rustKeyword.MatchString(name) {
		return "r#" + name
	}
	return name
}
