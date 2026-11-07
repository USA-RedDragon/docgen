// Package marker parses the docgen marker comments of a Markdown page.
package marker

// A Marker is one generated part of a page. Start and End are the byte offsets
// of its body in the page text, and Line is the line of its open tag.
type Marker struct {
	Kind   string
	raw    string
	Line   int
	Start  int
	End    int
	Body   string
	inline bool
	indent int
}

// String returns the marker as error messages and stale lists print it.
func (m Marker) String() string {
	return "docgen:" + m.Kind + " " + m.raw
}

func (m Marker) closeTag() string {
	return "<!-- /docgen:" + m.Kind + " -->"
}
