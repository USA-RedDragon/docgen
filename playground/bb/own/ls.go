package own

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Ls is ls [-1alRd] [file...]. u-root pkg/ls does not build for js/wasm.
func Ls() {
	opts, args := getopt("ls", "1alRdAF", os.Args[1:])
	_, long := opts['l']
	_, all := opts['a']
	_, almost := opts['A']
	_, rec := opts['R']
	_, dirOnly := opts['d']
	_, classify := opts['F']
	if len(args) == 0 {
		args = []string{"."}
	}
	code := 0
	show := func(dir, name string, fi fs.FileInfo) {
		s := name
		if classify && fi.IsDir() {
			s += "/"
		}
		if long {
			fmt.Printf("%s %8d %s %s\n", fi.Mode().String(), fi.Size(), fi.ModTime().Format("Jan _2 15:04"), s)
		} else {
			fmt.Println(s)
		}
	}
	var listDir func(dir string, header bool)
	listDir = func(dir string, header bool) {
		ents, err := os.ReadDir(dir)
		if err != nil {
			fmt.Fprintln(os.Stderr, "ls:", err)
			code = 2
			return
		}
		if header {
			fmt.Printf("%s:\n", dir)
		}
		sort.Slice(ents, func(i, j int) bool { return ents[i].Name() < ents[j].Name() })
		if all {
			for _, n := range []string{".", ".."} {
				if fi, err := os.Stat(filepath.Join(dir, n)); err == nil {
					show(dir, n, fi)
				}
			}
		}
		var subs []string
		for _, e := range ents {
			if strings.HasPrefix(e.Name(), ".") && !all && !almost {
				continue
			}
			fi, err := e.Info()
			if err != nil {
				continue
			}
			show(dir, e.Name(), fi)
			if rec && e.IsDir() {
				subs = append(subs, filepath.Join(dir, e.Name()))
			}
		}
		for _, s := range subs {
			fmt.Println()
			listDir(s, true)
		}
	}
	var dirs []string
	for _, a := range args {
		fi, err := os.Stat(a)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ls: cannot access '%s': %v\n", a, err.(*fs.PathError).Err)
			code = 2
			continue
		}
		if fi.IsDir() && !dirOnly {
			dirs = append(dirs, a)
			continue
		}
		show("", a, fi)
	}
	for i, d := range dirs {
		if i > 0 {
			fmt.Println()
		}
		listDir(d, len(args) > 1 || rec)
	}
	os.Exit(code)
}
