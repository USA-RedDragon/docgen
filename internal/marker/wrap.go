package marker

import (
	"strings"
)

// Wrap returns content as the body of m: as is for an inline marker, and
// otherwise on the lines between the open and the close tag. Outside code, each
// comment opener in the form of a docgen tag becomes "&lt;!--", so that the
// body holds no tag and parses the same way on the next run.
func (m Marker) Wrap(content string) string {
	body := content
	if !m.inline {
		body = "\n" + strings.TrimRight(content, "\n") + "\n"
	}
	for {
		next := m.neutralize(body)
		if next == body {
			return body
		}
		body = next
	}
}

// neutralize replaces the comment openers of one pass. A replaced opener at the
// start of a line no longer ends a paragraph, which can move the code spans
// after it, so Wrap repeats the pass until nothing changes.
func (m Marker) neutralize(body string) string {
	masked := maskCode(body, scope{inBody: true, indent: m.indent, closeTag: m.closeTag(), generated: true})
	var b strings.Builder
	last := 0
	for _, loc := range tagLike.FindAllIndex(masked, -1) {
		start := loc[0]
		if start > 0 && masked[start-1] == '\\' {
			start--
		}
		b.WriteString(body[last:start])
		b.WriteString("&lt;")
		last = loc[0] + 1
	}
	b.WriteString(body[last:])
	return b.String()
}
