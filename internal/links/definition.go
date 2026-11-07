package links

import "strings"

// A definition is a link reference definition. at is the offset of its destination
// and end the offset of the end of its last line, in the text that parseDefinition
// reads. A definition in a list item counts as a link when a reference uses it,
// since it renders as nothing.
type definition struct {
	label  string
	link   Link
	at     int
	end    int
	inList bool
}

// parseDefinition reads the definition at the start of s, the lines of a paragraph
// with no prefix. It reports false when s starts with none.
func parseDefinition(s string) (definition, bool) {
	i := columnBytes(s, indentOf(s))
	if i >= len(s) || s[i] != '[' {
		return definition{}, false
	}
	last := labelEnd([]byte(s), i+1)
	if last < 0 || last+1 >= len(s) || s[last+1] != ':' || strings.TrimSpace(s[i+1:last]) == "" {
		return definition{}, false
	}
	at := skipLine(s, last+2)
	end, ok := definitionTarget(s, at)
	if !ok {
		return definition{}, false
	}
	stop, ok := definitionEnd(s, end)
	if !ok {
		return definition{}, false
	}
	return definition{label: s[i+1 : last], link: Link{Target: destination(s[at:end])}, at: at, end: stop}, true
}

// labelEnd returns the offset of the ] that ends the label from offset i of b, or
// -1 when there is none or an open bracket comes first.
func labelEnd(b []byte, i int) int {
	for ; i < len(b); i++ {
		switch b[i] {
		case '\\':
			i++
		case '[':
			return -1
		case ']':
			return i
		}
	}
	return -1
}

// skipLine returns the offset after the white space from offset i of s, which can
// hold one line ending.
func skipLine(s string, i int) int {
	lines := 0
	for i < len(s) && (isSpace(s[i]) || (s[i] == '\n' && lines == 0)) {
		if s[i] == '\n' {
			lines++
		}
		i++
	}
	return i
}

// definitionTarget returns the offset after the destination at offset i of s.
func definitionTarget(s string, i int) (int, bool) {
	if i >= len(s) || s[i] == '\n' {
		return 0, false
	}
	if s[i] == '<' {
		for j := i + 1; j < len(s) && s[j] != '\n' && s[j] != '<'; j++ {
			switch s[j] {
			case '\\':
				j++
			case '>':
				return j + 1, true
			}
		}
		return 0, false
	}
	j := i
	for j < len(s) && !isSpace(s[j]) && s[j] != '\n' {
		j++
	}
	return j, j > i
}

// lineRest returns the offset of the end of the line of offset i of s, and false
// when text comes first.
func lineRest(s string, i int) (int, bool) {
	for i < len(s) && isSpace(s[i]) {
		i++
	}
	return i, i == len(s) || s[i] == '\n'
}

// definitionEnd returns the offset of the end of the last line of the definition
// with a destination that ends at offset i of s. A title can follow on the same
// line or on the next one. A line after the destination that is not a whole title
// is not part of the definition, and text on the line of the destination that is
// not a whole title makes it no definition.
func definitionEnd(s string, i int) (int, bool) {
	stop, alone := lineRest(s, i)
	at := skipLine(s, i)
	if at > i && at < len(s) {
		if after, ok := titleEnd([]byte(s), at); ok && after > at {
			if end, whole := lineRest(s, after); whole {
				return end, true
			}
		}
	}
	return stop, alone
}
