package excerpt

import (
	"strings"
	"testing"
)

const jsonSrc = `{
  "name": "demo",
  "a.b": 1,
  "server": {
    "port": 8080,
    "hosts": [
      "x",
      "y"
    ]
  },
  "steps": [
    {"name": "One", "run": "a"},
    {
      "name": "Two",
      "run": "b"
    },
    {"name": "Three:x", "run": "c"}
  ],
  "inline": {"k": {"z": 1,
                   "w": 2}},
  "tab": {
	"t": 1
  }
}
`

func TestJSONSymbol(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ sym, want string }{
		{"name", `"name": "demo"`},
		{`a\.b`, `"a.b": 1`},
		{"server", "\"server\": {\n  \"port\": 8080,\n  \"hosts\": [\n    \"x\",\n    \"y\"\n  ]\n}"},
		{"server.port", `"port": 8080`},
		{"server.hosts", "\"hosts\": [\n  \"x\",\n  \"y\"\n]"},
		{"server.hosts[1]", `"y"`},
		{"server.hosts[0:1]", "\"x\",\n\"y\""},
		{"steps[One]", `{"name": "One", "run": "a"}`},
		{"steps[One:Two]", "{\"name\": \"One\", \"run\": \"a\"},\n{\n  \"name\": \"Two\",\n  \"run\": \"b\"\n}"},
		{"steps[0:1]", "{\"name\": \"One\", \"run\": \"a\"},\n{\n  \"name\": \"Two\",\n  \"run\": \"b\"\n}"},
		{"steps[Three:x]", `{"name": "Three:x", "run": "c"}`},
		{"steps[2]", `{"name": "Three:x", "run": "c"}`},
		{"inline.k", "\"k\": {\"z\": 1,\n      \"w\": 2}"},
		{"tab.t", `"t": 1`},
	} {
		got, err := jsonExcerpt([]byte(jsonSrc), c.sym)
		if err != nil {
			t.Errorf("%s: %v", c.sym, err)
			continue
		}
		if got != c.want {
			t.Errorf("%s = %q, want %q", c.sym, got, c.want)
		}
	}
}

func TestJSONSymbolErrors(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ src, sym, want string }{
		{jsonSrc, "nope", "no key nope"},
		{jsonSrc, "server.nope", "no key server.nope"},
		{jsonSrc, "name.x", "not an object"},
		{jsonSrc, "server[One]", "not an array"},
		{jsonSrc, "steps[Nine]", "no items"},
		{jsonSrc, "steps[Two:One]", "no items"},
		{jsonSrc, "steps[3]", "no items"},
		{jsonSrc, "a..b", "empty key"},
		{jsonSrc, "steps[One", "no ]"},
		{`{"s": [{"name": "a"}, {"name": "a"}]}`, "s[a]", "ambiguous"},
		{`{"s": [{"name": "a:b"}, {"name": "a"}, {"name": "b"}]}`, "s[a:b]", "ambiguous"},
		{`{"a": 1,}`, "a", "invalid"},
		{`{"a": 1} x`, "a", "invalid"},
		{`{"a": 1, "a": 2}`, "a", "duplicate"},
		{`[1]`, "a", "not an object"},
		{``, "a", "invalid"},
	} {
		_, err := jsonExcerpt([]byte(c.src), c.sym)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s in %q: error %v, want one with %q", c.sym, c.src, err, c.want)
		}
	}
}

func TestJSONSymbolCRLFAndBOM(t *testing.T) {
	t.Parallel()
	crlf := strings.ReplaceAll(jsonSrc, "\n", "\r\n")
	for _, src := range []string{crlf, "\ufeff" + jsonSrc, "\ufeff" + crlf} {
		got, err := jsonExcerpt([]byte(src), "server.hosts")
		if want := "\"hosts\": [\n  \"x\",\n  \"y\"\n]"; err != nil || got != want {
			t.Errorf("server.hosts = %q, %v, want %q", got, err, want)
		}
	}
	if got, err := jsonExcerpt([]byte("\ufeff{\"a\": 1}"), "a"); err != nil || got != `"a": 1` {
		t.Errorf("a after a BOM = %q, %v", got, err)
	}
}

const tomlSrc = `# top comment
title = "demo"
"a.b" = 1

[server]
port = 8080 # the port
# hosts to serve
hosts = [
  "x", # first
  "y",
]
motd = """
Hello \"""
[not a table]
"""

[ server . "tls" ]
cert = 'c.pem'

[[steps]]
name = "One"
run = "a"

[[steps]]
name = 'Two'
run = '''
b
'''

  [steps.env]
  K = "v"

[[steps]]
name = "Thr\u0065e"
inline = { a = 1, b = [1, 2] }

# trailing comment
[other]
x.y = 2
t = { a = 1 }
when = 1979-05-27 07:32:00Z

[indent]
	k = [
	  1,
	]
`

func TestTOMLSymbol(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ sym, want string }{
		{"title", `title = "demo"`},
		{`a\.b`, `"a.b" = 1`},
		{"server", "[server]\nport = 8080 # the port\n# hosts to serve\nhosts = [\n  \"x\", # first\n  \"y\",\n]\nmotd = \"\"\"\nHello \\\"\"\"\n[not a table]\n\"\"\"\n\n[ server . \"tls\" ]\ncert = 'c.pem'"},
		{"server.tls", "[ server . \"tls\" ]\ncert = 'c.pem'"},
		{"server.port", "port = 8080 # the port"},
		{"server.hosts", "hosts = [\n  \"x\", # first\n  \"y\",\n]"},
		{"server.motd", "motd = \"\"\"\nHello \\\"\"\"\n[not a table]\n\"\"\""},
		{"server.tls.cert", "cert = 'c.pem'"},
		{"steps[One]", "[[steps]]\nname = \"One\"\nrun = \"a\""},
		{"steps[Two]", "[[steps]]\nname = 'Two'\nrun = '''\nb\n'''\n\n  [steps.env]\n  K = \"v\""},
		{"steps[Two:Three]", "[[steps]]\nname = 'Two'\nrun = '''\nb\n'''\n\n  [steps.env]\n  K = \"v\"\n\n[[steps]]\nname = \"Thr\\u0065e\"\ninline = { a = 1, b = [1, 2] }"},
		{"steps[2]", "[[steps]]\nname = \"Thr\\u0065e\"\ninline = { a = 1, b = [1, 2] }"},
		{"other.x.y", "x.y = 2"},
		{"other.t", "t = { a = 1 }"},
		{"other.when", "when = 1979-05-27 07:32:00Z"},
		{"indent.k", "k = [\n  1,\n]"},
	} {
		got, err := tomlExcerpt([]byte(tomlSrc), c.sym)
		if err != nil {
			t.Errorf("%s: %v", c.sym, err)
			continue
		}
		if got != c.want {
			t.Errorf("%s = %q, want %q", c.sym, got, c.want)
		}
	}
}

func TestTOMLSymbolErrors(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ src, sym, want string }{
		{tomlSrc, "nope", "no key nope"},
		{tomlSrc, "server.nope", "no key server.nope"},
		{tomlSrc, "steps", "array of tables"},
		{tomlSrc, "steps.name", "array of tables"},
		{tomlSrc, "server[One]", "not an array of tables"},
		{tomlSrc, "steps[Nine]", "no items"},
		{tomlSrc, "steps[Three:One]", "no items"},
		{tomlSrc, "other.x", "dotted keys"},
		{tomlSrc, "other.t.a", "inside the value of other.t"},
		{tomlSrc, "a..b", "empty key"},
		{"[[s]]\nname = \"a\"\n[[s]]\nname = \"a\"\n", "s[a]", "ambiguous"},
		{"[a]\nk = 1\n[a]\nj = 2\n", "a", "more than once"},
		{"a = \"\"\"x\n", "a", "line 1: unterminated string"},
		{"a = [1,\n", "a", "line 1: unterminated array"},
		{"a = { b = 1\n", "a", "line 1: unterminated inline table"},
		{"a = \"x\n\"\n", "a", "line 1: unterminated string"},
		{"a 1\n", "a", "line 1: no = after the key"},
		{"a =\n", "a", "line 1: no value"},
		{"[a\n", "a", "line 1: no ] after the table name"},
		{"a = 1 ]\n", "a", "line 1: text after the value"},
		{"a = \"\\q\"\n", "a", "line 1: invalid escape"},
		{strings.Repeat("a = [", 1) + strings.Repeat("[", 200), "a", "nested too deeply"},
	} {
		_, err := tomlExcerpt([]byte(c.src), c.sym)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s in %q: error %v, want one with %q", c.sym, c.src, err, c.want)
		}
	}
}

func TestTOMLSymbolCRLFAndBOM(t *testing.T) {
	t.Parallel()
	crlf := strings.ReplaceAll(tomlSrc, "\n", "\r\n")
	for _, src := range []string{crlf, "\ufeff" + tomlSrc, "\ufeff" + crlf} {
		got, err := tomlExcerpt([]byte(src), "server.hosts")
		if want := "hosts = [\n  \"x\", # first\n  \"y\",\n]"; err != nil || got != want {
			t.Errorf("server.hosts = %q, %v, want %q", got, err, want)
		}
	}
	if got, err := tomlExcerpt([]byte("\ufeffa = 1"), "a"); err != nil || got != "a = 1" {
		t.Errorf("a after a BOM = %q, %v", got, err)
	}
}

func TestObjectSymbolDispatch(t *testing.T) {
	t.Parallel()
	if got, err := Symbol("c.json", []byte(`{"a": {"b": 1}}`), "a.b"); err != nil || got != `"b": 1` {
		t.Errorf("JSON a.b = %q, %v", got, err)
	}
	if got, err := RawSymbol("c.toml", []byte("[a]\nb = 1\n"), "a.b"); err != nil || got != "b = 1" {
		t.Errorf("TOML a.b = %q, %v", got, err)
	}
	if Lang("c.json") != "json" || Lang("c.toml") != "toml" {
		t.Error("the fence language of JSON or TOML is wrong")
	}
}

func FuzzJSONSymbol(f *testing.F) {
	f.Add([]byte(jsonSrc), "steps[One:Two]")
	f.Add([]byte(jsonSrc), "inline.k")
	f.Add([]byte("\ufeff{\"a\": [1, {\"name\": \"x\"}]}"), "a[x]")
	f.Fuzz(func(_ *testing.T, src []byte, sym string) {
		_, _ = jsonExcerpt(src, sym)
	})
}

func FuzzTOMLSymbol(f *testing.F) {
	f.Add([]byte(tomlSrc), "steps[Two:Three]")
	f.Add([]byte(tomlSrc), "server.motd")
	f.Add([]byte("a = { b = [ '''x''', \"\"\"y\"\"\" ] }\n"), "a")
	f.Fuzz(func(_ *testing.T, src []byte, sym string) {
		_, _ = tomlExcerpt(src, sym)
	})
}
