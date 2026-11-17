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

func TestCIntegerValue(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{
		"42": "42", "42u": "42", "42U": "42", "42l": "42", "42LL": "42", "42ul": "42", "42LU": "42",
		"42ull": "42", "42LLU": "42", "42uLL": "42", "42z": "42", "42UZ": "42", "42zu": "42", "42wb": "42", "42uwb": "42",
		"0x10u": "0x10", "0XFFul": "0XFF", "0x1b": "0x1b", "0b101u": "0b101", "0B11": "0B11",
		"0": "0", "017": "017", "017u": "017", "1'000'000ul": "1'000'000", "0x1F'FFu": "0x1F'FF", "0b1'0": "0b1'0",
	} {
		got, err := CIntegerValue(in)
		if err != nil || got != want {
			t.Errorf("%s: got %q, %v, want %q", in, got, err, want)
		}
	}
}

func TestCIntegerValueErrors(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{
		"(10u)":    `the replacement text "(10u)" is not one integer literal`,
		"-1":       `the replacement text "-1" is not one integer literal`,
		"1 << 3":   `the replacement text "1 << 3" is not one integer literal`,
		"1.5":      `the replacement text "1.5" is not one integer literal`,
		"1e3":      `the replacement text "1e3" is not one integer literal`,
		`"a"`:      `the replacement text "\"a\"" is not one integer literal`,
		"A":        `the replacement text "A" is not one integer literal`,
		"1'":       `the replacement text "1'" is not one integer literal`,
		"0x":       `the integer literal "0x" has the suffix "x", which C does not have`,
		"10uu":     `the integer literal "10uu" has the suffix "uu", which C does not have`,
		"10lL":     `the integer literal "10lL" has the suffix "lL", which C does not have`,
		"10lll":    `the integer literal "10lll" has the suffix "lll", which C does not have`,
		"10zl":     `the integer literal "10zl" has the suffix "zl", which C does not have`,
		"10f":      `the integer literal "10f" has the suffix "f", which C does not have`,
		"0b":       `the integer literal "0b" has the suffix "b", which C does not have`,
		"09":       `the octal literal "09" has a digit that is not octal`,
		"0x10wbwb": `the integer literal "0x10wbwb" has the suffix "wbwb", which C does not have`,
	} {
		_, err := CIntegerValue(in)
		if err == nil || err.Error() != want {
			t.Errorf("%s: got %v, want %q", in, err, want)
		}
	}
}
