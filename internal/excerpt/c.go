package excerpt

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	cPreprocessor = regexp.MustCompile(`^[ \t]*#`)
	cExternC      = regexp.MustCompile(`\bextern\s*"\s*"\s*$`)
)

// cExcerpt returns the #define of sym, with its continuation lines, or the
// definition of the function sym, from the first line of its declaration to
// the line of its closing brace. A prototype is not a definition.
func cExcerpt(src []byte, sym string) (string, error) {
	lines := strings.Split(string(src), "\n")
	codeLines := strings.Split(cCode(string(src)), "\n")
	if first, last := cDefine(lines, codeLines, sym); first >= 0 {
		return strings.Join(lines[first:last+1], "\n"), nil
	}
	first, last, err := cFunction(codeLines, sym)
	if err != nil {
		return "", err
	}
	return strings.Join(lines[first:last+1], "\n"), nil
}

// cDefine returns the first and the last line of the #define of sym, or -1 and
// -1. codeLines holds lines with no comments.
func cDefine(lines, codeLines []string, sym string) (int, int) {
	define := regexp.MustCompile(`^[ \t]*#[ \t]*define[ \t]+` + regexp.QuoteMeta(sym) + `(?:[ \t(]|$)`)
	for i, l := range codeLines {
		if define.MatchString(l) {
			j := i
			for j < len(lines)-1 && strings.HasSuffix(lines[j], "\\") {
				j++
			}
			return i, j
		}
	}
	return -1, -1
}

// cFunction returns the first and the last line of the definition of the
// function sym in codeLines.
func cFunction(codeLines []string, sym string) (int, int, error) {
	code := cDropPreprocessor(codeLines)
	name, brace := cFindBody(code, sym)
	if name < 0 {
		return 0, 0, fmt.Errorf("no #define or function %s in the C file", sym)
	}
	end := cBodyEnd(code, brace)
	if end < 0 {
		return 0, 0, fmt.Errorf("the body of function %s in the C file has no closing brace", sym)
	}
	blanked := strings.Split(code, "\n")
	first := strings.Count(code[:name], "\n")
	for first > 0 {
		t := strings.TrimSpace(blanked[first-1])
		if t == "" || strings.ContainsAny(t, ";{}") {
			break
		}
		first--
	}
	return first, strings.Count(code[:end], "\n"), nil
}

// cCode returns src with spaces in place of comments and of the text of
// string and character literals. Line and byte positions do not change.
func cCode(src string) string {
	b := []byte(src)
	blank := func(from, to int) {
		for i := from; i < to && i < len(b); i++ {
			if b[i] != '\n' {
				b[i] = ' '
			}
		}
	}
	for i := 0; i < len(src); i++ {
		switch {
		case strings.HasPrefix(src[i:], "//"):
			end := strings.IndexByte(src[i:], '\n')
			if end < 0 {
				end = len(src) - i
			}
			blank(i, i+end)
			i += end
		case strings.HasPrefix(src[i:], "/*"):
			end := strings.Index(src[i+2:], "*/")
			if end < 0 {
				end = len(src) - i
			} else {
				end += 4
			}
			blank(i, i+end)
			i += end - 1
		case src[i] == '"' || src[i] == '\'':
			j := i + 1
			for j < len(src) && src[j] != src[i] && src[j] != '\n' {
				if src[j] == '\\' {
					j++
				}
				j++
			}
			blank(i+1, j)
			i = j
		}
	}
	return string(b)
}

// cDropPreprocessor joins lines with spaces in place of preprocessor lines
// and their continuation lines.
func cDropPreprocessor(lines []string) string {
	out := make([]string, len(lines))
	cont := false
	for i, l := range lines {
		if cont || cPreprocessor.MatchString(l) {
			cont = strings.HasSuffix(l, "\\")
			out[i] = strings.Repeat(" ", len(l))
			continue
		}
		out[i] = l
	}
	return strings.Join(out, "\n")
}

// cFindBody returns the offset of the name of the first top-level function
// definition of sym in code and the offset of the opening brace of its body,
// or -1 and -1. The closing brace of an extern "C" block and a closing brace
// at column 0 leave the nesting depth at 0, so that the scan finds the
// definitions after #ifdef branches with unbalanced braces.
func cFindBody(code, sym string) (int, int) {
	depth, paren := 0, 0
	for i := 0; i < len(code); i++ {
		switch c := code[i]; {
		case c == '{':
			if !cExternC.MatchString(code[max(i-64, 0):i]) {
				depth++
			}
		case c == '}':
			depth = max(depth-1, 0)
			if i == 0 || code[i-1] == '\n' {
				depth = 0
			}
		case c == '(':
			paren++
		case c == ')':
			paren = max(paren-1, 0)
		case isIdent(c):
			j := identEnd(code, i)
			if depth == 0 && paren == 0 && code[i:j] == sym {
				if brace := cDefinitionBrace(code, j); brace >= 0 {
					return i, brace
				}
			}
			i = j - 1
		}
	}
	return -1, -1
}

// cDefinitionBrace returns the offset of the body that follows the parameter
// list that starts at or after from, or -1 when the name at from is not
// followed by a parameter list and a body.
func cDefinitionBrace(code string, from int) int {
	i := from + len(code[from:]) - len(strings.TrimLeft(code[from:], " \t\r\n"))
	if i >= len(code) || code[i] != '(' {
		return -1
	}
	paren := 0
	for ; i < len(code); i++ {
		switch code[i] {
		case '(':
			paren++
		case ')':
			paren--
		}
		if paren == 0 {
			break
		}
	}
	for ; i < len(code); i++ {
		switch code[i] {
		case '{':
			return i
		case ';', '=':
			return -1
		}
	}
	return -1
}

// cBodyEnd returns the offset of the brace that closes the body at brace: the
// matching brace, or an earlier closing brace at column 0. It returns -1 when
// there is none.
func cBodyEnd(code string, brace int) int {
	depth := 0
	for i := brace; i < len(code); i++ {
		switch code[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 || code[i-1] == '\n' {
				return i
			}
		}
	}
	return -1
}

// identEnd returns the offset after the identifier at offset i.
func identEnd(code string, i int) int {
	for i < len(code) && isIdent(code[i]) {
		i++
	}
	return i
}

func isIdent(c byte) bool {
	return c == '_' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}
