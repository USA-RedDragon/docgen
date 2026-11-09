package udate

import (
	"flag"
	"os"
)

var cmdFlags = flag.NewFlagSet(os.Args[0], flag.ExitOnError)

var _ = flag.ErrHelp
