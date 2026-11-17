package gen

import (
	"strings"
	"testing"
)

func TestHandlerArgErrors(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ tag, want string }{
		{"nope a=b", "line 1: unknown marker kind docgen:nope"},
		{"output", "line 1: docgen:output must have exactly one of cmd=, exec= or test="},
		{"output cmd=examples/a test=b.C", "line 1: docgen:output must have exactly one of cmd=, exec= or test="},
		{"output cmd=examples/a x=1", "line 1: docgen:output must have exactly one of cmd=, exec= or test="},
		{"output x=1", "line 1: docgen:output must have exactly one of cmd=, exec= or test="},
		{"output crate=a", "line 1: docgen:output with crate=, must have exactly one of example=, bin= or doctest="},
		{"output crate=a example=b bin=c", "line 1: docgen:output with crate=, must have exactly one of example=, bin= or doctest="},
		{"output crate=a example=b x=1", "line 1: docgen:output with crate=, must have exactly one of example=, bin= or doctest="},
		{"output example=b", "line 1: docgen:output example=, bin=, doctest= and args= need crate="},
		{"output crate=a cmd=b", "line 1: docgen:output with crate=, there is no cmd=; give one of example=, bin= or doctest="},
		{"output crate=a doctest=b args=c", "line 1: docgen:output args= goes with example= or bin=, not doctest="},
		{`output crate=a example=b args="a \"b"`, "line 1: docgen:output args= has a \" with no end"},
		{`output crate="" example=b`, "line 1: docgen:output: crate= is empty"},
		{`output crate=a doctest=""`, "line 1: docgen:output: doctest= is empty"},
		{`output cmd=""`, "line 1: docgen:output: cmd= is empty"},
		{`code file=a region=""`, "line 1: docgen:code: region= is empty"},
		{"code", "line 1: docgen:code: file= is required"},
		{`code file=""`, "line 1: docgen:code: file= is required"},
		{"code lines=1-2 symbol=b", "line 1: docgen:code: file= is required"},
		{"code file=a lines=1-2 symbol=b", "line 1: docgen:code can have only one of lines=, symbol= and region="},
		{"code file=a symbol=b region=c", "line 1: docgen:code can have only one of lines=, symbol= and region="},
		{`code file=a lines="" region=""`, "line 1: docgen:code can have only one of lines=, symbol= and region="},
		{"code file=a lines=1-2 region=c nope=1", "line 1: docgen:code can have only one of lines=, symbol= and region="},
		{"code file=a nope=1", "line 1: docgen:code has no argument nope="},
		{"code file=a raw=false", "line 1: docgen:code raw= can only be true"},
		{"code file=a raw=1", "line 1: docgen:code raw= can only be true"},
		{`code file=a raw=""`, "line 1: docgen:code raw= can only be true"},
		{"code file=a region=b raw=true", "line 1: docgen:code raw=true does not apply to region="},
		{"states", "line 1: docgen:states: pkg= is required"},
		{"states pkg=a", "line 1: docgen:states: type= is required"},
		{"states pkg=a type=T name=1", "line 1: docgen:states has no argument name="},
		{"config type=T", "line 1: docgen:config: pkg= is required"},
		{"config pkg=a", "line 1: docgen:config: type= is required"},
		{"config pkg=a type=T run=1", "line 1: docgen:config has no argument run="},
		{"errors pkg=a", "line 1: docgen:errors: type= is required"},
		{"errors pkg=a type=T run=1", "line 1: docgen:errors has no argument run="},
		{"configulator pkg=a", "line 1: docgen:configulator: type= is required"},
		{"configulator pkg=a type=T run=1", "line 1: docgen:configulator has no argument run="},
		{"const name=N", "line 1: docgen:const: pkg= is required"},
		{"const pkg=a", "line 1: docgen:const: name= is required"},
		{"const file=a.h", "line 1: docgen:const: name= is required"},
		{"const file=a.h name=N extra=1", "line 1: docgen:const has no argument extra="},
		{"const file=a.txt name=N", "line 1: docgen:const file= is not a .c or .h file"},
		{"const file=a name=N", "line 1: docgen:const file= is not a .c or .h file"},
		{"const file=a.h name=N pkg=p", "line 1: docgen:const has both file= and pkg=; file= is for C, pkg= for Go and crate= for Rust"},
		{"const file=a.h name=N crate=c", "line 1: docgen:const has both file= and crate=; file= is for C, pkg= for Go and crate= for Rust"},
		{"const file=a.h name=N print=raw", "line 1: docgen:const print=raw does not apply to a C #define; the one value is print=value, an integer literal without its suffix"},
		{`const file=a.h name=N print=""`, "line 1: docgen:const: print= is empty"},
		{"const file=a.h name=N code=true", "line 1: docgen:const code= can only be false"},
		{`const file="" name=N`, "line 1: docgen:const: file= is required"},
		{"const pkg=a name=N extra=1", "line 1: docgen:const has no argument extra="},
		{"doc", "line 1: docgen:doc: pkg= is required"},
		{"doc pkg=a type=T", "line 1: docgen:doc has no argument type="},
		{"gomod", "line 1: docgen:gomod: field= is required"},
		{"gomod field=go x=1", "line 1: docgen:gomod has no argument x="},
		{"usage", "line 1: docgen:usage: cmd= is required"},
		{"usage cmd=a y=1", "line 1: docgen:usage has no argument y="},
		{"cargo", "line 1: docgen:cargo: field= is required"},
		{"cargo field=a x=1", "line 1: docgen:cargo has no argument x="},
		{"cargo field=a crate=b file=c", "line 1: docgen:cargo has both crate= and file=; use one"},
		{`cargo field=a crate=""`, "line 1: docgen:cargo: crate= is empty"},
		{`cargo field=a file=""`, "line 1: docgen:cargo: file= is empty"},
		{"cargo field=a code=true", "line 1: docgen:cargo code= can only be false"},
		{"cargo field=features", "line 1: docgen:cargo field=features needs crate="},
		{"cargo field=features crate=a file=b", "line 1: docgen:cargo has both crate= and file=; use one"},
		{"cargo field=features code=false crate=a", "line 1: docgen:cargo has no argument code="},
		{"crates kind=exe", "line 1: docgen:crates kind= is not lib, proc-macro, bin"},
		{"crates deps=all,dev", "line 1: docgen:crates deps= is not normal, dev, build, all"},
		{"crates match=[", "line 1: docgen:crates match= is not a glob: syntax error in pattern"},
		{"crates columns=owner", "line 1: docgen:crates columns= key \"owner\" is not metadata.<key>[.<key>...], with keys of letters, digits, - and _"},
		{`crates columns=""`, "line 1: docgen:crates: columns= is empty"},
		{"crates x=1", "line 1: docgen:crates has no argument x="},
		{`crates kind=""`, "line 1: docgen:crates: kind= is empty"},
		{"usage crate=a cmd=b", "line 1: docgen:usage has both cmd= and crate=; use one"},
		{"usage bin=a", "line 1: docgen:usage has bin= with no crate="},
		{"usage cmd=a bin=b", "line 1: docgen:usage has bin= with no crate="},
		{`usage crate=a bin=""`, "line 1: docgen:usage: bin= is empty"},
		{"docgen page=b", "line 1: docgen:docgen: root= is required"},
		{"docgen root=a", "line 1: docgen:docgen: page= is required"},
		{`docgen root=a page=""`, "line 1: docgen:docgen: page= is required"},
		{"docgen root=a page=b x=1", "line 1: docgen:docgen has no argument x="},
		{"var", "line 1: docgen:var: name= is required"},
		{"var name=a print=name", "line 1: docgen:var print=name is not valid for a value with no constant"},
		{"var name=a print=nope", "line 1: docgen:var print=nope is not go, raw, config, name or a fmt verb"},
		{"const pkg=a name=N print=nope", "line 1: docgen:const print=nope is not go, raw, config, name or a fmt verb"},
		{"config pkg=a type=T print=nope", "line 1: docgen:config print=nope is not go, raw, config, name or a fmt verb"},
		{`var name=a print=""`, "line 1: docgen:var: print= is empty"},
		{"var name=a code=true", "line 1: docgen:var code= can only be false"},
		{"const pkg=a name=N code=sh", "line 1: docgen:const code= can only be false"},
		{"gomod field=go code=true", "line 1: docgen:gomod code= can only be false"},
		{`gomod field=go code=""`, "line 1: docgen:gomod: code= is empty"},
		{"config pkg=a type=T code=false", "line 1: docgen:config has no argument code="},
		{`usage cmd=a args=""`, "line 1: docgen:usage: args= is empty"},
		{`usage cmd=a args=" "`, "line 1: docgen:usage args= has no argument"},
		{`usage cmd=a args="a \"b"`, "line 1: docgen:usage args= has a \" with no end"},
		{"text", "line 1: docgen:text: text= is required"},
		{`text text=""`, "line 1: docgen:text: text= is required"},
		{`text text=a code=""`, "line 1: docgen:text: code= is empty"},
		{"text text=a name=b", "line 1: docgen:text has no argument name="},
		{"text text=a{b", "line 1: docgen:text text= has a { with no }; {{ writes a literal {"},
		{"text text={}", "line 1: docgen:text text= has a {} with no name"},
		{`const pkg=a name=N print=""`, "line 1: docgen:const: print= is empty"},
		{`config pkg=a type=T print=""`, "line 1: docgen:config: print= is empty"},
		{`states pkg=a type=T run=""`, "line 1: docgen:states: run= is empty"},
		{`configulator pkg=a type=T env=""`, "line 1: docgen:configulator: env= is empty"},
		{`configulator pkg=a type=T envsep=""`, "line 1: docgen:configulator: envsep= is empty"},
		{`configulator pkg=a type=T flagsep=""`, "line 1: docgen:configulator: flagsep= is empty"},
		{"states pkg=a type=T diagram=a.md", "line 1: docgen:states diagram=a.md does not end in .svg"},
		{"states pkg=a type=T diagram=/sub/x.svg", "line 1: docgen:states diagram=/sub/x.svg is not relative to the page"},
	} {
		pg := newPage("p.md", "p.md", "<!-- docgen:"+tc.tag+" -->\n")
		if err := pg.parse(); err == nil || err.Error() != tc.want {
			t.Errorf("%s: got %v, want %q", tc.tag, err, tc.want)
		}
	}
}

func TestHandlerArgs(t *testing.T) {
	t.Parallel()
	for _, tag := range []string{
		"output cmd=examples/a",
		"output test=a.B",
		"code file=a",
		"output crate=a example=b",
		`output crate=a bin=b args="x y"`,
		"output crate=a doctest=b::c",
		"code file=a lines=1-2",
		"code file=a symbol=b",
		"code file=a region=c",
		"states pkg=a type=T run=R",
		"config pkg=a type=T",
		"errors pkg=a type=T",
		"configulator pkg=a type=T env=E envsep=S flagsep=F",
		"const pkg=a name=N",
		"const file=a.h name=N",
		"const file=native/a.c name=N code=false",
		"doc pkg=a name=N section=S",
		"gomod field=go file=a",
		"usage cmd=a",
		`usage cmd=a args="help config"`,
		"var name=a print=%q code=false",
		"const pkg=a name=N print=name code=false",
		"gomod field=go code=false",
		"docgen root=a page=b",
		"docgen root=a page=b module=c",
		"var name=a-b_1",
		`text text="a {b}" code=sh`,
	} {
		kind, _, _ := strings.Cut(tag, " ")
		pg := newPage("p.md", "p.md", "<!-- docgen:"+tag+" -->\n<!-- /docgen:"+kind+" -->\n")
		if err := pg.parse(); err != nil {
			t.Errorf("%s: %v", tag, err)
			continue
		}
		if len(pg.markers) != 1 || pg.markers[0].handler == nil {
			t.Errorf("%s: markers %v", tag, pg.markers)
		}
	}
}

func TestInlineKinds(t *testing.T) {
	t.Parallel()
	for tag, want := range map[string]string{
		"const pkg=a name=N":    "x",
		"const file=a.h name=N": "x",
		"gomod field=go":        "x",
		"var name=a":            "x",
		"doc pkg=a":             "\nx\n",
	} {
		kind, _, _ := strings.Cut(tag, " ")
		pg := newPage("p.md", "p.md", "<!-- docgen:"+tag+" -->\n<!-- /docgen:"+kind+" -->\n")
		if err := pg.parse(); err != nil {
			t.Errorf("%s: %v", tag, err)
			continue
		}
		if got := pg.markers[0].Wrap("x"); got != want {
			t.Errorf("%s: Wrap = %q, want %q", tag, got, want)
		}
	}
}
