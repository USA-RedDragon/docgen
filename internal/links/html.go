package links

import "regexp"

var (
	rawBlock    = regexp.MustCompile(`(?i)^ {0,3}<(?:pre|script|style|textarea)(?:[ \t>]|$)`)
	rawBlockEnd = regexp.MustCompile(`(?i)</(?:pre|script|style|textarea)>`)
	piBlock     = regexp.MustCompile(`^ {0,3}<\?`)
	piBlockEnd  = regexp.MustCompile(`\?>`)
	dataBlock   = regexp.MustCompile(`^ {0,3}<!\[CDATA\[`)
	dataEnd     = regexp.MustCompile(`\]\]>`)
	declBlock   = regexp.MustCompile(`^ {0,3}<![a-zA-Z]`)
	declEnd     = regexp.MustCompile(`>`)
	tagBlock    = regexp.MustCompile(`(?i)^ {0,3}</?(?:address|article|aside|base|basefont|blockquote|body|caption|center|col|colgroup|dd|details|dialog|dir|div|dl|dt|fieldset|figcaption|figure|footer|form|frame|frameset|h[1-6]|head|header|hr|html|iframe|legend|li|link|main|menu|menuitem|meta|nav|noframes|ol|optgroup|option|p|param|search|section|summary|table|tbody|td|tfoot|th|thead|title|tr|track|ul)(?:[ \t]|/?>|$)`)
	loneTag     = regexp.MustCompile(`^ {0,3}(?:<[a-zA-Z][a-zA-Z0-9-]*(?:[ \t]+[a-zA-Z_:][a-zA-Z0-9_.:-]*(?:[ \t]*=[ \t]*(?:[^ \t"'=<>` + "`" + `]+|'[^']*'|"[^"]*"))?)*[ \t]*/?>|</[a-zA-Z][a-zA-Z0-9-]*[ \t]*>)[ \t]*$`)
)

// htmlEnd reports whether the masked line starts an HTML block, and returns the
// pattern of the line that ends the block, or nil for a block that ends at a blank
// line. A block of a lone tag does not interrupt a paragraph.
func htmlEnd(masked []byte, inPara bool) (*regexp.Regexp, bool) {
	switch {
	case rawBlock.Match(masked):
		return rawBlockEnd, true
	case piBlock.Match(masked):
		return piBlockEnd, true
	case dataBlock.Match(masked):
		return dataEnd, true
	case declBlock.Match(masked):
		return declEnd, true
	case tagBlock.Match(masked):
		return nil, true
	case !inPara && loneTag.Match(masked):
		return nil, true
	}
	return nil, false
}

// htmlStart starts the HTML block at the line from start to end, with the text
// rest and masked after its containers, and reports whether there is one.
func (r *blockReader) htmlStart(rest string, masked []byte, start, end, matched int) bool {
	ends, ok := htmlEnd(masked, len(r.para) > 0)
	if !ok {
		return false
	}
	r.leave(matched)
	blank(r.links[start:end])
	r.inHTML = ends == nil || !ends.MatchString(rest)
	r.htmlEnds = ends
	return true
}

// htmlLine reads a line of the open HTML block, with the text rest after its
// containers. It reports false when the block ended before the line: at a blank
// line, or at the end of a container of the block.
func (r *blockReader) htmlLine(rest string, inside bool, start, end int) bool {
	if !inside || (r.htmlEnds == nil && isBlank([]byte(rest))) {
		r.inHTML = false
		return false
	}
	blank(r.links[start:end])
	if r.htmlEnds != nil && r.htmlEnds.MatchString(rest) {
		r.inHTML = false
	}
	return true
}
