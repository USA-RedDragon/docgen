// Package rustfixture lists the crates of the Rust test fixture, so that tests
// compare with the fixture as it is and not with a copy of its member list.
package rustfixture

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/pelletier/go-toml/v2"
)

// Crates returns the package names of the members of the Cargo workspace in
// dir, in name order.
func Crates(dir string) ([]string, error) {
	var root struct {
		Workspace struct {
			Members []string `toml:"members"`
		} `toml:"workspace"`
	}
	if err := read(filepath.Join(dir, "Cargo.toml"), &root); err != nil {
		return nil, err
	}
	names := make([]string, 0, len(root.Workspace.Members))
	for _, m := range root.Workspace.Members {
		var member struct {
			Package struct {
				Name string `toml:"name"`
			} `toml:"package"`
		}
		path := filepath.Join(dir, m, "Cargo.toml")
		if err := read(path, &member); err != nil {
			return nil, err
		}
		if member.Package.Name == "" {
			return nil, fmt.Errorf("%s has no package name", path)
		}
		names = append(names, member.Package.Name)
	}
	slices.Sort(names)
	return names, nil
}

func read(path string, v any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := toml.Unmarshal(b, v); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}
