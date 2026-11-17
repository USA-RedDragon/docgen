package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestMain removes the helper cache of the tests after they run.
func TestMain(m *testing.M) {
	code := m.Run()
	_ = os.RemoveAll(testOptions().rustHelperDir)
	os.Exit(code)
}

// testOptions returns the options of a run of a test: a helper cache that the
// tests of one process share, outside the user cache.
func testOptions() runOptions {
	return runOptions{rustHelperDir: filepath.Join(os.TempDir(), fmt.Sprintf("docgen-cli-test-rust-helper-%d", os.Getpid()))}
}

func needCargo(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("cargo"); err != nil {
		t.Skip("cargo is not on PATH; install a Rust toolchain to run this test")
	}
}

// fixtureRegions is a page with the regions of the Rust fixture that the
// guides of the repository excerpt, so that a check of a copy finds them used.
const fixtureRegions = "<!-- docgen:code file=crates/run-c/src/lib.rs region=area -->\n<!-- /docgen:code -->\n"

// rustRepo copies the Rust fixture workspace to a new directory with the
// files, and returns its path. The copy has no pages of the fixture, which the
// guides fill, so that a test checks only the pages that it writes, and a page
// docs/regions.md with fixtureRegions.
func rustRepo(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("testdata/rust")); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(root, "docs")); err != nil {
		t.Fatal(err)
	}
	writeTree(t, root, map[string]string{"docs/regions.md": fixtureRegions})
	writeTree(t, root, files)
	return root
}

// rustValuePage holds the Rust markers with values of the fixture.
const rustValuePage = `- <!-- docgen:const crate=core-a name=limits::MAX_JOBS --><!-- /docgen:const -->
- <!-- docgen:const crate=core-a name=crate::limits::SMALL_JOBS --><!-- /docgen:const -->
- <!-- docgen:const crate=core-a name=limits::NAME --><!-- /docgen:const -->
- <!-- docgen:const crate=core-a name=limits::TIMEOUT --><!-- /docgen:const -->
- <!-- docgen:const crate=core-a name=limits::MODES --><!-- /docgen:const -->
- <!-- docgen:const crate=core-a name=limits::LISTEN --><!-- /docgen:const -->
- <!-- docgen:const crate=core-a name=limits::LISTEN print=display --><!-- /docgen:const -->
- <!-- docgen:const crate=core-a name=limits::LISTEN print=debug --><!-- /docgen:const -->
- <!-- docgen:const crate=core-a name=limits::LISTEN print=name --><!-- /docgen:const -->
- <!-- docgen:const crate=core-a name=limits::LISTEN print=raw --><!-- /docgen:const -->
- <!-- docgen:const crate=core-a name=limits::MAX_JOBS print={:#06x} --><!-- /docgen:const -->
- <!-- docgen:const crate=core-a name=limits::Port::HTTP --><!-- /docgen:const -->
- <!-- docgen:const crate=core-a name=limits::RETRIES --><!-- /docgen:const -->
- <!-- docgen:const crate=core-a name=shapes::inner::RATIO --><!-- /docgen:const -->
- <!-- docgen:const crate=core-a name=renamed::deeper::DEPTH --><!-- /docgen:const -->
- <!-- docgen:const crate=core-a name=net::tcp::BACKLOG code=false --><!-- /docgen:const -->
- <!-- docgen:const crate=core-a-bin name=GREETING --><!-- /docgen:const -->

<!-- docgen:configulator crate=core-a type=config::AppConfig env=APP_ envsep=__ flagsep=- -->
<!-- /docgen:configulator -->
`

// rustBadValuePage holds Rust markers whose values are errors.
const rustBadValuePage = `- <!-- docgen:const crate=core-a name=limits::OPAQUE --><!-- /docgen:const -->
- <!-- docgen:const crate=core-a name=limits::NAME print={:x} --><!-- /docgen:const -->
- <!-- docgen:const crate=core-a name=limits::MAX_JOBS print=name --><!-- /docgen:const -->
- <!-- docgen:const crate=core-a name=limits::Port --><!-- /docgen:const -->
- <!-- docgen:const crate=core-a name=never::NEVER --><!-- /docgen:const -->
`

func TestRustValues(t *testing.T) {
	t.Parallel()
	needCargo(t)
	root := rustRepo(t, map[string]string{"docs/values.md": rustValuePage, "docs/bad.md": rustBadValuePage})
	rc, log, diff := docgenDiff(t, root, "check", "-diff")
	if rc != 1 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
	wantIn(t, diff,
		"name=limits::MAX_JOBS -->`8`<",
		"name=crate::limits::SMALL_JOBS -->`2`<",
		"name=limits::NAME -->`core-a`<",
		"name=limits::TIMEOUT -->`1.5s`<",
		"name=limits::MODES -->`[\"fast\", \"slow\"]`<",
		"name=limits::LISTEN -->`port 8080`<",
		"print=display -->`port 8080`<",
		"print=debug -->`Port(8080)`<",
		"print=name -->`limits::PORT_WEB`<",
		"print=raw -->`PORT_WEB`<",
		"print={:#06x} -->`0x0008`<",
		"name=limits::Port::HTTP -->`port 80`<",
		"name=limits::RETRIES -->`3`<",
		"name=shapes::inner::RATIO -->`1.5`<",
		"name=renamed::deeper::DEPTH -->`3`<",
		"code=false -->128<",
		"name=GREETING -->`hello`<",
		"| `jobs`      | integer | `4`     | `APP_JOBS`              | `-j`, `--jobs` | jobs that run at once                 |",
		"| `http.port` | integer | `8080`  | `APP_HTTP__LISTEN_PORT` | `--http-port`  | listen port                           |",
		"| `token`     | string  |         | `APP_TOKEN`             | `--token`      | API token (required) (secret)         |",
	)
	wantNotIn(t, diff, "# Configuration")
	wantIn(t, log,
		"docs/bad.md:1: docgen:const crate=core-a name=limits::OPAQUE: the value test of the library of crate core-a does not compile for limits::OPAQUE:",
		"docs/bad.md:2: docgen:const crate=core-a name=limits::NAME print={:x}: the value test of the library of crate core-a does not compile for limits::NAME:",
		"LowerHex",
		"docs/bad.md:3: docgen:const crate=core-a name=limits::MAX_JOBS print=name: no other const of type usize has the value of limits::MAX_JOBS",
		"docs/bad.md:4: docgen:const crate=core-a name=limits::Port: limits::Port is a struct, not a const or static",
		"docs/bad.md:5: docgen:const crate=core-a name=never::NEVER: the value test of the library of crate core-a printed no value for never::NEVER; it is in #[cfg(any())], which is false in the test build",
	)
	wantNotIn(t, log, "Compiling")
	for _, l := range strings.Split(log, "\n") {
		if strings.Contains(l, "values.md") && !strings.HasPrefix(l, "stale: docs/values.md:") {
			t.Errorf("an error of a good value: %s", l)
		}
	}
}

const rustReadPage = `- <!-- docgen:const crate=core-a name=limits::SMALL_JOBS --><!-- /docgen:const -->
- <!-- docgen:const crate=core-a name=limits::SMALL_JOBS print=debug --><!-- /docgen:const -->
- <!-- docgen:const crate=core-a name=never::NEVER --><!-- /docgen:const -->

<!-- docgen:doc crate=core-a -->
<!-- /docgen:doc -->

<!-- docgen:doc crate=core-a section=Limits -->
<!-- /docgen:doc -->

<!-- docgen:doc crate=core-a name=limits::Port::HTTP -->
<!-- /docgen:doc -->

<!-- docgen:doc crate=core-a name=net -->
<!-- /docgen:doc -->

<!-- docgen:errors crate=core-a type=errors::Error -->
<!-- /docgen:errors -->

<!-- docgen:errors crate=core-a type=limits::LimitError -->
<!-- /docgen:errors -->
`

func TestRustRead(t *testing.T) {
	t.Parallel()
	needCargo(t)
	root := rustRepo(t, map[string]string{".docgen.yaml": "print: raw\n", "docs/read.md": rustReadPage})
	if rc, log := docgen(t, root, "update"); rc != 0 {
		t.Fatalf("update: rc %d, log:\n%s", rc, log)
	}
	got := page(t, root, "read.md")
	wantIn(t, got,
		"name=limits::SMALL_JOBS -->`MAX_JOBS / 4`<",
		"print=debug -->`2`<",
		"name=never::NEVER -->`0`<",
		"<!-- docgen:doc crate=core-a -->\nThe core of the fixture app.\n\n### Limits\n\nThe app runs at most `limits::MAX_JOBS` jobs at once",
		"### Other\n\nNot copied.\n<!-- /docgen:doc -->",
		"section=Limits -->\nThe app runs at most `limits::MAX_JOBS` jobs at once, and each job has a\ntimeout.\n\n#### Timeouts\n\nA job that runs past its timeout stops.\n<!-- /docgen:doc -->",
		"name=limits::Port::HTTP -->\nThe port of a plain HTTP server.\n<",
		"name=net -->\nNetworking.\n<",
		"| Error | Message | Source | Meaning |\n|---|---|---|---|\n",
		"| `Error::Read` | `read {path}: {source}` | `std::io::Error` | A file could not be read. |\n",
		"| `Error::Number` | `line {line}: bad number` | `ParseIntError` | A number in the config is not valid. |\n",
		"| `Error::Limit` | the message of `crate::limits::LimitError` | `crate::limits::LimitError`, with From | The job count is out of range. |\n",
		"| `Error::NoJobs` | `no jobs` |  | The app has no jobs \\| none at all. |\n",
		"| `LimitError::TooLarge` | `{0} is above the limit` |  | The count is above MAX_JOBS. |\n",
	)
	if rc, log := docgen(t, root, "check"); rc != 0 {
		t.Fatalf("check: rc %d, log:\n%s", rc, log)
	}
}

const rustReadErrorPage = `- <!-- docgen:const crate=nope name=X --><!-- /docgen:const -->
- <!-- docgen:const crate=core-a name=limits::NOPE print=raw --><!-- /docgen:const -->
- <!-- docgen:const crate=core-a name=nomod::X print=raw --><!-- /docgen:const -->
- <!-- docgen:const crate=core-a name=net print=raw --><!-- /docgen:const -->

<!-- docgen:errors crate=core-a type=limits::Port -->
<!-- /docgen:errors -->

<!-- docgen:errors crate=core-a type=limits::MAX_JOBS -->
<!-- /docgen:errors -->

<!-- docgen:configulator crate=core-a type=limits::Port -->
<!-- /docgen:configulator -->

<!-- docgen:configulator crate=core-a type=limits::check_jobs -->
<!-- /docgen:configulator -->

<!-- docgen:doc crate=core-a name=renamed::deeper -->
<!-- /docgen:doc -->

<!-- docgen:doc crate=core-a section=Nope -->
<!-- /docgen:doc -->

<!-- docgen:doc crate=core-a name=limits::Nope -->
<!-- /docgen:doc -->
`

func TestRustReadErrors(t *testing.T) {
	t.Parallel()
	needCargo(t)
	root := rustRepo(t, map[string]string{"docs/errors.md": rustReadErrorPage})
	rc, log := docgen(t, root, "check")
	if rc != 1 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
	wantIn(t, log,
		"errors.md:1: docgen:const crate=nope name=X: crate \"nope\" is not in the Cargo workspace at ",
		"; the crates are: cargo-b-cli, cargo-b-core, core-a, core-a-bin, limits, run-c, run-c-broken, tool",
		"errors.md:2: docgen:const crate=core-a name=limits::NOPE print=raw: crate core-a has no item limits::NOPE; the names in module limits are LISTEN, LimitError, MAX_JOBS,",
		"errors.md:3: docgen:const crate=core-a name=nomod::X print=raw: crate core-a has no item nomod::X; the names in the crate root are config, errors, limits, net, never, renamed, shapes, unix",
		"errors.md:4: docgen:const crate=core-a name=net print=raw: net is a module, not a const or static",
		"errors.md:6: docgen:errors crate=core-a type=limits::Port: limits::Port has no #[derive(thiserror::Error)]",
		"errors.md:9: docgen:errors crate=core-a type=limits::MAX_JOBS: limits::MAX_JOBS is a const, not an enum or struct",
		"errors.md:12: docgen:configulator crate=core-a type=limits::Port: limits::Port has no #[derive(Config)] of configulator-rs",
		"errors.md:15: docgen:configulator crate=core-a type=limits::check_jobs: limits::check_jobs is a fn, not a struct",
		"errors.md:18: docgen:doc crate=core-a name=renamed::deeper: no doc comment for renamed::deeper in crate core-a",
		"errors.md:21: docgen:doc crate=core-a section=Nope: no heading # Nope in the doc of crate core-a",
		"errors.md:24: docgen:doc crate=core-a name=limits::Nope: crate core-a has no item limits::Nope",
	)
}

func TestRustHelperWarnings(t *testing.T) {
	t.Parallel()
	needCargo(t)
	root := rustRepo(t, map[string]string{
		"crates/core-a/src/net/tcp.rs": "//! TCP.\n\n/// The listen backlog.\n#[doc = include_str!(\"missing.md\")]\n#[doc = 3]\npub const BACKLOG: u32 = 128;\n",
		"docs/net.md":                  "<!-- docgen:doc crate=core-a name=net::tcp::BACKLOG -->\n<!-- /docgen:doc -->\n",
	})
	rc, log := docgen(t, root, "update")
	if rc != 0 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
	wantIn(t, log,
		"docgen: crate core-a: crates/core-a/src/net/missing.md: ",
		"docgen: crate core-a: crates/core-a/src/net/tcp.rs:5: a #[doc] value that is not a string: 3\n",
	)
	wantIn(t, page(t, root, "net.md"), "BACKLOG -->\nThe listen backlog.\n<")
}

func TestRustBindErrors(t *testing.T) {
	t.Parallel()
	for marker, want := range map[string]string{
		"<!-- docgen:const crate=core-a pkg=x name=A --><!-- /docgen:const -->":                    "docgen:const has both crate= and pkg=; crate= is for Rust and pkg= is for Go",
		"<!-- docgen:const crate=core-a name=A print=go --><!-- /docgen:const -->":                 "docgen:const print=go is not display, debug, raw, name or a Rust format spec such as {:x}",
		"<!-- docgen:const crate=core-a name=A print=%d --><!-- /docgen:const -->":                 "print=%d is not display",
		"<!-- docgen:const crate=core-a name=A code=yes --><!-- /docgen:const -->":                 "code= can only be false",
		"<!-- docgen:const crate=\"\" name=A --><!-- /docgen:const -->":                            "docgen:const: crate= is required",
		"<!-- docgen:doc crate=core-a pkg=. -->\n<!-- /docgen:doc -->":                             "docgen:doc has both crate= and pkg=",
		"<!-- docgen:errors crate=core-a -->\n<!-- /docgen:errors -->":                             "docgen:errors: type= is required",
		"<!-- docgen:configulator crate=core-a type=T env=\"\" -->\n<!-- /docgen:configulator -->": "docgen:configulator: env= is empty",
	} {
		root := writeRepo(t, map[string]string{"docs/p.md": marker + "\n"})
		rc, log := docgen(t, root, "check")
		if rc != 1 || !strings.Contains(log, want) {
			t.Errorf("%s: rc %d, log:\n%s\nwant %q", marker, rc, log, want)
		}
	}
}
