package gen

import (
	"strings"
	"testing"

	"github.com/USA-RedDragon/docgen/internal/rustsrc"
)

func configField(name, rename string) rustsrc.Field {
	f := rustsrc.Field{Name: name}
	if rename != "" {
		f.Serde = &rustsrc.Serde{Rename: rename}
	}
	return f
}

func configMarker(args string) string {
	return "<!-- docgen:config crate=cfg-app " + args + " -->\n<!-- /docgen:config -->\n"
}

func TestRustConfigTables(t *testing.T) {
	t.Parallel()
	page := configMarker("type=server::Server") +
		configMarker("type=server::Limits") +
		configMarker("type=server::Client") +
		configMarker("type=server::Tls print=debug") +
		configMarker("type=server::Server print=name") +
		configMarker("type=server::Server print=raw") +
		configMarker("type=server::Server print={:?}")
	got, err := rustRun(t, page)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"| Field | Type | Default | Description |\n|---|---|---|---|\n| `host` | `String` | `\"localhost\".to_string()` (localhost) | The host name to listen on. |\n",
		"| `port` | `u16` | `DEFAULT_PORT` (8080) | The port to listen on. |\n",
		"| `request-timeout` | `Duration` | `Duration::from_millis(1500)` (1.5s) |",
		"| `max-conns` | `usize` | `64 * 4` (256) | The most connections at once, in one number \\| or two. |\n",
		"| `tls` | `Tls` | `Tls::default()` (Tls { enabled: false, cert: None, versions: [] }) |",
		"| `headers` | `Vec<String>` | `Default::default()` ([]) |",
		"| `retries` | `u32` | `default_retries()` (3) |",
		"| `log-file` | `Option<String>` | `None` | The log file, if any. By default there is none. |\n",
		"| `note` | `String` | `String::from(\"kept\")` (kept) |  |\n",
		"| `rate` | `u32` | `50` | The most requests a second. |\n| `pool` | `usize` | `8` |",
		"| `token` | `Token` | `Default::default()` | A label with no print impl. |\n",
		"| `name` | `String` | no default | The name of the client. |\n| `depth` | `u8` | not machine-readable; see the description |",
		"| `server` | `Server` | no default | The nested settings. |\n| `peer` | `Option<Box<Server>>` | no default |",
		"| `enabled` | `bool` | `false` |",
		"| `versions` | `Vec<String>` | `[]` |",
		"| `port` | `u16` | `DEFAULT_PORT` (server::DEFAULT_PORT) |",
		"| `host` | `String` | `\"localhost\".to_string()` |",
		"| `host` | `String` | `\"localhost\".to_string()` (\"localhost\") |",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("page lacks %q:\n%s", want, got)
		}
	}
	for _, not := range []string{"`cache`", "`hidden`", "`private`", "`log_file`", "`max_connections`", "`request_timeout`"} {
		if strings.Contains(got, not) {
			t.Errorf("page has the row %s:\n%s", not, got)
		}
	}
}

func TestRustConfigErrors(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ tag, want string }{
		{"type=kinds::Mode", "kinds::Mode is a enum, not a struct"},
		{"type=kinds::Pair", "kinds::Pair is a tuple struct, which has no named fields"},
		{"type=kinds::Nothing", "kinds::Nothing is a unit struct, which has no named fields"},
		{"type=kinds::Wrapper", "Wrapper is generic"},
		{"type=kinds::Hidden", "Hidden has no row: it has no pub field and does not derive Deserialize"},
		{"type=kinds::LIMIT", "kinds::LIMIT is a const, not a struct"},
		{"type=kinds::Missing", "crate cfg-app has no item kinds::Missing; the names in module kinds are"},
		{"type=broken::Boom", "panicked for broken::Boom.size: no default for Boom"},
		{"type=server::Server print=none", "print=none is not display, debug, raw, name or a Rust format spec"},
		{"type=server::Tls print={:x}", "does not compile for server::Tls.enabled"},
		{"type=server::Server pkg=x", "has both crate= and pkg="},
		{"type=server::Tls features=nope", "nope"},
	} {
		_, err := rustRun(t, configMarker(tc.tag))
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: error %v, want %q", tc.tag, err, tc.want)
		}
	}
}

func TestRustConfigPanicIsolated(t *testing.T) {
	t.Parallel()
	_, err := rustRun(t, configMarker("type=broken::Boom")+configMarker("type=server::Tls"))
	if err == nil || !strings.Contains(err.Error(), "no default for Boom") {
		t.Fatalf("error %v", err)
	}
	if strings.Contains(err.Error(), "server::Tls") {
		t.Errorf("the panic is an error of the Tls marker: %v", err)
	}
}

func TestSerdeKey(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, all, want string
	}{
		{"max_conns", "", "max_conns"},
		{"max_conns", "lowercase", "max_conns"},
		{"max_conns", "UPPERCASE", "MAX_CONNS"},
		{"max_conns", "PascalCase", "MaxConns"},
		{"max_conns", "camelCase", "maxConns"},
		{"max_conns", "snake_case", "max_conns"},
		{"max_conns", "SCREAMING_SNAKE_CASE", "MAX_CONNS"},
		{"max_conns", "kebab-case", "max-conns"},
		{"max_conns", "SCREAMING-KEBAB-CASE", "MAX-CONNS"},
	} {
		fl := configField(tc.name, "")
		got, err := serdeKey(fl, tc.all)
		if err != nil || got != tc.want {
			t.Errorf("%s %s: %q, %v, want %q", tc.name, tc.all, got, err, tc.want)
		}
	}
	if got, err := serdeKey(configField("max_conns", "n"), "kebab-case"); err != nil || got != "n" {
		t.Errorf("rename: %q, %v", got, err)
	}
	if _, err := serdeKey(configField("a", ""), "Title"); err == nil || !strings.Contains(err.Error(), "rename_all") {
		t.Errorf("unknown case: %v", err)
	}
}
