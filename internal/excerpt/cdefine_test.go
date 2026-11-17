package excerpt

import "testing"

const cDefineSrc = `#define PLAIN 42
#define EXPR (1 << 3)
#  define SPACED   "a  b"  
#define COMMENT 7 /* seven */ // trailing
#define BLOCK 1 /* a \
  b */ + 2
#define LONG \
	(A | \
	 B)   \

#define TEXT "/* not a comment */"
#define FN(x) ((x) + 1)
#define SPACE_PAREN (x)
#define EMPTY
#define EMPTY_COMMENT /* nothing */
#ifdef DEBUG
#define BRANCH 1
#else
#define BRANCH 2
#endif
// #define COMMENTED 1
`

func TestCDefineValue(t *testing.T) {
	t.Parallel()
	for name, want := range map[string]string{
		"PLAIN":       "42",
		"EXPR":        "(1 << 3)",
		"SPACED":      `"a  b"`,
		"COMMENT":     "7",
		"BLOCK":       "1   + 2",
		"LONG":        "(A | B)",
		"TEXT":        `"/* not a comment */"`,
		"SPACE_PAREN": "(x)",
	} {
		got, err := CDefineValue([]byte(cDefineSrc), name)
		if err != nil || got != want {
			t.Errorf("%s: got %q, %v, want %q", name, got, err, want)
		}
	}
}

func TestCDefineValueErrors(t *testing.T) {
	t.Parallel()
	for name, want := range map[string]string{
		"FN":            "FN is a function-like macro; docgen:const gives the replacement text of an object-like #define",
		"EMPTY":         "the #define EMPTY at line 14 has no replacement text",
		"EMPTY_COMMENT": "the #define EMPTY_COMMENT at line 15 has no replacement text",
		"BRANCH":        "BRANCH has more than one definition in the C file: #define at line 17, #define at line 19",
		"COMMENTED":     "no #define COMMENTED in the C file; its #defines are: BLOCK, BRANCH, COMMENT, EMPTY, EMPTY_COMMENT, EXPR, FN, LONG, PLAIN, SPACED, SPACE_PAREN, TEXT",
		"a b":           `name="a b" is not a C identifier`,
	} {
		_, err := CDefineValue([]byte(cDefineSrc), name)
		if err == nil || err.Error() != want {
			t.Errorf("%s: got %v, want %q", name, err, want)
		}
	}
	if _, err := CDefineValue([]byte("int x;\n"), "X"); err == nil || err.Error() != "no #define X in the C file, which has no #define" {
		t.Errorf("no defines: %v", err)
	}
}
