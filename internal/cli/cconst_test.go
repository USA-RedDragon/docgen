package cli

import (
	"path/filepath"
	"strings"
	"testing"
)

const cConstHeader = `#define PORT 4001
#define FLAGS (1 << 3)
#define NAME "lamp" /* the name */
#define MASK \
	(FLAGS | \
	 0x10)
#define SIZE 0x10UL
#define OCT 017u
#define BIN 0b101u
#define SEP 1'000'000ull
#define PAREN (10u)
#define CALL(x) ((x) + 1)
#define FLAG
#ifdef DEBUG
#define LEVEL 1
#else
#define LEVEL 2
#endif
`

func cConstMarker(args string) string {
	return "<!-- docgen:const file=native/limits.h " + args + " --><!-- /docgen:const -->\n"
}

func TestCConst(t *testing.T) {
	t.Parallel()
	page := strings.Join([]string{
		cConstMarker("name=PORT"),
		cConstMarker("name=FLAGS"),
		cConstMarker("name=NAME"),
		cConstMarker("name=MASK"),
		cConstMarker("name=MASK code=false"),
	}, "")
	root := pageRepo(t, map[string]string{"native/limits.h": cConstHeader, "docs/p.md": page})
	wantIn(t, updateTwice(t, root, "docs/p.md"),
		"name=PORT -->`4001`<", "name=FLAGS -->`(1 << 3)`<", "name=NAME -->`\"lamp\"`<",
		"name=MASK -->`(FLAGS | 0x10)`<", "code=false -->(FLAGS | 0x10)<")
}

func TestCConstPrintValue(t *testing.T) {
	t.Parallel()
	page := strings.Join([]string{
		cConstMarker("name=SIZE print=value"),
		cConstMarker("name=OCT print=value"),
		cConstMarker("name=BIN print=value code=false"),
		cConstMarker("name=SEP print=value"),
		cConstMarker("name=PORT print=value"),
		cConstMarker("name=SIZE"),
	}, "")
	root := pageRepo(t, map[string]string{"native/limits.h": cConstHeader, "docs/p.md": page})
	wantIn(t, updateTwice(t, root, "docs/p.md"),
		"name=SIZE print=value -->`0x10`<", "name=OCT print=value -->`017`<", "print=value code=false -->0b101<",
		"name=SEP print=value -->`1'000'000`<", "name=PORT print=value -->`4001`<", "name=SIZE -->`0x10UL`<")
}

func TestCConstPrintValueNoCargo(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	root := pageRepo(t, map[string]string{"native/limits.h": cConstHeader, "docs/p.md": cConstMarker("name=SIZE print=value")})
	wantIn(t, updateTwice(t, root, "docs/p.md"), "`0x10`")
}

func TestCConstNoCargo(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	root := pageRepo(t, map[string]string{"native/limits.h": cConstHeader, "docs/p.md": cConstMarker("name=PORT")})
	wantIn(t, updateTwice(t, root, "docs/p.md"), "`4001`")
}

func TestCConstErrors(t *testing.T) {
	t.Parallel()
	for name, c := range map[string]struct{ marker, want string }{
		"function-like":  {cConstMarker("name=CALL"), "CALL is a function-like macro"},
		"empty":          {cConstMarker("name=FLAG"), "the #define FLAG at line 13 has no replacement text"},
		"not found":      {cConstMarker("name=NOPE"), "no #define NOPE in the C file; its #defines are: BIN, CALL, FLAG, FLAGS, LEVEL, MASK, NAME, OCT, PAREN, PORT, SEP, SIZE"},
		"ambiguous":      {cConstMarker("name=LEVEL"), "LEVEL has more than one definition in the C file: #define at line 15, #define at line 17"},
		"not C":          {"<!-- docgen:const file=native/limits.txt name=PORT --><!-- /docgen:const -->\n", "file= is not a .c or .h file"},
		"with pkg":       {"<!-- docgen:const file=native/limits.h pkg=. name=PORT --><!-- /docgen:const -->\n", "has both file= and pkg="},
		"with crate":     {"<!-- docgen:const file=native/limits.h crate=a name=PORT --><!-- /docgen:const -->\n", "has both file= and crate="},
		"print":          {cConstMarker("name=PORT print=raw"), "print=raw does not apply to a C #define; the one value is print=value"},
		"value of expr":  {cConstMarker("name=FLAGS print=value"), `limits.h: FLAGS: print=value: the replacement text "(1 << 3)" is not one integer literal`},
		"value of str":   {cConstMarker("name=NAME print=value"), `NAME: print=value: the replacement text "\"lamp\"" is not one integer literal`},
		"value of paren": {cConstMarker("name=PAREN print=value"), `PAREN: print=value: the replacement text "(10u)" is not one integer literal`},
		"value of name":  {cConstMarker("name=MASK print=value"), "MASK: print=value: the replacement text"},
		"missing file":   {"<!-- docgen:const file=native/none.h name=PORT --><!-- /docgen:const -->\n", "none.h"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := pageRepo(t, map[string]string{"native/limits.h": cConstHeader, "docs/p.md": c.marker})
			rc, log := docgen(t, root, "check")
			if rc != 1 {
				t.Fatalf("rc %d, log:\n%s", rc, log)
			}
			wantIn(t, log, c.want)
		})
	}
}

func TestCConstOutsideTheRepository(t *testing.T) {
	t.Parallel()
	root, outside := outsideRepo(t, map[string]string{"docs/p.md": cConstMarker("name=PORT")}, map[string]string{"limits.h": "#define PORT " + leak + "\n"})
	link(t, filepath.Join(outside, "limits.h"), filepath.Join(root, "native/limits.h"))
	wantRefused(t, root, "outside the repository")
}
