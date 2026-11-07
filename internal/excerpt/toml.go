package excerpt

import (
	"fmt"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

const tomlMaxDepth = 64

type tomlKind int

const (
	tomlKeyValue tomlKind = iota
	tomlTable
	tomlArrayTable
)

// A tomlEntry is a table header or a key/value pair of a TOML file. key is the
// full key: for a pair, the key of its table and then its own key. first and
// last are the lines of the entry, from 0.
type tomlEntry struct {
	kind        tomlKind
	key         []string
	first, last int
	str         string
	isStr       bool
}

// tomlExcerpt returns the lines of the table, array of tables item or key at
// the dotted path sym. A table holds its key/value pairs and its sub-tables up
// to the next other table header. A last segment key[A:B] or key[A] selects
// items of the array of tables at key by the "name" key of each item, or by
// index.
func tomlExcerpt(src []byte, sym string) (string, error) {
	p, err := parseObjPath(sym)
	if err != nil {
		return "", err
	}
	text := string(objSource(src))
	entries, err := tomlScan(text)
	if err != nil {
		return "", err
	}
	lines := strings.Split(text, "\n")
	path := strings.Join(p.keys, ".")
	var heads []int
	kind := tomlKeyValue
	for i, e := range entries {
		if slices.Equal(e.key, p.keys) {
			heads = append(heads, i)
			kind = e.kind
		}
	}
	for _, e := range entries {
		n := len(e.key)
		if e.kind == tomlArrayTable && n < len(p.keys) && slices.Equal(e.key, p.keys[:n]) {
			at := strings.Join(e.key, ".")
			return "", fmt.Errorf("%s in the TOML file is an array of tables: select items with %s[name]", at, at)
		}
	}
	if p.hasItems {
		if len(heads) == 0 || kind != tomlArrayTable {
			if len(heads) == 0 && !tomlHasPrefix(entries, p.keys) {
				return "", fmt.Errorf("no key %s in the TOML file", path)
			}
			return "", fmt.Errorf("%s in the TOML file is not an array of tables", path)
		}
		return tomlItems(entries, lines, heads, sym, p)
	}
	switch {
	case len(heads) > 1 && kind == tomlArrayTable:
		return "", fmt.Errorf("%s in the TOML file is an array of tables: select items with %s[name]", path, path)
	case len(heads) > 1:
		return "", fmt.Errorf("%s is more than once in the TOML file", path)
	case len(heads) == 1 && kind == tomlArrayTable:
		return "", fmt.Errorf("%s in the TOML file is an array of tables: select items with %s[name]", path, path)
	case len(heads) == 1 && kind == tomlTable:
		e := entries[heads[0]]
		return tomlLines(lines, e.first, tomlTableEnd(entries, heads[0], lines)), nil
	case len(heads) == 1:
		e := entries[heads[0]]
		return tomlLines(lines, e.first, e.last+1), nil
	}
	for _, e := range entries {
		n := len(e.key)
		if n < len(p.keys) && slices.Equal(e.key, p.keys[:n]) {
			if e.kind == tomlKeyValue {
				return "", fmt.Errorf("%s is inside the value of %s in the TOML file", path, strings.Join(e.key, "."))
			}
		}
	}
	if tomlHasPrefix(entries, p.keys) {
		return "", fmt.Errorf("%s in the TOML file is a table of dotted keys or sub-tables with no header [%s]: select a key in it", path, path)
	}
	return "", fmt.Errorf("no key %s in the TOML file", path)
}

func tomlHasPrefix(entries []tomlEntry, keys []string) bool {
	for _, e := range entries {
		if len(e.key) > len(keys) && slices.Equal(e.key[:len(keys)], keys) {
			return true
		}
	}
	return false
}

func tomlItems(entries []tomlEntry, lines []string, heads []int, sym string, p objPath) (string, error) {
	names := make([]string, len(heads))
	ends := make([]int, len(heads))
	for n, h := range heads {
		ends[n] = tomlTableEnd(entries, h, lines)
		for _, e := range entries[h+1:] {
			if e.kind != tomlKeyValue {
				break
			}
			if e.isStr && len(e.key) == len(p.keys)+1 && e.key[len(p.keys)] == "name" {
				names[n] = e.str
			}
		}
	}
	a, b, err := itemRange(names, sym, p.items, "TOML")
	if err != nil {
		return "", err
	}
	return tomlLines(lines, entries[heads[a]].first, ends[b]), nil
}

// tomlTableEnd returns the line after the table of the header at entry h: the
// line of the next header that is not of a sub-table, or the end of the file.
func tomlTableEnd(entries []tomlEntry, h int, lines []string) int {
	key := entries[h].key
	for _, e := range entries[h+1:] {
		if e.kind == tomlKeyValue {
			continue
		}
		if len(e.key) <= len(key) || !slices.Equal(e.key[:len(key)], key) {
			return e.first
		}
	}
	return len(lines)
}

// tomlLines returns lines from first up to end, without the blank and comment
// lines at the end, with the common indent removed.
func tomlLines(lines []string, first, end int) string {
	for end > first+1 {
		t := strings.TrimLeft(lines[end-1], " \t")
		if t != "" && t[0] != '#' {
			break
		}
		end--
	}
	return dedent(lines[first:end])
}

// A tomlScanner reads the entries of a TOML file: where each entry starts and
// ends. Its syntax checks go only as far as those bounds depend on them.
type tomlScanner struct {
	s     string
	i     int
	line  int
	depth int
}

func tomlScan(s string) ([]tomlEntry, error) {
	sc := &tomlScanner{s: s}
	var out []tomlEntry
	var table []string
	for {
		sc.space()
		if sc.i >= len(s) {
			return out, nil
		}
		switch s[sc.i] {
		case '\n':
			sc.i++
			sc.line++
			continue
		case '#':
			sc.comment()
			continue
		}
		first := sc.line
		if s[sc.i] == '[' {
			e, err := sc.header()
			if err != nil {
				return nil, err
			}
			table = e.key
			out = append(out, e)
			continue
		}
		e, err := sc.keyValue()
		if err != nil {
			return nil, err
		}
		e.key = append(slices.Clip(table), e.key...)
		e.first, e.last = first, sc.line
		if err := sc.lineEnd(); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
}

func (sc *tomlScanner) errorf(format string, a ...any) error {
	return fmt.Errorf("TOML file line %d: %s", sc.line+1, fmt.Sprintf(format, a...))
}

func (sc *tomlScanner) space() {
	for sc.i < len(sc.s) && (sc.s[sc.i] == ' ' || sc.s[sc.i] == '\t') {
		sc.i++
	}
}

func (sc *tomlScanner) comment() {
	for sc.i < len(sc.s) && sc.s[sc.i] != '\n' {
		sc.i++
	}
}

// fill skips white space, newlines and comments.
func (sc *tomlScanner) fill() {
	for {
		sc.space()
		switch {
		case sc.i >= len(sc.s):
			return
		case sc.s[sc.i] == '\n':
			sc.i++
			sc.line++
		case sc.s[sc.i] == '#':
			sc.comment()
		default:
			return
		}
	}
}

// lineEnd reads the rest of the line after an entry: white space and a comment.
func (sc *tomlScanner) lineEnd() error {
	sc.space()
	if sc.i < len(sc.s) && sc.s[sc.i] == '#' {
		sc.comment()
	}
	if sc.i < len(sc.s) && sc.s[sc.i] != '\n' {
		return sc.errorf("text after the value")
	}
	return nil
}

func (sc *tomlScanner) header() (tomlEntry, error) {
	e := tomlEntry{kind: tomlTable, first: sc.line, last: sc.line}
	sc.i++
	end := "]"
	if strings.HasPrefix(sc.s[sc.i:], "[") {
		e.kind, end = tomlArrayTable, "]]"
		sc.i++
	}
	key, err := sc.key()
	if err != nil {
		return e, err
	}
	e.key = key
	sc.space()
	if !strings.HasPrefix(sc.s[sc.i:], end) {
		return e, sc.errorf("no %s after the table name", end)
	}
	sc.i += len(end)
	return e, sc.lineEnd()
}

func (sc *tomlScanner) keyValue() (tomlEntry, error) {
	var e tomlEntry
	key, err := sc.key()
	if err != nil {
		return e, err
	}
	e.key = key
	sc.space()
	if sc.i >= len(sc.s) || sc.s[sc.i] != '=' {
		return e, sc.errorf("no = after the key")
	}
	sc.i++
	sc.space()
	if sc.i < len(sc.s) && (sc.s[sc.i] == '"' || sc.s[sc.i] == '\'') && !strings.HasPrefix(sc.s[sc.i:], `"""`) && !strings.HasPrefix(sc.s[sc.i:], "'''") {
		e.str, err = sc.str()
		e.isStr = true
		return e, err
	}
	return e, sc.value()
}

// key reads a dotted key of bare and quoted parts.
func (sc *tomlScanner) key() ([]string, error) {
	var parts []string
	for {
		sc.space()
		var part string
		if sc.i < len(sc.s) && (sc.s[sc.i] == '"' || sc.s[sc.i] == '\'') {
			var err error
			if part, err = sc.str(); err != nil {
				return nil, err
			}
		} else {
			j := sc.i
			for j < len(sc.s) && tomlBare(sc.s[j]) {
				j++
			}
			if j == sc.i {
				return nil, sc.errorf("no key")
			}
			part, sc.i = sc.s[sc.i:j], j
		}
		parts = append(parts, part)
		sc.space()
		if sc.i >= len(sc.s) || sc.s[sc.i] != '.' {
			return parts, nil
		}
		sc.i++
	}
}

func tomlBare(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || c == '-'
}

// value reads a value. A scalar other than a string ends at a comment, a
// newline, or a "," "]" or "}" of an array or inline table around it.
func (sc *tomlScanner) value() error {
	if sc.i >= len(sc.s) {
		return sc.errorf("no value")
	}
	sc.depth++
	defer func() { sc.depth-- }()
	if sc.depth > tomlMaxDepth {
		return sc.errorf("values nested too deeply")
	}
	switch c := sc.s[sc.i]; {
	case strings.HasPrefix(sc.s[sc.i:], `"""`), strings.HasPrefix(sc.s[sc.i:], "'''"):
		return sc.multiline()
	case c == '"' || c == '\'':
		_, err := sc.str()
		return err
	case c == '[':
		return sc.array()
	case c == '{':
		return sc.inlineTable()
	}
	j := sc.i
	for j < len(sc.s) && !strings.ContainsRune("\n#,]}[{=\"'", rune(sc.s[j])) {
		j++
	}
	if strings.TrimSpace(sc.s[sc.i:j]) == "" {
		return sc.errorf("no value")
	}
	sc.i = j
	return nil
}

func (sc *tomlScanner) array() error {
	start := sc.line
	sc.i++
	for {
		sc.fill()
		if sc.i >= len(sc.s) {
			return fmt.Errorf("TOML file line %d: unterminated array", start+1)
		}
		switch sc.s[sc.i] {
		case ']':
			sc.i++
			return nil
		case ',':
			sc.i++
			continue
		}
		if err := sc.value(); err != nil {
			return err
		}
	}
}

func (sc *tomlScanner) inlineTable() error {
	start := sc.line
	sc.i++
	for {
		sc.fill()
		if sc.i >= len(sc.s) {
			return fmt.Errorf("TOML file line %d: unterminated inline table", start+1)
		}
		switch sc.s[sc.i] {
		case '}':
			sc.i++
			return nil
		case ',':
			sc.i++
			continue
		}
		if _, err := sc.key(); err != nil {
			return err
		}
		sc.space()
		if sc.i >= len(sc.s) || sc.s[sc.i] != '=' {
			return sc.errorf("no = after the key")
		}
		sc.i++
		sc.space()
		if err := sc.value(); err != nil {
			return err
		}
	}
}

// str reads a basic or literal string on one line and returns its value.
func (sc *tomlScanner) str() (string, error) {
	q := sc.s[sc.i]
	sc.i++
	var b strings.Builder
	for sc.i < len(sc.s) {
		c := sc.s[sc.i]
		switch {
		case c == '\n':
			return "", sc.errorf("unterminated string")
		case c == q:
			sc.i++
			return b.String(), nil
		case c == '\\' && q == '"':
			if err := sc.escape(&b); err != nil {
				return "", err
			}
			continue
		}
		b.WriteByte(c)
		sc.i++
	}
	return "", sc.errorf("unterminated string")
}

// escape reads the escape sequence at sc.i into b.
func (sc *tomlScanner) escape(b *strings.Builder) error {
	if sc.i+1 >= len(sc.s) {
		return sc.errorf("unterminated string")
	}
	c := sc.s[sc.i+1]
	sc.i += 2
	if r, ok := map[byte]string{'b': "\b", 't': "\t", 'n': "\n", 'f': "\f", 'r': "\r", 'e': "\x1b", '"': `"`, '\\': `\`}[c]; ok {
		b.WriteString(r)
		return nil
	}
	n := map[byte]int{'x': 2, 'u': 4, 'U': 8}[c]
	if n == 0 || sc.i+n > len(sc.s) {
		return sc.errorf("invalid escape \\%c", c)
	}
	digits := sc.s[sc.i : sc.i+n]
	var r rune
	for _, h := range digits {
		h = unicode.ToLower(h)
		switch {
		case h >= '0' && h <= '9':
			h -= '0'
		case h >= 'a' && h <= 'f':
			h -= 'a' - 10
		default:
			return sc.errorf("invalid escape \\%c%s", c, digits)
		}
		r = r<<4 | h
	}
	if !utf8.ValidRune(r) {
		return sc.errorf("invalid escape \\%c%s", c, digits)
	}
	b.WriteRune(r)
	sc.i += n
	return nil
}

// multiline reads a multi-line basic or literal string. Up to two quotes can
// come just before the closing three.
func (sc *tomlScanner) multiline() error {
	start := sc.line
	q := sc.s[sc.i]
	delim := sc.s[sc.i : sc.i+3]
	sc.i += 3
	for sc.i < len(sc.s) {
		switch c := sc.s[sc.i]; {
		case c == '\\' && q == '"':
			if sc.i+1 < len(sc.s) && sc.s[sc.i+1] == '\n' {
				sc.line++
			}
			sc.i += 2
		case c == '\n':
			sc.line++
			sc.i++
		case strings.HasPrefix(sc.s[sc.i:], delim):
			sc.i += 3
			for n := 0; n < 2 && sc.i < len(sc.s) && sc.s[sc.i] == q; n++ {
				sc.i++
			}
			return nil
		default:
			sc.i++
		}
	}
	return fmt.Errorf("TOML file line %d: unterminated string", start+1)
}
