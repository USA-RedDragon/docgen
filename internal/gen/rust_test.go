package gen

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const rustFixture = "../cli/testdata/rust"

func needCargo(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("cargo"); err != nil {
		t.Skip("cargo is not on PATH; install a Rust toolchain to run this test")
	}
}

// rustRun copies the Rust fixture with no pages of its own, writes the page,
// and runs update.
func rustRun(t *testing.T, page string) (string, error) {
	t.Helper()
	root, err := rustRunRoot(t, page)
	return read(t, root, "docs/p.md"), err
}

// rustRunRoot is rustRun, and returns the root of the copy.
func rustRunRoot(t *testing.T, page string) (string, error) {
	t.Helper()
	needCargo(t)
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.CopyFS(root, os.DirFS(rustFixture)); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(root, "docs")); err != nil {
		t.Fatal(err)
	}
	writeTree(t, root, map[string]string{"docs/p.md": page})
	g := generator(t, root)
	_, err = g.Run(t.Context(), filepath.Join(root, "docs"), true)
	return root, err
}

func TestRustExample(t *testing.T) {
	t.Parallel()
	page := "<!-- docgen:output crate=run-c example=greet -->\n<!-- /docgen:output -->\n\n" +
		"<!-- docgen:output crate=run-c example=greet args=\"Ada Lovelace\" -->\n<!-- /docgen:output -->\n\n" +
		"<!-- docgen:output crate=run-c bin=run-c -->\n<!-- /docgen:output -->\n"
	got, err := rustRun(t, page)
	if err != nil {
		t.Fatal(err)
	}
	want := "<!-- docgen:output crate=run-c example=greet -->\n```text\nHello, world!\n```\n<!-- /docgen:output -->\n\n" +
		"<!-- docgen:output crate=run-c example=greet args=\"Ada Lovelace\" -->\n```text\nHello, Ada!\nHello, Lovelace!\n```\n<!-- /docgen:output -->\n\n" +
		"<!-- docgen:output crate=run-c bin=run-c -->\n```text\nHello, bin!\n```\n<!-- /docgen:output -->\n"
	if got != want {
		t.Errorf("page:\n%s\nwant:\n%s", got, want)
	}
}

func TestRustExampleEmptyOutput(t *testing.T) {
	t.Parallel()
	got, err := rustRun(t, "<!-- docgen:output crate=run-c example=silent -->\n<!-- /docgen:output -->\n")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "```text\n\n```") {
		t.Errorf("page: %q", got)
	}
}

func TestRustDoctest(t *testing.T) {
	t.Parallel()
	page := "<!-- docgen:output crate=run-c doctest=double -->\n<!-- /docgen:output -->\n\n" +
		"<!-- docgen:output crate=run-c doctest=run_c::shapes::area -->\n<!-- /docgen:output -->\n\n" +
		"<!-- docgen:output crate=run-c doctest=crate -->\n<!-- /docgen:output -->\n\n" +
		"<!-- docgen:output crate=run-c doctest=Counter::next -->\n<!-- /docgen:output -->\n"
	got, err := rustRun(t, page)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"```rust\nlet n = run_c::double(base);\nassert_eq!(n, 42);\n```",
		"```rust\nuse run_c::shapes::area;\n\nassert_eq!(area(3, 4), 12);\n```\n\n```rust\nassert_eq!(run_c::shapes::area(5, 5), 25);\n```",
		"```rust\nassert_eq!(run_c::double(2), 4);\n```",
		"```rust\nlet mut c = run_c::Counter::new();\nassert_eq!(c.next(), 1);\n```",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("page lacks %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "let base") {
		t.Errorf("hidden line shown:\n%s", got)
	}
}

func TestRustErrors(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ tag, want string }{
		{"crate=run-c example=nope", "crate run-c has no example nope; its examples are: fails, greet, silent"},
		{"crate=run-c bin=nope", "crate run-c has no binary nope; its binaries are: run-c"},
		{"crate=limits bin=x", "crate limits has no binary"},
		{"crate=nope example=a", "nope"},
		{"crate=run-c example=fails", "the example failed"},
		{"crate=run-c-broken doctest=inc", "assertion `left == right` failed"},
		{"crate=run-c doctest=plain", `crate run-c has no doctest for "plain"; the items with a doctest are: Counter::next, crate, double, shapes::area, unchecked`},
		{"crate=run-c doctest=unchecked", "is ignored"},
		{"crate=tool doctest=x", "crate tool has no library"},
	} {
		_, err := rustRun(t, "<!-- docgen:output "+tc.tag+" -->\n<!-- /docgen:output -->\n")
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: error %v, want %q", tc.tag, err, tc.want)
		}
	}
}

func TestDoctestCode(t *testing.T) {
	t.Parallel()
	lines := strings.Split(`/// Text.
///
///   ~~~
///   let x = 1;
///   # hidden();
///   #
///   ## attr
///   x
///   ~~~
fn f() {}`, "\n")
	got, err := doctestCode(lines, 3)
	if err != nil {
		t.Fatal(err)
	}
	if want := "let x = 1;\n# attr\nx"; got != want {
		t.Errorf("code %q, want %q", got, want)
	}
	for _, line := range []int{1, 0, 11, 10} {
		if _, err := doctestCode(lines, line); err == nil {
			t.Errorf("line %d: no error", line)
		}
	}
}

func TestParseDoctests(t *testing.T) {
	t.Parallel()
	out := "test crates/a/src/lib.rs - (line 3) ... ok\ntest crates/a/src/lib.rs - m::f (line 9) - compile ... ok\ncrates/a/src/lib.rs - g (line 1): test\ncrates/a/src/lib.rs - h (line 2) - compile fail ... ignored\nnoise\n"
	got := parseDoctests(out)
	want := []doctest{
		{"crates/a/src/lib.rs", "", 3, "ok"},
		{"crates/a/src/lib.rs", "m::f", 9, "ok"},
		{"crates/a/src/lib.rs", "g", 1, ""},
		{"crates/a/src/lib.rs", "h", 2, "ignored"},
	}
	if len(got) != len(want) {
		t.Fatalf("got %+v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("%d: %+v, want %+v", i, got[i], want[i])
		}
	}
}
