package excerpt

import (
	"slices"
	"strings"
	"testing"
)

func TestCodeRange(t *testing.T) {
	t.Parallel()
	src := []byte("package p\n\nfunc f() {\n\tx := 1\n\t_ = x\n}\n")
	if err := checkGoRange("p.go", src, 3, 6, false); err != nil {
		t.Errorf("whole func: %v", err)
	}
	if err := checkGoRange("p.go", src, 4, 5, false); err != nil {
		t.Errorf("statements: %v", err)
	}
	if err := checkGoRange("p.go", src, 2, 6, false); err == nil {
		t.Error("a range from a blank line passed")
	}
	if err := checkGoRange("p.go", src, 3, 5, false); err == nil {
		t.Error("a range that ends inside the func passed")
	}
}

func TestYAMLSymbol(t *testing.T) {
	t.Parallel()
	src := []byte("on: push\njobs:\n  a:\n    steps:\n      - run: x\n\n  # b runs y\n  b:\n    steps:\n      - run: y\n")
	got, err := yamlExcerpt(src, "jobs.a")
	if err != nil {
		t.Fatal(err)
	}
	if want := "a:\n  steps:\n    - run: x"; got != want {
		t.Errorf("jobs.a = %q, want %q", got, want)
	}
	got, err = yamlExcerpt(src, "jobs.b.steps")
	if err != nil {
		t.Fatal(err)
	}
	if want := "steps:\n  - run: y"; got != want {
		t.Errorf("jobs.b.steps = %q, want %q", got, want)
	}
	if _, err := yamlExcerpt(src, "jobs.c"); err == nil {
		t.Error("a missing key passed")
	}
	if _, err := yamlExcerpt(src, "steps"); err == nil {
		t.Error("a key below the top level passed as a top-level key")
	}
}

func TestYAMLItems(t *testing.T) {
	t.Parallel()
	src := []byte("jobs:\n  a:\n    steps:\n      - name: One\n        run: x\n\n      - name: Two.b\n        run: |\n          y\n\n      - run: z\n\n      - name: Four\n        run: w\n  b:\n    steps: []\n")
	got, err := yamlExcerpt(src, "jobs.a.steps[Two.b:Four]")
	if err != nil {
		t.Fatal(err)
	}
	if want := "- name: Two.b\n  run: |\n    y\n\n- run: z\n\n- name: Four\n  run: w"; got != want {
		t.Errorf("Two.b:Four = %q, want %q", got, want)
	}
	got, err = yamlExcerpt(src, "jobs.a.steps[One]")
	if err != nil {
		t.Fatal(err)
	}
	if want := "- name: One\n  run: x"; got != want {
		t.Errorf("One = %q, want %q", got, want)
	}
	if _, err := yamlExcerpt(src, "jobs.a.steps[Four:One]"); err == nil {
		t.Error("a reversed item range passed")
	}
}

func TestCSymbol(t *testing.T) {
	t.Parallel()
	src := []byte("#define A_B 4\n#define A 1 \\\n  + 2\nint f(void);\nstatic int f(void) {\n\treturn A;\n}\nint xf(void) {\n}\n")
	for _, c := range []struct{ sym, want string }{
		{"A", "#define A 1 \\\n  + 2"},
		{"A_B", "#define A_B 4"},
		{"f", "static int f(void) {\n\treturn A;\n}"},
	} {
		got, err := cExcerpt(src, c.sym)
		if err != nil {
			t.Fatal(err)
		}
		if got != c.want {
			t.Errorf("%s = %q, want %q", c.sym, got, c.want)
		}
	}
	if _, err := cExcerpt(src, "g"); err == nil {
		t.Error("a missing symbol passed")
	}
}

func TestRegion(t *testing.T) {
	t.Parallel()
	src := []byte("/*\n * a\n * docgen:region r\n * b\n *   c\n * docgen:end\n * d\n */\n")
	got, err := Region("r.c", src, "r")
	if err != nil {
		t.Fatal(err)
	}
	if want := "* b\n*   c"; got != want {
		t.Errorf("region r = %q, want %q", got, want)
	}
	if _, err := Region("r.c", src, "x"); err == nil {
		t.Error("a missing region passed")
	}
	if _, err := Region("r.txt", []byte("docgen:region r\nx\n"), "r"); err == nil {
		t.Error("a region without an end passed")
	}
	if got, want := DropRegionLines(string(src)), "/*\n * a\n * b\n *   c\n * d\n */\n"; got != want {
		t.Errorf("DropRegionLines = %q, want %q", got, want)
	}
}

func TestYAMLSameIndentList(t *testing.T) {
	t.Parallel()
	src := []byte("steps:\n- name: one\n  run: x\n# about two\n- name: two\n  with:\n  - a\n\n  - b\nnext: 1\njobs:\n  a:\n    steps:\n    - run: y\n    # about z\n    - run: z\n    after: 1\n  # about b\n  b: 2\n")
	for _, c := range []struct{ sym, want string }{
		{"steps", "steps:\n- name: one\n  run: x\n# about two\n- name: two\n  with:\n  - a\n\n  - b"},
		{"steps[one]", "- name: one\n  run: x"},
		{"steps[two]", "- name: two\n  with:\n  - a\n\n  - b"},
		{"steps[one:two]", "- name: one\n  run: x\n# about two\n- name: two\n  with:\n  - a\n\n  - b"},
		{"next", "next: 1"},
		{"jobs.a.steps", "steps:\n- run: y\n# about z\n- run: z"},
		{"jobs.a", "a:\n  steps:\n  - run: y\n  # about z\n  - run: z\n  after: 1"},
	} {
		got, err := yamlExcerpt(src, c.sym)
		if err != nil {
			t.Errorf("%s: %v", c.sym, err)
			continue
		}
		if got != c.want {
			t.Errorf("%s = %q, want %q", c.sym, got, c.want)
		}
	}
	if _, err := yamlExcerpt(src, "steps.name"); err == nil {
		t.Error("a key of a list item passed as a key of the list")
	}
}

func TestYAMLCommentInBlock(t *testing.T) {
	t.Parallel()
	src := []byte("a:\n  b: 1\n# between\n  c: 2\n# after\nd: 3\n")
	got, err := yamlExcerpt(src, "a")
	if err != nil {
		t.Fatal(err)
	}
	if want := "a:\n  b: 1\n# between\n  c: 2"; got != want {
		t.Errorf("a = %q, want %q", got, want)
	}
}

func TestYAMLItemNames(t *testing.T) {
	t.Parallel()
	src := []byte("s:\n" +
		"  - name: a:b\n    run: 1\n" +
		"  - name: \"quoted\"\n    run: 2\n" +
		"  - name: 'it''s'\n    run: 3\n" +
		"  - name: plain # why\n    run: 4\n" +
		"  - name: \"has # hash\" # why\n    run: 5\n" +
		"  - run: 6\n    with:\n      name: nested\n" +
		"  - run: |\n      name: scalar\n" +
		"  - run: 8\n    name: second\n" +
		"  -\n    name: bare\n" +
		"  -   name: wide\n")
	for _, c := range []struct{ sym, want string }{
		{"s[a:b]", "- name: a:b\n  run: 1"},
		{"s[quoted]", "- name: \"quoted\"\n  run: 2"},
		{"s[it's]", "- name: 'it''s'\n  run: 3"},
		{"s[plain]", "- name: plain # why\n  run: 4"},
		{"s[has # hash]", "- name: \"has # hash\" # why\n  run: 5"},
		{"s[a:b:quoted]", "- name: a:b\n  run: 1\n- name: \"quoted\"\n  run: 2"},
		{"s[second]", "- run: 8\n  name: second"},
		{"s[bare]", "-\n  name: bare"},
		{"s[wide]", "-   name: wide"},
	} {
		got, err := yamlExcerpt(src, c.sym)
		if err != nil {
			t.Errorf("%s: %v", c.sym, err)
			continue
		}
		if got != c.want {
			t.Errorf("%s = %q, want %q", c.sym, got, c.want)
		}
	}
	for _, sym := range []string{"s[nested]", "s[scalar]", "s[:quoted]", "s[quoted:]"} {
		if got, err := yamlExcerpt(src, sym); err == nil {
			t.Errorf("%s = %q, want an error", sym, got)
		}
	}
	amb := []byte("s:\n  - name: a\n  - name: b:c\n  - name: a:b\n  - name: c\n")
	if got, err := yamlExcerpt(amb, "s[a:b:c]"); err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Errorf("s[a:b:c] = %q, %v, want an ambiguous error", got, err)
	}
}

func TestCDefinitions(t *testing.T) {
	t.Parallel()
	src := []byte(`#include <stdio.h>
#  define SPACED 1
# define TABBED(x) \
	(x)
  #define INDENTED 2
/*
#define COMMENTED 1
*/
#define COMMENTED 2

int proto(int a,
	  int b);

static int
gnu_style(int a)
{
	return a;
}

int proto(int a, int b) {
	return a + b;
}

int one_line(void) { return 0; }

/* gone(1) { */
// gone(2) {
const char *s = "gone(3) {";

int after(void)
{
	const char *t = "}";
	int b = 1;
#ifdef A
	if (b) {
#else
	if (!b) {
#endif
		b = 2;
	}
	return nested(b);
}

struct s {
	int (*member)(int a);
};

int a_array[] = { after(), };

__attribute__((format(printf, 1, 2)))
void
attr(const char *f, ...)
{
}

#ifdef __cplusplus
extern "C" {
#endif

static inline int in_extern(void) {
	return 1;
}

#ifdef __cplusplus
}
#endif

int trailing(void) {
	return 2;
} /* trailing */
`)
	for _, c := range []struct{ sym, want string }{
		{"SPACED", "#  define SPACED 1"},
		{"TABBED", "# define TABBED(x) \\\n\t(x)"},
		{"INDENTED", "  #define INDENTED 2"},
		{"COMMENTED", "#define COMMENTED 2"},
		{"proto", "int proto(int a, int b) {\n\treturn a + b;\n}"},
		{"gnu_style", "static int\ngnu_style(int a)\n{\n\treturn a;\n}"},
		{"one_line", "int one_line(void) { return 0; }"},
		{"attr", "__attribute__((format(printf, 1, 2)))\nvoid\nattr(const char *f, ...)\n{\n}"},
		{"in_extern", "static inline int in_extern(void) {\n\treturn 1;\n}"},
		{"trailing", "int trailing(void) {\n\treturn 2;\n} /* trailing */"},
	} {
		got, err := cExcerpt(src, c.sym)
		if err != nil {
			t.Errorf("%s: %v", c.sym, err)
			continue
		}
		if got != c.want {
			t.Errorf("%s = %q, want %q", c.sym, got, c.want)
		}
	}
	got, err := cExcerpt(src, "after")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(got, "int after(void)\n{\n") || !strings.HasSuffix(got, "return nested(b);\n}") {
		t.Errorf("after = %q", got)
	}
	for _, sym := range []string{"gone", "member", "nested", "SPACE", "only_proto"} {
		if got, err := cExcerpt(src, sym); err == nil {
			t.Errorf("%s = %q, want an error", sym, got)
		}
	}
	if got, err := cExcerpt([]byte("int f(void) {\n\treturn 1;\n"), "f"); err == nil {
		t.Errorf("an open body = %q, want an error", got)
	}
}

func TestGenericMethod(t *testing.T) {
	t.Parallel()
	src := []byte("package g\n\ntype Map[K comparable, V any] struct{ m map[K]V }\n\nfunc (m *Map[K, V]) Get(k K) V { return m.m[k] }\n\nfunc (m Map[K, V]) Len() int { return len(m.m) }\n\ntype Box[T any] struct{ v T }\n\nfunc (b Box[T]) Val() T { return b.v }\n")
	for _, c := range []struct{ sym, want string }{
		{"Map.Get", "func (m *Map[K, V]) Get(k K) V { return m.m[k] }"},
		{"Map.Len", "func (m Map[K, V]) Len() int { return len(m.m) }"},
		{"Box.Val", "func (b Box[T]) Val() T { return b.v }"},
	} {
		got, err := Symbol("g.go", src, c.sym)
		if err != nil {
			t.Errorf("%s: %v", c.sym, err)
			continue
		}
		if got != c.want {
			t.Errorf("%s = %q, want %q", c.sym, got, c.want)
		}
	}
}

func TestDropRegionLinesKeepsCode(t *testing.T) {
	t.Parallel()
	for _, l := range []string{
		"\ts := \"docgen:region foo\"",
		"\treturn \"docgen:end\"",
		"x := 1 // docgen:end",
		"// the docgen:end comment closes a region",
		"docgen:endless",
	} {
		if got := DropRegionLines("a\n" + l + "\nb"); got != "a\n"+l+"\nb" {
			t.Errorf("%q was dropped: %q", l, got)
		}
	}
	for _, l := range []string{
		"docgen:end",
		"// docgen:end",
		"\t// docgen:region foo",
		"# docgen:region foo",
		"/* docgen:region foo */",
		"/* docgen:end */",
		" * docgen:end",
		"<!-- docgen:region foo -->",
		"<!-- docgen:end -->",
		"-- docgen:end",
		"; docgen:end",
		"% docgen:end",
	} {
		if got := DropRegionLines("a\n" + l + "\nb"); got != "a\nb" {
			t.Errorf("%q was kept: %q", l, got)
		}
	}
}

func TestLinesLeaveOutRegionMarks(t *testing.T) {
	t.Parallel()
	src := []byte("package p\n\n// docgen:region r\nfunc f() {\n\t// docgen:end\n\ts := \"docgen:end\"\n\t_ = s\n}\n")
	const want = "```go\nfunc f() {\n\ts := \"docgen:end\"\n\t_ = s\n}\n```"
	got, err := Lines("p.go", "p.go", src, "4-8", "")
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("4-8 = %q, want %q", got, want)
	}
	if got, err := Lines("p.go", "p.go", src, "4-8", want); err != nil || got != want {
		t.Errorf("4-8 with the current body = %q, %v", got, err)
	}
	if _, err := Lines("p.go", "p.go", src, "4-7", ""); err == nil {
		t.Error("a range that ends inside the func passed")
	}
	moved := append([]byte("package p\n\nvar x = 1\n"), src[len("package p\n"):]...)
	_, err = Lines("p.go", "p.go", moved, "4-8", want)
	if err == nil || !strings.Contains(err.Error(), "lines=6-10") {
		t.Errorf("a moved excerpt: %v, want the new range lines=6-10", err)
	}
	old := "```go\nfunc f() {\n\t// docgen:end\n\ts := \"docgen:end\"\n\t_ = s\n}\n```"
	if got, err := Lines("p.go", "p.go", src, "4-8", old); err != nil || got != want {
		t.Errorf("a body that holds the mark = %q, %v, want the new body", got, err)
	}
	txt := []byte("# docgen:region r\n# docgen:end\n")
	if got, err := Lines("t.yaml", "t.yaml", txt, "1-2", ""); err == nil {
		t.Errorf("a range of marks only = %q, want an error", got)
	}
}

func TestLinesCRLFSource(t *testing.T) {
	t.Parallel()
	src := Source([]byte("one\r\ntwo\r\nthree\r\n"))
	got, err := Lines("f.txt", "f.txt", src, "1-2", "")
	if err != nil {
		t.Fatal(err)
	}
	if want := "```\none\ntwo\n```"; got != want {
		t.Fatalf("first run = %q, want %q", got, want)
	}
	if _, err := Lines("f.txt", "f.txt", src, "1-2", got); err != nil {
		t.Fatalf("second run: %v", err)
	}
}

func TestYAMLItemsLeaveOutTheCommentOfTheNextItem(t *testing.T) {
	t.Parallel()
	src := []byte("steps:\n  - name: A\n    run: a\n    # the end of A\n\n  # about B\n\n  # more about B\n  - name: B\n    run: |\n      # shell\n  # after B\nother: 1\n")
	for sym, want := range map[string]string{
		"steps[A]":   "- name: A\n  run: a\n  # the end of A",
		"steps[B]":   "- name: B\n  run: |\n    # shell",
		"steps[A:B]": "- name: A\n  run: a\n  # the end of A\n\n# about B\n\n# more about B\n- name: B\n  run: |\n    # shell",
	} {
		got, err := yamlExcerpt(src, sym)
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Errorf("%s = %q, want %q", sym, got, want)
		}
	}
}

func TestLinesErrorNamesTheCommand(t *testing.T) {
	t.Parallel()
	_, err := Lines("x.txt", "x.txt", []byte("one\ntwo\n"), "1-1", "```\nold\n```")
	if err == nil || !strings.Contains(err.Error(), "run docgen update") || strings.Contains(err.Error(), "-update") {
		t.Errorf("err = %v", err)
	}
}

func TestMarks(t *testing.T) {
	t.Parallel()
	text := "// docgen:region a\nx // docgen:end\n/* docgen:end */\n\n<!-- docgen:region b -->\n * docgen:region c*/\n"
	want := []Mark{{1, "a"}, {3, ""}, {5, "b"}, {6, "c"}}
	if got := Marks("f.txt", text); !slices.Equal(got, want) {
		t.Errorf("Marks = %v, want %v", got, want)
	}
	if got := Marks("f.md", "```\n# docgen:end\n```\n`docgen:end`\n# docgen:end\n"); !slices.Equal(got, []Mark{{5, ""}}) {
		t.Errorf("Marks of a page = %v", got)
	}
}

func TestRegionOfAPageLeavesCodeInTheExcerpt(t *testing.T) {
	t.Parallel()
	src := []byte("<!-- docgen:region r -->\n```sh\n# docgen:end\n```\n<!-- docgen:end -->\n")
	got, err := Region("p.md", src, "r")
	if err != nil {
		t.Fatal(err)
	}
	if want := "```sh\n# docgen:end\n```"; got != want {
		t.Errorf("Region = %q, want %q", got, want)
	}
}

const rawSource = "//go:build linux\n\n// Package p is a sample.\n//\n// It has two lines.\npackage p\n\n// A is one.\nconst A = 1\n\ntype (\n\t// B is two.\n\tB int\n\n\tC int\n)\n\nvar (\n\t// D is four.\n\tD = 4\n)\n\n// docgen:region r\n// F is a func.\nfunc F() {\n\t// docgen:end\n}\n\n// T is a type.\ntype T struct{}\n\n// M is a method.\nfunc (t T) M() {}\n\nfunc ExampleF() {\n\tF()\n\t// Output:\n\t// x\n}\n"

func TestRawSymbol(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ sym, want string }{
		{"package", "// Package p is a sample.\n//\n// It has two lines.\npackage p"},
		{"A", "// A is one.\nconst A = 1"},
		{"B", "// B is two.\nB int"},
		{"C", "C int"},
		{"D", "var (\n\t// D is four.\n\tD = 4\n)"},
		{"F", "// docgen:region r\n// F is a func.\nfunc F() {\n\t// docgen:end\n}"},
		{"T", "// T is a type.\ntype T struct{}"},
		{"T.M", "// M is a method.\nfunc (t T) M() {}"},
		{"ExampleF", "func ExampleF() {\n\tF()\n\t// Output:\n\t// x\n}"},
	} {
		got, err := RawSymbol("p.go", []byte(rawSource), c.sym)
		if err != nil {
			t.Errorf("%s: %v", c.sym, err)
		} else if got != c.want {
			t.Errorf("%s = %q, want %q", c.sym, got, c.want)
		}
	}
	if _, err := RawSymbol("p.go", []byte(rawSource), "Nope"); err == nil {
		t.Error("a symbol that is not in the file passed")
	}
	if got, err := RawSymbol("p.go", []byte("package p\n"), "package"); err != nil || got != "package p" {
		t.Errorf("a package with no doc comment = %q, %v", got, err)
	}
	if _, err := Symbol("p.go", []byte(rawSource), "package"); err == nil {
		t.Error("symbol=package with no raw=true passed")
	}
	if got, err := RawSymbol("p.yaml", []byte("package:\n  a: 1\n"), "package"); err != nil || got != "package:\n  a: 1" {
		t.Errorf("a YAML key = %q, %v", got, err)
	}
	if got, err := RawSymbol("p.c", []byte("#define A 1\n"), "A"); err != nil || got != "#define A 1" {
		t.Errorf("a C define = %q, %v", got, err)
	}
}

func TestRawLinesKeepRegionMarks(t *testing.T) {
	t.Parallel()
	src := []byte("package p\n\n// docgen:region r\nfunc f() {\n\t// docgen:end\n\t_ = 1\n}\n\n// docgen:region s\nvar x = 1\n// docgen:end\n")
	const want = "```go\nfunc f() {\n\t// docgen:end\n\t_ = 1\n}\n```"
	got, err := LinesRaw("p.go", "p.go", src, "4-7", "")
	if err != nil || got != want {
		t.Errorf("4-7 = %q, %v, want %q", got, err, want)
	}
	if got, err := LinesRaw("p.go", "p.go", src, "4-7", want); err != nil || got != want {
		t.Errorf("4-7 with the current body = %q, %v", got, err)
	}
	plain := "```go\nfunc f() {\n\t_ = 1\n}\n```"
	if got, err := LinesRaw("p.go", "p.go", src, "4-7", plain); err != nil || got != want {
		t.Errorf("a body with no marks = %q, %v, want the new body", got, err)
	}
	const marks = "```go\n// docgen:region s\nvar x = 1\n// docgen:end\n```"
	if got, err := LinesRaw("p.go", "p.go", src, "9-11", ""); err != nil || got != marks {
		t.Errorf("a range with a mark at each end = %q, %v, want %q", got, err, marks)
	}
	if _, err := Lines("p.go", "p.go", src, "9-11", ""); err == nil {
		t.Error("a range with no raw that starts at a mark passed")
	}
	if _, err := LinesRaw("p.go", "p.go", src, "4-6", ""); err == nil {
		t.Error("a range that ends inside the func passed")
	}
	moved := append([]byte("package p\n\nvar y = 1\n"), src[len("package p\n"):]...)
	_, err = LinesRaw("p.go", "p.go", moved, "4-7", want)
	if err == nil || !strings.Contains(err.Error(), "lines=6-9") {
		t.Errorf("a moved excerpt: %v, want the new range lines=6-9", err)
	}
	txt := []byte("# docgen:region r\n# docgen:end\n")
	if got, err := LinesRaw("t.yaml", "t.yaml", txt, "1-2", ""); err != nil || got != "```yaml\n# docgen:region r\n# docgen:end\n```" {
		t.Errorf("a range of marks only = %q, %v", got, err)
	}
}
