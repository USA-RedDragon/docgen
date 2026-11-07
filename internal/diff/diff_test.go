package diff

import (
	"fmt"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestUnified(t *testing.T) {
	t.Parallel()
	nums := func(from, to int, change map[int]string) string {
		var b strings.Builder
		for i := from; i <= to; i++ {
			if s, ok := change[i]; ok {
				b.WriteString(s)
				continue
			}
			b.WriteByte(byte('a' + i%26))
			b.WriteByte('\n')
		}
		return b.String()
	}
	for name, c := range map[string]struct{ old, new, want string }{
		"same":             {"a\nb\n", "a\nb\n", ""},
		"both empty":       {"", "", ""},
		"new file":         {"", "a\nb\n", "--- f\n+++ f\n@@ -0,0 +1,2 @@\n+a\n+b\n"},
		"emptied":          {"a\nb\n", "", "--- f\n+++ f\n@@ -1,2 +0,0 @@\n-a\n-b\n"},
		"one line":         {"a\n", "b\n", "--- f\n+++ f\n@@ -1 +1 @@\n-a\n+b\n"},
		"insert":           {"a\nc\n", "a\nb\nc\n", "--- f\n+++ f\n@@ -1,2 +1,3 @@\n a\n+b\n c\n"},
		"delete":           {"a\nb\nc\n", "a\nc\n", "--- f\n+++ f\n@@ -1,3 +1,2 @@\n a\n-b\n c\n"},
		"no newline":       {"a", "a\n", "--- f\n+++ f\n@@ -1 +1 @@\n-a\n\\ No newline at end of file\n+a\n"},
		"no newline 2":     {"a\n", "b", "--- f\n+++ f\n@@ -1 +1 @@\n-a\n+b\n\\ No newline at end of file\n"},
		"crlf is a change": {"a\r\n", "a\n", "--- f\n+++ f\n@@ -1 +1 @@\n-a\r\n+a\n"},
		"context": {nums(0, 9, nil), nums(0, 9, map[int]string{5: "X\n"}),
			"--- f\n+++ f\n@@ -3,7 +3,7 @@\n c\n d\n e\n-f\n+X\n g\n h\n i\n"},
		"two hunks": {nums(0, 19, nil), nums(0, 19, map[int]string{2: "X\n", 15: "Y\n"}),
			"--- f\n+++ f\n@@ -1,6 +1,6 @@\n a\n b\n-c\n+X\n d\n e\n f\n@@ -13,7 +13,7 @@\n m\n n\n o\n-p\n+Y\n q\n r\n s\n"},
		"joined hunks": {nums(0, 9, nil), nums(0, 9, map[int]string{2: "X\n", 8: "Y\n"}),
			"--- f\n+++ f\n@@ -1,10 +1,10 @@\n a\n b\n-c\n+X\n d\n e\n f\n g\n h\n-i\n+Y\n j\n"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := Unified("f", []byte(c.old), []byte(c.new)); got != c.want {
				t.Errorf("got:\n%s\nwant:\n%s", got, c.want)
			}
		})
	}
}

// apply rebuilds the new text from the old text and the hunks of a diff.
func apply(t *testing.T, old, diff string) string {
	t.Helper()
	lines := strings.SplitAfter(old, "\n")
	var out strings.Builder
	at := 0
	for l := range strings.Lines(diff) {
		switch {
		case strings.HasPrefix(l, "--- "), strings.HasPrefix(l, "+++ "):
		case strings.HasPrefix(l, "@@ "):
			var a int
			for _, c := range strings.TrimPrefix(l, "@@ -") {
				if c < '0' || c > '9' {
					break
				}
				a = a*10 + int(c-'0')
			}
			for at < a-1 {
				out.WriteString(lines[at])
				at++
			}
		case l[0] == ' ':
			out.WriteString(l[1:])
			at++
		case l[0] == '-':
			at++
		case l[0] == '+':
			out.WriteString(l[1:])
		}
	}
	for ; at < len(lines); at++ {
		out.WriteString(lines[at])
	}
	return out.String()
}

func TestUnifiedRebuildsTheText(t *testing.T) {
	t.Parallel()
	state := uint32(1)
	next := func(n int) int {
		state = state*1664525 + 1013904223
		return int(state>>16) % n
	}
	text := func() string {
		var b strings.Builder
		for range next(40) {
			b.WriteByte(byte('a' + next(4)))
			b.WriteByte('\n')
		}
		return b.String()
	}
	for range 2000 {
		a, b := text(), text()
		if got := apply(t, a, Unified("f", []byte(a), []byte(b))); got != b {
			t.Fatalf("old %q new %q rebuilt %q", a, b, got)
		}
	}
}

func linesOf(n int, prefix string) string {
	var b strings.Builder
	for i := range n {
		fmt.Fprintf(&b, "%s %d\n", prefix, i)
	}
	return b.String()
}

func TestUnifiedBoundsMemory(t *testing.T) {
	t.Parallel()
	before, after := linesOf(30000, "old"), linesOf(30000, "new")
	var start, end runtime.MemStats
	runtime.ReadMemStats(&start)
	begin := time.Now()
	diff := Unified("a", []byte(before), []byte(after))
	took := time.Since(begin)
	runtime.ReadMemStats(&end)
	if alloc := end.TotalAlloc - start.TotalAlloc; alloc > 512<<20 {
		t.Errorf("Unified allocated %d MiB", alloc>>20)
	}
	if took > 20*time.Second {
		t.Errorf("Unified took %v", took)
	}
	if got := apply(t, before, diff); got != after {
		t.Error("the replacement hunk does not rebuild the text")
	}
}

func TestUnifiedReplacementKeepsCommonEnds(t *testing.T) {
	t.Parallel()
	n := maxEdits + 10
	before := "first\n" + linesOf(n, "old") + "last\n"
	after := "first\n" + linesOf(n, "new") + "last\n"
	diff := Unified("a", []byte(before), []byte(after))
	if got := apply(t, before, diff); got != after {
		t.Error("the replacement hunk does not rebuild the text")
	}
	if want := fmt.Sprintf("@@ -1,%d +1,%d @@\n first\n-old 0\n", n+2, n+2); !strings.Contains(diff, want) {
		t.Errorf("no %q in\n%s", want, diff[:80])
	}
}

func TestUnifiedBelowTheEditLimit(t *testing.T) {
	t.Parallel()
	var before, after strings.Builder
	for i := range maxEdits / 4 {
		fmt.Fprintf(&before, "old %d\nsame %d\n", i, i)
		fmt.Fprintf(&after, "new %d\nsame %d\n", i, i)
	}
	diff := Unified("a", []byte(before.String()), []byte(after.String()))
	if got := apply(t, before.String(), diff); got != after.String() {
		t.Error("the diff does not rebuild the text")
	}
	if !strings.Contains(diff, "\n same 1\n") {
		t.Error("the diff replaces the common lines")
	}
}
