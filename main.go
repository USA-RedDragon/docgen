package main

import (
	"os"

	"github.com/USA-RedDragon/docgen/internal/cli"
)

func main() {
	os.Exit(cli.Main(os.Args[1:], os.Stderr))
}
