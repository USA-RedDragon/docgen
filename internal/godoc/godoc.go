// Package godoc writes Go doc comments as Markdown.
package godoc

import (
	"fmt"
	"go/doc/comment"
	"regexp"
	"strings"

	"github.com/USA-RedDragon/docgen/internal/load"
	"github.com/USA-RedDragon/docgen/internal/markdown"
	"github.com/USA-RedDragon/docgen/internal/types"
)

// Markdown returns the doc of name in pi as Markdown, or the package doc when
// name is "". A section other than "" selects the text below that heading.
func Markdown(pi *load.Package, name, section string) (string, error) {
	text, err := docText(pi, name)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(text) == "" {
		return "", fmt.Errorf("no doc comment for %s", describe(pi, name))
	}
	d := pi.Doc.Parser().Parse(text)
	if section != "" {
		var blocks []comment.Block
		in := false
		for _, b := range d.Content {
			if h, ok := b.(*comment.Heading); ok {
				if in {
					break
				}
				in = headingText(h) == section
				continue
			}
			if in {
				blocks = append(blocks, b)
			}
		}
		if !in && len(blocks) == 0 {
			return "", fmt.Errorf("no heading # %s in the doc of %s", section, describe(pi, name))
		}
		if len(blocks) == 0 {
			return "", fmt.Errorf("heading # %s in the doc of %s has no text", section, describe(pi, name))
		}
		d.Content = blocks
	}
	p := pi.Doc.Printer()
	p.DocLinkURL = func(l *comment.DocLink) string {
		c := *l
		if c.ImportPath == "" {
			c.ImportPath = pi.Path
		}
		return c.DefaultURL("https://pkg.go.dev")
	}
	p.HeadingLevel = 3
	p.HeadingID = func(*comment.Heading) string { return "" }
	return codeSpans(strings.TrimRight(string(p.Markdown(d)), "\n")), nil
}

func headingText(h *comment.Heading) string {
	var b strings.Builder
	for _, t := range h.Text {
		inlineText(&b, t)
	}
	return b.String()
}

func inlineText(b *strings.Builder, t comment.Text) {
	switch x := t.(type) {
	case comment.Plain:
		b.WriteString(string(x))
	case comment.Italic:
		b.WriteString(string(x))
	case *comment.Link:
		for _, y := range x.Text {
			inlineText(b, y)
		}
	case *comment.DocLink:
		for _, y := range x.Text {
			inlineText(b, y)
		}
	}
}

func describe(pi *load.Package, name string) string {
	if name == "" {
		return "package " + pi.Path
	}
	return name + " in " + pi.Path
}

func docText(pi *load.Package, name string) (string, error) {
	if name == "" {
		return pi.Doc.Doc, nil
	}
	typ, method, _ := strings.Cut(name, ".")
	for _, t := range pi.Doc.Types {
		if t.Name != typ {
			continue
		}
		if method == "" {
			return t.Doc, nil
		}
		for _, f := range t.Methods {
			if f.Name == method {
				return f.Doc, nil
			}
		}
	}
	if d, ok := funcDoc(pi, name); ok {
		return d, nil
	}
	if d, ok := types.ValueDoc(pi, name); ok {
		return d, nil
	}
	return "", fmt.Errorf("no doc for %s", describe(pi, name))
}

// funcDoc returns the doc of the package-level func name of pi, a constructor
// of a type included.
func funcDoc(pi *load.Package, name string) (string, bool) {
	for _, f := range pi.Doc.Funcs {
		if f.Name == name {
			return f.Doc, true
		}
	}
	for _, t := range pi.Doc.Types {
		for _, f := range t.Funcs {
			if f.Name == name {
				return f.Doc, true
			}
		}
	}
	return "", false
}

var (
	escapedSpan = regexp.MustCompile("\\\\`([^`\n]+?)\\\\`")
	escapedChar = regexp.MustCompile("\\\\([!-/:-@\\[-`{-~])")
)

// codeSpans turns text in backticks into code spans. A doc comment has no
// syntax for inline code, so the printer escapes the backticks. Code blocks are
// not escaped, so they stay as they are.
func codeSpans(md string) string {
	lines := strings.Split(md, "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, "\t") {
			continue
		}
		lines[i] = escapedSpan.ReplaceAllStringFunc(line, func(span string) string {
			inner := escapedSpan.FindStringSubmatch(span)[1]
			return markdown.CodeSpan(escapedChar.ReplaceAllString(inner, "$1"))
		})
	}
	return strings.Join(lines, "\n")
}
