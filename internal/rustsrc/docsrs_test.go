package rustsrc

import (
	"slices"
	"testing"

	"github.com/USA-RedDragon/docgen/internal/cargo"
)

func item(kind, name string) *Item {
	return &Item{Kind: kind, Name: name, Vis: "pub"}
}

func mod(path, vis string, items []*Item, uses ...Use) *Module {
	return &Module{Path: path, Vis: vis, Items: items, Uses: uses}
}

func build(mods ...*Module) *Crate {
	c := &Crate{Modules: mods}
	c.index()
	return c
}

func testDocs(t *testing.T, c *Crate, registries []string) *Docs {
	t.Helper()
	pkg := &cargo.Package{
		Name: "my-crate", Version: "1.2.3", Registries: registries,
		Targets: []cargo.Target{{Name: "my_crate", Kind: cargo.KindLib}},
	}
	return NewDocs(pkg, c)
}

const testBase = "https://docs.rs/my-crate/latest/my_crate/"

func sample() *Crate {
	generic := item(KindStruct, "Config")
	generic.Generics = true
	pair := item(KindStruct, "Pair")
	pair.Fields = []Field{{Name: "left", Vis: "pub"}, {Name: "hidden"}, {Name: "gone", Vis: "pub", Cfg: []string{"unix"}}}
	choice := item(KindEnum, "Choice")
	choice.Variants = []Variant{{Name: "A"}, {Name: "B", Cfg: []string{"unix"}}}
	hiddenFn := item(KindFn, "hidden_fn")
	hiddenFn.Vis = ""
	crateFn := item(KindFn, "crate_fn")
	crateFn.Vis = "pub(crate)"
	cfgFn := item(KindFn, "cfg_fn")
	cfgFn.Cfg = []string{"feature=\"x\""}
	method := item(KindFn, "Config::new")
	assoc := item(KindConst, "Config::LIMIT")
	private := item(KindFn, "Config::secret")
	private.Vis = ""
	return build(
		mod("", "", []*Item{
			item(KindFn, "run"),
			item(KindConst, "MAX"),
			item(KindStatic, "COUNT"),
			item(KindTrait, "Handler"),
			item(KindType, "Alias"),
			item(KindUnion, "Bits"),
			item(KindMod, "api"),
			item(KindMod, "inner"),
			item(KindMod, "closed"),
			hiddenFn, crateFn, cfgFn,
		},
			Use{Name: "Hidden", Path: []string{"inner", "Hidden"}, Vis: "pub"},
			Use{Name: "Renamed", Path: []string{"inner", "Plain"}, Vis: "pub"},
			Use{Name: "Private", Path: []string{"inner", "Plain"}, Vis: ""},
			Use{Path: []string{"closed"}, Vis: "pub", Glob: true},
		),
		mod("api", "pub", []*Item{generic, method, assoc, private, pair, choice, item(KindMacro, "shout")},
			Use{Name: "Hidden", Path: []string{"crate", "inner", "Hidden"}, Vis: "pub"}),
		mod("inner", "", []*Item{item(KindStruct, "Hidden"), item(KindStruct, "Plain"), item(KindStruct, "Deep")}),
		mod("closed", "", []*Item{item(KindFn, "from_glob")}),
	)
}

func TestDocsURLs(t *testing.T) {
	t.Parallel()
	d := testDocs(t, sample(), nil)
	for link, want := range map[string]string{
		"run":                     "fn.run.html",
		"crate::run":              "fn.run.html",
		"MAX":                     "constant.MAX.html",
		"COUNT":                   "static.COUNT.html",
		"Handler":                 "trait.Handler.html",
		"Alias":                   "type.Alias.html",
		"Bits":                    "union.Bits.html",
		"api":                     "api/index.html",
		"api::Config":             "api/struct.Config.html",
		"struct@api::Config":      "api/struct.Config.html",
		"api::Config::new":        "api/struct.Config.html#method.new",
		"api::Config::new()":      "api/struct.Config.html#method.new",
		"api::Config::LIMIT":      "api/struct.Config.html#associatedconstant.LIMIT",
		"api::Choice":             "api/enum.Choice.html",
		"api::Choice::A":          "api/enum.Choice.html#variant.A",
		"api::Pair::left":         "api/struct.Pair.html#structfield.left",
		"api::shout":              "macro.shout.html",
		"api::shout!":             "macro.shout.html",
		"Hidden":                  "struct.Hidden.html",
		"Private":                 "struct.Renamed.html",
		"Renamed":                 "struct.Renamed.html",
		"from_glob":               "fn.from_glob.html",
		"`api::Config`":           "api/struct.Config.html",
		"inner::Hidden":           "struct.Hidden.html",
		"crate::inner::Plain":     "struct.Renamed.html",
		"self::api::Config":       "api/struct.Config.html",
		"crate::api::Config::new": "api/struct.Config.html#method.new",
	} {
		got, ok := d.Link(sample().Module(""), link)
		if !ok || got != testBase+want {
			t.Errorf("Link(%q) = %q, %v; want %q", link, got, ok, testBase+want)
		}
	}
	for _, link := range []string{
		"hidden_fn", "crate_fn", "cfg_fn", "inner::Deep", "api::Config::secret", "api::Choice::B",
		"api::Pair::hidden", "api::Pair::gone", "api::Nope", "std::fmt", "::std::fmt", "Self", "Self::x", "serde::Serialize",
		"", "not a path", "api::Config::new::x",
	} {
		if got, ok := d.Link(sample().Module(""), link); ok {
			t.Errorf("Link(%q) = %q, want no link", link, got)
		}
	}
}

func TestDocsScope(t *testing.T) {
	t.Parallel()
	c := sample()
	d := testDocs(t, c, nil)
	api := c.Module("api")
	for link, want := range map[string]string{
		"Config":       "api/struct.Config.html",
		"self::Config": "api/struct.Config.html",
		"super::run":   "fn.run.html",
		"Hidden":       "struct.Hidden.html",
	} {
		if got, ok := d.Link(api, link); !ok || got != testBase+want {
			t.Errorf("Link(api, %q) = %q, %v; want %q", link, got, ok, testBase+want)
		}
	}
	if got, ok := d.Link(api, "run"); ok {
		t.Errorf("Link(api, run) = %q: run is in the root, not in api", got)
	}
	if got, ok := d.Item(Found{Module: c.Module("api"), Item: c.Module("api").Item("Config", KindStruct), Path: "api::Config"}); !ok || got != testBase+"api/struct.Config.html" {
		t.Errorf("Item generic struct = %q, %v", got, ok)
	}
	if got, ok := d.Item(Found{Module: c.Module("")}); !ok || got != testBase+"index.html" {
		t.Errorf("Item crate root = %q, %v", got, ok)
	}
	if got, ok := d.Item(Found{Module: c.Module("inner")}); ok {
		t.Errorf("Item private module = %q", got)
	}
}

func TestDocsPublished(t *testing.T) {
	t.Parallel()
	c := sample()
	for name, registries := range map[string][]string{"unset": nil, "crates-io": {"crates-io"}} {
		if testDocs(t, c, registries) == nil {
			t.Errorf("publish %s: no Docs", name)
		}
	}
	for name, registries := range map[string][]string{"false": {}, "private": {"internal"}} {
		if d := testDocs(t, c, registries); d != nil {
			t.Errorf("publish %s: Docs %+v", name, d)
		}
	}
	bin := &cargo.Package{Name: "tool", Version: "1.0.0", Targets: []cargo.Target{{Name: "tool", Kind: "bin"}}}
	if NewDocs(bin, c) != nil {
		t.Error("a crate with no library has Docs")
	}
	var none *Docs
	if _, ok := none.Link(c.Module(""), "run"); ok {
		t.Error("nil Docs links")
	}
	if none.Resolver(c.Module("")) != nil {
		t.Error("nil Docs has a resolver")
	}
}

func TestReexportChoice(t *testing.T) {
	t.Parallel()
	c := build(
		mod("", "", []*Item{item(KindMod, "a"), item(KindMod, "b"), item(KindMod, "deep")},
			Use{Name: "Thing", Path: []string{"deep", "x", "Thing"}, Vis: "pub"}),
		mod("a", "pub", nil, Use{Name: "Thing", Path: []string{"crate", "deep", "x", "Thing"}, Vis: "pub"}),
		mod("b", "pub", nil, Use{Name: "Thing", Path: []string{"crate", "deep", "x", "Thing"}, Vis: "pub"}),
		mod("deep", "", []*Item{item(KindMod, "x")}),
		mod("deep::x", "pub", []*Item{item(KindStruct, "Thing")}),
	)
	d := testDocs(t, c, nil)
	got, ok := d.Link(c.Module(""), "deep::x::Thing")
	if !ok || got != testBase+"struct.Thing.html" {
		t.Errorf("shortest re-export: %q, %v", got, ok)
	}
}

func TestReturnedBy(t *testing.T) {
	t.Parallel()
	err := item(KindEnum, "Error")
	err.Variants = []Variant{{Name: "NotFound"}, {Name: "Locked"}, {Name: "Other"}}
	other := item(KindEnum, "Error")
	other.Variants = []Variant{{Name: "NotFound"}}
	fn := func(name, ret, doc string) *Item {
		f := item(KindFn, name)
		f.Ret = ret
		f.Doc = doc
		return f
	}
	c := build(
		mod("", "pub", []*Item{err, item(KindMod, "io"), item(KindMod, "work")}),
		mod("io", "pub", []*Item{other, fn("read", "Result<(), Error>", "# Errors\n\n[`Error::NotFound`] of this module.")}),
		mod("work", "pub", []*Item{
			fn("plain", "Result<(), crate::Error>", "# Errors\n\n[`Error::NotFound`] and `crate::Error::Locked`."),
			fn("bare", "Result<(), crate::Error>", "# Errors\n\n`Locked` when busy, [`Other`] otherwise."),
			fn("bare_no_return", "Result<()>", "# Errors\n\n`Locked` when busy."),
			fn("link_text", "Result<(), crate::Error>", "# Errors\n\nSee [the lock][l].\n\n[l]: crate::Error::Locked"),
			fn("fenced", "Result<(), crate::Error>", "# Errors\n\n```\nError::Locked\n```"),
			fn("elsewhere", "Result<(), crate::Error>", "Returns [`Error::Locked`].\n\n# Panics\n\n`Error::NotFound`"),
			fn("nested", "Result<(), crate::Error>", "# Errors\n\n## When\n\n`Error::Other`\n\n# Next\n\n`Error::Locked`"),
			fn("other_type", "Result<(), io::Error>", "# Errors\n\n[`crate::io::Error::NotFound`]"),
			fn("enum_only", "Result<(), crate::Error>", "# Errors\n\n[`Error`]"),
			fn("hidden", "Result<(), crate::Error>", "# Errors\n\n`Error::Locked`"),
			{Kind: KindFn, Name: "private", Doc: "# Errors\n\n`Error::Locked`", Ret: "Result<(), crate::Error>"},
		}, Use{Name: "Error", Path: []string{"crate", "Error"}}),
	)
	c.Module("work").Items[9].Vis = "pub(crate)"
	typ, lookupErr := c.Lookup("x", "Error")
	if lookupErr != nil {
		t.Fatal(lookupErr)
	}
	got := c.ReturnedBy(typ)
	want := map[string][]string{
		"NotFound": {"work::plain"},
		"Locked":   {"work::bare", "work::link_text", "work::plain"},
		"Other":    {"work::bare", "work::nested"},
	}
	if len(got) != len(want) {
		t.Errorf("ReturnedBy = %v, want %v", got, want)
	}
	for v, fns := range want {
		if !slices.Equal(got[v], fns) {
			t.Errorf("ReturnedBy[%s] = %v, want %v", v, got[v], fns)
		}
	}
	io, lookupErr := c.Lookup("x", "io::Error")
	if lookupErr != nil {
		t.Fatal(lookupErr)
	}
	if got := c.ReturnedBy(io); !slices.Equal(got["NotFound"], []string{"io::read", "work::other_type"}) {
		t.Errorf("ReturnedBy(io::Error) = %v", got)
	}
}

func TestReturnedByStructAndSelf(t *testing.T) {
	t.Parallel()
	st := item(KindStruct, "Failure")
	en := item(KindEnum, "Kind")
	en.Variants = []Variant{{Name: "Bad"}}
	parse := item(KindFn, "Kind::parse")
	parse.Ret = "Result<Self, Failure>"
	parse.Doc = "# Errors\n\n[`Self::Bad`] and [`Failure`]."
	mk := item(KindFn, "mk")
	mk.Ret = "Result<(), Failure>"
	mk.Doc = "# Errors\n\n`Failure`"
	c := build(mod("", "", []*Item{st, en, parse, mk}))
	f, err := c.Lookup("x", "Failure")
	if err != nil {
		t.Fatal(err)
	}
	if got := c.ReturnedBy(f)[""]; !slices.Equal(got, []string{"Kind::parse", "mk"}) {
		t.Errorf("struct: %v", got)
	}
	k, err := c.Lookup("x", "Kind")
	if err != nil {
		t.Fatal(err)
	}
	if got := c.ReturnedBy(k)["Bad"]; !slices.Equal(got, []string{"Kind::parse"}) {
		t.Errorf("Self: %v", got)
	}
}

func TestMarkdownLinks(t *testing.T) {
	t.Parallel()
	c := sample()
	d := testDocs(t, c, nil)
	doc := "See [`run`], [the config](api::Config), [`Config`][cfg], [label][cfg], [`Choice::A`] and [gone] or [`Nope`].\n\n" +
		"[Web](https://example.com) and [Docs] stay.\n\n" +
		"# Section\n\n" +
		"Only [`MAX`][max] and [cfg].\n\n" +
		"```\n[`run`]\n```\n\n" +
		"[cfg]: api::Config\n[max]: crate::MAX\n[Docs]: https://example.com/docs\n[gone]: hidden_fn\n"
	got, err := Markdown(doc, "", "x", d.Resolver(c.Module("")))
	if err != nil {
		t.Fatal(err)
	}
	want := "See [`run`](" + testBase + "fn.run.html), [the config](" + testBase + "api/struct.Config.html), " +
		"[`Config`](" + testBase + "api/struct.Config.html), [label](" + testBase + "api/struct.Config.html), " +
		"`Choice::A` and gone or `Nope`.\n\n" +
		"[Web](https://example.com) and [Docs] stay.\n\n" +
		"### Section\n\n" +
		"Only [`MAX`](" + testBase + "constant.MAX.html) and [cfg](" + testBase + "api/struct.Config.html).\n\n" +
		"```rust\n[`run`]\n```\n\n" +
		"[Docs]: https://example.com/docs"
	if got != want {
		t.Errorf("Markdown =\n%s\nwant\n%s", got, want)
	}
	sec, err := Markdown(doc, "Section", "x", d.Resolver(c.Module("")))
	if err != nil {
		t.Fatal(err)
	}
	if want := "Only [`MAX`](" + testBase + "constant.MAX.html) and [cfg](" + testBase + "api/struct.Config.html).\n\n```rust\n[`run`]\n```\n\n[Docs]: https://example.com/docs"; sec != want {
		t.Errorf("section =\n%s\nwant\n%s", sec, want)
	}
	text, err := Markdown("[`run`] and [x](api::Config)", "", "x", nil)
	if err != nil || text != "`run` and x" {
		t.Errorf("no resolver: %q, %v", text, err)
	}
}
