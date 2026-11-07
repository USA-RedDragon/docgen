package excerpt

import (
	"fmt"
	"strconv"
	"strings"
)

// yamlExcerpt returns the block of the key at the dotted path sym, such as
// jobs.test, with the key line and without the blank lines at its end. A
// last segment key[A:B] or key[A] selects named items of the list at key: the
// items whose own mapping has that name key.
func yamlExcerpt(src []byte, sym string) (string, error) {
	path, items := sym, ""
	if j := strings.IndexByte(sym, '['); j >= 0 && strings.HasSuffix(sym, "]") {
		path, items = sym[:j], sym[j+1:len(sym)-1]
	}
	lines := strings.Split(string(src), "\n")
	start, end, indent := 0, len(lines), -1
	at := -1
	for _, key := range strings.Split(path, ".") {
		at = yamlFindKey(lines[start:end], key, indent) + start
		if at < start {
			return "", fmt.Errorf("no key %s in the YAML file", path)
		}
		indent = yamlIndent(lines[at])
		start, end = at+1, yamlBlockEnd(lines, at+1, end, indent)
	}
	if items != "" {
		return yamlItems(lines, start, end, sym, items)
	}
	for end > at+1 && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}
	return dedent(lines[at:end]), nil
}

// yamlFindKey returns the index in lines of the line of key among the children
// of a block with the given indent, or -1.
func yamlFindKey(lines []string, key string, indent int) int {
	child := -1
	for i, l := range lines {
		t := strings.TrimLeft(l, " ")
		if t == "" || strings.HasPrefix(t, "#") {
			continue
		}
		ind := yamlIndent(l)
		if ind <= indent {
			return -1
		}
		if child < 0 {
			child = ind
		}
		if ind == child && (t == key+":" || strings.HasPrefix(t, key+": ")) {
			return i
		}
	}
	return -1
}

func yamlItems(lines []string, start, end int, sym, items string) (string, error) {
	var starts []int
	var names []string
	for i := start; i < end; i++ {
		t := strings.TrimLeft(lines[i], " ")
		if !yamlIsItem(t) {
			continue
		}
		if len(starts) > 0 && yamlIndent(lines[i]) != yamlIndent(lines[starts[0]]) {
			continue
		}
		starts = append(starts, i)
		names = append(names, yamlItemName(lines, i, end))
	}
	a, b, err := yamlItemRange(names, sym, items)
	if err != nil {
		return "", err
	}
	stop := end
	if b+1 < len(starts) {
		stop = starts[b+1]
	}
	return dedent(lines[starts[a]:yamlItemsEnd(lines, starts[a], stop)]), nil
}

// yamlItemsEnd returns the line after the last line of the items from line
// first up to line stop. The blank lines at the end are not in the items, and
// neither is a comment at or left of the indent of the items, which is part of
// the text after them.
func yamlItemsEnd(lines []string, first, stop int) int {
	ind := yamlIndent(lines[first])
	for stop > first+1 {
		l := lines[stop-1]
		t := strings.TrimLeft(l, " ")
		if t != "" && (t[0] != '#' || yamlIndent(l) > ind) {
			break
		}
		stop--
	}
	return stop
}

// yamlItemRange returns the first and last index in names that items selects.
// items is one name or two names joined by ":", and a name can hold ":" of its
// own, so each reading of items is tried. More than one match is an error.
func yamlItemRange(names []string, sym, items string) (int, int, error) {
	a, b, n := -1, -1, 0
	try := func(first, last string) {
		if x, y, ok := yamlFindItems(names, first, last); ok {
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
	switch n {
	case 0:
		return 0, 0, fmt.Errorf("no items %s in the YAML file", sym)
	case 1:
		return a, b, nil
	}
	return 0, 0, fmt.Errorf("items %s in the YAML file are ambiguous: the item names hold \":\"", sym)
}

func yamlFindItems(names []string, first, last string) (int, int, bool) {
	if first == "" || last == "" {
		return 0, 0, false
	}
	for a, n := range names {
		if n != first {
			continue
		}
		for b := a; b < len(names); b++ {
			if names[b] == last {
				return a, b, true
			}
		}
		break
	}
	return 0, 0, false
}

// yamlItemName returns the value of the name key of the mapping of the list
// item at line at, or "" when the item has none.
func yamlItemName(lines []string, at, end int) string {
	ind := yamlIndent(lines[at])
	rest := strings.TrimLeft(lines[at][ind+1:], " ")
	col := len(lines[at]) - len(rest)
	if rest == "" || strings.HasPrefix(rest, "#") {
		col = -1
	} else if n, ok := strings.CutPrefix(rest, "name: "); ok {
		return yamlScalar(n)
	}
	for i := at + 1; i < end; i++ {
		t := strings.TrimLeft(lines[i], " ")
		if t == "" || t[0] == '#' {
			continue
		}
		k := yamlIndent(lines[i])
		if k <= ind {
			break
		}
		if col < 0 {
			col = k
		}
		if n, ok := strings.CutPrefix(t, "name: "); ok && k == col {
			return yamlScalar(n)
		}
	}
	return ""
}

// yamlScalar returns the value of a one-line YAML scalar, without its quotes
// and without a comment after it.
func yamlScalar(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	switch s[0] {
	case '\'':
		var b strings.Builder
		for i := 1; i < len(s); i++ {
			if s[i] != '\'' {
				b.WriteByte(s[i])
				continue
			}
			if i+1 == len(s) || s[i+1] != '\'' {
				break
			}
			b.WriteByte('\'')
			i++
		}
		return b.String()
	case '"':
		for i := 1; i < len(s); i++ {
			switch s[i] {
			case '\\':
				i++
			case '"':
				if v, err := strconv.Unquote(s[:i+1]); err == nil {
					return v
				}
				return s[1:i]
			}
		}
		return s
	}
	if s[0] == '#' {
		return ""
	}
	if i := strings.Index(s, " #"); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}

// yamlBlockEnd returns the line after the block of the key at indent, whose
// lines start at from. A list can be at the indent of its key. A comment at
// or left of indent is in the block only when more of the block follows it.
func yamlBlockEnd(lines []string, from, end, indent int) int {
	list := yamlSameIndentList(lines, from, end, indent)
	comment := -1
	for i := from; i < end; i++ {
		t := strings.TrimLeft(lines[i], " ")
		if t == "" {
			continue
		}
		ind := yamlIndent(lines[i])
		switch {
		case ind > indent, list && ind == indent && yamlIsItem(t):
			comment = -1
		case t[0] == '#':
			if comment < 0 {
				comment = i
			}
		case comment >= 0:
			return comment
		default:
			return i
		}
	}
	if comment >= 0 {
		return comment
	}
	return end
}

// yamlSameIndentList reports whether the first line of content from line from
// is a list item at indent.
func yamlSameIndentList(lines []string, from, end, indent int) bool {
	for i := from; i < end; i++ {
		if t := strings.TrimLeft(lines[i], " "); t != "" && t[0] != '#' {
			return yamlIndent(lines[i]) == indent && yamlIsItem(t)
		}
	}
	return false
}

func yamlIsItem(t string) bool {
	return t == "-" || strings.HasPrefix(t, "- ")
}

func yamlIndent(l string) int {
	return len(l) - len(strings.TrimLeft(l, " "))
}
