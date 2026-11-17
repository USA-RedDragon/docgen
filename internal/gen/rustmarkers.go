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

// A rustConstHandler is a docgen:const marker with crate=: the value of the
// const or static name of the crate, printed by a test on a copy of the crate,
// or with print=raw its expression as written.
type rustConstHandler struct {
	valueArgs
	crate string
	name  string
	rp    *rustPlan
	value *rustValue
	feat  featureArgs
}

func (h *rustConstHandler) Bind(a *marker.Args) {
	h.feat.bind(a)
	h.crate = bindCrate(a)
	h.name = a.Required(argName)
	h.printAs, _ = a.Optional(argPrint)
	a.NonEmpty(argPrint)
	if err := rustsrc.CheckPrint(h.printAs); err != nil {
		a.Fail(err.Error())
	}
	h.bindCode(a)
	a.Inline()
}

func (h *rustConstHandler) plan(p *plan) {
	h.printAs = p.gen.rustPrint(h.printAs)
	h.rp = p.rust()
	if h.printAs == rustsrc.PrintRaw {
		h.rp.readCrate(h.crate)
		return
	}
	h.value = h.rp.value(&rustValue{crate: h.crate, name: h.name, printAs: h.printAs, feats: p.gen.features(&h.feat)})
}

func (h *rustConstHandler) fill(context.Context, job) (string, []outFile, error) {
	_, f, err := h.rp.lookup(h.crate, h.name)
	if err != nil {
		return "", nil, err
	}
	if err := checkValueItem(f, h.name); err != nil {
		return "", nil, err
	}
	if h.value == nil {
		return h.inline(f.Item.Expr), nil, nil
	}
	if h.value.err != nil {
		return "", nil, h.value.err
	}
	text := h.value.text
	if h.printAs == rustsrc.PrintName {
		if text, err = oneName(text, h.name, f.Item.Type); err != nil {
			return "", nil, err
		}
	}
	return h.inline(text), nil, nil
}

// oneName returns the one name of the print=name output out.
func oneName(out, name, typ string) (string, error) {
	switch names := strings.Split(out, "\n"); {
	case out == "":
		return "", fmt.Errorf("no other const of type %s has the value of %s", typ, name)
	case len(names) > 1:
		return "", fmt.Errorf("the consts %s each have the value of %s", strings.Join(names, " and "), name)
	}
	return out, nil
}

// A rustDocHandler is a docgen:doc marker with crate=: the doc of the item or
// module name of the crate, or the crate doc, or the text below the heading
// section.
type rustDocHandler struct {
	crate   string
	name    string
	section string
	rp      *rustPlan
}

func (h *rustDocHandler) Bind(a *marker.Args) {
	h.crate = bindCrate(a)
	h.name, _ = a.Optional(argName)
	h.section, _ = a.Optional("section")
	a.NonEmpty(argName, "section")
}

func (h *rustDocHandler) plan(p *plan) {
	h.rp = p.rust()
	h.rp.readCrate(h.crate)
}

func (h *rustDocHandler) fill(context.Context, job) (string, []outFile, error) {
	_, f, err := h.rp.lookup(h.crate, h.name)
	if err != nil {
		return "", nil, err
	}
	doc, what := f.Module.Doc, "crate "+h.crate
	if f.Item != nil {
		doc = f.Item.Doc
	}
	if f.Path != "" {
		what = f.Path + " in crate " + h.crate
	}
	body, err := rustsrc.Markdown(doc, h.section, what)
	if err != nil {
		return "", nil, err
	}
	return body, nil, nil
}

// A rustErrorsHandler is a docgen:errors marker with crate=: the table of the
// variants of an error enum, or of an error struct, that derives
// thiserror::Error.
type rustErrorsHandler struct {
	crate string
	typ   string
	rp    *rustPlan
}

func (h *rustErrorsHandler) Bind(a *marker.Args) {
	h.crate = bindCrate(a)
	h.typ = a.Required(argType)
}

func (h *rustErrorsHandler) plan(p *plan) {
	h.rp = p.rust()
	h.rp.readCrate(h.crate)
}

func (h *rustErrorsHandler) fill(context.Context, job) (string, []outFile, error) {
	_, f, err := h.rp.lookup(h.crate, h.typ)
	if err != nil {
		return "", nil, err
	}
	it := f.Item
	if it == nil || it.Kind != rustsrc.KindEnum && it.Kind != rustsrc.KindStruct {
		return "", nil, fmt.Errorf("%s is a %s, not an enum or struct", h.typ, f.Kind())
	}
	if !derives(it, "Error") {
		return "", nil, fmt.Errorf("%s has no #[derive(thiserror::Error)]", h.typ)
	}
	var b strings.Builder
	b.WriteString("| Error | Message | Source | Meaning |\n|---|---|---|---|\n")
	if it.Kind == rustsrc.KindStruct {
		b.WriteString(errorRow(it.Name, it.Error, nil, it.Fields, it.Doc))
		return b.String(), nil, nil
	}
	if len(it.Variants) == 0 {
		return "", nil, fmt.Errorf("enum %s has no variants", h.typ)
	}
	for _, v := range it.Variants {
		b.WriteString(errorRow(it.Name+"::"+v.Name, v.Error, it.Error, v.Fields, v.Doc))
	}
	return b.String(), nil, nil
}

// errorRow returns the row of one error: its name, the message of attr or else
// of the enum attr outer, the source field and the first sentence of doc.
func errorRow(name string, attr, outer *rustsrc.ErrorAttr, fields []rustsrc.Field, doc string) string {
	if attr == nil {
		attr = outer
	}
	msg := ""
	switch {
	case attr == nil:
	case attr.Transparent && len(fields) == 1:
		msg = "the message of " + markdown.CodeSpan(fields[0].Type)
	case attr.Format != nil:
		msg = markdown.CodeSpan(*attr.Format)
	default:
		msg = markdown.CodeSpan(attr.Text)
	}
	var src []string
	for _, f := range fields {
		switch {
		case f.From:
			src = append(src, markdown.CodeSpan(f.Type)+", with From")
		case f.Source || f.Name == "source":
			src = append(src, markdown.CodeSpan(f.Type))
		}
	}
	return fmt.Sprintf("| %s | %s | %s | %s |\n", markdown.CodeSpan(name), markdown.Cell(msg), markdown.Cell(strings.Join(src, ", ")), markdown.Cell(firstSentence(doc)))
}

// derives reports whether it derives a trait whose path ends in name.
func derives(it *rustsrc.Item, name string) bool {
	for _, d := range it.Derives {
		if d == name || strings.HasSuffix(d, "::"+name) {
			return true
		}
	}
	return false
}

// A rustConfigulatorHandler is a docgen:configulator marker with crate=: the
// table that configulator-rs prints for the config type typ, from a test on a
// copy of the crate.
type rustConfigulatorHandler struct {
	crate string
	typ   string
	table rustsrc.Table
	value *rustValue
	feat  featureArgs
}

func (h *rustConfigulatorHandler) Bind(a *marker.Args) {
	h.feat.bind(a)
	h.crate = bindCrate(a)
	h.typ = a.Required(argType)
	h.table = rustsrc.Table{EnvSep: "_", FlagSep: "."}
	for _, o := range []struct {
		arg string
		to  *string
	}{{"env", &h.table.EnvPrefix}, {"envsep", &h.table.EnvSep}, {"flagsep", &h.table.FlagSep}} {
		if v, ok := a.Optional(o.arg); ok {
			*o.to = v
		}
	}
	a.NonEmpty("env", "envsep", "flagsep")
}

func (h *rustConfigulatorHandler) plan(p *plan) {
	h.value = p.rust().value(&rustValue{crate: h.crate, name: h.typ, table: &h.table, feats: p.gen.features(&h.feat)})
}

func (h *rustConfigulatorHandler) fill(context.Context, job) (string, []outFile, error) {
	if h.value.err != nil {
		return "", nil, h.value.err
	}
	return tableOf(h.value.text), nil, nil
}

var configulatorCrate = regexp.MustCompile(`(?:^|[,\s])crate\s*=\s*"([^"]+)"`)

// configStruct returns the error of a type that is not a struct that derives
// configulator's Config, and else sets the configulator path of t.
func configStruct(f rustsrc.Found, name string, t *rustsrc.Table) error {
	it := f.Item
	if it == nil || it.Kind != rustsrc.KindStruct {
		return fmt.Errorf("%s is a %s, not a struct", name, f.Kind())
	}
	if !derives(it, "Config") {
		return fmt.Errorf("%s has no #[derive(Config)] of configulator-rs", name)
	}
	t.Crate = "::configulator"
	for _, c := range it.Configulator {
		if m := configulatorCrate.FindStringSubmatch(c); m != nil {
			t.Crate = m[1]
			if !strings.HasPrefix(t.Crate, "::") && !strings.HasPrefix(t.Crate, "crate") {
				t.Crate = "::" + t.Crate
			}
		}
	}
	return nil
}
