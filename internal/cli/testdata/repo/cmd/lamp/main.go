// Command lamp is the fixture command with a help command of the docgen tests.
package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	config := flag.NewFlagSet("lamp config", flag.ExitOnError)
	config.Duration("wait", 0, "the time before the lamp turns on")
	if len(os.Args) == 3 && os.Args[1] == "help" && os.Args[2] == "config" {
		config.SetOutput(os.Stdout)
		config.Usage()
		return
	}
	fmt.Println("usage: lamp <config|help> [arguments]")
}
