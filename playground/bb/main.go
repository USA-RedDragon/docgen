// Command bb is a multi-call binary of a POSIX shell and common commands. The
// base name of argv[0] selects the command, as in busybox. "bb <name> args"
// also works.
package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"

	"github.com/u-root/u-root/pkg/core"
	"github.com/u-root/u-root/pkg/core/cat"
	"github.com/u-root/u-root/pkg/core/cp"
	"github.com/u-root/u-root/pkg/core/mkdir"
	"github.com/u-root/u-root/pkg/core/mv"
	"github.com/u-root/u-root/pkg/core/rm"
	"github.com/u-root/u-root/pkg/core/touch"

	"playground/bb/own"
	"playground/bb/ucmd/basename"
	"playground/bb/ucmd/date"
	"playground/bb/ucmd/dirname"
	"playground/bb/ucmd/echo"
	ufalse "playground/bb/ucmd/false"
	"playground/bb/ucmd/grep"
	"playground/bb/ucmd/head"
	"playground/bb/ucmd/pwd"
	usort "playground/bb/ucmd/sort"
	"playground/bb/ucmd/tail"
	utrue "playground/bb/ucmd/true"
	"playground/bb/ucmd/uniq"
	"playground/bb/ucmd/wc"
)

func coreCmd(name string, c func() core.Command) func() {
	return func() {
		log.SetFlags(0)
		cmd := c()
		cmd.SetIO(os.Stdin, os.Stdout, os.Stderr)
		if err := cmd.RunContext(context.Background(), os.Args[1:]...); err != nil {
			log.Fatalf("%s: %v", name, err)
		}
	}
}

var commands = map[string]func(){
	"sh":       own.Sh,
	"printf":   own.Builtin("printf"),
	"test":     own.Builtin("test"),
	"[":        own.Builtin("["),
	"sed":      own.Sed,
	"cut":      own.Cut,
	"cat":      coreCmd("cat", cat.New),
	"cp":       coreCmd("cp", cp.New),
	"ls":       own.Ls,
	"mkdir":    coreCmd("mkdir", mkdir.New),
	"mv":       coreCmd("mv", mv.New),
	"rm":       coreCmd("rm", rm.New),
	"touch":    coreCmd("touch", touch.New),
	"basename": ubasename.Main,
	"date":     udate.Main,
	"dirname":  udirname.Main,
	"echo":     uecho.Main,
	"false":    ufalse.Main,
	"grep":     ugrep.Main,
	"head":     uhead.Main,
	"pwd":      upwd.Main,
	"sort":     usort.Main,
	"tail":     utail.Main,
	"tr":       own.Tr,
	"true":     utrue.Main,
	"uniq":     uuniq.Main,
	"wc":       uwc.Main,
}

func main() {
	name := filepath.Base(os.Args[0])
	if _, ok := commands[name]; !ok && len(os.Args) > 1 {
		os.Args = os.Args[1:]
		name = filepath.Base(os.Args[0])
	}
	run, ok := commands[name]
	if !ok {
		names := make([]string, 0, len(commands))
		for n := range commands {
			names = append(names, n)
		}
		sort.Strings(names)
		fmt.Fprintf(os.Stderr, "bb: unknown command %q\ncommands: %v\n", name, names)
		os.Exit(127)
	}
	run()
	os.Exit(0)
}

var _ = io.EOF
