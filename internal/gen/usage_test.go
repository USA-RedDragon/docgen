package gen

import (
	"slices"
	"testing"
)

func TestSplitArgs(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		value string
		want  []string
	}{
		{"-h", []string{"-h"}},
		{"help config", []string{"help", "config"}},
		{"  --help  ", []string{"--help"}},
		{`--name "a b" x`, []string{"--name", "a b", "x"}},
		{`a"b c"d`, []string{"ab cd"}},
		{`""`, []string{""}},
	} {
		got, err := splitArgs(tc.value)
		if err != nil || !slices.Equal(got, tc.want) {
			t.Errorf("splitArgs(%q) = %q, %v, want %q", tc.value, got, err, tc.want)
		}
	}
	for _, value := range []string{"   ", `a "b`} {
		if got, err := splitArgs(value); err == nil {
			t.Errorf("splitArgs(%q) = %q, want an error", value, got)
		}
	}
}
