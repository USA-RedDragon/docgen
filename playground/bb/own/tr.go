package own

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"unicode"
)

var trClasses = map[string]func(rune) bool{
	"alnum": func(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) },
	"alpha": unicode.IsLetter, "digit": unicode.IsDigit, "lower": unicode.IsLower, "upper": unicode.IsUpper,
	"space": unicode.IsSpace, "blank": func(r rune) bool { return r == ' ' || r == '\t' },
	"punct": unicode.IsPunct, "print": unicode.IsPrint, "cntrl": unicode.IsControl,
	"xdigit": func(r rune) bool { return unicode.Is(unicode.ASCII_Hex_Digit, r) },
}

// trSet expands a tr set: ranges a-z, classes [:upper:] and escapes \n \t \\ \NNN.
func trSet(s string) []rune {
	in := []rune(s)
	var out []rune
	one := func(i int) (rune, int) {
		if in[i] == '\\' && i+1 < len(in) {
			switch in[i+1] {
			case 'n':
				return '\n', i + 2
			case 't':
				return '\t', i + 2
			case 'r':
				return '\r', i + 2
			case '\\':
				return '\\', i + 2
			}
			if in[i+1] >= '0' && in[i+1] <= '7' {
				v, j := rune(0), i+1
				for ; j < len(in) && j < i+4 && in[j] >= '0' && in[j] <= '7'; j++ {
					v = v*8 + in[j] - '0'
				}
				return v, j
			}
			return in[i+1], i + 2
		}
		return in[i], i + 1
	}
	for i := 0; i < len(in); {
		if in[i] == '[' && i+1 < len(in) && in[i+1] == ':' {
			end := -1
			for j := i + 2; j+1 < len(in); j++ {
				if in[j] == ':' && in[j+1] == ']' {
					end = j
					break
				}
			}
			if end > 0 {
				name := string(in[i+2 : end])
				f, ok := trClasses[name]
				if !ok {
					fatal("tr: invalid character class '" + name + "'")
				}
				for r := rune(0); r < 128; r++ {
					if f(r) {
						out = append(out, r)
					}
				}
				i = end + 2
				continue
			}
		}
		r, j := one(i)
		if j < len(in)-1 && in[j] == '-' {
			r2, k := one(j + 1)
			if r2 < r {
				fatal(fmt.Sprintf("tr: range-endpoints of '%c-%c' are in reverse collating sequence order", r, r2))
			}
			for x := r; x <= r2; x++ {
				out = append(out, x)
			}
			i = k
			continue
		}
		out = append(out, r)
		i = j
	}
	return out
}

// Tr is tr [-cds] set1 [set2].
func Tr() {
	opts, args := getopt("tr", "cdsC", os.Args[1:])
	_, del := opts['d']
	_, sq := opts['s']
	_, comp := opts['c']
	if _, ok := opts['C']; ok {
		comp = true
	}
	if len(args) == 0 || len(args) == 1 && !del && !sq || len(args) > 2 {
		fatal("tr: missing or extra operand\nUsage: tr [-cds] SET1 [SET2]")
	}
	set1 := trSet(args[0])
	var set2 []rune
	if len(args) == 2 {
		set2 = trSet(args[1])
	}
	in1 := map[rune]bool{}
	for _, r := range set1 {
		in1[r] = true
	}
	member := func(r rune) bool { return in1[r] != comp }
	mapTo := map[rune]rune{}
	if !del && len(set2) > 0 && !comp {
		for i, r := range set1 {
			j := i
			if j >= len(set2) {
				j = len(set2) - 1
			}
			mapTo[r] = set2[j]
		}
	}
	sqSet := map[rune]bool{}
	sqFrom := set1
	if len(set2) > 0 {
		sqFrom = set2
	}
	for _, r := range sqFrom {
		sqSet[r] = true
	}
	br := bufio.NewReader(os.Stdin)
	w := bufio.NewWriter(os.Stdout)
	defer w.Flush()
	last, have := rune(0), false
	for {
		r, _, err := br.ReadRune()
		if err == io.EOF {
			return
		}
		if err != nil {
			w.Flush()
			fatal("tr: " + err.Error())
		}
		if del && member(r) {
			continue
		}
		if !del && len(set2) > 0 && member(r) {
			if comp {
				r = set2[len(set2)-1]
			} else {
				r = mapTo[r]
			}
		}
		if sq && have && r == last && sqSet[r] {
			continue
		}
		last, have = r, true
		w.WriteRune(r)
	}
}
