package own

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// A subset of POSIX sed: the commands s, d, p, q, =, and { }; line, $ and
// /regexp/ addresses with ranges and !; -n, -e, -E (-r) and -i.

type addr struct {
	kind byte // 0 none, 'n' line, '$' last, 'r' regexp
	line int
	re   *regexp.Regexp
}

type sedCmd struct {
	a1, a2  addr
	neg     bool
	name    byte
	re      *regexp.Regexp
	repl    string
	global  bool
	nth     int
	print   bool
	block   []*sedCmd
	inRange bool
}

type sedParser struct {
	s   string
	i   int
	ere bool
}

func (p *sedParser) errf(f string, a ...any) {
	fatal(fmt.Sprintf("sed: -e expression #1, char %d: ", p.i) + fmt.Sprintf(f, a...))
}

// breToGo converts a basic or extended regular expression to Go syntax.
func breToGo(s string, ere bool) string {
	if ere {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '\\' && i+1 < len(s) {
			n := s[i+1]
			i++
			switch n {
			case '(', ')', '{', '}', '+', '?', '|':
				b.WriteByte(n)
			default:
				b.WriteByte('\\')
				b.WriteByte(n)
			}
			continue
		}
		switch c {
		case '(', ')', '{', '}', '+', '?', '|':
			b.WriteByte('\\')
		}
		b.WriteByte(c)
	}
	return b.String()
}

func (p *sedParser) delimited(d byte) string {
	var b strings.Builder
	for p.i < len(p.s) {
		c := p.s[p.i]
		if c == '\\' && p.i+1 < len(p.s) {
			if p.s[p.i+1] == d {
				b.WriteByte(d)
			} else if p.s[p.i+1] == 'n' {
				b.WriteString("\\n")
			} else {
				b.WriteByte('\\')
				b.WriteByte(p.s[p.i+1])
			}
			p.i += 2
			continue
		}
		if c == d {
			p.i++
			return b.String()
		}
		b.WriteByte(c)
		p.i++
	}
	p.errf("unterminated `s' command")
	return ""
}

func (p *sedParser) compile(re string) *regexp.Regexp {
	r, err := regexp.Compile(breToGo(re, p.ere))
	if err != nil {
		p.errf("%v", err)
	}
	return r
}

func (p *sedParser) skipSpace() {
	for p.i < len(p.s) && (p.s[p.i] == ' ' || p.s[p.i] == '\t') {
		p.i++
	}
}

func (p *sedParser) addr() addr {
	if p.i >= len(p.s) {
		return addr{}
	}
	c := p.s[p.i]
	switch {
	case c >= '0' && c <= '9':
		j := p.i
		for p.i < len(p.s) && p.s[p.i] >= '0' && p.s[p.i] <= '9' {
			p.i++
		}
		n, _ := strconv.Atoi(p.s[j:p.i])
		return addr{kind: 'n', line: n}
	case c == '$':
		p.i++
		return addr{kind: '$'}
	case c == '/' || c == '\\':
		d := byte('/')
		if c == '\\' {
			p.i++
			d = p.s[p.i]
		}
		p.i++
		return addr{kind: 'r', re: p.compile(p.delimited(d))}
	}
	return addr{}
}

func (p *sedParser) parse(inBlock bool) []*sedCmd {
	var out []*sedCmd
	for {
		for p.i < len(p.s) && strings.IndexByte(" \t\n;", p.s[p.i]) >= 0 {
			p.i++
		}
		if p.i >= len(p.s) {
			if inBlock {
				p.errf("unmatched `{'")
			}
			return out
		}
		if p.s[p.i] == '}' {
			if !inBlock {
				p.errf("unexpected `}'")
			}
			p.i++
			return out
		}
		c := &sedCmd{}
		c.a1 = p.addr()
		if c.a1.kind != 0 && p.i < len(p.s) && p.s[p.i] == ',' {
			p.i++
			c.a2 = p.addr()
		}
		p.skipSpace()
		if p.i < len(p.s) && p.s[p.i] == '!' {
			c.neg = true
			p.i++
			p.skipSpace()
		}
		if p.i >= len(p.s) {
			p.errf("missing command")
		}
		c.name = p.s[p.i]
		p.i++
		switch c.name {
		case '{':
			c.block = p.parse(true)
		case 'd', 'p', 'q', '=':
		case 's':
			if p.i >= len(p.s) {
				p.errf("unterminated `s' command")
			}
			d := p.s[p.i]
			p.i++
			c.re = p.compile(p.delimited(d))
			c.repl = p.delimited(d)
			for p.i < len(p.s) && strings.IndexByte(" \t\n;}", p.s[p.i]) < 0 {
				f := p.s[p.i]
				switch {
				case f == 'g':
					c.global = true
				case f == 'p':
					c.print = true
				case f >= '1' && f <= '9':
					c.nth = int(f - '0')
				default:
					p.errf("unknown option to `s'")
				}
				p.i++
			}
		default:
			p.errf("unknown command: `%c'", c.name)
		}
		out = append(out, c)
	}
}

func expandRepl(repl string, src string, m []int) string {
	var b strings.Builder
	for i := 0; i < len(repl); i++ {
		c := repl[i]
		if c == '\\' && i+1 < len(repl) {
			n := repl[i+1]
			i++
			switch {
			case n >= '0' && n <= '9':
				k := int(n - '0')
				if 2*k+1 < len(m) && m[2*k] >= 0 {
					b.WriteString(src[m[2*k]:m[2*k+1]])
				}
			case n == 'n':
				b.WriteByte('\n')
			case n == 't':
				b.WriteByte('\t')
			default:
				b.WriteByte(n)
			}
			continue
		}
		if c == '&' {
			b.WriteString(src[m[0]:m[1]])
			continue
		}
		b.WriteByte(c)
	}
	return b.String()
}

type sedState struct {
	line   int
	last   bool
	quiet  bool
	quit   bool
	w      io.Writer
	result bool
}

func (st *sedState) match(a addr, s string) bool {
	switch a.kind {
	case 'n':
		return st.line == a.line
	case '$':
		return st.last
	case 'r':
		return a.re.MatchString(s)
	}
	return true
}

func (st *sedState) selected(c *sedCmd, s string) bool {
	var ok bool
	switch {
	case c.a1.kind == 0:
		ok = true
	case c.a2.kind == 0:
		ok = st.match(c.a1, s)
	case c.inRange:
		ok = true
		if c.a2.kind == 'n' && st.line >= c.a2.line || c.a2.kind != 'n' && st.match(c.a2, s) {
			c.inRange = false
		}
	case st.match(c.a1, s):
		ok = true
		c.inRange = !(c.a2.kind == 'n' && c.a2.line <= st.line)
	}
	return ok != c.neg
}

// run applies cmds to the pattern space; it reports false when the line is deleted.
func (st *sedState) run(cmds []*sedCmd, ps *string) bool {
	for _, c := range cmds {
		if !st.selected(c, *ps) {
			continue
		}
		switch c.name {
		case '{':
			if !st.run(c.block, ps) {
				return false
			}
		case 'd':
			return false
		case 'p':
			fmt.Fprintln(st.w, *ps)
		case '=':
			fmt.Fprintln(st.w, st.line)
		case 'q':
			st.quit = true
			return true
		case 's':
			src := *ps
			all := c.re.FindAllStringSubmatchIndex(src, -1)
			if len(all) == 0 {
				continue
			}
			var b strings.Builder
			prev, n, did := 0, 0, false
			for _, m := range all {
				n++
				if c.nth > 0 && n != c.nth || c.nth == 0 && !c.global && n > 1 || c.nth > 0 && !c.global && n > c.nth {
					if !(c.global && c.nth > 0 && n > c.nth) {
						continue
					}
				}
				b.WriteString(src[prev:m[0]])
				b.WriteString(expandRepl(c.repl, src, m))
				prev = m[1]
				did = true
			}
			if did {
				b.WriteString(src[prev:])
				*ps = b.String()
				if c.print {
					fmt.Fprintln(st.w, *ps)
				}
			}
		}
	}
	return true
}

func sedStream(cmds []*sedCmd, r io.Reader, w io.Writer, st *sedState) {
	br := bufio.NewReader(r)
	next, err := br.ReadString('\n')
	for !(err != nil && next == "") && !st.quit {
		cur := strings.TrimSuffix(next, "\n")
		noNL := !strings.HasSuffix(next, "\n")
		next, err = br.ReadString('\n')
		st.line++
		st.last = err != nil && next == ""
		st.w = w
		keep := st.run(cmds, &cur)
		if keep && !st.quiet {
			io.WriteString(w, cur)
			if !noNL {
				io.WriteString(w, "\n")
			}
		}
	}
}

// Sed is sed [-n] [-E|-r] [-i] [-e script]... [script] [file...].
func Sed() {
	args := os.Args[1:]
	var scripts []string
	quiet, ere, inPlace := false, false, false
	for len(args) > 0 && strings.HasPrefix(args[0], "-") && len(args[0]) > 1 {
		a := args[0]
		args = args[1:]
		if a == "--" {
			break
		}
		if a == "-e" || a == "--expression" {
			if len(args) == 0 {
				fatal("sed: option requires an argument -- 'e'")
			}
			scripts = append(scripts, args[0])
			args = args[1:]
			continue
		}
		for _, c := range a[1:] {
			switch c {
			case 'n':
				quiet = true
			case 'E', 'r':
				ere = true
			case 'i':
				inPlace = true
			default:
				fatal(fmt.Sprintf("sed: invalid option -- '%c'", c))
			}
		}
	}
	if len(scripts) == 0 {
		if len(args) == 0 {
			fatal("Usage: sed [-n] [-E] [-i] [-e script]... [script] [file...]")
		}
		scripts, args = []string{args[0]}, args[1:]
	}
	p := &sedParser{s: strings.Join(scripts, "\n"), ere: ere}
	cmds := p.parse(false)
	w := bufio.NewWriter(os.Stdout)
	defer w.Flush()
	if len(args) == 0 {
		sedStream(cmds, os.Stdin, w, &sedState{quiet: quiet})
		return
	}
	if inPlace {
		for _, name := range args {
			b, err := os.ReadFile(name)
			if err != nil {
				fatal("sed: " + err.Error())
			}
			var out bytes.Buffer
			sedStream(cmds, bytes.NewReader(b), &out, &sedState{quiet: quiet})
			if err := os.WriteFile(name, out.Bytes(), 0o644); err != nil {
				fatal("sed: " + err.Error())
			}
		}
		return
	}
	var rs []io.Reader
	for _, name := range args {
		if name == "-" {
			rs = append(rs, os.Stdin)
			continue
		}
		f, err := os.Open(name)
		if err != nil {
			w.Flush()
			fatal("sed: can't read " + name + ": " + err.Error())
		}
		defer f.Close()
		rs = append(rs, f)
	}
	sedStream(cmds, io.MultiReader(rs...), w, &sedState{quiet: quiet})
}
