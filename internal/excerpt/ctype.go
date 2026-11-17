package excerpt

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	cAttributeCall = regexp.MustCompile(`\b(?:__attribute__|__attribute|__declspec)\s*\(`)
	cBrackets      = regexp.MustCompile(`\[[^\]]*\]`)
)

// cTag is the keyword and the name of a tagged type, as in struct foo.
type cTag struct{ keyword, name string }

// cType is a typedef, or a struct, union or enum with a body, at the top
// level of a C file.
type cType struct {
	names []string
	tags  []cTag
	span  cSpan
	err   error
}

// cTypes returns the typedefs and the tagged type definitions of code, which
// holds no comments and no preprocessor lines. A statement that shares a line
// with other code has err set, because an excerpt is whole lines.
func cTypes(code string) []cType {
	lines := strings.Split(code, "\n")
	var out []cType
	cWalk(code, func(i, j int) int {
		word := code[i:j]
		if word != cTypedef && !cTypeKeyword(word) {
			return j
		}
		end, tags := cScanStatement(code, i)
		if end < 0 {
			return j
		}
		t := cType{tags: tags}
		if word == cTypedef {
			t.names = cTypedefNames(code[j:end])
		}
		if len(t.names)+len(t.tags) == 0 {
			return end + 1
		}
		first, last := strings.Count(code[:i], "\n"), strings.Count(code[:end], "\n")
		before := code[strings.LastIndexByte(code[:i], '\n')+1 : i]
		after := code[end+1 : end+1+max(strings.IndexByte(code[end+1:], '\n'), 0)]
		if strings.ContainsAny(before, ";{}") || strings.TrimSpace(after) != "" {
			t.err = fmt.Errorf("the definition at line %d of the C file shares a line with other code", first+1)
		}
		t.span = cSpan{cFirstLine(lines, first), last, cTypedef}
		if len(t.names) == 0 {
			t.span.what = t.tags[0].keyword + " " + t.tags[0].name
		}
		out = append(out, t)
		return end + 1
	})
	return out
}

// cScanStatement returns the offset of the semicolon that ends the statement
// that starts at from, and the tagged types that the statement defines. It
// returns -1 when the statement is a function definition or when it has no
// end that can be found.
func cScanStatement(code string, from int) (int, []cTag) {
	var tags []cTag
	afterParams := false
	for i := from; i < len(code); i++ {
		c := code[i]
		fn := afterParams
		if c != ' ' && c != '\t' && c != '\n' && c != '\r' && c != '(' {
			afterParams = false
		}
		switch {
		case c == ';':
			return i, tags
		case c == '}':
			return -1, nil
		case c == '(':
			end := cMatch(code, i)
			if end < 0 {
				return -1, nil
			}
			afterParams = !cAttribute(code[:i])
			i = end
		case c == '{':
			end := cBodyEnd(code, i)
			if fn || end < 0 {
				return -1, nil
			}
			i = end
		case isIdent(c):
			j := identEnd(code, i)
			if w := code[i:j]; cTypeKeyword(w) {
				if name, body := cTagAt(code, j); body && name != "" {
					tags = append(tags, cTag{w, name})
				}
			}
			i = j - 1
		}
	}
	return -1, nil
}

// cTagAt returns the tag that follows a struct, union or enum keyword that
// ends at from, and whether a body follows it.
func cTagAt(code string, from int) (string, bool) {
	name := ""
	for i := from; i < len(code); i++ {
		switch c := code[i]; {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
		case c == '{':
			return name, true
		case isIdent(c):
			j := identEnd(code, i)
			switch {
			case cAttributeName(code[i:j]):
				k := strings.IndexByte(code[j:], '(')
				if k < 0 || strings.TrimSpace(code[j:j+k]) != "" {
					return "", false
				}
				end := cMatch(code, j+k)
				if end < 0 {
					return "", false
				}
				j = end + 1
			case name == "":
				name = code[i:j]
			default:
				return "", false
			}
			i = j - 1
		default:
			return "", false
		}
	}
	return "", false
}

// cMatch returns the offset of the parenthesis that closes the one at open,
// or -1.
func cMatch(s string, open int) int {
	depth := 0
	for i := open; i < len(s); i++ {
		switch s[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

func cAttributeName(w string) bool {
	return w == "__attribute__" || w == "__attribute" || w == "__declspec"
}

// cAttribute reports whether prefix ends with the name of an attribute, so
// that a parenthesis group that follows it is not a parameter list.
func cAttribute(prefix string) bool {
	p := strings.TrimRight(prefix, " \t\r\n")
	start := len(p)
	for start > 0 && isIdent(p[start-1]) {
		start--
	}
	return cAttributeName(p[start:])
}

// cTypedefNames returns the names that a typedef declares. s is the text
// between the typedef keyword and the semicolon.
func cTypedefNames(s string) []string {
	s = cStripAttributes(cFlatten(s))
	var names []string
	for _, d := range cSplitTop(s) {
		if n := cDeclName(d); n != "" {
			names = append(names, n)
		}
	}
	return names
}

// cFlatten returns s with each brace body replaced by {}.
func cFlatten(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] != '{' {
			b.WriteByte(s[i])
			continue
		}
		end := cBodyEnd(s, i)
		if end < 0 {
			end = len(s) - 1
		}
		b.WriteString("{}")
		i = end
	}
	return b.String()
}

func cStripAttributes(s string) string {
	for {
		loc := cAttributeCall.FindStringIndex(s)
		if loc == nil {
			return s
		}
		end := cMatch(s, loc[1]-1)
		if end < 0 {
			return s[:loc[0]]
		}
		s = s[:loc[0]] + " " + s[end+1:]
	}
}

// cSplitTop splits s at the commas outside parentheses and brackets.
func cSplitTop(s string) []string {
	var out []string
	depth, start := 0, 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '(', '[':
			depth++
		case ')', ']':
			depth--
		case ',':
			if depth == 0 {
				out = append(out, s[start:i])
				start = i + 1
			}
		}
	}
	return append(out, s[start:])
}

// cDeclName returns the name that a declarator declares: the identifier in a
// function pointer such as (*name)(int), the name of a function type, or the
// last identifier before any array brackets.
func cDeclName(d string) string {
	for {
		open := strings.IndexByte(d, '(')
		if open < 0 {
			break
		}
		end := cMatch(d, open)
		if end < 0 {
			return ""
		}
		inner := strings.TrimSpace(d[open+1 : end])
		rest := strings.TrimSpace(d[end+1:])
		if strings.HasPrefix(inner, "*") || strings.HasPrefix(inner, "^") || strings.HasPrefix(inner, "(") || cIdent(inner) && strings.HasPrefix(rest, "(") {
			d = inner
			continue
		}
		d = d[:open]
	}
	d = strings.TrimRight(cBrackets.ReplaceAllString(d, ""), " \t\r\n")
	end := len(d)
	start := end
	for start > 0 && isIdent(d[start-1]) {
		start--
	}
	name := d[start:end]
	if !cIdent(name) || cTypeKeyword(name) {
		return ""
	}
	return name
}
