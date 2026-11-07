package marker

import (
	"bytes"
	"regexp"
	"strings"
)

var (
	listItem = regexp.MustCompile(`\A(?:[-*+]|\d{1,9}[.)])[ \t]+`)
	// blockStart matches a line that ends a paragraph, and with it any code span.
	blockStart = regexp.MustCompile(`\A[ \t>]*(?:\z|<!--|#{1,6}[ \t]|[-*+][ \t]|\d{1,9}[.)][ \t]|\|)`)
)

// fenceOf returns the fence that line opens: three or more backticks or
// tildes at the start of the trimmed line, or "". The info string of a backtick
// fence holds no backtick, so "```a```" is a code span.
func fenceOf(line string) string {
	if line == "" || (line[0] != '`' && line[0] != '~') {
		return ""
	}
	n := 0
	for n < len(line) && line[n] == line[0] {
		n++
	}
	if n < 3 || (line[0] == '`' && strings.IndexByte(line[n:], '`') >= 0) {
		return ""
	}
	return line[:n]
}

// closesFence reports whether line closes the fence: the same character, at
// least as many times, and nothing after it.
func closesFence(line, fence string) bool {
	return strings.HasPrefix(line, fence) && strings.Trim(line, fence[:1]) == ""
}

// indentOf returns the width of the leading white space of line, with a tab
// as four columns.
func indentOf(line string) int {
	n := 0
	for _, r := range line {
		switch r {
		case ' ':
			n++
		case '\t':
			n += 4
		default:
			return n
		}
	}
	return n
}

// A fence is an open fenced code block: its fence, and the number of block
// quotes that hold it.
type fence struct {
	mark  string
	depth int
}

// openFence returns the fence that line opens, after the block quote and list
// item prefixes of the line.
func openFence(line string) fence {
	rest := strings.TrimLeft(line, " \t")
	depth := 0
	for {
		if strings.HasPrefix(rest, ">") {
			rest = strings.TrimLeft(rest[1:], " \t")
			depth++
			continue
		}
		item := listItem.FindString(rest)
		if item == "" {
			return fence{mark: fenceOf(strings.TrimSpace(rest)), depth: depth}
		}
		rest = rest[len(item):]
	}
}

// next returns whether line is part of the code block and the fence that stays
// open after it. A line outside the block quotes of the fence ends the block
// and is not part of it.
func (f fence) next(line string) (bool, fence) {
	rest := strings.TrimLeft(line, " \t")
	for range f.depth {
		if !strings.HasPrefix(rest, ">") {
			return false, fence{}
		}
		rest = strings.TrimLeft(rest[1:], " \t")
	}
	if closesFence(strings.TrimSpace(rest), f.mark) {
		return true, fence{}
	}
	return true, f
}

// A scope is the place in a page of a text for maskCode.
type scope struct {
	// inBody is true for a text that starts in a marker body.
	inBody bool
	// indent is the indent of the open tag of that marker.
	indent int
	// closeTag is the close tag of that marker.
	closeTag string
	// generated is true for a body that holds no close tag yet. Such a text
	// is in the body to its end, and its escaped comment openers stay.
	generated bool
}

func blank(b []byte) {
	for i := range b {
		if b[i] != '\n' {
			b[i] = ' '
		}
	}
}

func lineEnd(b []byte, start int) int {
	if i := bytes.IndexByte(b[start:], '\n'); i >= 0 {
		return start + i
	}
	return len(b)
}

// holdsTag reports whether line holds tag outside a code span and an escape.
func holdsTag(line []byte, tag string) bool {
	if !bytes.Contains(line, []byte(tag)) {
		return false
	}
	masked := bytes.Clone(line)
	maskSpans(masked, true)
	return bytes.Contains(masked, []byte(tag))
}

// maskLines blanks the lines of code blocks. It blanks fenced blocks
// everywhere, and in a marker body the lines indented four columns or more,
// where a doc comment prints its code that way. A close tag indented less than
// four columns past its open tag stays.
func maskLines(b []byte, sc scope) {
	start := 0
	if sc.inBody {
		end := lineEnd(b, 0)
		if !sc.generated && holdsTag(b[:end], sc.closeTag) {
			sc.inBody = false
		}
		start = end + 1
	}
	var open fence
	for start < len(b) {
		end := lineEnd(b, start)
		line := string(b[start:end])
		mask := false
		if open.mark != "" {
			mask, open = open.next(line)
		}
		if !mask {
			mask, open = sc.nextLine(b[start:end], line, open)
		}
		if mask {
			blank(b[start:end])
		}
		start = end + 1
	}
}

// nextLine returns whether line, outside a fenced block, is code, and the fence
// that stays open after it. raw is the same line. A close tag ends the body of
// sc.
func (sc *scope) nextLine(raw []byte, line string, open fence) (bool, fence) {
	width := indentOf(line)
	isClose := sc.inBody && holdsTag(raw, sc.closeTag)
	switch {
	case sc.inBody && width >= 4 && (!isClose || width >= sc.indent+4):
		return true, open
	case isClose && !sc.generated:
		sc.inBody = false
	case !isClose:
		if f := openFence(line); f.mark != "" {
			return true, f
		}
	}
	return false, open
}

// spanEnd returns the offset after the run of n backticks that closes a code
// span, or -1 when the paragraph ends first.
func spanEnd(b []byte, from, n int) int {
	for i := from; i < len(b); {
		switch b[i] {
		case '\n':
			if blockStart.Match(b[i+1 : lineEnd(b, i+1)]) {
				return -1
			}
			i++
		case '`':
			j := i
			for j < len(b) && b[j] == '`' {
				j++
			}
			if j-i == n {
				return j
			}
			i = j
		default:
			i++
		}
	}
	return -1
}

// maskSpans blanks inline code spans, from left to right in CommonMark order: a
// comment or a backslash escape has precedence when it starts first. With
// escapes true, it also blanks each escaped "<", which opens no comment.
func maskSpans(b []byte, escapes bool) {
	for i := 0; i < len(b); {
		switch {
		case b[i] == '\\':
			if escapes && i+1 < len(b) && b[i+1] == '<' {
				blank(b[i : i+2])
			}
			i += 2
		case bytes.HasPrefix(b[i:], []byte("<!--")):
			end := bytes.Index(b[i+2:], []byte("-->"))
			if end < 0 {
				i += 4
				continue
			}
			i += 2 + end + 3
		case b[i] == '`':
			n := 1
			for i+n < len(b) && b[i+n] == '`' {
				n++
			}
			end := spanEnd(b, i+n, n)
			if end < 0 {
				i += n
				continue
			}
			blank(b[i:end])
			i = end
		default:
			i++
		}
	}
}

// maskCode returns text with its code blocks and code spans replaced by
// spaces, so that a marker shown as an example in code is not a marker. An
// indented code block outside a marker body stays, since a real marker can
// be indented in a list.
func maskCode(text string, sc scope) []byte {
	b := []byte(text)
	maskLines(b, sc)
	maskSpans(b, !sc.generated)
	return b
}

// maskBody masks the text from offset start as text that starts in the body of a
// marker, and returns the offset of closeTag from start, or -1.
func maskBody(text string, masked []byte, start, indent int, closeTag string) int {
	copy(masked[start:], maskCode(text[start:], scope{inBody: true, indent: indent, closeTag: closeTag}))
	return bytes.Index(masked[start:], []byte(closeTag))
}

// Prose returns text, a page with LF line endings, with its code and the bodies
// of its markers replaced by spaces. Each line stays at its number. Unlike Parse,
// it accepts a page with a comment that is not a valid tag.
func Prose(text string) string {
	return maskPage(text, true)
}

// MaskCode returns text, a page with LF line endings, with its code replaced by
// spaces, in the bodies of its markers also. Each line stays at its number.
func MaskCode(text string) string {
	return maskPage(text, false)
}

// maskPage masks the code of a page, and with bodies true the bodies of its
// markers.
func maskPage(text string, bodies bool) string {
	masked := maskCode(text, scope{})
	for pos := 0; ; {
		loc := tagLike.FindIndex(masked[pos:])
		if loc == nil {
			return string(masked)
		}
		at := pos + loc[0]
		pos += loc[1]
		open := openAt.FindSubmatchIndex(masked[at:])
		if open == nil || regionMark.Match(masked[at:]) {
			continue
		}
		start := at + open[1]
		closeTag := "<!-- /docgen:" + string(masked[at+open[2]:at+open[3]]) + " -->"
		indent := indentOf(text[strings.LastIndexByte(text[:at], '\n')+1 : at])
		if rel := maskBody(text, masked, start, indent, closeTag); rel >= 0 {
			if bodies {
				blank(masked[start : start+rel])
			}
			pos = start + rel + len(closeTag)
		}
	}
}
