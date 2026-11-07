// Package cli implements the command line of docgen.
package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	configulator "github.com/USA-RedDragon/configulator/v2"
	cstd "github.com/USA-RedDragon/configulator/v2/flags/std"
	"github.com/USA-RedDragon/docgen/internal/gen"
)

const (
	cmdUpdate  = "update"
	cmdCheck   = "check"
	configFile = ".docgen.yaml"
)

// fileOptions returns the options of the config file of the repository at root.
func fileOptions(root string) *configulator.FileOptions {
	return &configulator.FileOptions{
		Search:   []string{filepath.Join(root, configFile)},
		Decoders: configulator.Decoders{".yaml": strictYAML, ".yml": strictYAML},
	}
}

func bind(fs *flag.FlagSet, root string) *configulator.Configulator[Config] {
	c := configulator.New(ConfigSchema()).
		WithEnvironmentVariables(&configulator.EnvironmentVariableOptions{Prefix: "DOCGEN_", Separator: "_"}).
		WithFile(fileOptions(root))
	cstd.Bind(c, fs, ConfigStdFlagHooks(), nil)
	if f := fs.Lookup("config"); f != nil {
		f.DefValue = configFile
	}
	return c
}

func usage(w io.Writer, fs *flag.FlagSet) {
	fmt.Fprint(w, `Usage: docgen <command> [flags] [directory]

Commands:
  update  fill in the markers
  check   list stale markers, unused regions and broken links, and exit 1 if there are any

Arguments:
  directory  the directory to scan (default the repository, or docs: in .docgen.yaml)

Flags:
`)
	fs.SetOutput(w)
	fs.PrintDefaults()
}

// Main runs docgen with the arguments that follow the command name and
// returns the exit code. SIGINT and SIGTERM stop the run with exit code 1.
func Main(args []string, stderr io.Writer) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	wd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(stderr, "docgen:", err)
		return 1
	}
	return run(ctx, wd, args, stderr)
}

// run runs docgen in dir with the diff on the standard output of the process.
func run(ctx context.Context, dir string, args []string, stderr io.Writer) int {
	return runTo(ctx, dir, args, os.Stdout, stderr)
}

// varsFile returns the slash path in root of the config file that c loaded, with
// a path relative to dir.
func varsFile(c *configulator.Configulator[Config], dir, root string) string {
	rel, err := filepath.Rel(root, abs(dir, c.Report().File().Path))
	if err != nil || !filepath.IsLocal(rel) {
		return configFile
	}
	return filepath.ToSlash(rel)
}

// runTo runs docgen in dir. The diff of check -diff goes to stdout, and all other
// text goes to stderr.
func runTo(ctx context.Context, dir string, args []string, stdout, stderr io.Writer) int {
	root := findRoot(dir)
	fs := flag.NewFlagSet("docgen", flag.ContinueOnError)
	c := bind(fs, root)
	diff := fs.Bool("diff", false, "with check, print a unified diff of each stale page and generated file")
	fs.SetOutput(stderr)
	fs.Usage = func() { usage(stderr, fs) }
	if len(args) == 0 {
		usage(stderr, fs)
		return 2
	}
	cmd := args[0]
	switch cmd {
	case cmdUpdate, cmdCheck:
	case "-h", "-help", "--help", "help":
		usage(stderr, fs)
		return 0
	default:
		fmt.Fprintf(stderr, "docgen: unknown command %q\n\n", cmd)
		usage(stderr, fs)
		return 2
	}
	if err := fs.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if fs.NArg() > 1 {
		fmt.Fprintf(stderr, "docgen: unexpected argument %q\n", fs.Arg(1))
		return 2
	}
	if *diff && cmd != cmdCheck {
		fmt.Fprintln(stderr, "docgen: -diff is valid only with check")
		return 2
	}
	cfg, err := c.Load()
	if err != nil {
		fmt.Fprintln(stderr, "docgen:", err)
		return 1
	}
	g, err := gen.New(root, abs(root, cfg.Module), stderr, gen.Options{
		Jobs:     cfg.Jobs,
		Tags:     cfg.buildTags(),
		Diff:     *diff,
		Audit:    cmd == cmdCheck,
		Depth:    cfg.Depth,
		Print:    cfg.Print,
		Vars:     cfg.Vars,
		VarsFile: varsFile(c, dir, root),
		Settings: copySettings,
	})
	if err != nil {
		fmt.Fprintln(stderr, "docgen:", err)
		return 1
	}
	docs := abs(root, cfg.Docs)
	if fs.NArg() == 1 {
		docs = abs(dir, fs.Arg(0))
	}
	stale, err := g.Run(ctx, docs, cmd == cmdUpdate)
	rc := 0
	if cmd == cmdCheck && len(stale) > 0 {
		for _, s := range stale {
			fmt.Fprintln(stderr, "stale:", s)
		}
		if _, err := io.WriteString(stdout, g.Diff()); err != nil {
			fmt.Fprintln(stderr, "docgen:", err)
		}
		fmt.Fprintf(stderr, "docgen: %d stale; run docgen update and commit the result\n", len(stale))
		rc = 1
	}
	if unused := g.Unused(); len(unused) > 0 {
		for _, s := range unused {
			fmt.Fprintln(stderr, "unused:", s)
		}
		fmt.Fprintf(stderr, "docgen: %d unused; remove each one or add a marker that uses it\n", len(unused))
		rc = 1
	}
	if broken := g.Broken(); len(broken) > 0 {
		for _, s := range broken {
			fmt.Fprintln(stderr, "broken:", s)
		}
		fmt.Fprintf(stderr, "docgen: %d broken; fix each link\n", len(broken))
		rc = 1
	}
	if err != nil {
		fmt.Fprintln(stderr, "docgen:", err)
		rc = 1
	}
	return rc
}
