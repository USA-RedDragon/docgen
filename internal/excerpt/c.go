package excerpt

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
)

const cTypedef = "typedef"

var (
	cPreprocessor = regexp.MustCompile(`^[ \t]*#`)
	cExternC      = regexp.MustCompile(`\bextern\s*"\s*"\s*$`)
)

// cSpan is a range of lines of a C file that holds a definition. The lines are
// numbered from 0.
type cSpan struct {
	first, last int
	what        string
}

// cExcerpt returns the lines of the definition that sym names. A bare name is
// a #define, a function definition or a typedef. "struct Tag", "union Tag"
// and "enum Tag" name the definition of a tagged type. A name that has more
// than one definition is an error.
func cExcerpt(src []byte, sym string) (string, error) {
	lines := strings.Split(string(src), "\n")
	codeLines := strings.Split(cCode(string(src)), "\n")
	code := cDropPreprocessor(codeLines)
	fields := strings.Fields(sym)
	var spans []cSpan
	var err error
	switch {
	case len(fields) == 1 && cIdent(fields[0]) && fields[0] != cTypedef && !cTypeKeyword(fields[0]):
		sym = fields[0]
		spans, err = cBareSpans(lines, codeLines, code, sym)
	case len(fields) == 2 && cTypeKeyword(fields[0]) && cIdent(fields[1]):
		sym = fields[0] + " " + fields[1]
		spans, err = cTagSpans(code, fields[0], fields[1])
	default:
		return "", fmt.Errorf("symbol=%q in a C file is a name, or struct, union or enum and a tag", sym)
	}
	if err != nil {
		return "", err
	}
	spans = cDedupe(spans)
	switch len(spans) {
	case 0:
		return "", cMissing(code, sym, fields)
	case 1:
		return strings.Join(lines[spans[0].first:spans[0].last+1], "\n"), nil
	}
	return "", cAmbiguous(sym, spans)
}

// cAmbiguous returns the error for a name that has the definitions spans.
func cAmbiguous(sym string, spans []cSpan) error {
	where := make([]string, 0, len(spans))
	for _, s := range spans {
		where = append(where, fmt.Sprintf("%s at line %d", s.what, s.first+1))
	}
	return fmt.Errorf("%s has more than one definition in the C file: %s", sym, strings.Join(where, ", "))
}

func cBareSpans(lines, codeLines []string, code, sym string) ([]cSpan, error) {
	spans := cDefines(lines, codeLines, sym)
	fns, err := cFunctions(code, sym)
	if err != nil {
		return nil, err
	}
	spans = append(spans, fns...)
	for _, t := range cTypes(code) {
		if slices.Contains(t.names, sym) {
			if t.err != nil {
				return nil, t.err
			}
			spans = append(spans, t.span)
		}
	}
	return spans, nil
}

func cTagSpans(code, keyword, name string) ([]cSpan, error) {
	var spans []cSpan
	for _, t := range cTypes(code) {
		if slices.Contains(t.tags, cTag{keyword, name}) {
			if t.err != nil {
				return nil, t.err
			}
			spans = append(spans, t.span)
		}
	}
	return spans, nil
}

func cMissing(code, sym string, fields []string) error {
	if len(fields) == 1 {
		for _, t := range cTypes(code) {
			for _, tag := range t.tags {
				if tag.name == sym {
					return fmt.Errorf("no #define, function or typedef %s in the C file; %s %s is a tag, so use symbol=\"%s %s\"", sym, tag.keyword, sym, tag.keyword, sym)
				}
			}
		}
		return fmt.Errorf("no #define, function or typedef %s in the C file", sym)
	}
	return fmt.Errorf("no definition of %s with a body in the C file", sym)
}

func cDedupe(spans []cSpan) []cSpan {
	var out []cSpan
	for _, s := range spans {
		if !slices.ContainsFunc(out, func(o cSpan) bool { return o.first == s.first && o.last == s.last }) {
			out = append(out, s)
		}
	}
	return out
}

func cTypeKeyword(w string) bool {
	return w == "struct" || w == "union" || w == "enum"
}

func cIdent(w string) bool {
	return w != "" && (w[0] < '0' || w[0] > '9') && identEnd(w, 0) == len(w)
}

// cDefines returns the lines of each #define of sym, with its continuation
// lines. codeLines holds lines with no comments.
func cDefines(lines, codeLines []string, sym string) []cSpan {
	define := regexp.MustCompile(`^[ \t]*#[ \t]*define[ \t]+` + regexp.QuoteMeta(sym) + `(?:[ \t(]|$)`)
	var spans []cSpan
	for i, l := range codeLines {
		if define.MatchString(l) {
			j := i
			for j < len(lines)-1 && strings.HasSuffix(lines[j], "\\") {
				j++
			}
			spans = append(spans, cSpan{i, j, "#define"})
		}
	}
	return spans
}

// cFunctions returns the lines of each definition of the function sym in
// code, from the first line of its declaration to its closing brace.
func cFunctions(code, sym string) ([]cSpan, error) {
	blanked := strings.Split(code, "\n")
	var spans []cSpan
	var err error
	cWalk(code, func(i, j int) int {
		if err != nil || code[i:j] != sym {
			return j
		}
		brace := cDefinitionBrace(code, j)
		if brace < 0 {
			return j
		}
		end := cBodyEnd(code, brace)
		if end < 0 {
			err = fmt.Errorf("the body of function %s in the C file has no closing brace", sym)
			return j
		}
		first := cFirstLine(blanked, strings.Count(code[:i], "\n"))
		spans = append(spans, cSpan{first, strings.Count(code[:end], "\n"), "function"})
		return end + 1
	})
	return spans, err
}

// cFirstLine returns the first line of the declaration that has a token on
// line n: the lines above it that are not blank and hold no ; { or }.
func cFirstLine(blanked []string, n int) int {
	for n > 0 {
		t := strings.TrimSpace(blanked[n-1])
		if t == "" || strings.ContainsAny(t, ";{}") {
			break
		}
		n--
	}
	return n
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

// cWalk calls visit for each identifier of code at nesting depth 0 outside
// parentheses, with the offsets of its start and end. When visit returns a
// larger offset than the end, the scan resumes there. The closing brace of an
// extern "C" block and a closing brace at column 0 leave the nesting depth at
// 0, so that the scan finds the definitions after #ifdef branches with
// unbalanced braces.
func cWalk(code string, visit func(start, end int) int) {
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
			if depth == 0 && paren == 0 {
				j = max(j, visit(i, j))
			}
			i = j - 1
		}
	}
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
