package gen

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/USA-RedDragon/docgen/internal/cargo"
	"github.com/USA-RedDragon/docgen/internal/markdown"
)

const (
	argColumns = "columns"
	metaPrefix = "metadata"
	// tomlDatetime is the one key of the table that cargo metadata writes for
	// a TOML date or time.
	tomlDatetime = "$__toml_private_datetime"
)

var metaSegment = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

// A metaKey is one columns= key of a docgen:crates marker: the key path under
// package.metadata.
type metaKey struct {
	text string
	path []string
}

// header returns the column header, which is the last segment of the key.
func (k metaKey) header() string { return k.path[len(k.path)-1] }

// parseMetaKeys reads the comma-separated keys of columns=. Each is
// metadata.<key>[.<key>...] with bare TOML keys, and none is empty.
func parseMetaKeys(list string) ([]metaKey, error) {
	var keys []metaKey
	for _, text := range strings.Split(list, ",") {
		segs := strings.Split(strings.TrimSpace(text), ".")
		bad := segs[0] != metaPrefix || len(segs) < 2
		for _, s := range segs {
			bad = bad || !metaSegment.MatchString(s)
		}
		if bad {
			return nil, fmt.Errorf("columns= key %q is not metadata.<key>[.<key>...], with keys of letters, digits, - and _", text)
		}
		keys = append(keys, metaKey{text: strings.Join(segs, "."), path: segs[1:]})
	}
	return keys, nil
}

// cell returns the table cell of the key for p: "" when p has no such key, a
// scalar as text, and an array of scalars with its items joined by ", ". A
// table, and an array with an item that is not a scalar, are errors.
func (k metaKey) cell(p *cargo.Package) (string, error) {
	v, ok := p.MetadataValue(k.path)
	if !ok {
		return "", nil
	}
	if items, ok := v.([]any); ok {
		cells := make([]string, len(items))
		for i, it := range items {
			s, ok := scalarText(it)
			if !ok {
				return "", fmt.Errorf("crate %s: %s has %s as item %d, not a scalar", p.Name, k.text, valueKind(it), i+1)
			}
			cells[i] = markdown.Cell(s)
		}
		return strings.Join(cells, ", "), nil
	}
	s, ok := scalarText(v)
	if !ok {
		return "", fmt.Errorf("crate %s: %s is %s, not a scalar or an array of scalars", p.Name, k.text, valueKind(v))
	}
	return markdown.Cell(s), nil
}

// scalarText returns the text of a string, boolean, number or date of TOML or
// JSON, and whether v is one.
func scalarText(v any) (string, bool) {
	switch v := v.(type) {
	case string:
		return v, true
	case bool:
		return strconv.FormatBool(v), true
	case int64:
		return strconv.FormatInt(v, 10), true
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64), true
	case time.Time:
		return v.Format(time.RFC3339Nano), true
	case fmt.Stringer:
		return v.String(), true
	case map[string]any:
		if d, ok := v[tomlDatetime].(string); ok && len(v) == 1 {
			return d, true
		}
	}
	return "", false
}

func valueKind(v any) string {
	switch v.(type) {
	case map[string]any:
		return "a table"
	case []any:
		return "an array"
	}
	return "a value that is not a scalar"
}
