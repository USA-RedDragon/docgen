package marker

import (
	"testing"
)

func TestArgs(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ text, want string }{
		{"<!-- docgen:config --><!-- /docgen:config -->", "line 1: docgen:config: pkg= is required"},
		{"\n<!-- docgen:config pkg=\"\" type=T --><!-- /docgen:config -->", "line 2: docgen:config: pkg= is required"},
		{"<!-- docgen:config pkg=a b=1 --><!-- /docgen:config -->", "line 1: docgen:config: type= is required"},
		{"<!-- docgen:config pkg=a type=T z=1 b=2 --><!-- /docgen:config -->", "line 1: docgen:config has no argument b="},
		{"<!-- docgen:output x=1 --><!-- /docgen:output -->", "line 1: docgen:output must have exactly one of cmd=, exec= or test="},
		{"<!-- docgen:nope z=1 -->", "line 1: unknown marker kind docgen:nope"},
	} {
		if _, err := parse(tc.text); err == nil || err.Error() != tc.want {
			t.Errorf("%q: got %v, want %q", tc.text, err, tc.want)
		}
	}
}

func TestQuotedValues(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ raw, want string }{
		{`v="echo \"hi\""`, `echo "hi"`},
		{`v="\""`, `"`},
		{`v="a\"b\"c"`, `a"b"c`},
		{`v="C:\dir\file.txt"`, `C:\dir\file.txt`},
		{`v="^a\d+\.go$"`, `^a\d+\.go$`},
		{`v="\\server\share"`, `\\server\share`},
		{`v="a\nb"`, `a\nb`},
		{`v="a\\"`, `a\\`},
		{`v="a\\\"b"`, `a\\"b`},
		{"v=\"a\\\nb\"", "a\\\nb"},
		{`v=C:\dir`, `C:\dir`},
	} {
		text := "<!-- docgen:doc pkg=a section=x " + tc.raw + " --><!-- /docgen:doc -->"
		ms, err := Parse(text, func(string) Binder { return valueBinder{t, tc.want} })
		if err != nil {
			t.Errorf("%s: %v", tc.raw, err)
			continue
		}
		if len(ms) != 1 {
			t.Errorf("%s: got %d markers", tc.raw, len(ms))
		}
	}
}

type valueBinder struct {
	t    *testing.T
	want string
}

func (b valueBinder) Bind(a *Args) {
	a.Required("pkg")
	a.Required("section")
	if got := a.Required("v"); got != b.want {
		b.t.Errorf("value %q, want %q", got, b.want)
	}
}
