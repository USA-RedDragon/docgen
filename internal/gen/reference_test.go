package gen

import (
	"testing"
)

func TestTableOf(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{
		"# Config\n\n| A |\n|---|\n": "| A |\n|---|\n",
		"# Config\n| A |\n":          "| A |\n",
		"| A |\n|---|\n":             "| A |\n|---|\n",
		"# Config\n\nno table\n":     "# Config\n\nno table\n",
		"text\n| A |\n":              "text\n| A |\n",
	} {
		if got := tableOf(in); got != want {
			t.Errorf("tableOf(%q) = %q, want %q", in, got, want)
		}
	}
}
