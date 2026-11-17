package rustsrc

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/USA-RedDragon/docgen/internal/cargo"
)

// The print= values of a Rust value, other than a format spec.
const (
	PrintDisplay = "display"
	PrintDebug   = "debug"
	PrintRaw     = "raw"
	PrintName    = "name"
)

var formatSpec = regexp.MustCompile(`^\{:[^{}"\\]*\}$`)

// CheckPrint returns an error for a print= value of a Rust marker that is not
// "", display, debug, raw, name or one format spec such as {:x}.
func CheckPrint(p string) error {
	switch p {
	case "", PrintDisplay, PrintDebug, PrintRaw, PrintName:
		return nil
	}
	if formatSpec.MatchString(p) {
		return nil
	}
	return fmt.Errorf("print=%s is not display, debug, raw, name or a Rust format spec such as {:x}", p)
}

// A Candidate is a const that print=name compares with the value: its path
// from the test module, and its name in the output.
type Candidate struct {
	Expr string
	Name string
}

// A Table selects the reference table of a configulator-rs config type.
type Table struct {
	// Crate is the path of the configulator crate, such as ::configulator.
	Crate     string
	EnvPrefix string
	EnvSep    string
	FlagSep   string
}

// A Request is one value that the test on a copy of the crate prints. Text and
// Err hold the result.
type Request struct {
	// Module is the module of the item. The test goes in a module inside it.
	Module *Module
	// Name is the path of the item in errors, such as limits::MAX_JOBS.
	Name string
	// Expr is the item as the test names it, such as super::MAX_JOBS.
	Expr  string
	Print string
	// Names holds the consts that print=name compares with the value.
	Names []Candidate
	// Table, when it is not nil, selects the configulator table of the type
	// Expr.
	Table *Table
	// Cfg holds the cfg predicates of the item and its modules.
	Cfg   []string
	Text  string
	Err   error
	done  bool
	file  string
	first int
	last  int
}

// A Target is a target of a package with the requests of its values.
type Target struct {
	Package string
	// Bin is the name of a binary target, and "" for the library.
	Bin      string
	Requests []*Request
}

// code returns the lines of the test that print the value of r with the index i.
func (r *Request) code(i int) []string {
	emit := func(expr string) []string {
		return []string{fmt.Sprintf("__docgen_emit(%d, &%s);", i, expr)}
	}
	switch {
	case r.Table != nil:
		return emit(fmt.Sprintf("%s::__schema::markdown(&<%s as %s::HasShadow>::fields(), %s, %s, %s)",
			r.Table.Crate, r.Expr, r.Table.Crate, rustString(r.Table.FlagSep), rustString(r.Table.EnvPrefix), rustString(r.Table.EnvSep)))
	case r.Print == PrintName:
		lines := []string{"{", fmt.Sprintf("let __docgen_v = &%s;", r.Expr), "let mut __docgen_n: std::vec::Vec<&str> = std::vec::Vec::new();"}
		for _, c := range r.Names {
			lines = append(lines, fmt.Sprintf("if *__docgen_v == %s { __docgen_n.push(%s); }", c.Expr, rustString(c.Name)))
		}
		return append(lines, fmt.Sprintf("__docgen_emit(%d, &__docgen_n.join(\"\\n\"));", i), "}")
	case r.Print == PrintDisplay:
		return emit(fmt.Sprintf("std::format!(\"{}\", %s)", r.Expr))
	case r.Print == PrintDebug:
		return emit(fmt.Sprintf("std::format!(\"{:?}\", %s)", r.Expr))
	case r.Print != "":
		return emit(fmt.Sprintf("std::format!(%s, %s)", rustString(r.Print), r.Expr))
	}
	return emit(fmt.Sprintf("(&__DocgenW(&%s)).__docgen_text()", r.Expr))
}

// rustString returns s as a Rust string literal.
func rustString(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, c := range s {
		switch {
		case c == '"' || c == '\\':
			b.WriteByte('\\')
			b.WriteRune(c)
		case c < ' ' || c == 0x7f:
			fmt.Fprintf(&b, "\\u{%x}", c)
		default:
			b.WriteRune(c)
		}
	}
	b.WriteByte('"')
	return b.String()
}

const testHead = `
#[cfg(test)]
#[allow(warnings, unused, clippy::all)]
mod __docgen_values {
    extern crate std;
    struct __DocgenW<'a, T: ?Sized>(&'a T);
    trait __DocgenDisplay {
        fn __docgen_text(&self) -> std::string::String;
    }
    impl<T: ?Sized + std::fmt::Display> __DocgenDisplay for __DocgenW<'_, T> {
        fn __docgen_text(&self) -> std::string::String {
            std::format!("{}", self.0)
        }
    }
    trait __DocgenDebug {
        fn __docgen_text(&self) -> std::string::String;
    }
    impl<T: ?Sized + std::fmt::Debug> __DocgenDebug for &__DocgenW<'_, T> {
        fn __docgen_text(&self) -> std::string::String {
            std::format!("{:?}", self.0)
        }
    }
    fn __docgen_emit(i: usize, s: &str) {
        let mut h = std::string::String::new();
        for b in s.bytes() {
            h.push_str(&std::format!("{:02x}", b));
        }
        std::println!("docgen-value {} {}", i, h);
    }
    #[test]
    fn __docgen_values() {
`

const testTail = `    }
}
`

// testName is the name of each test function and module, and the filter of
// cargo test.
const testName = "__docgen_values"

// An insertion is the test of one module: its place in the file and its
// requests, in order.
type insertion struct {
	offset int
	reqs   []*Request
	index  []int
}

// insertTests returns text with the tests of ins, and sets the lines of each
// request in the result.
func insertTests(text string, ins []*insertion) string {
	slices.SortFunc(ins, func(a, b *insertion) int { return a.offset - b.offset })
	var b strings.Builder
	at := 0
	line := 1
	for _, in := range ins {
		off := min(max(in.offset, at), len(text))
		b.WriteString(text[at:off])
		line += strings.Count(text[at:off], "\n")
		at = off
		head := testHead
		if off > 0 && text[off-1] != '\n' {
			head = "\n" + head
		}
		b.WriteString(head)
		line += strings.Count(head, "\n")
		for k, r := range in.reqs {
			r.first = line
			for _, l := range r.code(in.index[k]) {
				b.WriteString("        " + l + "\n")
				line++
			}
			r.last = line - 1
		}
		b.WriteString(testTail)
		line += strings.Count(testTail, "\n")
	}
	b.WriteString(text[at:])
	return b.String()
}

// RunValues prints the values of the requests of targets with cargo test on a
// copy of the workspace at root, made in work: one run for each target, and one
// more without the requests whose code does not compile. Each request gets its
// Text or its Err. The error is of the copy.
func RunValues(ctx context.Context, r *cargo.Runner, root, work string, targets []*Target) error {
	if err := copyWorkspace(ctx, root, work); err != nil {
		return err
	}
	originals := map[string]string{}
	for _, t := range targets {
		for _, req := range t.Requests {
			rel, err := filepath.Rel(root, req.Module.File)
			if err != nil || !filepath.IsLocal(rel) {
				req.Err = fmt.Errorf("the file %s of %s is outside the Cargo workspace at %s", req.Module.File, req.Name, root)
				continue
			}
			req.file = filepath.Join(work, rel)
			if _, ok := originals[req.file]; ok {
				continue
			}
			b, err := os.ReadFile(req.file)
			if err != nil {
				return err
			}
			originals[req.file] = string(b)
		}
	}
	for _, t := range targets {
		for range 2 {
			if err := writeTests(t, originals); err != nil {
				return err
			}
			if !runTarget(ctx, r, root, work, t) {
				break
			}
		}
	}
	return nil
}

// pending returns the requests of t with no result.
func (t *Target) pending() []*Request {
	var out []*Request
	for _, req := range t.Requests {
		if req.Err == nil && !req.done {
			out = append(out, req)
		}
	}
	return out
}

// writeTests writes each file of the pending requests of t: its original text
// with the tests of those requests.
func writeTests(t *Target, originals map[string]string) error {
	byFile := map[string]map[*Module]*insertion{}
	for i, req := range t.Requests {
		if req.Err != nil || req.done {
			continue
		}
		if byFile[req.file] == nil {
			byFile[req.file] = map[*Module]*insertion{}
		}
		in := byFile[req.file][req.Module]
		if in == nil {
			in = &insertion{offset: req.Module.Insert}
			byFile[req.file][req.Module] = in
		}
		in.reqs = append(in.reqs, req)
		in.index = append(in.index, i)
	}
	for file, text := range originals {
		ins := make([]*insertion, 0, len(byFile[file]))
		for _, in := range byFile[file] {
			ins = append(ins, in)
		}
		if err := os.WriteFile(file, []byte(insertTests(text, ins)), 0o600); err != nil {
			return err
		}
	}
	return nil
}

var (
	valueLine    = regexp.MustCompile(`(?m)docgen-value (\d+) ([0-9a-f]*)$`)
	diagStart    = regexp.MustCompile(`^error(\[\w+\])?: `)
	diagPlace    = regexp.MustCompile(`-->\s+(.+?):(\d+):\d+`)
	panicPlace   = regexp.MustCompile(`panicked at (.+?):(\d+):\d+:`)
	progressLine = regexp.MustCompile(`^\s+(Compiling|Checking|Downloaded|Downloading|Finished|Running|Blocking|Updating|Locking|Fresh|Adding|Doc-tests)\b`)
)

// runTarget runs the test of the pending requests of t. It reports whether
// another run can print more values: the code of some requests did not
// compile, and other requests have no result.
func runTarget(ctx context.Context, r *cargo.Runner, root, work string, t *Target) bool {
	pending := t.pending()
	if len(pending) == 0 {
		return false
	}
	which := []string{"--lib"}
	what := "the library of crate " + t.Package
	if t.Bin != "" {
		which = []string{"--bin", t.Bin}
		what = "the binary " + t.Bin + " of crate " + t.Package
	}
	args := append([]string{"test", "--locked", "--manifest-path", filepath.Join(work, "Cargo.toml"), "-p", t.Package}, which...)
	args = append(args, "--", testName, "--nocapture", "--test-threads=1")
	out, errOut, err := r.Run(ctx, work, args...)
	for _, m := range valueLine.FindAllStringSubmatch(out, -1) {
		i, _ := strconv.Atoi(m[1])
		b, _ := hex.DecodeString(m[2])
		if i < len(t.Requests) && t.Requests[i].Err == nil {
			t.Requests[i].Text = string(b)
			t.Requests[i].done = true
		}
	}
	if err == nil {
		for _, req := range pending {
			if !req.done {
				req.Err = fmt.Errorf("the value test of %s printed no value for %s%s", what, req.Name, cfgNote(req.Cfg))
			}
		}
		return false
	}
	if errors.Is(err, cargo.ErrNotFound) || ctx.Err() != nil {
		for _, req := range pending {
			req.Err = err
		}
		return false
	}
	scrub := func(s string) string { return strings.ReplaceAll(s, work, root) }
	failed := 0
	for _, d := range diagnostics(errOut) {
		m := diagPlace.FindStringSubmatch(d)
		if m == nil {
			continue
		}
		line, _ := strconv.Atoi(m[2])
		for _, req := range ownersOf(pending, work, m[1], line) {
			if req.Err == nil {
				req.Err = fmt.Errorf("the value test of %s does not compile for %s:\n%s", what, req.Name, scrub(d))
				failed++
			}
		}
	}
	if failed > 0 && len(t.pending()) > 0 {
		return true
	}
	all := strings.TrimSpace(errOut + "\n" + out)
	for _, req := range pending {
		if req.Err != nil || req.done {
			continue
		}
		if panicPlace.MatchString(all) {
			req.Err = fmt.Errorf("the value test of %s panicked before it printed %s:\n%s", what, req.Name, scrub(panicText(all)))
			continue
		}
		req.Err = fmt.Errorf("the value test of %s for %s: %w", what, req.Name, cargo.LockedError("cargo test", err, scrub(withoutProgress(errOut))))
	}
	return false
}

// cfgNote returns the note of an item with the cfg predicates cfg, which the
// test does not build when they do not hold.
func cfgNote(cfg []string) string {
	if len(cfg) == 0 {
		return ""
	}
	return "; it is in #[cfg(" + strings.Join(cfg, ")], #[cfg(") + ")], which is false in the test build"
}

// withoutProgress returns the standard error of cargo with no progress lines,
// such as Compiling.
func withoutProgress(s string) string {
	lines := strings.Split(s, "\n")
	out := lines[:0]
	for _, l := range lines {
		if !progressLine.MatchString(l) {
			out = append(out, l)
		}
	}
	return strings.Join(out, "\n")
}

// diagnostics returns the error blocks of the standard error of cargo.
func diagnostics(s string) []string {
	lines := strings.Split(s, "\n")
	var out []string
	start := -1
	flush := func(end int) {
		if start >= 0 {
			out = append(out, strings.TrimRight(strings.Join(lines[start:end], "\n"), "\n"))
		}
		start = -1
	}
	for i, l := range lines {
		switch {
		case diagStart.MatchString(l):
			flush(i)
			start = i
		case start >= 0 && (strings.HasPrefix(l, "warning") || strings.HasPrefix(l, "error: could not compile") ||
			strings.HasPrefix(l, "Some errors have") || strings.HasPrefix(l, "For more information")):
			flush(i)
		}
	}
	flush(len(lines))
	return out
}

// ownersOf returns the requests whose code holds line of file, a path of the
// diagnostic, relative to work or absolute.
func ownersOf(reqs []*Request, work, file string, line int) []*Request {
	if !filepath.IsAbs(file) {
		file = filepath.Join(work, file)
	}
	var out []*Request
	for _, r := range reqs {
		if r.file == file && line >= r.first && line <= r.last {
			out = append(out, r)
		}
	}
	return out
}

// panicText returns the lines of the panic message in the output of a test.
func panicText(s string) string {
	i := strings.Index(s, "panicked at")
	if i < 0 {
		return s
	}
	if j := strings.LastIndex(s[:i], "\n"); j >= 0 {
		s = s[j+1:]
	} else {
		s = s[i:]
	}
	lines := strings.Split(s, "\n")
	end := len(lines)
	for k, l := range lines {
		if k > 0 && (strings.HasPrefix(l, "note:") || l == "" || strings.HasPrefix(l, "test ")) {
			end = k
			break
		}
	}
	return strings.Join(lines[:end], "\n")
}

// copyWorkspace copies the workspace at root to work, with no .git and no
// cargo target directory: a directory with a CACHEDIR.TAG file, or target at
// root.
func copyWorkspace(ctx context.Context, root, work string) error {
	dst, err := os.OpenRoot(work)
	if err != nil {
		return err
	}
	defer dst.Close()
	return filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		rel, err := filepath.Rel(root, p)
		if err != nil || rel == "." {
			return err
		}
		return copyEntry(dst, p, rel, d)
	})
}

// copyEntry copies the entry d at p to rel in dst. A directory that holds a
// build is skipped.
func copyEntry(dst *os.Root, p, rel string, d fs.DirEntry) error {
	switch {
	case d.IsDir():
		if d.Name() == ".git" || rel == "target" || isFile(filepath.Join(p, "CACHEDIR.TAG")) {
			return filepath.SkipDir
		}
		return dst.Mkdir(rel, 0o750)
	case d.Type()&fs.ModeSymlink != 0:
		target, err := os.Readlink(p)
		if err != nil {
			return err
		}
		return dst.Symlink(target, rel)
	case d.Type().IsRegular():
		return copyFile(dst, p, rel)
	}
	return nil
}

func isFile(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.Mode().IsRegular()
}

func copyFile(dst *os.Root, from, rel string) error {
	in, err := os.Open(from)
	if err != nil {
		return err
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil {
		return err
	}
	out, err := dst.OpenFile(rel, os.O_WRONLY|os.O_CREATE|os.O_EXCL, info.Mode().Perm()|0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}
