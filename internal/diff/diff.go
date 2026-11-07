// Package diff writes the difference of two texts as a unified diff.
package diff

import (
	"fmt"
	"strings"
)

const context = 3

// maxEdits limits the insertions and deletions of an edit script of myers. Its
// trace takes 8*maxEdits*maxEdits bytes at most.
const maxEdits = 2048

type op struct {
	kind byte
	line string
}

// Unified returns the unified diff of before and after, with three lines of context
// and path in the --- and +++ lines, or "" when they are the same.
func Unified(path string, before, after []byte) string {
	if string(before) == string(after) {
		return ""
	}
	ops := diff(split(before), split(after))
	var b strings.Builder
	fmt.Fprintf(&b, "--- %s\n+++ %s\n", path, path)
	a, c := 1, 1
	for i := 0; i < len(ops); {
		if ops[i].kind == ' ' {
			a, c = a+1, c+1
			i++
			continue
		}
		start := max(i-context, 0)
		for start < i && ops[start].kind != ' ' {
			start++
		}
		end := hunkEnd(ops, i)
		writeHunk(&b, ops[start:end], a-(i-start), c-(i-start))
		for _, o := range ops[i:end] {
			if o.kind != '+' {
				a++
			}
			if o.kind != '-' {
				c++
			}
		}
		i = end
	}
	return b.String()
}

// hunkEnd returns the index after the hunk with the change at index i. Changes
// with at most twice the context between them share a hunk.
func hunkEnd(ops []op, i int) int {
	last := i
	for j := i; j < len(ops) && j-last <= 2*context; j++ {
		if ops[j].kind != ' ' {
			last = j
		}
	}
	return min(last+1+context, len(ops))
}

func writeHunk(b *strings.Builder, ops []op, a, c int) {
	na, nc := 0, 0
	for _, o := range ops {
		if o.kind != '+' {
			na++
		}
		if o.kind != '-' {
			nc++
		}
	}
	fmt.Fprintf(b, "@@ -%s +%s @@\n", span(a, na), span(c, nc))
	for _, o := range ops {
		b.WriteByte(o.kind)
		b.WriteString(o.line)
		if !strings.HasSuffix(o.line, "\n") {
			b.WriteString("\n\\ No newline at end of file\n")
		}
	}
}

// span returns the range of a hunk header. An empty range is the line before
// it.
func span(start, n int) string {
	switch n {
	case 0:
		return fmt.Sprintf("%d,0", start-1)
	case 1:
		return fmt.Sprint(start)
	}
	return fmt.Sprintf("%d,%d", start, n)
}

// split returns the lines of b, each with its line ending.
func split(b []byte) []string {
	if len(b) == 0 {
		return nil
	}
	lines := strings.SplitAfter(string(b), "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// diff returns the edit script from a to b, with each deletion before the
// insertion that replaces it.
func diff(a, b []string) []op {
	pre := 0
	for pre < len(a) && pre < len(b) && a[pre] == b[pre] {
		pre++
	}
	suf := 0
	for suf < len(a)-pre && suf < len(b)-pre && a[len(a)-1-suf] == b[len(b)-1-suf] {
		suf++
	}
	ops := make([]op, 0, len(a)+len(b))
	for _, l := range a[:pre] {
		ops = append(ops, op{' ', l})
	}
	ops = append(ops, myers(a[pre:len(a)-suf], b[pre:len(b)-suf])...)
	for _, l := range a[len(a)-suf:] {
		ops = append(ops, op{' ', l})
	}
	return ops
}

// myers returns a shortest edit script from a to b, by the greedy algorithm of
// Myers. trace holds the furthest point of each diagonal after each step. Above
// maxEdits edits, the script deletes all of a and inserts all of b.
func myers(a, b []string) []op {
	n, m := len(a), len(b)
	v := make([]int, 2*(n+m)+3)
	off := n + m + 1
	var trace [][]int
	for d := 0; d <= min(n+m, maxEdits); d++ {
		trace = append(trace, append([]int(nil), v[off-d:off+d+1]...))
		for k := -d; k <= d; k += 2 {
			x := v[off+k-1] + 1
			if k == -d || k != d && v[off+k-1] < v[off+k+1] {
				x = v[off+k+1]
			}
			y := x - k
			for x < n && y < m && a[x] == b[y] {
				x, y = x+1, y+1
			}
			v[off+k] = x
			if x >= n && y >= m {
				return backtrack(a, b, trace)
			}
		}
	}
	return replace(a, b)
}

// replace returns the edit script that deletes a and inserts b.
func replace(a, b []string) []op {
	ops := make([]op, 0, len(a)+len(b))
	for _, l := range a {
		ops = append(ops, op{'-', l})
	}
	for _, l := range b {
		ops = append(ops, op{'+', l})
	}
	return ops
}

// backtrack follows trace from the end of a and b to the start.
func backtrack(a, b []string, trace [][]int) []op {
	var rev []op
	x, y := len(a), len(b)
	for d := len(trace) - 1; d >= 0; d-- {
		v := trace[d]
		at := func(k int) int { return v[k+d] }
		k := x - y
		prev := k - 1
		if k == -d || k != d && at(k-1) < at(k+1) {
			prev = k + 1
		}
		px := 0
		if d > 0 {
			px = at(prev)
		}
		py := px - prev
		for x > px && y > py && x > 0 && y > 0 && a[x-1] == b[y-1] {
			x, y = x-1, y-1
			rev = append(rev, op{' ', a[x]})
		}
		if d == 0 {
			break
		}
		if x == px {
			y--
			rev = append(rev, op{'+', b[y]})
		} else {
			x--
			rev = append(rev, op{'-', a[x]})
		}
	}
	out := make([]op, 0, len(rev))
	for i := len(rev) - 1; i >= 0; i-- {
		out = append(out, rev[i])
	}
	return order(out)
}

// order moves each deletion of a run of changes before its insertions.
func order(ops []op) []op {
	for i := 0; i < len(ops); {
		if ops[i].kind == ' ' {
			i++
			continue
		}
		j := i
		for j < len(ops) && ops[j].kind != ' ' {
			j++
		}
		var del, ins []op
		for _, o := range ops[i:j] {
			if o.kind == '-' {
				del = append(del, o)
			} else {
				ins = append(ins, o)
			}
		}
		copy(ops[i:], append(del, ins...))
		i = j
	}
	return ops
}
