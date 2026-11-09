package own

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

type span struct{ lo, hi int }

func parseList(s string) ([]span, error) {
	var out []span
	for _, part := range strings.Split(s, ",") {
		lo, hi, rng := strings.Cut(part, "-")
		a, b := 1, int(^uint(0)>>1)
		var err error
		if lo != "" {
			if a, err = strconv.Atoi(lo); err != nil || a < 1 {
				return nil, fmt.Errorf("invalid list value %q", part)
			}
		}
		if !rng {
			b = a
		} else if hi != "" {
			if b, err = strconv.Atoi(hi); err != nil || b < a {
				return nil, fmt.Errorf("invalid list value %q", part)
			}
		}
		if lo == "" && hi == "" {
			return nil, fmt.Errorf("invalid list value %q", part)
		}
		out = append(out, span{a, b})
	}
	return out, nil
}

func inList(l []span, n int) bool {
	for _, s := range l {
		if n >= s.lo && n <= s.hi {
			return true
		}
	}
	return false
}

// Cut is cut -b list, -c list or -f list [-d delim] [-s] [file...].
func Cut() {
	opts, operands := getopt("cut", "b:c:f:d:sn", os.Args[1:])
	get := func(k byte) *string { v := opts[k]; return &v }
	b, c, f := get('b'), get('c'), get('f')
	d := get('d')
	if _, ok := opts['d']; !ok {
		*d = "\t"
	}
	_, sOK := opts['s']
	s := &sOK
	mode, list := "", ""
	for _, m := range []struct{ name, v string }{{"b", *b}, {"c", *c}, {"f", *f}} {
		if m.v != "" {
			if mode != "" {
				fatal("cut: only one type of list may be specified")
			}
			mode, list = m.name, m.v
		}
	}
	if mode == "" {
		fatal("cut: you must specify a list of bytes, characters, or fields")
	}
	if len([]rune(*d)) != 1 {
		fatal("cut: the delimiter must be a single character")
	}
	spans, err := parseList(list)
	if err != nil {
		fatal("cut: " + err.Error())
	}
	w := bufio.NewWriter(os.Stdout)
	defer w.Flush()
	do := func(r io.Reader) {
		sc := bufio.NewScanner(r)
		sc.Buffer(make([]byte, 64<<10), 64<<20)
		for sc.Scan() {
			line := sc.Text()
			switch mode {
			case "b":
				var o []byte
				for i := 0; i < len(line); i++ {
					if inList(spans, i+1) {
						o = append(o, line[i])
					}
				}
				w.Write(o)
			case "c":
				var o []rune
				for i, r := range []rune(line) {
					if inList(spans, i+1) {
						o = append(o, r)
					}
				}
				w.WriteString(string(o))
			case "f":
				if !strings.Contains(line, *d) {
					if *s {
						continue
					}
					w.WriteString(line)
					break
				}
				var o []string
				for i, x := range strings.Split(line, *d) {
					if inList(spans, i+1) {
						o = append(o, x)
					}
				}
				w.WriteString(strings.Join(o, *d))
			}
			w.WriteByte('\n')
		}
		if err := sc.Err(); err != nil {
			w.Flush()
			fatal("cut: " + err.Error())
		}
	}
	files := operands
	if len(files) == 0 {
		files = []string{"-"}
	}
	code := 0
	for _, name := range files {
		if name == "-" {
			do(os.Stdin)
			continue
		}
		fh, err := os.Open(name)
		if err != nil {
			fmt.Fprintln(os.Stderr, "cut:", err)
			code = 1
			continue
		}
		do(fh)
		fh.Close()
	}
	w.Flush()
	os.Exit(code)
}

func fatal(msg string) {
	fmt.Fprintln(os.Stderr, msg)
	os.Exit(1)
}
