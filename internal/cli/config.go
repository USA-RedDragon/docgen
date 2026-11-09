package cli

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"unicode"

	"github.com/USA-RedDragon/docgen/internal/goprint"
)

// maxJobs limits the jobs setting. A go command with a huge -p does not finish.
const maxJobs = 1024

// maxDepth limits the depth setting. Each level holds a copy of its root.
const maxDepth = 8

//go:generate go tool configulator -type Config -flags std

// Config holds the settings of a repository. A .docgen.yaml at the repository
// root sets them, the DOCGEN_ variables override the file, and the directory
// argument and the flags override both.
type Config struct {
	Docs   string            `name:"docs" flag:"-" default:"." description:"directory to scan for Markdown pages, relative to the repository root"`
	Module string            `name:"module" default:"." description:"directory of the Go module, relative to the repository root"`
	Jobs   int               `name:"jobs" default:"0" description:"-p of each go command; 0 leaves it to go"`
	Tags   string            `name:"tags" description:"comma-separated build tags of each go command and package load"`
	Depth  int               `name:"depth" flag:"-" default:"1" description:"levels of nested docgen:docgen markers; 0 allows none"`
	Print  string            `name:"print" flag:"-" default:"go" description:"how a marker with no print= prints a value: go, raw or config; raw runs no code"`
	Vars   map[string]string `name:"vars" flag:"-" env:"-" description:"values of the docgen:var and docgen:text markers, by name"`
}

// Validate implements the configulator interface.
func (c Config) Validate() error {
	if c.Jobs < 0 {
		return fmt.Errorf("jobs is %d; it must be 0 or more", c.Jobs)
	}
	if c.Jobs > maxJobs {
		return fmt.Errorf("jobs is %d; it must be at most %d", c.Jobs, maxJobs)
	}
	if c.Depth < 0 {
		return fmt.Errorf("depth is %d; it must be 0 or more", c.Depth)
	}
	if c.Depth > maxDepth {
		return fmt.Errorf("depth is %d; it must be at most %d", c.Depth, maxDepth)
	}
	if err := checkPrint(c.Print); err != nil {
		return err
	}
	if err := checkVars(c.Vars); err != nil {
		return err
	}
	for _, tag := range c.buildTags() {
		for _, r := range tag {
			if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' && r != '.' {
				return fmt.Errorf("build tag %q is not valid; a tag holds only letters, digits, _ and dots", tag)
			}
		}
	}
	return nil
}

// checkPrint returns an error for a print setting that is not go, raw or config.
func checkPrint(value string) error {
	if value != goprint.PrintGo && value != goprint.PrintRaw && value != goprint.PrintConfig {
		return fmt.Errorf("print is %q; it must be go, raw or config", value)
	}
	return nil
}

// checkVars returns an error for the first key of vars, in sorted order, that is
// not valid.
func checkVars(vars map[string]string) error {
	for _, key := range slices.Sorted(maps.Keys(vars)) {
		valid := key != ""
		for _, r := range key {
			valid = valid && (unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-')
		}
		if !valid {
			return fmt.Errorf("var key %q is not valid; a key holds only letters, digits, _ and -", key)
		}
	}
	return nil
}

// buildTags returns the tags of the tags setting, which commas or white space
// separate.
func (c Config) buildTags() []string {
	return strings.FieldsFunc(c.Tags, func(r rune) bool { return r == ',' || unicode.IsSpace(r) })
}
