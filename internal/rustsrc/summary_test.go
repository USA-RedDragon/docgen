package rustsrc

import "testing"

func TestCrateSummary(t *testing.T) {
	t.Parallel()
	for name, c := range map[string]struct{ src, want string }{
		"first line":      {"//! A store.\n//!\n//! More text.\n", "A store."},
		"no cut":          {"//! Use e.g. a store. And more.\n", "Use e.g. a store. And more."},
		"heading":         {"//! # My crate\n//!\n//! Text.\n", "My crate"},
		"blank first":     {"//!\n//! After a blank.\n", "After a blank."},
		"before":          {"\n// plain\n#![forbid(unsafe_code)]\n//! Documented.\n", "Documented."},
		"no indent":       {"  //!   Spaced  \n", "Spaced"},
		"item first":      {"use a;\n//! Late.\n", ""},
		"outer doc":       {"/// An item.\nfn f() {}\n", ""},
		"block doc":       {"/*! Block. */\n", ""},
		"fence":           {"//! ```\n//! code\n//! ```\n", ""},
		"empty":           {"", ""},
		"only blank docs": {"//!\n//!\n", ""},
	} {
		if got := CrateSummary([]byte(c.src)); got != c.want {
			t.Errorf("%s: got %q, want %q", name, got, c.want)
		}
	}
}
