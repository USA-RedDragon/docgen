package cli

import (
	"errors"
	iofs "io/fs"
	"os"
	"path/filepath"

	configulator "github.com/USA-RedDragon/configulator/v2"
	"github.com/USA-RedDragon/docgen/internal/gen"
	"github.com/goccy/go-yaml"
)

// copyFile holds the keys of the config file of a copy. A nested run reads docs,
// print and vars. It ignores the others, so their values can be anything.
type copyFile struct {
	Docs   *string           `yaml:"docs"`
	Print  *string           `yaml:"print"`
	Vars   map[string]string `yaml:"vars"`
	Module any               `yaml:"module"`
	Jobs   any               `yaml:"jobs"`
	Tags   any               `yaml:"tags"`
	Depth  any               `yaml:"depth"`
}

func ignoreOrigin(string, configulator.Layer, string) {}

// copySettings returns the settings of a nested run in the copy at root. They
// come from its config file alone: the variables and flags of the process are
// for the outer run. The jobs, tags and depth of the outer run apply, and so does
// the module of the marker. An outer print setting of raw also applies.
func copySettings(root string) (gen.Settings, error) {
	var defaults Config
	if err := ConfigSchema().ApplyDefaults(&defaults, "", ignoreOrigin); err != nil {
		return gen.Settings{}, err
	}
	s := gen.Settings{Docs: defaults.Docs, Print: defaults.Print, VarsFile: configFile}
	b, err := os.ReadFile(filepath.Join(root, configFile))
	if err != nil && !errors.Is(err, iofs.ErrNotExist) {
		return gen.Settings{}, err
	}
	var f copyFile
	if err := yaml.UnmarshalWithOptions(b, &f, yaml.Strict()); err != nil {
		return gen.Settings{}, err
	}
	if f.Print != nil {
		s.Print = *f.Print
	}
	if err := checkPrint(s.Print); err != nil {
		return gen.Settings{}, err
	}
	if err := checkVars(f.Vars); err != nil {
		return gen.Settings{}, err
	}
	s.Vars = f.Vars
	if f.Docs != nil {
		s.Docs = *f.Docs
	}
	return s, nil
}
