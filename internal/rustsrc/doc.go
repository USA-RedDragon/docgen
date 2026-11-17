package rustsrc

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	headingLine = regexp.MustCompile(`^ {0,3}(#{1,6})(?:[ \t]+(.*?))?(?:[ \t]+#+)?[ \t]*$`)
	fenceLine   = regexp.MustCompile("^ {0,3}(`{3,}|~{3,})[ \t]*([^`]*)$")
	refDef      = regexp.MustCompile(`^ {0,3}\[([^\]]+)\]:[ \t]*(\S+)[ \t]*$`)
	link        = regexp.MustCompile("\\[((?:[^\\[\\]`]|`[^`]*`)+)\\](?:\\(([^()\\s]+)\\)|\\[([^\\]]*)\\])?")
	rustPath    = regexp.MustCompile(`^(?:::)?[A-Za-z_][A-Za-z0-9_]*(?:::[A-Za-z_][A-Za-z0-9_]*)*$`)
	disambig    = regexp.MustCompile(`^(?:struct|enum|trait|union|mod|module|const|constant|fn|function|method|derive|type|value|macro|prim|primitive|static|field|variant|tymethod)@`)
)

// rustdocAttr reports whether w is a word of the info string of a code block
// that rustdoc tests as Rust.
func rustdocAttr(w string) bool {
	switch w {
	case "rust", "ignore", "should_panic", "no_run", "compile_fail", "test_harness", "standalone_crate",
		"edition2015", "edition2018", "edition2021", "edition2024":
		return true
	}
	return strings.HasPrefix(w, "ignore-")
}

// Markdown returns a rustdoc comment as Markdown for a page:
//
//   - A heading moves down two levels, so # is ###.
//   - A code block with no language, or with only rustdoc words such as
//     no_run, is a rust block, with no hidden lines (# at the start).
//   - An intra-doc link, such as [`Config`] or [text](crate::Config), is its
//     text, since a page has no rustdoc to resolve it.
//
// A section other than "" selects the text below the heading with that text,
// up to the next heading of the same or a higher level. what names the item in
// errors.
func Markdown(doc, section, what string) (string, error) {
	if strings.TrimSpace(doc) == "" {
		return "", fmt.Errorf("no doc comment for %s", what)
	}
	lines := strings.Split(doc, "\n")
	if section != "" {
		var err error
		if lines, err = sectionLines(lines, section, what); err != nil {
			return "", err
		}
	}
	return strings.Trim(strings.Join(rewrite(lines), "\n"), "\n"), nil
}

// sectionLines returns the lines below the heading section.
func sectionLines(lines []string, section, what string) ([]string, error) {
	level := 0
	var out []string
	inFence := ""
	for _, l := range lines {
		if inFence == "" {
			if m := headingLine.FindStringSubmatch(l); m != nil {
				n := len(m[1])
				if level > 0 && n <= level {
					break
				}
				if level == 0 && headingText(m[2]) == section {
					level = n
					continue
				}
			}
		}
		inFence = fenceState(inFence, l)
		if level > 0 {
			out = append(out, l)
		}
	}
	if level == 0 {
		return nil, fmt.Errorf("no heading # %s in the doc of %s", section, what)
	}
	if strings.TrimSpace(strings.Join(out, "")) == "" {
		return nil, fmt.Errorf("heading # %s in the doc of %s has no text", section, what)
	}
	return out, nil
}

func headingText(s string) string {
	return strings.TrimSpace(strings.ReplaceAll(s, "`", ""))
}

// fenceState returns the fence that is open after line l, with fence the one
// open before it.
func fenceState(fence, l string) string {
	m := fenceLine.FindStringSubmatch(l)
	switch {
	case m == nil:
		return fence
	case fence == "":
		return m[1]
	case m[1][0] == fence[0] && len(m[1]) >= len(fence) && strings.TrimSpace(m[2]) == "":
		return ""
	}
	return fence
}

func isRustInfo(info string) bool {
	info = strings.TrimSpace(info)
	if info == "" {
		return true
	}
	for _, w := range strings.FieldsFunc(info, func(r rune) bool { return r == ',' || r == ' ' || r == '\t' }) {
		if !rustdocAttr(w) {
			return false
		}
	}
	return true
}

func rewrite(lines []string) []string {
	defs := map[string]bool{}
	for _, l := range lines {
		if m := refDef.FindStringSubmatch(l); m != nil && !isPath(m[2]) {
			defs[strings.ToLower(m[1])] = true
		}
	}
	out := make([]string, 0, len(lines))
	fence, rust := "", false
	for _, l := range lines {
		if fence != "" {
			next := fenceState(fence, l)
			if next == "" {
				fence = ""
				out = append(out, l)
				continue
			}
			if rust {
				t := strings.TrimLeft(l, " \t")
				if t == "#" || strings.HasPrefix(t, "# ") {
					continue
				}
				if strings.HasPrefix(t, "##") {
					l = l[:len(l)-len(t)] + t[1:]
				}
			}
			out = append(out, l)
			continue
		}
		if m := fenceLine.FindStringSubmatch(l); m != nil {
			fence, rust = m[1], isRustInfo(m[2])
			if rust {
				l = l[:len(l)-len(strings.TrimLeft(l, " "))] + m[1] + "rust"
			}
			out = append(out, l)
			continue
		}
		if m := refDef.FindStringSubmatch(l); m != nil && isPath(m[2]) {
			continue
		}
		if m := headingLine.FindStringSubmatch(l); m != nil {
			level := min(len(m[1])+2, 6)
			out = append(out, strings.Repeat("#", level)+" "+strings.TrimSpace(m[2]))
			continue
		}
		out = append(out, links(l, defs))
	}
	return out
}

// isPath reports whether s is the target of an intra-doc link.
func isPath(s string) bool {
	s = strings.Trim(s, "`")
	s = disambig.ReplaceAllString(s, "")
	s = strings.TrimSuffix(strings.TrimSuffix(s, "()"), "!")
	return rustPath.MatchString(s)
}

// links returns l with each intra-doc link as its text. Links in code spans
// stay.
func links(l string, defs map[string]bool) string {
	spans := codeSpans(l)
	var b strings.Builder
	at := 0
	for _, m := range link.FindAllStringSubmatchIndex(l, -1) {
		if inSpan(spans, m[0]) {
			continue
		}
		text := l[m[2]:m[3]]
		intra := isIntra(l, m, text, defs)
		if !intra {
			continue
		}
		b.WriteString(l[at:m[0]])
		b.WriteString(text)
		at = m[1]
	}
	b.WriteString(l[at:])
	return b.String()
}

// isIntra reports whether the link match m of l, with the text text, is an
// intra-doc link: its target, or its label with no definition in defs, is a
// Rust path.
func isIntra(l string, m []int, text string, defs map[string]bool) bool {
	switch {
	case m[4] >= 0:
		return isPath(l[m[4]:m[5]])
	case m[6] >= 0:
		label := l[m[6]:m[7]]
		if label == "" {
			label = text
		}
		return !defs[strings.ToLower(label)] && isPath(label)
	}
	next := ""
	if m[1] < len(l) {
		next = l[m[1] : m[1]+1]
	}
	return next != ":" && !defs[strings.ToLower(text)] && isPath(text)
}

// codeSpans returns the byte ranges of the code spans of l.
func codeSpans(l string) [][2]int {
	var out [][2]int
	for i := 0; i < len(l); {
		if l[i] != '`' {
			i++
			continue
		}
		n := 0
		for i+n < len(l) && l[i+n] == '`' {
			n++
		}
		closeAt := -1
		for j := i + n; j < len(l); {
			if l[j] != '`' {
				j++
				continue
			}
			k := 0
			for j+k < len(l) && l[j+k] == '`' {
				k++
			}
			if k == n {
				closeAt = j + k
				break
			}
			j += k
		}
		if closeAt < 0 {
			i += n
			continue
		}
		out = append(out, [2]int{i, closeAt})
		i = closeAt
	}
	return out
}

func inSpan(spans [][2]int, at int) bool {
	for _, s := range spans {
		if at > s[0] && at < s[1] {
			return true
		}
	}
	return false
}
