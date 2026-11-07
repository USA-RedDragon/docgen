package links

import (
	"html"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

var (
	idAttr      = regexp.MustCompile(`<[a-zA-Z][^<>]*?\sid\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s"'<>]+))`)
	nameAttr    = regexp.MustCompile(`<[aA]\s[^<>]*?\bname\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s"'<>]+))`)
	image       = regexp.MustCompile(`!\[[^\]]*\](?:\([^)]*\)|\[[^\]]*\])`)
	linkText    = regexp.MustCompile(`\[([^\]]*)\](?:\([^)]*\)|\[[^\]]*\])`)
	htmlTag     = regexp.MustCompile(`</?[a-zA-Z][^<>]*>`)
	emphasis    = regexp.MustCompile(`(^|[^\pL\pN_])_+([^\s_](?:.*?[^\s_])??)_+($|[^\pL\pN_])`)
	htmlHeading = regexp.MustCompile(`(?is)<h[1-6](?:\s[^<>]*)?>(.*?)</h[1-6]\s*>`)
)

// Anchors returns the anchors of a page with LF line endings: the id of each
// heading by the rules of GitHub, HTML headings included, and each id= attribute
// and <a name= attribute.
func Anchors(text string) map[string]bool {
	text = strings.TrimPrefix(text, byteOrderMark)
	pg := parseBlocks(text)
	heads := pg.headings
	for _, m := range htmlHeading.FindAllSubmatchIndex(pg.masked, -1) {
		heads = append(heads, heading{m[0], strings.TrimSpace(text[m[2]:m[3]])})
	}
	sort.Slice(heads, func(i, j int) bool { return heads[i].at < heads[j].at })
	out := map[string]bool{}
	counts := map[string]int{}
	for _, h := range heads {
		base := slug(h.text)
		id := base
		for out[id] {
			counts[base]++
			id = base + "-" + strconv.Itoa(counts[base])
		}
		out[id] = true
	}
	for _, re := range []*regexp.Regexp{idAttr, nameAttr} {
		for _, m := range re.FindAllSubmatch(pg.masked, -1) {
			out[string(m[1])+string(m[2])+string(m[3])] = true
		}
	}
	return out
}

// plain returns the Markdown text s as it renders, with no inline syntax.
func plain(s string) string {
	s = image.ReplaceAllString(s, "")
	s = linkText.ReplaceAllString(s, "$1")
	s = htmlTag.ReplaceAllString(s, "")
	for {
		next := emphasis.ReplaceAllString(s, "$1$2$3")
		if next == s {
			break
		}
		s = next
	}
	return html.UnescapeString(escape.ReplaceAllString(s, "$1"))
}

// backticks returns the length of the run of backticks at offset i of s.
func backticks(s string, i int) int {
	n := 0
	for i+n < len(s) && s[i+n] == '`' {
		n++
	}
	return n
}

// spanClose returns the offset of the run of exactly n backticks from offset i of s,
// or -1.
func spanClose(s string, i, n int) int {
	for i < len(s) {
		run := backticks(s, i)
		if run == n {
			return i
		}
		i += max(run, 1)
	}
	return -1
}

// spanText returns the text of a code span with the content s: one space at each
// end goes when both ends hold one and s is not all spaces.
func spanText(s string) string {
	if len(s) > 2 && s[0] == ' ' && s[len(s)-1] == ' ' && strings.TrimSpace(s) != "" {
		return s[1 : len(s)-1]
	}
	return s
}

// rendered returns the heading text s as it renders: its code spans as they are,
// and the rest with no inline syntax.
func rendered(s string) string {
	var text strings.Builder
	last := 0
	for i := 0; i < len(s); {
		n := backticks(s, i)
		if n == 0 {
			i++
			continue
		}
		end := spanClose(s, i+n, n)
		if end < 0 {
			i += n
			continue
		}
		text.WriteString(plain(s[last:i]))
		text.WriteString(spanText(s[i+n : end]))
		i = end + n
		last = i
	}
	text.WriteString(plain(s[last:]))
	return text.String()
}

// slug returns the anchor of the heading text s without its number: lower case,
// with no character but word characters, hyphens and spaces, and with a hyphen for
// each space. Text that renders as nothing leaves the spaces around it.
func slug(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(rendered(comment.ReplaceAllString(s, ""))) {
		switch {
		case r == ' ':
			b.WriteByte('-')
		case r == '-' || unicode.In(r, unicode.L, unicode.M, unicode.Nd, unicode.Nl, unicode.Pc, unicode.Other_Alphabetic):
			b.WriteRune(r)
		}
	}
	return b.String()
}
