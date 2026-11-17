package rustsrc

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/USA-RedDragon/docgen/internal/cargo"
)

// A StatesTarget is a crate with the machines that one cargo test records.
type StatesTarget struct {
	Package string
	Crate   *Crate
	// Flags are cargo flags for the test, such as --features.
	Flags []string
	// Filters are the test filters of cargo test; none runs all tests.
	Filters  []string
	Machines []*Machine
}

// statesOut is the name of the file in the copy of the workspace that the
// hook writes.
const statesOut = "docgen-states.out"

// allowLints are the lints that the code of the hook can trigger. A lint group
// such as warnings does not allow a lint that the crate denies by name.
const allowLints = "dead_code, unused, unreachable_code, unused_qualifications, unused_results, trivial_casts, unreachable_pub, missing_docs, missing_debug_implementations, missing_copy_implementations, elided_lifetimes_in_paths, single_use_lifetimes"

// supportCode is the module that each hook calls, at the end of the crate
// root. The verbs are the lints to allow and the path of the file that it
// writes, as a string literal. It names each item of std by its path and
// imports nothing, so that it builds in a crate of any edition, with or
// without no_std.
const supportCode = `
#[doc(hidden)]
#[allow(warnings, %s)]
pub(crate) mod __docgen_states {
    extern crate std;
    std::thread_local! {
        static DEPTH: std::cell::RefCell<std::vec::Vec<(usize, u32)>> = std::cell::RefCell::new(std::vec::Vec::new());
    }
    static SEEN: std::sync::Mutex<std::vec::Vec<std::string::String>> = std::sync::Mutex::new(std::vec::Vec::new());
    fn write(line: std::string::String) {
        let mut seen = match SEEN.lock() {
            Ok(g) => g,
            Err(e) => e.into_inner(),
        };
        if seen.contains(&line) {
            return;
        }
        if let Ok(mut f) = std::fs::OpenOptions::new().create(true).append(true).open(%s) {
            if std::io::Write::write_all(&mut f, line.as_bytes()).is_ok() {
                seen.push(line);
            }
        }
    }
    pub(crate) fn key<T>(at: &T) -> usize {
        let p: *const T = at;
        p as usize
    }
    pub(crate) fn first(view: &str, state: std::option::Option<&str>) {
        if let Some(s) = state {
            write(std::format!("first\t{}\t{}\n", view, s));
        }
    }
    pub(crate) struct Call {
        key: usize,
        outer: bool,
        view: &'static str,
        call: std::string::String,
        from: &'static str,
    }
    impl Call {
        pub(crate) fn enter(key: usize, view: &'static str, name: &str, from: &'static str, args: &[&'static str]) -> Call {
            let outer = DEPTH.with(|d| {
                let mut d = d.borrow_mut();
                match d.iter_mut().find(|e| e.0 == key) {
                    Some(e) => {
                        e.1 += 1;
                        false
                    }
                    None => {
                        d.push((key, 1));
                        true
                    }
                }
            });
            let mut call = std::string::String::from(name);
            if !args.is_empty() {
                call.push('(');
                call.push_str(&args.join(", "));
                call.push(')');
            }
            Call { key, outer, view, call, from }
        }
        pub(crate) fn end(&mut self, to: std::option::Option<&'static str>) {
            if let (true, Some(to)) = (self.outer, to) {
                if to != self.from {
                    write(std::format!("edge\t{}\t{}\t{}\t{}\n", self.view, self.from, self.call, to));
                }
            }
        }
    }
    impl Drop for Call {
        fn drop(&mut self) {
            let key = self.key;
            let _ = DEPTH.try_with(|d| {
                let mut d = d.borrow_mut();
                if let Some(i) = d.iter().position(|e| e.0 == key) {
                    d[i].1 -= 1;
                    if d[i].1 == 0 {
                        d.swap_remove(i);
                    }
                }
            });
        }
    }
}
`

// An edit inserts text at a byte offset of a file.
type edit struct {
	offset int
	text   string
}

// variantFn returns the impl that names the variant of each value of the enum
// e, on one line.
func variantFn(e *Item) string {
	var b strings.Builder
	fmt.Fprintf(&b, " impl %s { #[doc(hidden)] #[allow(%s)] pub(crate) fn __docgen_variant(&self) -> &'static str { match *self {", e.Name, allowLints)
	for _, v := range e.Variants {
		for _, c := range v.Cfg {
			fmt.Fprintf(&b, " #[cfg(%s)]", c)
		}
		fmt.Fprintf(&b, " Self::%s { .. } => %s,", v.Name, rustString(v.Name))
	}
	b.WriteString(" } } }")
	return b.String()
}

// stateFn returns the impl that names the state of each value of m, on one
// line.
func stateFn(m *Machine) string {
	expr := "self.__docgen_variant()"
	switch m.From {
	case StateMethod:
		expr = "self." + m.Via + "().__docgen_variant()"
	case StateField:
		expr = "self." + m.Via + ".__docgen_variant()"
	}
	return fmt.Sprintf(" impl %s { #[doc(hidden)] #[allow(%s)] pub(crate) fn __docgen_state(&self) -> &'static str { %s } }", m.Type.Name, allowLints, expr)
}

// endExpr returns the state of the result of a call with the shape, as an
// Option.
func endExpr(shape int) string {
	switch shape {
	case shapeOption:
		return "__docgen_r.as_ref().map(Self::__docgen_state)"
	case shapeResult:
		return "__docgen_r.as_ref().ok().map(Self::__docgen_state)"
	}
	return "Some(Self::__docgen_state(&__docgen_r))"
}

// hookEdits returns the edits of the body of h, a method of m, on the lines of
// its braces.
func hookEdits(m *Machine, h *hook) []edit {
	mt := h.item.Method
	ret := h.item.Ret
	if ret == "" {
		ret = "()"
	}
	view := rustString(m.Path)
	args := make([]string, 0, len(h.labels))
	for _, l := range h.labels {
		args = append(args, l+".__docgen_variant()")
	}
	open := " #![allow(" + allowLints + ")]"
	var pre, post string
	switch mt.Receiver {
	case RecvNone:
		pre = open + " let __docgen_r = (|| -> " + ret + " { "
		post = " })(); crate::__docgen_states::first(" + view + ", " + endExpr(h.shape) + "); __docgen_r "
	case RecvMut:
		pre = fmt.Sprintf("%s let mut __docgen_c = crate::__docgen_states::Call::enter(crate::__docgen_states::key(self), %s, %s, Self::__docgen_state(self), &[%s]); let __docgen_r = (|| -> %s { ",
			open, view, rustString(h.name), strings.Join(args, ", "), ret)
		post = " })(); __docgen_c.end(Some(Self::__docgen_state(self))); __docgen_r "
	default:
		pre = fmt.Sprintf("%s let mut __docgen_c = crate::__docgen_states::Call::enter(0, %s, %s, Self::__docgen_state(&self), &[%s]); let __docgen_r = (|| -> %s { ",
			open, view, rustString(h.name), strings.Join(args, ", "), ret)
		post = " })(); __docgen_c.end(" + endExpr(h.shape) + "); __docgen_r "
	}
	return []edit{{offset: mt.BodyStart, text: pre}, {offset: mt.BodyEnd, text: post}}
}

// statesEdits returns the edits of the files of t, by path, for a hook that
// writes to out.
func statesEdits(t *StatesTarget, out string) map[string][]edit {
	edits := map[string][]edit{}
	add := func(file string, e ...edit) { edits[file] = append(edits[file], e...) }
	enums := map[*Item]bool{}
	states := map[*Item]bool{}
	enumModule := func(e *Item) *Module {
		for _, m := range t.Crate.Modules {
			if slices.Contains(m.Items, e) {
				return m
			}
		}
		return nil
	}
	for _, m := range t.Machines {
		if states[m.Type] {
			continue
		}
		states[m.Type] = true
		add(m.module.File, edit{offset: m.module.Insert, text: stateFn(m)})
		need := []*Item{m.Enum}
		for _, h := range m.hooks {
			add(h.item.Span.File, hookEdits(m, h)...)
			for _, l := range h.labels {
				for _, p := range h.item.Method.Params {
					if p.Name == l {
						e, _, _ := t.Crate.enumOf(h.module, p.Type)
						need = append(need, e)
					}
				}
			}
		}
		for _, e := range need {
			if e == nil || enums[e] {
				continue
			}
			enums[e] = true
			if em := enumModule(e); em != nil {
				add(em.File, edit{offset: em.Insert, text: variantFn(e)})
			}
		}
	}
	root := t.Crate.Module("")
	add(root.File, edit{offset: root.Insert, text: fmt.Sprintf(supportCode, allowLints, rustString(out))})
	return edits
}

// applyEdits returns text with the edits, in order of offset.
func applyEdits(text string, edits []edit) string {
	slices.SortStableFunc(edits, func(a, b edit) int { return a.offset - b.offset })
	var b strings.Builder
	at := 0
	end := false
	for _, e := range edits {
		off := min(max(e.offset, at), len(text))
		b.WriteString(text[at:off])
		if off == len(text) && !end {
			end = true
			if off > 0 && text[off-1] != '\n' {
				b.WriteString("\n")
			}
		}
		b.WriteString(e.text)
		at = off
	}
	b.WriteString(text[at:])
	return b.String()
}

// RunStates records the machines of t with cargo test on a copy of the
// workspace at root, made in work: the hook of each machine writes each change
// of state and each state that a constructor returned. A method whose hook
// does not build moves to Skipped, and the test runs again without it.
func RunStates(ctx context.Context, r *cargo.Runner, root, work string, t *StatesTarget) error {
	if err := copyWorkspace(ctx, root, work); err != nil {
		return err
	}
	out := filepath.Join(work, statesOut)
	originals := map[string]string{}
	for file := range statesEdits(t, out) {
		rel, err := filepath.Rel(root, file)
		if err != nil || !filepath.IsLocal(rel) {
			return fmt.Errorf("the file %s of crate %s is outside the Cargo workspace at %s", file, t.Package, root)
		}
		b, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		originals[rel] = string(b)
	}
	for {
		if err := os.Remove(out); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		for file, edits := range statesEdits(t, out) {
			rel, _ := filepath.Rel(root, file)
			if err := os.WriteFile(filepath.Join(work, rel), []byte(applyEdits(originals[rel], edits)), 0o600); err != nil {
				return err
			}
		}
		again, err := runStates(ctx, r, root, work, t)
		if err != nil {
			return err
		}
		if !again {
			return readStates(out, t.Machines)
		}
	}
}

// runStates runs the tests of t once. It reports whether to run them again,
// after a hook that did not build moved to Skipped.
func runStates(ctx context.Context, r *cargo.Runner, root, work string, t *StatesTarget) (bool, error) {
	args := append([]string{"test", "--locked", "--manifest-path", filepath.Join(work, "Cargo.toml"), "-p", t.Package}, t.Flags...)
	if len(t.Filters) > 0 {
		args = append(append(args, "--"), t.Filters...)
	}
	out, errOut, err := r.Run(ctx, work, args...)
	if err == nil {
		return false, nil
	}
	if errors.Is(err, cargo.ErrNotFound) || ctx.Err() != nil {
		return false, err
	}
	scrub := func(s string) string { return strings.ReplaceAll(s, work, root) }
	diags := diagnostics(errOut)
	if err := forbidError(t.Package, work, diags); err != nil {
		return false, errors.New(scrub(err.Error()))
	}
	var other []string
	var moved []*hook
	for _, d := range diags {
		place := diagPlace.FindStringSubmatch(d)
		if place == nil {
			continue
		}
		line, _ := strconv.Atoi(place[2])
		file := place[1]
		if !filepath.IsAbs(file) {
			file = filepath.Join(work, file)
		}
		within := func(h *hook) bool {
			rel, _ := filepath.Rel(root, h.item.Span.File)
			return filepath.Join(work, rel) == file && line >= h.item.Span.Line && line <= h.item.Span.EndLine
		}
		found := slices.ContainsFunc(moved, within)
		for _, m := range t.Machines {
			for _, h := range slices.Clone(m.hooks) {
				if within(h) {
					m.skip(h, SkipBuild)
					moved = append(moved, h)
					found = true
				}
			}
		}
		if !found {
			other = append(other, d)
		}
	}
	if len(moved) > 0 && len(other) == 0 {
		return true, nil
	}
	if len(other) > 0 {
		return false, fmt.Errorf("the tests of crate %s do not build with the state hook:\n%s", t.Package, scrub(strings.Join(diags, "\n")))
	}
	if strings.Contains(out, "test result: FAILED") || strings.Contains(out, "\nfailures:\n") {
		return false, fmt.Errorf("the tests of crate %s failed with the state hook:\n%s", t.Package, scrub(failures(out)))
	}
	return false, cargo.LockedError("cargo test of crate "+t.Package+" with the state hook", err, scrub(withoutProgress(errOut)))
}

// failures returns the failures section of the output of a test binary, or
// the whole output when it has none.
func failures(out string) string {
	i := strings.Index(out, "\nfailures:\n")
	if i < 0 {
		return strings.TrimSpace(out)
	}
	s := out[i+1:]
	if j := strings.Index(s, "\ntest result:"); j >= 0 {
		s = s[:j]
	}
	return strings.TrimSpace(s)
}

// readStates reads the file of the hook into the machines.
func readStates(path string, machines []*Machine) error {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close()
	byPath := map[string][]*Machine{}
	for _, m := range machines {
		byPath[m.Path] = append(byPath[m.Path], m)
	}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		parts := strings.Split(sc.Text(), "\t")
		switch {
		case len(parts) == 3 && parts[0] == "first":
			for _, m := range byPath[parts[1]] {
				m.First[parts[2]] = true
			}
		case len(parts) == 5 && parts[0] == "edge":
			for _, m := range byPath[parts[1]] {
				k := [2]string{parts[2], parts[4]}
				if m.Edges[k] == nil {
					m.Edges[k] = map[string]bool{}
				}
				m.Edges[k][parts[3]] = true
			}
		}
	}
	return sc.Err()
}
