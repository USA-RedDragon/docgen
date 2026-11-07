// Package links finds the links and the anchors of a Markdown page.
package links

import (
	"bytes"
	"html"
	"net/url"
	"regexp"
	"strings"

	"github.com/USA-RedDragon/docgen/internal/marker"
)

// A Link is an inline link or image, or a link reference definition. Line is the
// line of its target. Target is the destination with its escapes resolved. For a
// reference with no definition, Undefined is true and Target is "[label]".
type Link struct {
	Line      int
	Target    string
	Undefined bool
}

var (
	scheme  = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9+.-]*:`)
	comment = regexp.MustCompile(`(?s)<!--.*?-->`)
	escape  = regexp.MustCompile(`\\([!-/:-@\[-` + "`" + `{-~])`)
)

// byteOrderMark starts some pages. It is not part of their first line.
const byteOrderMark = "\ufeff"

func blank(b []byte) {
	for i := range b {
		if b[i] != '\n' {
			b[i] = ' '
		}
	}
}

// maskText returns text with its fenced code blocks, code spans and comments
// replaced by spaces. Each line stays at its number.
func maskText(text string) []byte {
	b := []byte(marker.MaskCode(text))
	for _, loc := range comment.FindAllIndex(b, -1) {
		blank(b[loc[0]:loc[1]])
	}
	return b
}

func lineLen(b []byte) int {
	if i := bytes.IndexByte(b, '\n'); i >= 0 {
		return i
	}
	return len(b)
}

func isSpace(c byte) bool {
	return c == ' ' || c == '\t'
}

// label returns the form of a reference label that two labels compare in.
func label(s string) string {
	return strings.Join(strings.Fields(fold(s)), " ")
}

// destination returns the target of the destination s of a link.
func destination(s string) string {
	s = strings.TrimSuffix(strings.TrimPrefix(s, "<"), ">")
	return html.UnescapeString(escape.ReplaceAllString(s, "$1"))
}

// Scan returns the links of a page with LF line endings, in page order within each
// kind: code is text, and the bodies of markers are part of the page. HTML blocks
// hold no link.
func Scan(text string) []Link {
	pg := parseBlocks(strings.TrimPrefix(text, byteOrderMark))
	var out []Link
	labels := make(map[string]*definition, len(pg.defs))
	for i := range pg.defs {
		d := &pg.defs[i]
		key := label(d.label)
		if labels[key] == nil {
			labels[key] = d
		}
		if !d.inList {
			out = append(out, d.link)
		}
	}
	return append(out, inline(pg.links, labels)...)
}

func lineOf(b []byte, at int) int {
	return bytes.Count(b[:at], []byte("\n")) + 1
}

// An opener is an open bracket of a paragraph: its offset, whether it opens an
// image, and false in active after a link inside it, since a link holds no link.
type opener struct {
	at     int
	image  bool
	active bool
}

// A linkScan holds the state of inline: the page, its definitions, and the links.
type linkScan struct {
	b      []byte
	labels map[string]*definition
	out    []Link
	stack  []opener
}

// inline returns the inline links of b, a Link for each reference with a label
// that is not in labels, and the target of each definition in a list item that a
// reference uses. Brackets pair inside one paragraph.
func inline(b []byte, labels map[string]*definition) []Link {
	s := &linkScan{b: b, labels: labels}
	for i := 0; i < len(b); {
		switch b[i] {
		case '\\':
			i += 2
			continue
		case '\n':
			if next := b[i+1:]; len(bytes.TrimSpace(next[:lineLen(next)])) == 0 {
				s.stack = nil
			}
		case '!':
			if i+1 < len(b) && b[i+1] == '[' {
				s.stack = append(s.stack, opener{i + 1, true, true})
				i += 2
				continue
			}
		case '[':
			s.stack = append(s.stack, opener{i, false, true})
		case ']':
			if end, ok := s.closeBracket(i); ok {
				i = end
				continue
			}
		}
		i++
	}
	return s.out
}

// closeBracket reads the ] at offset i, and returns the offset after what follows
// it when that is part of a link.
func (s *linkScan) closeBracket(i int) (int, bool) {
	if len(s.stack) == 0 {
		return 0, false
	}
	o := s.stack[len(s.stack)-1]
	s.stack = s.stack[:len(s.stack)-1]
	if !o.active {
		return 0, false
	}
	formed, end, ok := s.after(o, i)
	if formed && !o.image {
		for k := range s.stack {
			s.stack[k].active = s.stack[k].image
		}
	}
	return end, ok
}

// use returns whether the label name has a definition. A definition in a list item
// adds its target once.
func (s *linkScan) use(name string) bool {
	d := s.labels[label(name)]
	if d == nil {
		return false
	}
	if d.inList {
		d.inList = false
		s.out = append(s.out, d.link)
	}
	return true
}

// after reads what follows the ] at offset i, the end of the text of o. It reports
// whether the brackets form a link, the offset after what that link reads, and
// whether it read anything.
func (s *linkScan) after(o opener, i int) (bool, int, bool) {
	b, at := s.b, i+1
	text := string(b[o.at+1 : i])
	if at < len(b) && b[at] == '(' {
		if dest, pos, end, ok := parseTarget(b, at+1); ok {
			s.out = append(s.out, Link{Line: lineOf(b, pos), Target: destination(dest)})
			return true, end, true
		}
	}
	if at < len(b) && b[at] == '[' {
		if last := labelEnd(b, at+1); last >= 0 {
			return s.reference(text, string(b[at+1:last]), at, last+1)
		}
	}
	return s.use(text), 0, false
}

// reference reads the reference at offset at up to the offset end. text is what
// its first brackets hold, and name is its label, which is empty for a collapsed
// reference. An undefined label before another bracket or a target is text.
func (s *linkScan) reference(text, name string, at, end int) (bool, int, bool) {
	if strings.TrimSpace(name) == "" {
		name = text
	} else if end < len(s.b) && (s.b[end] == '(' || s.b[end] == '[') && s.labels[label(name)] == nil {
		return false, 0, false
	}
	if strings.TrimSpace(name) == "" || name[0] == '^' {
		return false, end, true
	}
	if s.use(name) {
		return true, end, true
	}
	s.out = append(s.out, Link{Line: lineOf(s.b, at), Target: "[" + name + "]", Undefined: true})
	return false, end, true
}

func skipSpace(b []byte, i int) int {
	for i < len(b) && (isSpace(b[i]) || b[i] == '\n') {
		i++
	}
	return i
}

// parseTarget reads a destination and an optional title from offset i, after the
// ( of a link, up to the closing ). It returns the destination, its offset and the
// offset after the ).
func parseTarget(b []byte, i int) (string, int, int, bool) {
	i = skipSpace(b, i)
	start := i
	end, ok := destinationEnd(b, i)
	if !ok {
		return "", 0, 0, false
	}
	dest := string(b[start:end])
	i = skipSpace(b, end)
	if i < len(b) && i > end {
		if i, ok = titleEnd(b, i); !ok {
			return "", 0, 0, false
		}
		i = skipSpace(b, i)
	}
	if i >= len(b) || b[i] != ')' {
		return "", 0, 0, false
	}
	return dest, start, i + 1, true
}

// destinationEnd returns the offset after the destination at offset i: text in
// angle brackets on one line, or text with no space and balanced parentheses.
func destinationEnd(b []byte, i int) (int, bool) {
	if i < len(b) && b[i] == '<' {
		for j := i + 1; j < len(b) && b[j] != '\n' && b[j] != '<'; j++ {
			switch b[j] {
			case '\\':
				j++
			case '>':
				return j + 1, true
			}
		}
		return 0, false
	}
	depth := 0
	for ; i < len(b); i++ {
		switch c := b[i]; {
		case c == '\\':
			i++
		case c == '(':
			depth++
		case c == ')' && depth == 0:
			return i, true
		case c == ')':
			depth--
		case c <= ' ':
			return i, depth == 0
		}
	}
	return 0, false
}

// titleEnd returns the offset after the title at offset i, or i for a ).
func titleEnd(b []byte, i int) (int, bool) {
	var closer byte
	switch b[i] {
	case ')':
		return i, true
	case '"', '\'':
		closer = b[i]
	case '(':
		closer = ')'
	default:
		return 0, false
	}
	for j := i + 1; j < len(b); j++ {
		switch b[j] {
		case '\\':
			j++
		case closer:
			return j + 1, true
		}
	}
	return 0, false
}

// Split returns the decoded path and fragment of target, with no query. It returns
// false for a target that is not in the repository: one with a scheme or a host.
func Split(target string) (path, fragment string, local bool) {
	if scheme.MatchString(target) || strings.HasPrefix(target, "//") {
		return "", "", false
	}
	path, fragment, _ = strings.Cut(target, "#")
	path, _, _ = strings.Cut(path, "?")
	return decode(path), decode(fragment), true
}

func decode(s string) string {
	if d, err := url.PathUnescape(s); err == nil {
		return d
	}
	return s
}
