package gen

import "testing"

func TestExpand(t *testing.T) {
	t.Parallel()
	vars := map[string]string{"v": "1.0", "a-b_2": "x", "brace": "{v}", "nl": `a\nb`}
	for _, tc := range []struct{ template, want string }{
		{"plain", "plain"},
		{"go@{v} run", "go@1.0 run"},
		{"{v}{a-b_2}", "1.0x"},
		{"{{v}", "{v}"},
		{"{{{v}", "{1.0"},
		{"a}", "a}"},
		{`one\ntwo`, "one\ntwo"},
		{"{brace}", "{v}"},
		{"{nl}", `a\nb`},
		{`a\\nb`, `a\nb`},
		{`a\\b`, `a\b`},
		{`a\\`, `a\`},
		{`a\\\nb`, "a\\\nb"},
		{`a\b`, `a\b`},
		{`\\{v}`, `\1.0`},
	} {
		parts, err := parseTemplate(tc.template)
		if err != nil {
			t.Errorf("%q: %v", tc.template, err)
			continue
		}
		if got, err := expand(parts, vars); err != nil || got != tc.want {
			t.Errorf("%q: got %q, %v, want %q", tc.template, got, err, tc.want)
		}
	}
}

func TestExpandUnknownVar(t *testing.T) {
	t.Parallel()
	parts, err := parseTemplate("{one} {two}")
	if err != nil {
		t.Fatal(err)
	}
	want := "var two is not in the vars setting"
	if _, err := expand(parts, map[string]string{"one": "1"}); err == nil || err.Error() != want {
		t.Errorf("got %v, want %q", err, want)
	}
}

func TestParseTemplateErrors(t *testing.T) {
	t.Parallel()
	for template, want := range map[string]string{
		"a{b": "has a { with no }; {{ writes a literal {",
		"{}":  "has a {} with no name",
	} {
		if _, err := parseTemplate(template); err == nil || err.Error() != want {
			t.Errorf("%q: got %v, want %q", template, err, want)
		}
	}
}
