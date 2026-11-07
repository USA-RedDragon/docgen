// Package markdown writes the Markdown parts of a generated body.
package markdown

import "strings"

// Fence returns s as a code block in language lang, with a fence longer than
// any fence in s.
func Fence(lang, s string) string {
	s = strings.TrimRight(s, "\n")
	f := "```"
	for strings.Contains(s, f) {
		f += "`"
	}
	return f + lang + "\n" + s + "\n" + f
}

// CodeSpan returns s as an inline code span, with a delimiter longer than any
// run of backticks in s. A value that starts or ends with a backtick or that
// has a space at both ends gets a space at each end, which Markdown removes. A
// line ending is a space, as Markdown shows it, so a blank line does not end the
// span. An empty s has no span, so it shows as two quotes.
func CodeSpan(s string) string {
	if s == "" {
		return "`\"\"`"
	}
	s = strings.NewReplacer("\r\n", " ", "\n", " ").Replace(s)
	longest, run := 0, 0
	for i := range len(s) {
		run++
		if s[i] != '`' {
			run = 0
		}
		longest = max(longest, run)
	}
	d := strings.Repeat("`", longest+1)
	if strings.HasPrefix(s, "`") || strings.HasSuffix(s, "`") || strings.HasPrefix(s, " ") && strings.HasSuffix(s, " ") && strings.TrimSpace(s) != "" {
		s = " " + s + " "
	}
	return d + s + d
}

// Cell returns s as the text of one table cell.
func Cell(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	return strings.ReplaceAll(s, "|", `\|`)
}

// PkgLink returns the pkg.go.dev URL of the symbol name of the package ip.
func PkgLink(ip, name string) string {
	return "https://pkg.go.dev/" + ip + "#" + name
}
