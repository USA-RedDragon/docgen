package excerpt

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
)

var cDefineName = regexp.MustCompile(`^[ \t]*#[ \t]*define[ \t]+([A-Za-z_][A-Za-z0-9_]*)`)

// CDefineValue returns the replacement list of the object-like #define name in
// the C source src, as written: line continuations join with one space,
// comments are gone, and the text has no whitespace around it. The value is
// never evaluated. A function-like macro, a #define with no replacement list,
// a name with no #define and a name with more than one are errors.
func CDefineValue(src []byte, name string) (string, error) {
	lines := strings.Split(string(src), "\n")
	codeLines := strings.Split(cCode(string(src)), "\n")
	if !cIdent(name) {
		return "", fmt.Errorf("name=%q is not a C identifier", name)
	}
	spans := cDedupe(cDefines(lines, codeLines, name))
	switch len(spans) {
	case 0:
		return "", cNoDefine(codeLines, name)
	case 1:
	default:
		return "", cAmbiguous(name, spans)
	}
	s := spans[0]
	text := cStripComments(cSplice(lines[s.first : s.last+1]))
	rest := cDefineName.ReplaceAllString(text, "")
	if strings.HasPrefix(rest, "(") {
		return "", fmt.Errorf("%s is a function-like macro; docgen:const gives the replacement text of an object-like #define", name)
	}
	if rest = strings.TrimSpace(rest); rest == "" {
		return "", fmt.Errorf("the #define %s at line %d has no replacement text", name, s.first+1)
	}
	return rest, nil
}

func cNoDefine(codeLines []string, name string) error {
	var names []string
	for _, l := range codeLines {
		if m := cDefineName.FindStringSubmatch(l); m != nil && !slices.Contains(names, m[1]) {
			names = append(names, m[1])
		}
	}
	if len(names) == 0 {
		return fmt.Errorf("no #define %s in the C file, which has no #define", name)
	}
	slices.Sort(names)
	return fmt.Errorf("no #define %s in the C file; its #defines are: %s", name, strings.Join(names, ", "))
}

// cSplice joins the lines of one #define: a trailing backslash is dropped,
// and the whitespace at the join becomes one space.
func cSplice(lines []string) string {
	parts := make([]string, len(lines))
	for i, l := range lines {
		if i < len(lines)-1 {
			l = strings.TrimSuffix(l, "\\")
		}
		if i > 0 {
			l = strings.TrimLeft(l, " \t")
		}
		if i < len(lines)-1 {
			l = strings.TrimRight(l, " \t")
		}
		parts[i] = l
	}
	return strings.Join(parts, " ")
}

// cStripComments returns s with each comment replaced by one space. The text
// of string and character literals stays. A block comment with no end runs to
// the end of s.
func cStripComments(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		switch {
		case strings.HasPrefix(s[i:], "//"):
			b.WriteByte(' ')
			return b.String()
		case strings.HasPrefix(s[i:], "/*"):
			b.WriteByte(' ')
			end := strings.Index(s[i+2:], "*/")
			if end < 0 {
				return b.String()
			}
			i += end + 3
		case s[i] == '"' || s[i] == '\'':
			j := i + 1
			for j < len(s) && s[j] != s[i] {
				if s[j] == '\\' {
					j++
				}
				j++
			}
			j = min(j+1, len(s))
			b.WriteString(s[i:j])
			i = j - 1
		default:
			b.WriteByte(s[i])
		}
	}
	return b.String()
}

var cIntegerLiteral = regexp.MustCompile(`^(0[xX][0-9a-fA-F]+(?:'[0-9a-fA-F]+)*|0[bB][01]+(?:'[01]+)*|[0-9]+(?:'[0-9]+)*)([a-zA-Z]*)$`)

// CIntegerValue returns text, the replacement list of a #define, without the
// suffix of its integer literal: 0x10u is 0x10. The base and the digits stay as
// written. Text that is not exactly one integer literal is an error, and so is
// a literal in parentheses, and a literal with a suffix that C does not have.
// Decimal, octal, hexadecimal and binary literals work, with ' between digits.
func CIntegerValue(text string) (string, error) {
	m := cIntegerLiteral.FindStringSubmatch(text)
	if m == nil {
		return "", fmt.Errorf("the replacement text %q is not one integer literal", text)
	}
	digits, suffix := m[1], m[2]
	if !cIntSuffix(suffix) {
		return "", fmt.Errorf("the integer literal %q has the suffix %q, which C does not have", text, suffix)
	}
	if len(digits) > 1 && digits[0] == '0' && !strings.ContainsAny(digits[1:2], "xXbB") {
		if strings.ContainsAny(digits, "89") {
			return "", fmt.Errorf("the octal literal %q has a digit that is not octal", text)
		}
	}
	return digits, nil
}

// cIntSuffix reports whether s is empty or an integer suffix of C: u with at
// most one of l, ll, z and wb, in either order and in any case, but with
// both letters of ll in one case.
func cIntSuffix(s string) bool {
	if strings.Contains(s, "lL") || strings.Contains(s, "Ll") {
		return false
	}
	s = strings.ToLower(s)
	var u bool
	size := ""
	for s != "" {
		switch {
		case s[0] == 'u' && !u:
			u = true
			s = s[1:]
		case size == "" && strings.HasPrefix(s, "ll"):
			size, s = "ll", s[2:]
		case size == "" && strings.HasPrefix(s, "wb"):
			size, s = "wb", s[2:]
		case size == "" && (s[0] == 'l' || s[0] == 'z'):
			size, s = s[:1], s[1:]
		default:
			return false
		}
	}
	return true
}
