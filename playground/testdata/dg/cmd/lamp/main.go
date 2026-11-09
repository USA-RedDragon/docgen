// Command lamp turns a Lamp on, waits, and prints its state.
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/USA-RedDragon/docgen/v2/machine"
)

func main() {
	wait := flag.Duration("wait", machine.DefaultWait, "how long the lamp stays on")
	tries := flag.Int("tries", 1, "the number of blinks, at most 3")
	flag.Usage = func() {
		fmt.Fprintln(flag.CommandLine.Output(), "usage: lamp [-wait duration] [-tries n]")
		flag.PrintDefaults()
	}
	flag.Parse()
	l, err := machine.NewLamp(machine.Config{Wait: *wait, Tries: *tries})
	if err != nil {
		fmt.Fprintln(os.Stderr, "lamp:", err)
		os.Exit(1)
	}
	_ = l.Start(0)
	l.Tick(time.Duration(*wait))
	fmt.Println(l.State() == machine.LampOff)
}
