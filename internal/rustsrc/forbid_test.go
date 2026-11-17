package rustsrc

import (
	"os"
	"path/filepath"
	"testing"
)

func TestForbidError(t *testing.T) {
	t.Parallel()
	now := "error[E0453]: allow(dead_code) incompatible with previous forbid\n" +
		" --> src/door.rs:7:47\n" +
		"  |\n" +
		"7 |     pub fn f(&mut self) { #![allow(dead_code, missing_docs)] let _ = 1; }\n" +
		"  |                                    ^^^^^^^^^ overruled by previous forbid\n" +
		"  |\n" +
		" ::: src/lib.rs:2:11\n" +
		"  |\n" +
		"2 | #![forbid(unused, missing_docs, unsafe...\n" +
		"  |           ------ `forbid` level set here"
	old := "error[E0453]: allow(missing_docs) overruled by outer forbid(missing_docs)\n" +
		" --> src/lib.rs:7:47\n" +
		"  |\n" +
		"7 |     pub fn f(&mut self) { #![allow(missing_docs)] let _ = 1; }\n" +
		"  |                                    ^^^^^^^^^^^^ overruled by outer forbid(missing_docs)"
	work := t.TempDir()
	if err := os.MkdirAll(filepath.Join(work, "src"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(work, "src/lib.rs"), []byte("//! Doors.\n#![forbid(unused, missing_docs, unsafe_code)]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	err := forbidError("door", work, []string{"error: other", now, now, old})
	if err == nil {
		t.Fatal("no error")
	}
	want := "crate door forbids a lint that the state hook allows in the code that it adds, and a forbid admits no allow, so no method with the hook builds:\n" +
		"  - #![forbid(unused, missing_docs, unsafe_code)] at src/lib.rs:2 forbids unused\n" +
		"  - forbid(missing_docs), from an attribute or from -F in RUSTFLAGS\n" +
		"Use deny in place of forbid, such as #![deny(unused)]: a deny keeps the lint an error and admits the allow of the hook"
	if err.Error() != want {
		t.Errorf("error:\n%s\nwant:\n%s", err, want)
	}
	if err := forbidError("door", work, []string{"error[E0308]: mismatched types"}); err != nil {
		t.Errorf("error with no E0453: %v", err)
	}
}
