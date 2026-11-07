package gen

import (
	"testing"
)

func TestHasDefaultPhrase(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]bool{
		"0 means no limit.":          true,
		"Nil means the default.":     true,
		"An empty string means all.": true,
		"A zero value means off.":    true,
		"zero\nmeans off":            true,
		"It defaults to 5.":          true,
		"On by default.":             true,
		"The limit.":                 false,
		"10 means ten.":              false,
	} {
		if got := hasDefaultPhrase(in); got != want {
			t.Errorf("hasDefaultPhrase(%q) = %v, want %v", in, got, want)
		}
	}
}
