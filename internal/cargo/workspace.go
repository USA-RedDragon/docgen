package cargo

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// KindLib is the Kind of a library target.
const KindLib = "lib"

// Target is a target of a package.
type Target struct {
	Name string
	// Kind is KindLib, bin, example or test, or else the first kind that cargo
	// reports, such as bench, proc-macro or custom-build.
	Kind    string
	Path    string
	Edition string
}

// Dependency is a dependency of a package on another package of the workspace.
type Dependency struct {
	Name string
	// Path is the directory of the dependency.
	Path string
	// Kind is normal, dev or build.
	Kind string
}

// Package is a package of the workspace.
type Package struct {
	Name         string
	Version      string
	Edition      string
	RustVersion  string
	ManifestPath string
	// Dir is the directory of the manifest.
	Dir     string
	Targets []Target
	// Features maps each feature, explicit or from an optional dependency, to
	// the items it enables, as written in the manifest.
	Features map[string][]string
	// LocalDeps are the dependencies on other packages of the workspace, in the
	// order of the manifest.
	LocalDeps []Dependency
	// Registries are the registries named by publish in the manifest: nil when
	// publish is not set, and empty for publish = false.
	Registries []string
}

// CratesIO is the name that publish uses for crates.io.
const CratesIO = "crates-io"

// Published reports whether the manifest allows a publish to crates.io, which
// is where docs.rs builds the docs: publish is not set, or lists crates-io. It
// does not know whether the version was published.
func (p *Package) Published() bool {
	return p.Registries == nil || slices.Contains(p.Registries, CratesIO)
}

// Target returns the target of kind with the name, or nil.
func (p *Package) Target(kind, name string) *Target {
	for i := range p.Targets {
		if p.Targets[i].Kind == kind && p.Targets[i].Name == name {
			return &p.Targets[i]
		}
	}
	return nil
}

// Lib returns the library target of the package, or nil.
func (p *Package) Lib() *Target {
	for i := range p.Targets {
		if p.Targets[i].Kind == KindLib {
			return &p.Targets[i]
		}
	}
	return nil
}

// Workspace is a Cargo workspace.
type Workspace struct {
	// Root is the directory of the workspace manifest.
	Root string
	// Packages are the packages of the workspace, sorted by name.
	Packages []*Package
	byName   map[string]*Package
}

// Package returns the package with the name. An unknown name is an error that
// lists the packages of the workspace.
func (w *Workspace) Package(name string) (*Package, error) {
	if p, ok := w.byName[name]; ok {
		return p, nil
	}
	names := make([]string, len(w.Packages))
	for i, p := range w.Packages {
		names[i] = p.Name
	}
	return nil, fmt.Errorf("crate %q is not in the Cargo workspace at %s; the crates are: %s", name, w.Root, strings.Join(names, ", "))
}

// Open reads the Cargo workspace whose manifest is dir/Cargo.toml, with cargo
// metadata. Dependencies outside the workspace are not resolved, so it needs no
// network and no registry, and Cargo.lock is not read: a command that builds
// must pass --locked. dir is the root of the repository, or the directory
// of the module setting.
func (r *Runner) Open(ctx context.Context, dir string) (*Workspace, error) {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	manifest := filepath.Join(dir, "Cargo.toml")
	if _, err := os.Stat(manifest); errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("no Cargo.toml in %s; set module to the directory of the Cargo workspace", dir)
	} else if err != nil {
		return nil, err
	}
	out, errOut, err := r.Run(ctx, dir, "metadata", "--format-version", "1", "--no-deps", "--locked", "--manifest-path", manifest)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, err
		}
		return nil, LockedError("cargo metadata", err, errOut)
	}
	return parseMetadata([]byte(out))
}

type metadata struct {
	Packages      []metaPackage `json:"packages"`
	WorkspaceRoot string        `json:"workspace_root"`
}

type metaPackage struct {
	Name         string              `json:"name"`
	Version      string              `json:"version"`
	Edition      string              `json:"edition"`
	RustVersion  string              `json:"rust_version"`
	ManifestPath string              `json:"manifest_path"`
	Targets      []metaTarget        `json:"targets"`
	Features     map[string][]string `json:"features"`
	Dependencies []metaDep           `json:"dependencies"`
	Publish      []string            `json:"publish"`
}

type metaTarget struct {
	Name    string   `json:"name"`
	Kind    []string `json:"kind"`
	SrcPath string   `json:"src_path"`
	Edition string   `json:"edition"`
}

type metaDep struct {
	Name string `json:"name"`
	Kind string `json:"kind"`
	Path string `json:"path"`
}

func parseMetadata(b []byte) (*Workspace, error) {
	var m metadata
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("cargo metadata: %w", err)
	}
	w := &Workspace{Root: m.WorkspaceRoot, byName: map[string]*Package{}}
	for _, mp := range m.Packages {
		p := &Package{
			Name:         mp.Name,
			Version:      mp.Version,
			Edition:      mp.Edition,
			RustVersion:  mp.RustVersion,
			ManifestPath: mp.ManifestPath,
			Dir:          filepath.Dir(mp.ManifestPath),
			Features:     mp.Features,
			Registries:   mp.Publish,
		}
		for _, t := range mp.Targets {
			p.Targets = append(p.Targets, Target{Name: t.Name, Kind: targetKind(t.Kind), Path: t.SrcPath, Edition: t.Edition})
		}
		w.Packages = append(w.Packages, p)
		w.byName[p.Name] = p
	}
	slices.SortFunc(w.Packages, func(a, b *Package) int { return strings.Compare(a.Name, b.Name) })
	for _, mp := range m.Packages {
		p := w.byName[mp.Name]
		for _, d := range mp.Dependencies {
			if _, ok := w.byName[d.Name]; !ok || d.Path == "" {
				continue
			}
			kind := d.Kind
			if kind == "" {
				kind = "normal"
			}
			p.LocalDeps = append(p.LocalDeps, Dependency{Name: d.Name, Path: d.Path, Kind: kind})
		}
	}
	return w, nil
}

func targetKind(kinds []string) string {
	for _, k := range kinds {
		switch k {
		case "lib", "rlib", "dylib", "cdylib", "staticlib":
			return KindLib
		}
	}
	if len(kinds) == 0 {
		return ""
	}
	return kinds[0]
}
