package excerpt

import (
	"strings"
	"testing"
)

const cTypesSrc = `#include <stdint.h>

/* A point. */
struct point {
	int x, y;
};

typedef struct point point_t;

typedef struct {
	int w;
	struct { int a; } inner;
} anon_t;

typedef struct node {
	struct node *next;
#ifdef DEBUG
	int id;
#endif
	/* } */
	const char *s; // {
} node_t, *node_ptr;

union value {
	int i;
	float f;
};

enum color { RED, GREEN = (1 + 2), BLUE };

typedef enum { OFF, ON } state_t;

typedef int (*handler_fn)(int sig, void (*next)(int));

typedef void fn_type(int);

typedef unsigned long u64;

typedef int matrix_t[3][3];

struct __attribute__((packed)) packed_s {
	uint8_t a;
	uint32_t b;
};

typedef struct __attribute__((packed)) {
	uint8_t a;
} packed_t;

typedef struct { int v; } __attribute__((aligned(8))) aligned_t;

struct fwd;
typedef struct fwd fwd_t;

struct both { int a; };
typedef struct both_other { int a; } both;

struct make_s { int a; } make(void) { struct make_s m = {1}; return m; }

struct with_var { int a; } global_var = { 1 };

typedef struct lone { int a; } lone_t;
struct lone_user *use(void);

int fn_and_type(void) {
	return 0;
}
typedef int fn_and_type;

#define MACRO_AND_TYPE 1
typedef int MACRO_AND_TYPE;

#ifdef A
typedef int twice;
#else
typedef long twice;
#endif

struct a { int x; }; struct b { int y; };

#ifdef __cplusplus
extern "C" {
#endif

typedef struct in_extern { int z; } in_extern_t;

#ifdef __cplusplus
}
#endif

static const struct qualified {
	int q;
} qualified_val = { 1 };
`

func TestCTypes(t *testing.T) {
	t.Parallel()
	src := []byte(cTypesSrc)
	for _, c := range []struct{ sym, want string }{
		{"struct point", "struct point {\n\tint x, y;\n};"},
		{"point_t", "typedef struct point point_t;"},
		{"anon_t", "typedef struct {\n\tint w;\n\tstruct { int a; } inner;\n} anon_t;"},
		{"struct node", "typedef struct node {\n\tstruct node *next;\n#ifdef DEBUG\n\tint id;\n#endif\n\t/* } */\n\tconst char *s; // {\n} node_t, *node_ptr;"},
		{"node_t", "typedef struct node {\n\tstruct node *next;\n#ifdef DEBUG\n\tint id;\n#endif\n\t/* } */\n\tconst char *s; // {\n} node_t, *node_ptr;"},
		{"node_ptr", "typedef struct node {\n\tstruct node *next;\n#ifdef DEBUG\n\tint id;\n#endif\n\t/* } */\n\tconst char *s; // {\n} node_t, *node_ptr;"},
		{"union value", "union value {\n\tint i;\n\tfloat f;\n};"},
		{"enum   color", "enum color { RED, GREEN = (1 + 2), BLUE };"},
		{"state_t", "typedef enum { OFF, ON } state_t;"},
		{"handler_fn", "typedef int (*handler_fn)(int sig, void (*next)(int));"},
		{"fn_type", "typedef void fn_type(int);"},
		{"u64", "typedef unsigned long u64;"},
		{"matrix_t", "typedef int matrix_t[3][3];"},
		{"struct packed_s", "struct __attribute__((packed)) packed_s {\n\tuint8_t a;\n\tuint32_t b;\n};"},
		{"packed_t", "typedef struct __attribute__((packed)) {\n\tuint8_t a;\n} packed_t;"},
		{"aligned_t", "typedef struct { int v; } __attribute__((aligned(8))) aligned_t;"},
		{"fwd_t", "typedef struct fwd fwd_t;"},
		{"struct both", "struct both { int a; };"},
		{"both", "typedef struct both_other { int a; } both;"},
		{"struct both_other", "typedef struct both_other { int a; } both;"},
		{"struct with_var", "struct with_var { int a; } global_var = { 1 };"},
		{"struct lone", "typedef struct lone { int a; } lone_t;"},
		{"lone_t", "typedef struct lone { int a; } lone_t;"},
		{"in_extern_t", "typedef struct in_extern { int z; } in_extern_t;"},
		{"struct qualified", "static const struct qualified {\n\tint q;\n} qualified_val = { 1 };"},
		{"make", "struct make_s { int a; } make(void) { struct make_s m = {1}; return m; }"},
	} {
		got, err := cExcerpt(src, c.sym)
		if err != nil {
			t.Errorf("%s: %v", c.sym, err)
			continue
		}
		if got != c.want {
			t.Errorf("%s = %q, want %q", c.sym, got, c.want)
		}
	}
}

func TestCTypeErrors(t *testing.T) {
	t.Parallel()
	src := []byte(cTypesSrc)
	for _, c := range []struct{ sym, want string }{
		{"point", "use symbol=\"struct point\""},
		{"color", "use symbol=\"enum color\""},
		{"struct fwd", "no definition of struct fwd"},
		{"struct missing", "no definition of struct missing"},
		{"union point", "no definition of union point"},
		{"enum point", "no definition of enum point"},
		{"struct make_s", "no definition of struct make_s"},
		{"struct lone_user", "no definition of struct lone_user"},
		{"inner", "no #define, function or typedef inner"},
		{"struct inner", "no definition of struct inner"},
		{"fn_and_type", "more than one definition"},
		{"MACRO_AND_TYPE", "#define at line"},
		{"twice", "typedef at line"},
		{"struct a", "shares a line with other code"},
		{"struct b", "shares a line with other code"},
		{"struct", "is a name, or struct, union or enum and a tag"},
		{"struct a b", "is a name, or struct, union or enum and a tag"},
		{"typedef point_t", "is a name, or struct, union or enum and a tag"},
		{"struct 1a", "is a name, or struct, union or enum and a tag"},
		{"", "is a name, or struct, union or enum and a tag"},
	} {
		got, err := cExcerpt(src, c.sym)
		if err == nil {
			t.Errorf("%q = %q, want an error", c.sym, got)
			continue
		}
		if !strings.Contains(err.Error(), c.want) {
			t.Errorf("%q: error %q, want %q", c.sym, err, c.want)
		}
	}
}

func TestCTypeUnsafe(t *testing.T) {
	t.Parallel()
	for name, src := range map[string]string{
		"open struct":  "struct s {\n\tint a;\n",
		"no semicolon": "typedef int x\n",
		"stray brace":  "} typedef int y;\n",
	} {
		sym := "struct s"
		if name != "open struct" {
			sym = "x"
		}
		if got, err := cExcerpt([]byte(src), sym); err == nil {
			t.Errorf("%s = %q, want an error", name, got)
		}
	}
}

func TestCTypesInHeader(t *testing.T) {
	t.Parallel()
	src := []byte("#ifndef H\n#define H\ntypedef struct h {\n\tint a;\n} h_t;\n#endif\n")
	for _, path := range []string{"h.h", "h.c"} {
		got, err := Symbol(path, src, "h_t")
		if err != nil || got != "typedef struct h {\n\tint a;\n} h_t;" {
			t.Errorf("%s = %q, %v", path, got, err)
		}
		raw, err := RawSymbol(path, src, "struct h")
		if err != nil || raw != got {
			t.Errorf("raw %s = %q, %v", path, raw, err)
		}
	}
}

func TestCTypesLeaveOutTheDocComment(t *testing.T) {
	t.Parallel()
	src := []byte("/* Doc. */\n// More.\nstruct s {\n\tint a;\n};\n")
	for _, f := range []func(string, []byte, string) (string, error){Symbol, RawSymbol} {
		got, err := f("s.c", src, "struct s")
		if err != nil || got != "struct s {\n\tint a;\n};" {
			t.Errorf("got %q, %v", got, err)
		}
	}
}

func TestCTypesCommentOnTheLastLine(t *testing.T) {
	t.Parallel()
	src := []byte("struct s {\n\tint a;\n}; /* end */\n")
	got, err := cExcerpt(src, "struct s")
	if err != nil || got != "struct s {\n\tint a;\n}; /* end */" {
		t.Errorf("got %q, %v", got, err)
	}
}
