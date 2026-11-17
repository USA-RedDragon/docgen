package cargo

import (
	"fmt"
	"slices"
	"strings"
	"unicode"
)

// Features is the set of Cargo features that a cargo command enables. A
// command that builds a package passes the flags of Args.
type Features struct {
	// List holds the features to enable. An entry is a feature name, which
	// applies to each package of the workspace that has the feature, or
	// crate/feature, which applies to the one package crate.
	List []string
	// All enables every feature of the package.
	All bool
	// NoDefault leaves the default feature of the package off.
	NoDefault bool
}

// SplitFeatures returns the entries of the features value s, which commas or
// white space separate, and an error for an entry that is not a feature name or
// crate/feature.
func SplitFeatures(s string) ([]string, error) {
	list := strings.FieldsFunc(s, func(r rune) bool { return r == ',' || unicode.IsSpace(r) })
	for _, e := range list {
		crate, feature, qualified := strings.Cut(e, "/")
		if qualified && (crate == "" || feature == "") || strings.Contains(feature, "/") {
			return nil, fmt.Errorf("feature %q is not valid; write a feature, or crate/feature", e)
		}
		for _, name := range strings.Split(e, "/") {
			for _, r := range name {
				if !unicode.IsLetter(r) && !unicode.IsDigit(r) && !strings.ContainsRune("_-+.", r) {
					return nil, fmt.Errorf("feature %q is not valid; a name holds only letters, digits, _, -, + and dots", e)
				}
			}
		}
	}
	return list, nil
}

// Zero reports whether f enables nothing beyond what cargo does with no flags.
func (f Features) Zero() bool {
	return len(f.List) == 0 && !f.All && !f.NoDefault
}

// Key returns a string that is the same for two Features that give a command
// the same flags.
func (f Features) Key() string {
	list := slices.Clone(f.List)
	slices.Sort(list)
	list = slices.Compact(list)
	return fmt.Sprintf("%t\x00%t\x00%s", f.All, f.NoDefault, strings.Join(list, ","))
}

// Validate returns an error for an entry that names no crate or no feature of
// ws. The error lists the crates or the features to choose from.
func (f Features) Validate(ws *Workspace) error {
	for _, e := range f.List {
		crate, feature, qualified := strings.Cut(e, "/")
		if !qualified {
			if !slices.ContainsFunc(ws.Packages, func(p *Package) bool { _, ok := p.Features[e]; return ok }) {
				return fmt.Errorf("no crate of the Cargo workspace at %s has the feature %q; the features are: %s", ws.Root, e, ws.featureList())
			}
			continue
		}
		pkg, err := ws.Package(crate)
		if err != nil {
			return fmt.Errorf("feature %q: %w", e, err)
		}
		if _, ok := pkg.Features[feature]; !ok {
			return fmt.Errorf("feature %q: crate %s has no feature %q; %s", e, crate, feature, pkg.featureList())
		}
	}
	return nil
}

func (p *Package) featureList() string {
	names := p.featureNames()
	if len(names) == 0 {
		return "it has no features"
	}
	return "its features are: " + strings.Join(names, ", ")
}

func (p *Package) featureNames() []string {
	names := make([]string, 0, len(p.Features))
	for name := range p.Features {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

func (w *Workspace) featureList() string {
	var parts []string
	for _, p := range w.Packages {
		if names := p.featureNames(); len(names) > 0 {
			parts = append(parts, p.Name+": "+strings.Join(names, ", "))
		}
	}
	if len(parts) == 0 {
		return "none, since no crate has a feature"
	}
	return strings.Join(parts, "; ")
}

// roots returns the features of pkg that f lists.
func (f Features) roots(pkg *Package) []string {
	var out []string
	for _, e := range f.List {
		crate, feature, qualified := strings.Cut(e, "/")
		switch {
		case qualified && crate == pkg.Name:
			out = append(out, feature)
		case !qualified:
			if _, ok := pkg.Features[e]; ok {
				out = append(out, e)
			}
		}
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// Args returns the cargo flags that enable f for the package pkg: --features
// with the features that apply to it, --all-features and
// --no-default-features. They go with --package pkg.
func (f Features) Args(pkg *Package) []string {
	var args []string
	if roots := f.roots(pkg); len(roots) > 0 {
		args = append(args, "--features", strings.Join(roots, ","))
	}
	if f.All {
		args = append(args, "--all-features")
	}
	if f.NoDefault {
		args = append(args, "--no-default-features")
	}
	return args
}

// Enabled returns the features of pkg that a command with f enables: the
// features of f, the default feature unless NoDefault, every feature with All,
// and what each of them enables in the same package.
func (f Features) Enabled(pkg *Package) map[string]bool {
	on := map[string]bool{}
	var enable func(name string)
	enable = func(name string) {
		if on[name] {
			return
		}
		items, ok := pkg.Features[name]
		if !ok {
			return
		}
		on[name] = true
		for _, item := range items {
			dep, _, qualified := strings.Cut(item, "/")
			switch {
			case strings.HasPrefix(item, "dep:"), strings.HasSuffix(dep, "?"):
			case qualified:
				enable(dep)
			default:
				enable(item)
			}
		}
	}
	if !f.NoDefault {
		enable("default")
	}
	for _, name := range f.roots(pkg) {
		enable(name)
	}
	if f.All {
		for name := range pkg.Features {
			enable(name)
		}
	}
	return on
}
