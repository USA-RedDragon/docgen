package gen

import (
	"context"
	"errors"
	"strings"

	"github.com/USA-RedDragon/docgen/internal/markdown"
	"github.com/USA-RedDragon/docgen/internal/marker"
)

// A textHandler is a docgen:text marker: the template parts with each variable
// replaced by its value, inline in the page, or as a code block in the language
// code when code is set.
type textHandler struct {
	noPlan
	parts []part
	code  string
}

func (h *textHandler) Bind(a *marker.Args) {
	var err error
	if h.parts, err = parseTemplate(a.Required("text")); err != nil {
		a.Fail("text= " + err.Error())
	}
	var ok bool
	if h.code, ok = a.Optional("code"); !ok {
		a.Inline()
	}
	a.NonEmpty("code")
}

func (h *textHandler) fill(_ context.Context, j job) (string, []outFile, error) {
	text, err := expand(h.parts, j.vars)
	if err != nil {
		return "", nil, err
	}
	if h.code == "" {
		return text, nil, nil
	}
	return markdown.Fence(h.code, text), nil, nil
}

func (h *textHandler) varNames() []string {
	var names []string
	for _, p := range h.parts {
		if p.isVar {
			names = append(names, p.text)
		}
	}
	return names
}

// A part is one piece of a template: literal text, or the name of a variable when
// isVar is true.
type part struct {
	text  string
	isVar bool
}

// parseTemplate returns the parts of a template. {name} is a variable, {{ is a
// literal {, the two characters \n are a line ending, and the two characters \\
// are a backslash.
func parseTemplate(s string) ([]part, error) {
	var parts []part
	var lit strings.Builder
	flush := func() {
		if lit.Len() > 0 {
			parts = append(parts, part{text: lit.String()})
			lit.Reset()
		}
	}
	for i := 0; i < len(s); {
		rest := s[i:]
		switch {
		case strings.HasPrefix(rest, `\\`):
			lit.WriteByte('\\')
			i += 2
		case strings.HasPrefix(rest, `\n`):
			lit.WriteByte('\n')
			i += 2
		case strings.HasPrefix(rest, "{{"):
			lit.WriteByte('{')
			i += 2
		case rest[0] == '{':
			end := strings.IndexByte(rest, '}')
			if end < 0 {
				return nil, errors.New("has a { with no }; {{ writes a literal {")
			}
			if end == 1 {
				return nil, errors.New("has a {} with no name")
			}
			flush()
			parts = append(parts, part{text: rest[1:end], isVar: true})
			i += end + 1
		default:
			lit.WriteByte(rest[0])
			i++
		}
	}
	flush()
	return parts, nil
}

// expand returns the text of parts with the values of vars. The text of a value
// stays as it is.
func expand(parts []part, vars map[string]string) (string, error) {
	var b strings.Builder
	for _, p := range parts {
		if !p.isVar {
			b.WriteString(p.text)
			continue
		}
		v, err := varValue(vars, p.text)
		if err != nil {
			return "", err
		}
		b.WriteString(v)
	}
	return b.String(), nil
}
