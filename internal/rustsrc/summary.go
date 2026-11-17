package rustsrc

import (
	"strings"
)

// CrateSummary returns the first line of the inner doc comment (//!) at the top
// of the crate root src, trimmed, with the # and the space of a Markdown
// heading removed. It reads the text and does not parse Rust, so it needs no
// helper. Blank lines, line comments and inner attributes may come before the
// doc comment. A source with no such comment, and one whose first doc line is
// a code fence, give "". A block doc comment (/*! */) is not read.
func CrateSummary(src []byte) string {
	for _, l := range strings.Split(string(src), "\n") {
		t := strings.TrimSpace(l)
		switch {
		case strings.HasPrefix(t, "//!"):
			text := strings.TrimSpace(strings.TrimPrefix(t, "//!"))
			if text == "" {
				continue
			}
			if strings.HasPrefix(text, "```") || strings.HasPrefix(text, "~~~") {
				return ""
			}
			return strings.TrimSpace(strings.TrimLeft(text, "#"))
		case t == "", strings.HasPrefix(t, "//"), strings.HasPrefix(t, "#!["):
		default:
			return ""
		}
	}
	return ""
}
