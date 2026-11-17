package rustsrc

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckPrint(t *testing.T) {
	t.Parallel()
	for _, ok := range []string{"", "display", "debug", "raw", "name", "{:x}", "{:#06x}", "{:>8}", "{:.2}", "{:#?}"} {
		if err := CheckPrint(ok); err != nil {
			t.Errorf("CheckPrint(%q): %v", ok, err)
		}
	}
	for _, bad := range []string{"go", "config", "%d", "{}", "{:x} {:y}", "{:x}}", `{:"}`, "x{:x}"} {
		if err := CheckPrint(bad); err == nil {
			t.Errorf("CheckPrint(%q) = nil", bad)
		}
	}
}

func TestInsertTests(t *testing.T) {
	t.Parallel()
	text := "const A: u8 = 1;\nmod m {\n    const B: u8 = 2;\n}\nconst C: u8 = 3;"
	root := &Module{Insert: len(text)}
	inner := &Module{Path: "m", Insert: strings.Index(text, "}")}
	a := &Request{Module: root, Expr: "super::A"}
	b := &Request{Module: inner, Expr: "super::B", Print: PrintName, Names: []Candidate{{"crate::A", "A"}}}
	c := &Request{Module: root, Expr: "super::C", Print: "{:x}"}
	got := insertTests(text, []*insertion{
		{offset: root.Insert, reqs: []*Request{a, c}, index: []int{0, 2}},
		{offset: inner.Insert, reqs: []*Request{b}, index: []int{1}},
	})
	lines := strings.Split(got, "\n")
	line := func(r *Request) string { return strings.TrimSpace(lines[r.first-1]) }
	if line(a) != "__docgen_emit(0, &(&__DocgenW(&super::A)).__docgen_text());" || a.first != a.last {
		t.Errorf("a at %d-%d: %q", a.first, a.last, line(a))
	}
	if line(c) != `__docgen_emit(2, &std::format!("{:x}", super::C));` {
		t.Errorf("c: %q", line(c))
	}
	if line(b) != "{" || strings.TrimSpace(lines[b.last-1]) != "}" || !strings.Contains(strings.Join(lines[b.first-1:b.last], "\n"), `if *__docgen_v == crate::A { __docgen_n.push("A"); }`) {
		t.Errorf("b at %d-%d:\n%s", b.first, b.last, strings.Join(lines[b.first-1:b.last], "\n"))
	}
	if !strings.HasPrefix(got, "const A: u8 = 1;\nmod m {\n    const B: u8 = 2;\n\n#[cfg(test)]") {
		t.Errorf("the test of m is not inside it:\n%s", got)
	}
	if !strings.Contains(got, "const C: u8 = 3;\n\n#[cfg(test)]") {
		t.Errorf("the test of the root does not start on a new line:\n%s", got)
	}
	if want := `"a\"b\\c\u{a}"`; rustString("a\"b\\c\n") != want {
		t.Errorf("rustString = %s, want %s", rustString("a\"b\\c\n"), want)
	}
}

func TestRunValues(t *testing.T) {
	t.Parallel()
	crates := readFixture(t, "crates/core-a-bin/src/main.rs")
	root, err := filepath.Abs(fixture)
	if err != nil {
		t.Fatal(err)
	}
	m := crates[0].Module("")
	good := &Request{Module: m, Name: "GREETING", Expr: "super::GREETING"}
	debug := &Request{Module: m, Name: "GREETING", Expr: "super::GREETING", Print: PrintDebug}
	bad := &Request{Module: m, Name: "GREETING", Expr: "super::GREETING", Print: "{:x}"}
	target := &Target{Package: "core-a-bin", Bin: "core-a-bin", Requests: []*Request{good, bad, debug}}
	r := newRunner(t)
	work, err := r.Session.WorkDir("values-")
	if err != nil {
		t.Fatal(err)
	}
	if err := RunValues(t.Context(), r, root, work, []*Target{target}); err != nil {
		t.Fatal(err)
	}
	if good.Err != nil || good.Text != "hello" {
		t.Errorf("default: %q, %v", good.Text, good.Err)
	}
	if debug.Err != nil || debug.Text != `"hello"` {
		t.Errorf("debug: %q, %v", debug.Text, debug.Err)
	}
	if bad.Err == nil || !strings.Contains(bad.Err.Error(), "does not compile for GREETING") || !strings.Contains(bad.Err.Error(), "LowerHex") {
		t.Errorf("{:x}: %q, %v", bad.Text, bad.Err)
	}
	src, err := os.ReadFile(filepath.Join(root, "crates/core-a-bin/src/main.rs"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(src), testName) {
		t.Error("the test went into the fixture")
	}
}
