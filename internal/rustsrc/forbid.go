package rustsrc

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

var (
	// forbidHead matches the first line of E0453, in the form of rustc 1.52
	// and later and in the older form.
	forbidHead  = regexp.MustCompile(`^error\[E0453\]: allow\(([^)]*)\) (?:incompatible with previous forbid|overruled by outer forbid\(([^)]*)\))`)
	snippetFile = regexp.MustCompile(`^\s*(?:-->|:::)\s+(.+?):\d+:\d+`)
	snippetLine = regexp.MustCompile(`^\s*(\d+)\s*\|(.*)$`)
	forbidLabel = regexp.MustCompile(`^\s*\|(\s*)([-^]+)\s+` + "`forbid` level set here")
)

// A forbid is a lint that a crate forbids, where it does, and the attribute
// that forbids it, which are empty when the snippet of rustc shows no
// attribute.
type forbid struct {
	lint, place, attr string
}

// forbidError returns the error of a test build of crate pkg in the workspace
// copy work whose diagnostics diags hold E0453: the crate forbids a lint that
// the hook allows, and a forbid admits no allow, so no method with a hook
// builds.
func forbidError(pkg, work string, diags []string) error {
	var found []forbid
	for _, d := range diags {
		f, ok := forbidOf(work, d)
		if ok && !slices.Contains(found, f) {
			found = append(found, f)
		}
	}
	if len(found) == 0 {
		return nil
	}
	lines := make([]string, 0, len(found))
	for _, f := range found {
		switch {
		case f.attr != "":
			lines = append(lines, fmt.Sprintf("  - %s at %s forbids %s", f.attr, f.place, f.lint))
		default:
			lines = append(lines, "  - forbid("+f.lint+"), from an attribute or from -F in RUSTFLAGS")
		}
	}
	return fmt.Errorf("crate %s forbids a lint that the state hook allows in the code that it adds, and a forbid admits no allow, so no method with the hook builds:\n%s\n"+
		"Use deny in place of forbid, such as #![deny(%s)]: a deny keeps the lint an error and admits the allow of the hook",
		pkg, strings.Join(lines, "\n"), found[0].lint)
}

// forbidOf reads the forbidden lint and its attribute from diagnostic d of a
// build in work, and reports whether d is E0453. The attribute is the line of
// the source file, since rustc cuts a long line of a snippet.
func forbidOf(work, d string) (forbid, bool) {
	lines := strings.Split(d, "\n")
	head := forbidHead.FindStringSubmatch(lines[0])
	if head == nil {
		return forbid{}, false
	}
	f := forbid{lint: head[1]}
	if head[2] != "" {
		f.lint = head[2]
	}
	var file, num, src string
	for _, l := range lines[1:] {
		if m := snippetFile.FindStringSubmatch(l); m != nil {
			file = m[1]
			continue
		}
		if m := snippetLine.FindStringSubmatch(l); m != nil {
			num, src = m[1], m[2]
			continue
		}
		m := forbidLabel.FindStringSubmatch(l)
		if m == nil || file == "" || src == "" {
			continue
		}
		start := len(m[1])
		end := start + len(m[2])
		if end <= len(src) {
			f.lint = src[start:end]
		}
		f.place = file + ":" + num
		f.attr = strings.TrimSpace(src)
		if line := sourceLine(work, file, num); line != "" {
			f.attr = strings.TrimSpace(line)
		}
		break
	}
	return f, true
}

// sourceLine returns line num of file, a path from the directory work or an
// absolute path, and "" when it cannot be read.
func sourceLine(work, file, num string) string {
	n, err := strconv.Atoi(num)
	if err != nil || n < 1 {
		return ""
	}
	if !filepath.IsAbs(file) {
		file = filepath.Join(work, file)
	}
	b, err := os.ReadFile(file)
	if err != nil {
		return ""
	}
	lines := strings.Split(string(b), "\n")
	if n > len(lines) {
		return ""
	}
	return lines[n-1]
}
