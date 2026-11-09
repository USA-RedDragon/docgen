package own

import (
	"fmt"
	"os"
	"strings"
)

// getopt parses POSIX short options. spec lists the option letters, and a
// letter followed by ':' takes a value, attached ("-d,") or as the next
// argument ("-d ,"). It returns the values by letter and the operands.
func getopt(name, spec string, args []string) (map[byte]string, []string) {
	opts := map[byte]string{}
	for len(args) > 0 {
		a := args[0]
		if a == "--" {
			return opts, args[1:]
		}
		if len(a) < 2 || a[0] != '-' {
			break
		}
		args = args[1:]
		for i := 1; i < len(a); i++ {
			c := a[i]
			k := strings.IndexByte(spec, c)
			if k < 0 || c == ':' {
				fmt.Fprintf(os.Stderr, "%s: invalid option -- '%c'\n", name, c)
				os.Exit(1)
			}
			if k+1 < len(spec) && spec[k+1] == ':' {
				if i+1 < len(a) {
					opts[c] = a[i+1:]
				} else if len(args) > 0 {
					opts[c], args = args[0], args[1:]
				} else {
					fmt.Fprintf(os.Stderr, "%s: option requires an argument -- '%c'\n", name, c)
					os.Exit(1)
				}
				break
			}
			opts[c] = ""
		}
	}
	return opts, args
}
