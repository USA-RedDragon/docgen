package gen

import (
	"slices"
	"testing"
)

func TestDocWords(t *testing.T) {
	t.Parallel()
	got := docWords("Open returns ErrClosed, or transport.ErrBusy; (err_2)... Done.")
	want := []string{"Open", "returns", "ErrClosed", "or", "transport.ErrBusy", "err_2", "Done"}
	if !slices.Equal(got, want) {
		t.Errorf("docWords = %q, want %q", got, want)
	}
}

func TestFirstSentence(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{
		"One. Two.":                        "One.",
		"One\ntwo. Three.":                 "One two.",
		"No stop":                          "No stop",
		"Ends here.":                       "Ends here.",
		"Version 1.2 works.":               "Version 1.2 works.",
		"":                                 "",
		"Use a port, e.g. 80. Then stop.":  "Use a port, e.g. 80.",
		"A name, i.e. a label. More.":      "A name, i.e. a label.",
		"Dogs, cats, etc. are here. More.": "Dogs, cats, etc. are here.",
	} {
		if got := firstSentence(in); got != want {
			t.Errorf("firstSentence(%q) = %q, want %q", in, got, want)
		}
	}
}
