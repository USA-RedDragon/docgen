package gen

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestScanSkips(t *testing.T) {
	t.Parallel()
	page := modMarker + "\n"
	files := map[string]string{"go.mod": goModText, "outside.md": page}
	for _, p := range []string{"_docs/kept.md", "_docs/sub/kept.md", "_docs/not.txt", "_docs/testdata/p.md", "_docs/vendor/p.md", "_docs/node_modules/p.md", "_docs/.hidden/p.md", "_docs/_private/p.md"} {
		files[p] = page
	}
	root, g := setup(t, files)
	if err := os.Symlink(filepath.Join(root, "outside.md"), filepath.Join(root, "_docs/link.md")); err != nil {
		t.Fatal(err)
	}
	stale, err := g.Run(t.Context(), filepath.Join(root, "_docs"), false)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"_docs/kept.md:1: docgen:gomod field=module", "_docs/sub/kept.md:1: docgen:gomod field=module"}
	if !slices.Equal(stale, want) {
		t.Errorf("stale = %q, want %q", stale, want)
	}
}

func TestPageLineEndings(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name, page, want string
	}{
		{"crlf keeps crlf", "a\r\nb\r\n" + modMarker + "\r\nc\r\n", "a\r\nb\r\n" + modFilled + "\r\nc\r\n"},
		{"lf keeps lf", "a\nb\n" + modMarker + "\nc\n", "a\nb\n" + modFilled + "\nc\n"},
		{"mixed keeps each line", "a\r\nb\n" + modMarker + "\nc\nd\n", "a\r\nb\n" + modFilled + "\nc\nd\n"},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			root, g := setup(t, map[string]string{"go.mod": goModText, "docs/p.md": c.page})
			if _, err := g.Run(t.Context(), filepath.Join(root, "docs"), true); err != nil {
				t.Fatal(err)
			}
			if got := read(t, root, "docs/p.md"); got != c.want {
				t.Errorf("page = %q, want %q", got, c.want)
			}
		})
	}
}

func TestBlockBodyTakesPageLineEnding(t *testing.T) {
	t.Parallel()
	open := "<!-- docgen:code file=a.txt -->"
	shut := "<!-- /docgen:code -->"
	root, g := setup(t, map[string]string{
		"go.mod":    goModText,
		"a.txt":     "one\ntwo\n",
		"docs/p.md": open + "\r\n" + shut + "\r\ntext\r\n",
	})
	if _, err := g.Run(t.Context(), filepath.Join(root, "docs"), true); err != nil {
		t.Fatal(err)
	}
	want := open + "\r\n```\r\none\r\ntwo\r\n```\r\n" + shut + "\r\ntext\r\n"
	if got := read(t, root, "docs/p.md"); got != want {
		t.Errorf("page = %q, want %q", got, want)
	}
}

func TestSkipDirPages(t *testing.T) {
	t.Parallel()
	for name, want := range map[string]bool{
		"testdata": true, "vendor": true, "node_modules": true, ".git": true, "_site": true,
		"guides": false, "node_modules2": false,
	} {
		if got := skipDir(name); got != want {
			t.Errorf("skipDir(%q) = %v, want %v", name, got, want)
		}
	}
}
