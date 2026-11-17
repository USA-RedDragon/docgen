package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// looseFile returns a file of the testdata directory loose, which is in no
// crate.
func looseFile(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "rust", "loose", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// codeMarker returns a docgen:code marker of loose/symbols.rs.
func codeMarker(args string) string {
	return "<!-- docgen:code file=docs/symbols.rs " + args + " -->\n<!-- /docgen:code -->\n"
}

func TestRustSymbols(t *testing.T) {
	t.Parallel()
	needCargo(t)
	for _, c := range []struct{ args, want string }{
		{"symbol=Port", "#[derive(Debug)]\npub struct Port<T> {\n    /// The number.\n    number: T,\n}"},
		{"symbol=Port raw=true", "/// A port.\n/// It holds a number.\n#[derive(Debug)]\n/// A second doc block.\npub struct Port<T> {\n    /// The number.\n    number: T,\n}"},
		{"symbol=\"impl Port\"", "impl<T> Port<T> {\n    /// Makes a port.\n    #[inline]\n    pub const fn new(number: T) -> Self {\n        Port { number }\n    }\n}"},
		{"symbol=Port::new", "#[inline]\npub const fn new(number: T) -> Self {\n    Port { number }\n}"},
		{"symbol=\"Port<T>::new\" raw=true", "/// Makes a port.\n#[inline]\npub const fn new(number: T) -> Self {\n    Port { number }\n}"},
		{"symbol=\"impl Display for Port\"", "impl std::fmt::Display for Port<u8> {\n    fn fmt(&self, f: &mut std::fmt::Formatter) -> std::fmt::Result {\n        write!(f, \"{}\", self.number)\n    }\n}"},
		{"symbol=\"impl std::fmt::Debug for Port<u16>\"", "impl std::fmt::Debug for Port<u16> {\n    fn fmt(&self, f: &mut std::fmt::Formatter) -> std::fmt::Result {\n        write!(f, \"{}\", self.number)\n    }\n}"},
		{"symbol=\"<Port as Display>::fmt\"", "fn fmt(&self, f: &mut std::fmt::Formatter) -> std::fmt::Result {\n    write!(f, \"{}\", self.number)\n}"},
		{"symbol=net", "pub mod net {\n\n    /// Connects.\n    pub fn connect() {}\n}"},
		{"symbol=net raw=true", "/// The nets.\npub mod net {\n    //! Inside the nets.\n\n    /// Connects.\n    pub fn connect() {}\n}"},
		{"symbol=net::connect", "pub fn connect() {}"},
		{"symbol=tiny", "fn tiny() {}"},
		{"symbol=twice", "macro_rules! twice {\n    ($x:expr) => {\n        $x * 2\n    };\n}"},
		{"symbol=Shape", "pub trait Shape {\n    /// The sides.\n    const SIDES: u8;\n\n    /// The area.\n    fn area(&self) -> u32;\n}"},
		{"symbol=Shape::area", "fn area(&self) -> u32;"},
		{"symbol=Shape::SIDES", "const SIDES: u8;"},
		{"symbol=Color", "pub enum Color {\n    Red,\n    Green,\n}"},
		{"symbol=Bits", "pub union Bits {\n    int: u32,\n    float: f32,\n}"},
		{"symbol=Id", "pub type Id = u64;"},
		{"symbol=LIMIT", "pub const LIMIT: u32 = 8;"},
		{"symbol=NAME", "pub static NAME: &str = \"loose\";"},
		{"symbol=Color::name", "pub fn name(&self) -> &str {\n    \"color\"\n}"},
		{"symbol=\"<Port<u16> as Shape>::SIDES\"", "const SIDES: u8 = 1;"},
		{"symbol=crate raw=true", "//! A file with no crate.\n//! It holds one of each item."},
	} {
		got := fill(t, map[string]string{"docs/symbols.rs": looseFile(t, "symbols.rs")}, codeMarker(c.args))
		want := "```rust\n" + c.want + "\n```\n"
		if !strings.Contains(got, "-->\n"+want+"<!-- /docgen:code -->") {
			t.Errorf("%s: page\n%s\nwant\n%s", c.args, got, want)
		}
	}
}

func TestRustSymbolErrors(t *testing.T) {
	t.Parallel()
	needCargo(t)
	root := writeRepo(t, map[string]string{
		"go.mod":          "module example.com/w\n\ngo 1.24.0\n",
		"docs/symbols.rs": looseFile(t, "symbols.rs"),
		"docs/broken.rs":  looseFile(t, "broken.rs"),
		"docs/nodoc.rs":   looseFile(t, "nodoc.rs"),
		"docs/p.md": codeMarker("symbol=dup") + codeMarker("symbol=nope") + codeMarker("symbol=crate") +
			codeMarker("symbol=\"Port::fmt\"") + codeMarker("symbol=\"impl Shape for Port\"") +
			codeMarker("symbol=\"Port::area\"") + codeMarker("symbol=\"impl Display for Nope\"") +
			"<!-- docgen:code file=docs/broken.rs symbol=f -->\n<!-- /docgen:code -->\n" +
			"<!-- docgen:code file=docs/nodoc.rs symbol=crate raw=true -->\n<!-- /docgen:code -->\n",
	})
	rc, log := docgen(t, root, "check")
	if rc != 1 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
	wantIn(t, log,
		"docs/p.md:1: docgen:code file=docs/symbols.rs symbol=dup: docs/symbols.rs: symbol=\"dup\" is more than one item in symbols.rs: fn dup at line 42 with cfg(unix), fn dup at line 45 with cfg(windows); they have the same name, so symbol= cannot tell them apart; use region= or lines=\n",
		"docs/p.md:3: docgen:code file=docs/symbols.rs symbol=nope: docs/symbols.rs: no item nope in symbols.rs; the symbols are <Port<u16> as Shape>::SIDES, ",
		", impl Port<T> and 8 more\n",
		"docs/p.md:5: docgen:code file=docs/symbols.rs symbol=crate: docs/symbols.rs: symbol=crate is the //! documentation of symbols.rs and needs raw=true\n",
		"docs/p.md:7: docgen:code file=docs/symbols.rs symbol=\"Port::fmt\": docs/symbols.rs: symbol=\"Port::fmt\" is more than one item in symbols.rs: fn <Port<u8> as std::fmt::Display>::fmt at line 23, fn <Port<u16> as std::fmt::Debug>::fmt at line 29; use one of the names above as symbol=\n",
		"docs/p.md:9: docgen:code file=docs/symbols.rs symbol=\"impl Shape for Port\": docs/symbols.rs: symbol=\"impl Shape for Port\" is more than one item in symbols.rs: impl Shape for Port<u8> at line 82, impl Shape for Port<u16> at line 90; use one of the names above as symbol=\n",
		"docs/p.md:11: docgen:code file=docs/symbols.rs symbol=\"Port::area\": docs/symbols.rs: symbol=\"Port::area\" is more than one item in symbols.rs: fn <Port<u8> as Shape>::area at line 85, fn <Port<u16> as Shape>::area at line 93; use one of the names above as symbol=\n",
		"docs/p.md:13: docgen:code file=docs/symbols.rs symbol=\"impl Display for Nope\": docs/symbols.rs: no item impl Display for Nope in symbols.rs; the symbols are ",
		"docs/p.md:15: docgen:code file=docs/broken.rs symbol=f: docs/broken.rs: docs/broken.rs:1:16: ",
		"docs/p.md:17: docgen:code file=docs/nodoc.rs symbol=crate raw=true: docs/nodoc.rs: nodoc.rs has no //! documentation\n",
	)
	wantNotIn(t, log, "uses lines=")
}

func TestRustSymbolSameNameItems(t *testing.T) {
	t.Parallel()
	needCargo(t)
	root := writeRepo(t, map[string]string{
		"go.mod":      "module example.com/w\n\ngo 1.24.0\n",
		"docs/two.rs": "fn f() {}\nfn f() {}\n",
		"docs/p.md":   "<!-- docgen:code file=docs/two.rs symbol=f -->\n<!-- /docgen:code -->\n",
	})
	rc, log := docgen(t, root, "check")
	wantIn(t, log, "fn f at line 1, fn f at line 2; they have the same name, so symbol= cannot tell them apart; use region= or lines=")
	if rc != 1 {
		t.Fatalf("rc %d", rc)
	}
}
