package gen

import (
	"errors"
	"fmt"
	gotypes "go/types"
	"regexp"
	"strings"

	"github.com/USA-RedDragon/docgen/internal/defaults"
	"github.com/USA-RedDragon/docgen/internal/goprint"
	"github.com/USA-RedDragon/docgen/internal/load"
	"github.com/USA-RedDragon/docgen/internal/markdown"
	"github.com/USA-RedDragon/docgen/internal/types"
)

// configTable returns the table of fields with the defaults defs, with the print=
// value printAs.
func configTable(vs *valueSet, pi *load.Package, fields []types.Field, defs map[*gotypes.Var][]defaults.Default, printAs string) (string, error) {
	var b strings.Builder
	b.WriteString("| Field | Type | Default | Description |\n|---|---|---|---|\n")
	for _, f := range fields {
		var def []string
		for _, d := range defs[f.Var] {
			text, err := defaultText(vs, pi, d, printAs)
			if err != nil {
				return "", err
			}
			def = append(def, text)
		}
		if len(def) == 0 {
			if hasDefaultPhrase(f.Doc) {
				def = []string{"not machine-readable; see the description"}
			} else {
				def = []string{"zero value"}
			}
		}
		fmt.Fprintf(&b, "| `%s` | `%s` | %s | %s |\n", f.Name, markdown.Cell(goprint.Type(f.Type, pi.Pkg.Types)), markdown.Cell(strings.Join(def, " or ")), markdown.Cell(f.Doc))
	}
	return b.String(), nil
}

// defaultText returns the source of the default d and its value.
func defaultText(vs *valueSet, pi *load.Package, d defaults.Default, printAs string) (string, error) {
	src := markdown.CodeSpan(d.Source)
	if d.Value == nil {
		return src, nil
	}
	v, err := defaultValue(vs, pi, d, printAs)
	if err != nil {
		return "", fmt.Errorf("the default %s: %w", d.Source, err)
	}
	if v == "" || v == d.Source {
		return src, nil
	}
	return src + " (" + v + ")", nil
}

// defaultValue returns the text of the value of the default d with the print=
// value printAs, as docgen:const prints it, and "" when the source of d is all
// that the column holds: a literal is its own value, except with a fmt verb, and a
// value with no constant has no name.
func defaultValue(vs *valueSet, pi *load.Package, d defaults.Default, printAs string) (string, error) {
	if printAs == goprint.PrintName {
		if d.Literal {
			return "", nil
		}
		v, err := types.ConstantName(pi, d.Type, d.Value)
		if errors.Is(err, types.ErrNoConstant) {
			return "", nil
		}
		return v, err
	}
	var expr string
	if goprint.NeedsRun(d.Type, printAs) {
		var ok bool
		if expr, ok = vs.constExpr(pi, d.Type, d.Value); !ok {
			return "", nil
		}
	} else if d.Literal && (printAs == "" || printAs == goprint.PrintRaw) {
		return "", nil
	}
	return vs.text(pi, valueRef{expr, printAs}, d.Type, d.Value)
}

var defaultPhrase = regexp.MustCompile(`(?i)\b(0|nil|zero|empty|an? empty \w+|a zero \w+) means\b|defaults to|by default`)

func hasDefaultPhrase(d string) bool {
	return defaultPhrase.MatchString(strings.Join(strings.Fields(d), " "))
}
