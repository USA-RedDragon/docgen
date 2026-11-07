package marker

import (
	"fmt"
	"regexp"
	"strings"
)

const (
	// quotedText matches the text between the double quotes of a value. A
	// backslash and the character after it are one unit, so \" is text.
	quotedText  = `(?:[^"\\]|\\[\s\S])*`
	openPattern = `<!-- docgen:([a-z]+)((?:\s+[a-z]+=(?:"` + quotedText + `"|[^\s>"]+))*)\s*-->`
)

var (
	openTag = regexp.MustCompile(openPattern)
	openAt  = regexp.MustCompile(`\A` + openPattern)
	// tagLike matches the start of each comment in the form of a docgen tag.
	tagLike = regexp.MustCompile(`(?i)<!--+\s*/?\s*docgen\s*:`)
	// regionMark matches a region mark, the bounds of an excerpt of the page.
	regionMark      = regexp.MustCompile(`\A<!--\s*docgen:(?:end\s*|region \S+\s+)-->`)
	wellFormedClose = regexp.MustCompile(`\A<!-- /docgen:[a-z]+ -->\z`)
)

// badTag returns the error for the comment at offset at, which has the form of
// a docgen tag and is not one.
func badTag(text string, at int) error {
	line := strings.Count(text[:at], "\n") + 1
	tag := text[at:]
	if i := strings.IndexByte(tag, '\n'); i >= 0 {
		tag = tag[:i]
	}
	if i := strings.Index(tag, "-->"); i >= 0 {
		tag = tag[:i+3]
	}
	if wellFormedClose.MatchString(tag) {
		return fmt.Errorf("line %d: %s has no open tag", line, tag)
	}
	return fmt.Errorf("line %d: malformed marker comment %s", line, tag)
}

// Parse returns the markers of a page with LF line endings. A marker in code
// is text, not a marker. A comment outside a marker body in the form of a
// docgen tag that is neither a marker nor a region mark is an error. Parse calls
// lookup once for each marker, in page order, and stops at the first error.
func Parse(text string, lookup Lookup) ([]Marker, error) {
	var out []Marker
	masked := maskCode(text, scope{})
	for pos := 0; ; {
		loc := tagLike.FindIndex(masked[pos:])
		if loc == nil {
			return out, nil
		}
		at := pos + loc[0]
		if mark := regionMark.FindIndex(masked[at:]); mark != nil {
			pos = at + mark[1]
			continue
		}
		open := openAt.FindSubmatchIndex(masked[at:])
		if open == nil {
			return nil, badTag(text, at)
		}
		m, next, err := parseMarker(text, masked, at, open, lookup)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
		pos = next
	}
}

// parseMarker reads the marker whose open tag starts at offset at, with the
// submatches open of openAt, and returns it with the offset after its close
// tag. It masks the rest of the page again as text that starts in the body.
func parseMarker(text string, masked []byte, at int, open []int, lookup Lookup) (Marker, int, error) {
	m := Marker{
		Kind:   text[at+open[2] : at+open[3]],
		raw:    strings.TrimSpace(text[at+open[4] : at+open[5]]),
		Line:   strings.Count(text[:at], "\n") + 1,
		Start:  at + open[1],
		indent: indentOf(text[strings.LastIndexByte(text[:at], '\n')+1 : at]),
	}
	binder := lookup(m.Kind)
	if binder == nil {
		return m, 0, fmt.Errorf("line %d: unknown marker kind docgen:%s", m.Line, m.Kind)
	}
	args := newArgs(m)
	binder.Bind(args)
	if err := args.check(); err != nil {
		return m, 0, err
	}
	m.inline = args.inline
	closeTag := m.closeTag()
	rel := maskBody(text, masked, m.Start, m.indent, closeTag)
	if rel < 0 {
		return m, 0, fmt.Errorf("line %d: docgen:%s has no %s", m.Line, m.Kind, closeTag)
	}
	closeAt := m.Start + rel
	body := masked[m.Start:closeAt]
	for _, next := range openTag.FindAllIndex(body, -1) {
		if !regionMark.Match(body[next[0]:]) {
			return m, 0, fmt.Errorf("line %d: docgen:%s holds another marker before its %s", m.Line, m.Kind, closeTag)
		}
	}
	m.End = closeAt
	if !m.inline {
		m.End = bodyEnd(text, m.Start, closeAt)
	}
	m.Body = text[m.Start:m.End]
	if !m.inline && !strings.HasPrefix(m.Body, "\n") && m.Body != "" {
		return m, 0, fmt.Errorf("line %d: docgen:%s: the body goes on the lines after the open tag", m.Line, m.Kind)
	}
	return m, closeAt + len(closeTag), nil
}

// bodyEnd returns the end of a block body whose close tag is at closeAt: the
// start of the line of the tag when only white space and block quote marks
// come before the tag on that line. They are part of the page, not the body.
func bodyEnd(text string, start, closeAt int) int {
	nl := strings.LastIndexByte(text[start:closeAt], '\n')
	if nl < 0 || strings.Trim(text[start+nl+1:closeAt], " \t>") != "" {
		return closeAt
	}
	return start + nl + 1
}
