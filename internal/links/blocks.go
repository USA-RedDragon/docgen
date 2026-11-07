package links

import (
	"bytes"
	"regexp"
	"strings"
)

var (
	atxHeading = regexp.MustCompile(`^ {0,3}#{1,6}(?:[ \t]+(.*?))??(?:[ \t]+#+)?[ \t]*$`)
	setextLine = regexp.MustCompile(`^ {0,3}(?:=+|-+)[ \t]*$`)
	breakLine  = regexp.MustCompile(`^ {0,3}(?:(?:-[ \t]*){3,}|(?:\*[ \t]*){3,}|(?:_[ \t]*){3,})$`)
)

// A heading is a heading of a page: its offset and its Markdown text.
type heading struct {
	at   int
	text string
}

// A container is an open block quote or list item. width is the indent of the
// content of a list item, relative to the content of its parent.
type container struct {
	quote bool
	width int
}

// A paraLine is one line of an open paragraph: the number of the line, its
// offsets, and the offset of its text after the prefixes of its containers.
type paraLine struct {
	num   int
	start int
	text  int
	end   int
}

// blocks holds the block structure of a page. masked is the page with its code
// and comments as spaces, and links is the same with HTML blocks and reference
// definitions as spaces too. Each line stays at its number.
type blocks struct {
	text     string
	masked   []byte
	links    []byte
	headings []heading
	defs     []definition
}

// A blockReader reads a page line by line, as CommonMark does: it matches the open
// containers, opens new ones, and then reads the rest of the line as a leaf block.
type blockReader struct {
	*blocks
	stack    []container
	para     []paraLine
	inHTML   bool
	htmlEnds *regexp.Regexp
}

// parseBlocks returns the block structure of text, a page with LF line endings
// and no byte order mark.
func parseBlocks(text string) *blocks {
	masked := maskText(text)
	r := &blockReader{blocks: &blocks{text: text, masked: masked, links: bytes.Clone(masked)}}
	num := 1
	for start := 0; start <= len(text); num++ {
		end := start + lineLen(masked[start:])
		r.line(num, start, end)
		start = end + 1
	}
	r.closePara()
	return r.blocks
}

func isBlank(s []byte) bool {
	return len(bytes.TrimSpace(s)) == 0
}

// indentOf returns the width of the leading white space of s, with a tab to the
// next multiple of four columns.
func indentOf(s string) int {
	n := 0
	for _, c := range []byte(s) {
		switch c {
		case ' ':
			n++
		case '\t':
			n += 4 - n%4
		default:
			return n
		}
	}
	return n
}

// columnBytes returns the bytes of the leading white space of s that fill width
// columns.
func columnBytes(s string, width int) int {
	i := 0
	for n := 0; n < width && i < len(s); i++ {
		if s[i] == '\t' {
			n += 4 - n%4
		} else {
			n++
		}
	}
	return i
}

// match returns the offset in the line raw after the containers that it continues,
// and their number. A blank line continues a list item.
func (r *blockReader) match(raw string) (int, int) {
	pos, matched := 0, 0
	for _, c := range r.stack {
		rest := raw[pos:]
		if c.quote {
			p := indentOf(rest)
			if p > 3 || p == len(rest) || rest[p] != '>' {
				break
			}
			pos = quoteEnd(raw, pos+p)
		} else if strings.TrimSpace(rest) != "" {
			if indentOf(rest) < c.width {
				break
			}
			pos += columnBytes(rest, c.width)
		}
		matched++
	}
	return pos, matched
}

// quoteEnd returns the offset after the > at offset at of raw and one space.
func quoteEnd(raw string, at int) int {
	at++
	if at < len(raw) && raw[at] == ' ' {
		at++
	}
	return at
}

// spanLine reports whether the line raw, which has no text outside code, starts
// with a code span. Such a line is text of a paragraph.
func spanLine(raw string) bool {
	s := strings.TrimLeft(raw, " \t>")
	return strings.HasPrefix(s, "`") && !strings.HasPrefix(s, "```")
}

func (r *blockReader) line(num, start, end int) {
	raw := r.text[start:end]
	pos, matched := r.match(raw)
	if r.inHTML && r.htmlLine(raw[pos:], matched == len(r.stack), start, end) {
		return
	}
	switch {
	case strings.TrimSpace(raw[pos:]) == "":
		r.leave(matched)
	case isBlank(r.masked[start:end]) && !spanLine(raw):
		r.closePara()
	default:
		r.content(num, start, end, pos, matched)
	}
}

// leave ends the open paragraph and the containers after the first matched.
func (r *blockReader) leave(matched int) {
	r.closePara()
	r.stack = r.stack[:matched]
}

// content reads the line at start, which has text, after its open containers.
func (r *blockReader) content(num, start, end, pos, matched int) {
	if matched == len(r.stack) && len(r.para) > 0 && setextLine.Match(r.masked[start+pos:end]) && r.setext() {
		return
	}
	pos, matched = r.open(start, end, pos, matched)
	rest, masked := r.text[start+pos:end], r.masked[start+pos:end]
	indent := indentOf(rest)
	switch {
	case strings.TrimSpace(rest) == "":
	case indent >= 4 && len(r.para) == 0:
		r.leave(matched)
		blank(r.masked[start:end])
		blank(r.links[start:end])
	case indent >= 4:
		r.para = append(r.para, paraLine{num, start, start + pos, end})
	case breakLine.Match(masked):
		r.leave(matched)
	case r.atx(rest, masked, start+pos, matched):
	case r.htmlStart(rest, masked, start, end, matched):
	default:
		if len(r.para) == 0 {
			r.stack = r.stack[:matched]
		}
		r.para = append(r.para, paraLine{num, start, start + pos, end})
	}
}

// open opens the containers that start on the line at start, from offset pos in
// the line. It returns the offset after them and the number of open containers.
func (r *blockReader) open(start, end, pos, matched int) (int, int) {
	raw := r.text[start:end]
	for {
		rest := raw[pos:]
		p := indentOf(rest)
		if p >= 4 || p == len(rest) {
			return pos, matched
		}
		if rest[p] == '>' && r.masked[start+pos+p] == '>' {
			r.leave(matched)
			r.stack = append(r.stack, container{quote: true})
			pos, matched = quoteEnd(raw, pos+p), len(r.stack)
			continue
		}
		if breakLine.Match(r.masked[start+pos : end]) {
			return pos, matched
		}
		size, width := listMarker(rest, len(r.para) > 0)
		if size == 0 {
			return pos, matched
		}
		r.leave(matched)
		r.stack = append(r.stack, container{width: width})
		pos, matched = pos+size, len(r.stack)
	}
}

// listMarker returns the size in bytes of the list marker and the white space at
// the start of s, and the indent of the content of the item. The size is 0 when s
// starts no item. In a paragraph, only an item with text can start, and an ordered
// item only with the number 1.
func listMarker(s string, inPara bool) (int, int) {
	i := columnBytes(s, indentOf(s))
	first := i
	switch {
	case i < len(s) && strings.IndexByte("-*+", s[i]) >= 0:
		i++
	default:
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			i++
		}
		if i == first || i-first > 9 || i == len(s) || (s[i] != '.' && s[i] != ')') {
			return 0, 0
		}
		if inPara && s[first:i] != "1" {
			return 0, 0
		}
		i++
	}
	rest := s[i:]
	if rest != "" && rest[0] != ' ' && rest[0] != '\t' {
		return 0, 0
	}
	if strings.TrimSpace(rest) == "" {
		if inPara {
			return 0, 0
		}
		return len(s), i + 1
	}
	spaces := indentOf(rest)
	if spaces >= 5 {
		return i + 1, i + 1
	}
	return i + columnBytes(rest, spaces), i + spaces
}

// atx records the ATX heading in rest, the text of a line at offset at, and
// reports whether there is one. A comment after the hashes in masked ends no
// heading, and the text comes from rest, with its code.
func (r *blockReader) atx(rest string, masked []byte, at, matched int) bool {
	if !atxHeading.Match(masked) {
		return false
	}
	m := atxHeading.FindStringSubmatch(rest)
	if m == nil {
		return false
	}
	r.leave(matched)
	r.headings = append(r.headings, heading{at, m[1]})
	return true
}

// setext makes the open paragraph a heading, with no definition at its start. It
// reports false when no text stays, or the text is a table.
func (r *blockReader) setext() bool {
	r.readDefinitions()
	if len(r.para) == 0 {
		return false
	}
	last := r.para[len(r.para)-1]
	if strings.HasPrefix(strings.TrimSpace(r.text[last.text:last.end]), "|") {
		return false
	}
	parts := make([]string, 0, len(r.para))
	for _, l := range r.para {
		parts = append(parts, strings.TrimSpace(r.text[l.text:l.end]))
	}
	r.headings = append(r.headings, heading{r.para[0].text, strings.Join(parts, "\n")})
	r.para = nil
	return true
}

// closePara ends the open paragraph.
func (r *blockReader) closePara() {
	r.readDefinitions()
	r.para = nil
}

func (r *blockReader) inList() bool {
	for _, c := range r.stack {
		if !c.quote {
			return true
		}
	}
	return false
}

// readDefinitions removes the reference definitions at the start of the open
// paragraph. A label that starts with ^ is a footnote.
func (r *blockReader) readDefinitions() {
	for len(r.para) > 0 {
		var joined strings.Builder
		for i, l := range r.para {
			if i > 0 {
				joined.WriteByte('\n')
			}
			joined.Write(r.masked[l.text:l.end])
		}
		s := joined.String()
		d, ok := parseDefinition(s)
		if !ok || d.label[0] == '^' {
			return
		}
		lines := strings.Count(s[:d.end], "\n") + 1
		d.link.Line = r.para[strings.Count(s[:d.at], "\n")].num
		d.inList = r.inList()
		r.defs = append(r.defs, d)
		for _, l := range r.para[:lines] {
			blank(r.links[l.start:l.end])
		}
		r.para = r.para[lines:]
	}
}
