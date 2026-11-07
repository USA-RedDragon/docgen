package excerpt

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
)

// An objPath is a parsed symbol= of a JSON or TOML file: the keys of a dotted
// path, and the item selector of a last segment key[items].
type objPath struct {
	keys     []string
	items    string
	hasItems bool
}

// parseObjPath parses sym. A backslash puts the next character, such as a dot
// or a bracket, in the key. The text between the first other "[" and the last
// "]" is the item selector.
func parseObjPath(sym string) (objPath, error) {
	var p objPath
	var key strings.Builder
	for i := 0; i < len(sym); i++ {
		switch c := sym[i]; c {
		case '\\':
			if i+1 < len(sym) {
				i++
			}
			key.WriteByte(sym[i])
		case '.':
			p.keys = append(p.keys, key.String())
			key.Reset()
		case '[':
			if !strings.HasSuffix(sym, "]") || i == len(sym)-1 {
				return p, fmt.Errorf("no ] at the end of %s", sym)
			}
			p.items, p.hasItems = sym[i+1:len(sym)-1], true
			i = len(sym)
		default:
			key.WriteByte(c)
		}
	}
	p.keys = append(p.keys, key.String())
	for _, k := range p.keys {
		if k == "" {
			return p, fmt.Errorf("empty key in %s", sym)
		}
	}
	if p.hasItems && p.items == "" {
		return p, fmt.Errorf("no item name in %s", sym)
	}
	return p, nil
}

// itemRange returns the first and last index in names that items selects. items
// is one item or two items joined by ":", and each is a name, or else an index
// from 0. A name can hold ":" of its own, so each reading of items is tried.
// More than one match, and a name that more than one item has, are errors.
func itemRange(names []string, sym, items, format string) (int, int, error) {
	a, b, n := -1, -1, 0
	var dup string
	try := func(first, last string) {
		x, dx := itemIndex(names, first)
		y, dy := itemIndex(names, last)
		switch {
		case dx != "" || dy != "":
			dup = dx + dy
		case x >= 0 && y >= x:
			a, b = x, y
			n++
		}
	}
	try(items, items)
	for i, c := range items {
		if c == ':' {
			try(items[:i], items[i+1:])
		}
	}
	switch {
	case n > 1:
		return 0, 0, fmt.Errorf("items %s in the %s file are ambiguous: the item names hold \":\"", sym, format)
	case n == 1:
		return a, b, nil
	case dup != "":
		return 0, 0, fmt.Errorf("items %s in the %s file are ambiguous: more than one item has the name %q", sym, format, dup)
	}
	return 0, 0, fmt.Errorf("no items %s in the %s file", sym, format)
}

// itemIndex returns the index of the item with name s, or else the index s,
// or -1. The second result is s when more than one item has the name s.
func itemIndex(names []string, s string) (int, string) {
	if s == "" {
		return -1, ""
	}
	at := -1
	for i, n := range names {
		if n != s {
			continue
		}
		if at >= 0 {
			return -1, s
		}
		at = i
	}
	if at >= 0 {
		return at, ""
	}
	if i, err := strconv.Atoi(s); err == nil && i >= 0 && i < len(names) && s == strconv.Itoa(i) {
		return i, ""
	}
	return -1, ""
}

// objSource returns src with LF line endings and without a byte order mark.
func objSource(src []byte) []byte {
	return bytes.TrimPrefix(Source(src), []byte("\ufeff"))
}

// cutDedent returns the text of src from offset start to offset end, with the
// common indent of its lines removed. Text before start on its line counts as
// indent, so the lines after the first keep their place.
func cutDedent(src []byte, start, end int) string {
	ls := lineStart(src, start)
	lead := strings.Map(func(r rune) rune {
		if r == '\t' {
			return r
		}
		return ' '
	}, string(src[ls:start]))
	text := lead + string(src[start:end])
	return dedent(strings.Split(text, "\n"))
}
