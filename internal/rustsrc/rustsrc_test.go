package rustsrc

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/USA-RedDragon/docgen/internal/cargo"
	"github.com/USA-RedDragon/docgen/internal/gocmd"
)

const fixture = "../cli/testdata/rust"

func needCargo(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("cargo"); err != nil {
		t.Skip("cargo is not on PATH; install a Rust toolchain to run this test")
	}
}

func newRunner(t *testing.T) *cargo.Runner {
	t.Helper()
	s := &gocmd.Session{}
	t.Cleanup(func() { _ = s.Close() })
	return &cargo.Runner{Session: s, Jobs: 4}
}

// readFixture builds the helper in a new cache directory and reads the root
// files, relative to the fixture.
func readFixture(t *testing.T, roots ...string) []*Crate {
	t.Helper()
	needCargo(t)
	r := newRunner(t)
	h := &Helper{CacheDir: t.TempDir()}
	bin, err := h.Path(t.Context(), r)
	if err != nil {
		t.Fatal(err)
	}
	abs, err := filepath.Abs(fixture)
	if err != nil {
		t.Fatal(err)
	}
	paths := make([]string, len(roots))
	for i, root := range roots {
		paths[i] = filepath.Join(abs, root)
	}
	crates, err := Read(t.Context(), r, bin, paths)
	if err != nil {
		t.Fatal(err)
	}
	return crates
}

func TestHelperCache(t *testing.T) {
	t.Parallel()
	needCargo(t)
	r := newRunner(t)
	h := &Helper{CacheDir: t.TempDir()}
	bin, err := h.Path(t.Context(), r)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(bin)
	if err != nil {
		t.Fatal(err)
	}
	again, err := h.Path(t.Context(), r)
	if err != nil {
		t.Fatal(err)
	}
	info2, err := os.Stat(again)
	if err != nil {
		t.Fatal(err)
	}
	if again != bin || !info2.ModTime().Equal(info.ModTime()) {
		t.Errorf("second Path built again: %s, %s", bin, again)
	}
	entries, err := os.ReadDir(h.CacheDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || !strings.HasPrefix(entries[0].Name(), helperName+"-") {
		t.Errorf("cache holds %v, want only the helper", entries)
	}
}

func TestCheckRelease(t *testing.T) {
	t.Parallel()
	for out, want := range map[string]string{
		"rustc 1.97.1\nrelease: 1.97.1\n": "",
		"release: 1.71.0\n":               "",
		"release: 2.0.0\n":                "",
		"release: 1.70.9\n":               "rustc 1.70.9 is too old for docgen, which needs Rust 1.71 or later",
		"no version":                      "printed no release line",
	} {
		err := checkRelease(out)
		if want == "" && err != nil || want != "" && (err == nil || !strings.Contains(err.Error(), want)) {
			t.Errorf("checkRelease(%q) = %v, want %q", out, err, want)
		}
	}
}

func TestHelperJSON(t *testing.T) {
	t.Parallel()
	crates := readFixture(t, "crates/core-a/src/lib.rs", "crates/core-a-bin/src/main.rs")
	c := crates[0]
	if len(c.Errors) != 0 || len(c.Warnings) != 0 {
		t.Fatalf("errors %v, warnings %v", c.Errors, c.Warnings)
	}
	paths := make([]string, 0, len(c.Modules))
	for _, m := range c.Modules {
		paths = append(paths, m.Path)
	}
	slices.Sort(paths)
	want := []string{"", "config", "errors", "limits", "net", "net::tcp", "never", "renamed", "renamed::deeper", "shapes", "shapes::inner", "unix"}
	if !slices.Equal(paths, want) {
		t.Fatalf("modules %q, want %q", paths, want)
	}
	for path, file := range map[string]string{
		"net":             "src/net/mod.rs",
		"net::tcp":        "src/net/tcp.rs",
		"renamed":         "src/plain/renamed.rs",
		"renamed::deeper": "src/plain/deeper.rs",
		"shapes::inner":   "src/lib.rs",
	} {
		if got := c.Module(path).File; !strings.HasSuffix(filepath.ToSlash(got), "crates/core-a/"+file) {
			t.Errorf("module %s is in %s, want %s", path, got, file)
		}
	}
	if m := c.Module("unix"); !slices.Equal(m.Cfg, []string{"unix"}) || m.Doc != "Only on Unix." {
		t.Errorf("unix: cfg %q, doc %q", m.Cfg, m.Doc)
	}
	if m := c.Module("shapes::inner"); m.Vis != "" || len(m.Cfg) != 0 {
		t.Errorf("shapes::inner: vis %q, cfg %q", m.Vis, m.Cfg)
	}
	if doc := c.Module("").Doc; !strings.HasPrefix(doc, "The core of the fixture app.\n\n# Limits\n") {
		t.Errorf("crate doc %q", doc)
	}
	checkItems(t, c)
	checkInsert(t, c)
	bin := crates[1]
	if g := bin.Module("").Item("GREETING", "const"); g == nil || g.Expr != `"hello"` || g.Vis != "" {
		t.Errorf("GREETING: %+v", g)
	}
}

func checkItems(t *testing.T, c *Crate) {
	t.Helper()
	limits := c.Module("limits")
	small := limits.Item("SMALL_JOBS", "const")
	if small == nil || small.Type != "usize" || small.Expr != "MAX_JOBS / 4" || small.Vis != "pub" || small.Doc != "The jobs of a small machine." {
		t.Errorf("SMALL_JOBS: %+v", small)
	}
	if modes := limits.Item("MODES", "static"); modes == nil || modes.Type != "[&str; 2]" || modes.Expr != `["fast", "slow"]` {
		t.Errorf("MODES: %+v", modes)
	}
	if http := limits.Item("Port::HTTP", "const"); http == nil || http.Type != "Port" || http.Expr != "Port(80)" {
		t.Errorf("Port::HTTP: %+v", http)
	}
	if r := limits.Item("RETRIES", ""); r == nil || r.Vis != "" {
		t.Errorf("RETRIES: %+v", r)
	}
	if port := limits.Item("Port", "struct"); port == nil || port.Style != "tuple" || !slices.Contains(port.Derives, "PartialEq") {
		t.Errorf("Port: %+v", port)
	}
	checkTypes(t, c)
}

// checkTypes checks the error enum and the config struct of the fixture.
func checkTypes(t *testing.T, c *Crate) {
	t.Helper()
	errEnum := c.Module("errors").Item("Error", "enum")
	if errEnum == nil || !slices.Contains(errEnum.Derives, "thiserror::Error") || len(errEnum.Variants) != 4 {
		t.Fatalf("errors::Error: %+v", errEnum)
	}
	read := errEnum.Variants[0]
	if read.Name != "Read" || read.Error == nil || read.Error.Format == nil || *read.Error.Format != "read {path}: {source}" || read.Fields[0].Doc != "The path of the file." {
		t.Errorf("Read: %+v", read)
	}
	if n := errEnum.Variants[1]; !n.Fields[1].Source || n.Fields[1].Type != "ParseIntError" {
		t.Errorf("Number: %+v", n)
	}
	if l := errEnum.Variants[2]; !l.Error.Transparent || !l.Fields[0].From || l.Style != "tuple" {
		t.Errorf("Limit: %+v", l)
	}
	app := c.Module("config").Item("AppConfig", "struct")
	if app == nil || !slices.Contains(app.Derives, "Config") || len(app.Fields) != 4 {
		t.Fatalf("AppConfig: %+v", app)
	}
	if got := app.Fields[1].Configulator; len(got) != 1 || !strings.Contains(got[0], "short = 'j'") {
		t.Errorf("jobs configulator %q", got)
	}
}

// checkInsert checks the place of the test of a module in a file of its own
// and of an inline module.
func checkInsert(t *testing.T, c *Crate) {
	t.Helper()
	m := c.Module("limits")
	text, err := os.ReadFile(m.File)
	if err != nil {
		t.Fatal(err)
	}
	if m.Insert != len(text) {
		t.Errorf("insert %d, want the end %d", m.Insert, len(text))
	}
	inner := c.Module("shapes::inner")
	lib, err := os.ReadFile(inner.File)
	if err != nil {
		t.Fatal(err)
	}
	if lib[inner.Insert] != '}' || !strings.Contains(string(lib[:inner.Insert]), "const RATIO") {
		t.Errorf("shapes::inner insert %d is not at its closing brace", inner.Insert)
	}
}

const modulesJSON = `[{"root":"/w/src/lib.rs","errors":[],"warnings":[],"modules":[
{"path":"","file":"/w/src/lib.rs","vis":"pub","doc":"Root.","insert":10,
 "items":[{"kind":"mod","name":"a"},{"kind":"mod","name":"b"},{"kind":"const","name":"TOP","type":"u8","vis":"pub"}],
 "uses":[{"name":"Inner","path":["a","Inner"],"vis":"pub"},{"name":"*","path":["b"],"vis":"pub","glob":true}]},
{"path":"a","file":"/w/src/a.rs","vis":"pub","doc":"A.","insert":5,
 "items":[{"kind":"struct","name":"Inner"},{"kind":"const","name":"Inner::C","type":"u8"},{"kind":"const","name":"X","type":"u8"}],
 "uses":[{"name":"Up","path":["super","TOP"],"vis":""}]},
{"path":"b","file":"/w/src/b.rs","vis":"","insert":0,"items":[{"kind":"fn","name":"run","vis":"pub"}]}]}]`

func TestLookup(t *testing.T) {
	t.Parallel()
	crates, err := ParseCrates([]byte(modulesJSON))
	if err != nil {
		t.Fatal(err)
	}
	c := crates[0]
	for name, want := range map[string]string{
		"":              "the crate root",
		"crate":         "the crate root",
		"a":             "module a",
		"TOP":           "const TOP",
		"crate::TOP":    "const TOP",
		"a::X":          "const a::X",
		"a::Inner":      "struct a::Inner",
		"a::Inner::C":   "const a::Inner::C",
		"Inner":         "struct a::Inner",
		"Inner::C":      "const a::Inner::C",
		"run":           "fn b::run",
		"a::Up":         "const TOP",
		"crate::b::run": "fn b::run",
	} {
		f, err := c.Lookup("w", name)
		if err != nil {
			t.Errorf("Lookup(%q): %v", name, err)
			continue
		}
		if got := f.Describe(); got != want {
			t.Errorf("Lookup(%q) = %s, want %s", name, got, want)
		}
	}
	for name, want := range map[string]string{
		"a::Nope": "crate w has no item a::Nope; the names in module a are Inner, Inner::C, X",
		"Nope":    "crate w has no item Nope; the names in the crate root are TOP, a, b",
		"z::Y":    "crate w has no item z::Y; the names in the crate root are TOP, a, b",
		"b::nope": "crate w has no item b::nope; the names in module b are run",
	} {
		if _, err := c.Lookup("w", name); err == nil || err.Error() != want {
			t.Errorf("Lookup(%q) error %v, want %q", name, err, want)
		}
	}
	if p := c.Module("a").Parent(); p == nil || p.Path != "" {
		t.Errorf("parent of a: %+v", p)
	}
}

func TestMarkdown(t *testing.T) {
	t.Parallel()
	doc := "Intro with [`Config`] and [the loader](crate::config::load) and [`a::B`][].\n\n" +
		"# Usage\n\n" +
		"```\n# use x::y;\nlet a = 1;\n## not hidden\n```\n\n" +
		"```no_run,edition2021\nrun();\n```\n\n" +
		"```text\n# kept\n```\n\n" +
		"## Details\n\nSee [docs] and `[not a link]` and [Rust](https://rust-lang.org).\n\n" +
		"[docs]: https://example.com/docs\n[`Config`]: crate::Config\n\n" +
		"# Other\n\nNot copied."
	got, err := Markdown(doc, "", "x", nil)
	if err != nil {
		t.Fatal(err)
	}
	want := "Intro with `Config` and the loader and `a::B`.\n\n" +
		"### Usage\n\n" +
		"```rust\nlet a = 1;\n# not hidden\n```\n\n" +
		"```rust\nrun();\n```\n\n" +
		"```text\n# kept\n```\n\n" +
		"#### Details\n\nSee [docs] and `[not a link]` and [Rust](https://rust-lang.org).\n\n" +
		"[docs]: https://example.com/docs\n\n" +
		"### Other\n\nNot copied."
	if got != want {
		t.Errorf("Markdown =\n%s\nwant\n%s", got, want)
	}
	sec, err := Markdown(doc, "Usage", "x", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(sec, "```rust\nlet a = 1;") || !strings.Contains(sec, "#### Details") || strings.Contains(sec, "Other") {
		t.Errorf("section Usage =\n%s", sec)
	}
	for section, want := range map[string]string{
		"Nope":  "no heading # Nope in the doc of x",
		"Empty": "heading # Empty in the doc of x has no text",
	} {
		if _, err := Markdown("# Empty\n\n# Next\n\ntext", section, "x", nil); err == nil || err.Error() != want {
			t.Errorf("section %s: %v, want %q", section, err, want)
		}
	}
	if _, err := Markdown("  \n", "", "x", nil); err == nil || err.Error() != "no doc comment for x" {
		t.Errorf("empty doc: %v", err)
	}
}

func TestHelperReturnTypes(t *testing.T) {
	t.Parallel()
	c := readFixture(t, "crates/err-store/src/lib.rs")[0]
	if len(c.Errors) != 0 || len(c.Warnings) != 0 {
		t.Fatalf("errors %v, warnings %v", c.Errors, c.Warnings)
	}
	store := c.Module("store")
	for name, want := range map[string]string{
		"count":           "Result<usize>",
		"Store::open":     "Result<Self>",
		"Store::get":      "std::result::Result<&T, Error>",
		"Store::remove":   "Result<T>",
		"Store::CAPACITY": "",
	} {
		it := store.Item(name, "")
		if it == nil || it.Ret != want {
			t.Errorf("%s: %+v, want Ret %q", name, it, want)
		}
	}
	f, err := c.Lookup("err-store", "Error")
	if err != nil {
		t.Fatal(err)
	}
	got := c.ReturnedBy(f)
	want := map[string][]string{
		"NotFound": {"store::Store::get", "store::Store::remove", "store::count"},
		"Locked":   {"store::Store::open", "store::Store::remove", "store::count"},
		"Corrupt":  {"error::Error::from_code", "repair"},
	}
	for v, fns := range want {
		slices.Sort(fns)
		if !slices.Equal(got[v], fns) {
			t.Errorf("ReturnedBy[%s] = %v, want %v", v, got[v], fns)
		}
	}
}
