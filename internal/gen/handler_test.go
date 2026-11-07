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
		{"const pkg=a name=N extra=1", "line 1: docgen:const has no argument extra="},
		{"doc", "line 1: docgen:doc: pkg= is required"},
		{"doc pkg=a type=T", "line 1: docgen:doc has no argument type="},
		{"gomod", "line 1: docgen:gomod: field= is required"},
		{"gomod field=go x=1", "line 1: docgen:gomod has no argument x="},
		{"usage", "line 1: docgen:usage: cmd= is required"},
		{"usage cmd=a y=1", "line 1: docgen:usage has no argument y="},
		{"docgen page=b", "line 1: docgen:docgen: root= is required"},
		{"docgen root=a", "line 1: docgen:docgen: page= is required"},
		{`docgen root=a page=""`, "line 1: docgen:docgen: page= is required"},
		{"docgen root=a page=b x=1", "line 1: docgen:docgen has no argument x="},
		{"var", "line 1: docgen:var: name= is required"},
		{"var name=a print=name", "line 1: docgen:var print=name is not valid for a value with no constant"},
		{"var name=a print=nope", "line 1: docgen:var print=nope is not go, raw, name or a fmt verb"},
		{"const pkg=a name=N print=nope", "line 1: docgen:const print=nope is not go, raw, name or a fmt verb"},
		{"config pkg=a type=T print=nope", "line 1: docgen:config print=nope is not go, raw, name or a fmt verb"},
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
		"code file=a lines=1-2",
		"code file=a symbol=b",
		"code file=a region=c",
		"states pkg=a type=T run=R",
		"config pkg=a type=T",
		"errors pkg=a type=T",
		"configulator pkg=a type=T env=E envsep=S flagsep=F",
		"const pkg=a name=N",
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
		"const pkg=a name=N": "x",
		"gomod field=go":     "x",
		"var name=a":         "x",
		"doc pkg=a":          "\nx\n",
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
