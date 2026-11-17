package gen

import (
	"context"
	"fmt"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/USA-RedDragon/docgen/internal/cargo"
	"github.com/USA-RedDragon/docgen/internal/gocmd"
	"github.com/USA-RedDragon/docgen/internal/markdown"
	"github.com/USA-RedDragon/docgen/internal/marker"
)

// A rustBuild is the build of one binary or example of a run.
type rustBuild struct {
	once sync.Once
	exe  string
	err  error
}

// cargoBuild builds the target of kind bin or example with the name of the
// package pkg once for g, for all the markers of the run, and returns the path
// of its executable.
func (g *Generator) cargoBuild(ctx context.Context, ws *cargo.Workspace, pkg, kind, name string, feats cargo.Features) (string, error) {
	key := pkg + "\x00" + kind + "\x00" + name + "\x00" + feats.Key()
	g.rust.mu.Lock()
	b := g.rust.builds[key]
	if b == nil {
		b = &rustBuild{}
		g.rust.builds[key] = b
	}
	g.rust.mu.Unlock()
	b.once.Do(func() {
		var flags []string
		if flags, b.err = g.featureFlags(ctx, pkg, feats); b.err != nil {
			return
		}
		fmt.Fprintf(g.log, "docgen: building %s %s of %s\n", kind, name, pkg)
		b.exe, b.err = g.rust.runner.Build(ctx, ws.Root, pkg, kind, name, flags...)
	})
	return b.exe, b.err
}

// repoRel returns the slash path of p relative to the repository root, for
// messages and table cells.
func (g *Generator) repoRel(p string) string {
	rel, err := filepath.Rel(g.mod.Root, p)
	if err != nil || !filepath.IsLocal(rel) {
		return filepath.ToSlash(p)
	}
	return filepath.ToSlash(rel)
}

// A cargoHandler is a docgen:cargo marker: a field of a Cargo.toml, of the one
// of crate, or of the TOML file at the repository path file, or with
// field=features the table of the features of crate.
type cargoHandler struct {
	valueArgs
	field string
	crate string
	file  string
	feat  featureArgs
}

func (h *cargoHandler) Bind(a *marker.Args) {
	h.field = a.Required("field")
	h.crate, _ = a.Optional(argCrate)
	h.file, _ = a.Optional(argFile)
	a.NonEmpty(argCrate, argFile)
	if h.crate != "" && h.file != "" {
		a.Fail("has both crate= and file=; use one")
	}
	if h.field == "features" {
		if h.crate == "" {
			a.Fail("field=features needs crate=")
		}
		if h.file != "" {
			a.Fail("field=features has no file=")
		}
		h.feat.bind(a)
		return
	}
	h.bindCode(a)
	a.Inline()
}

func (h *cargoHandler) plan(p *plan) {
	if h.crate != "" {
		p.rust()
	}
}

func (h *cargoHandler) fill(ctx context.Context, j job) (string, []outFile, error) {
	if h.field == "features" {
		return h.features(ctx, j)
	}
	manifest, wsManifest, err := h.paths(ctx, j)
	if err != nil {
		return "", nil, err
	}
	m, err := cargo.ReadManifest(manifest, j.repoRel(manifest))
	if err != nil {
		return "", nil, err
	}
	var ws func() (*cargo.Manifest, error)
	if wsManifest != "" {
		ws = func() (*cargo.Manifest, error) {
			if wsManifest == manifest {
				return m, nil
			}
			return cargo.ReadManifest(wsManifest, j.repoRel(wsManifest))
		}
	}
	v, err := m.Field(h.field, ws)
	if err != nil {
		return "", nil, err
	}
	return h.inline(v), nil, nil
}

// paths returns the TOML file of the marker and the Cargo.toml of its
// workspace, which is "" when there is none.
func (h *cargoHandler) paths(ctx context.Context, j job) (string, string, error) {
	rootManifest := filepath.Join(j.mod.Dir, "Cargo.toml")
	switch {
	case h.crate != "":
		ws, pkg, err := j.cargoPackage(ctx, h.crate)
		if err != nil {
			return "", "", err
		}
		return pkg.ManifestPath, filepath.Join(ws.Root, "Cargo.toml"), nil
	case h.file != "":
		p, err := j.repoPath(h.file)
		return p, rootManifest, err
	}
	return rootManifest, rootManifest, nil
}

func (h *cargoHandler) features(ctx context.Context, j job) (string, []outFile, error) {
	ws, pkg, err := j.cargoPackage(ctx, h.crate)
	if err != nil {
		return "", nil, err
	}
	var feats cargo.Features
	if h.feat.count > 0 || !j.rust.features.Zero() {
		feats = j.features(&h.feat)
		if err := feats.Validate(ws); err != nil {
			return "", nil, fmt.Errorf("crate %s: %w", h.crate, err)
		}
		return featureTable(pkg, &feats)
	}
	return featureTable(pkg, nil)
}

// featureTable returns the table of the features of pkg, in name order. A
// feature is a default feature when the default feature lists it. With feats,
// the table has an Enabled column: yes for each feature that a command with
// feats enables, with the features that those enable.
func featureTable(pkg *cargo.Package, feats *cargo.Features) (string, []outFile, error) {
	var names []string
	for name := range pkg.Features {
		if name != "default" {
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		return "", nil, fmt.Errorf("crate %s has no features", pkg.Name)
	}
	slices.Sort(names)
	var b strings.Builder
	var on map[string]bool
	if feats == nil {
		b.WriteString("| Feature | Default | Enables |\n|---|---|---|\n")
	} else {
		on = feats.Enabled(pkg)
		b.WriteString("| Feature | Default | Enabled | Enables |\n|---|---|---|---|\n")
	}
	for _, name := range names {
		def := ""
		if slices.Contains(pkg.Features["default"], name) {
			def = cellYes
		}
		enables := make([]string, len(pkg.Features[name]))
		for i, e := range pkg.Features[name] {
			enables[i] = markdown.CodeSpan(e)
		}
		if on == nil {
			fmt.Fprintf(&b, "| %s | %s | %s |\n", markdown.CodeSpan(name), def, strings.Join(enables, ", "))
			continue
		}
		enabled := ""
		if on[name] {
			enabled = cellYes
		}
		fmt.Fprintf(&b, "| %s | %s | %s | %s |\n", markdown.CodeSpan(name), def, enabled, strings.Join(enables, ", "))
	}
	return b.String(), nil, nil
}

// A cratesHandler is a docgen:crates marker: the table of the crates of the
// Cargo workspace with their path, kind, version and the crates of the workspace
// that they depend on.
type cratesHandler struct {
	kind  string
	deps  string
	match string
}

const (
	depNormal = "normal"
	depAll    = "all"
)

// crateKinds are the kinds of target of a crate, as a docgen:crates marker
// shows them.
func crateKinds() []string { return []string{cargo.KindLib, "proc-macro", "bin"} }

func depKinds() []string { return []string{depNormal, "dev", "build", depAll} }

func (h *cratesHandler) Bind(a *marker.Args) {
	h.kind, _ = a.Optional("kind")
	h.deps, _ = a.Optional("deps")
	h.match, _ = a.Optional("match")
	a.NonEmpty("kind", "deps", "match")
	if h.kind != "" && !slices.Contains(crateKinds(), h.kind) {
		a.Fail("kind= is not " + strings.Join(crateKinds(), ", "))
	}
	if h.deps == "" {
		h.deps = depNormal
	}
	if !slices.Contains(depKinds(), h.deps) {
		a.Fail("deps= is not " + strings.Join(depKinds(), ", "))
	}
	if _, err := path.Match(h.match, ""); err != nil {
		a.Fail("match= is not a glob: " + err.Error())
	}
}

func (h *cratesHandler) plan(p *plan) {
	p.rust()
}

func (h *cratesHandler) fill(ctx context.Context, j job) (string, []outFile, error) {
	ws, err := j.cargoWorkspace(ctx)
	if err != nil {
		return "", nil, err
	}
	var b strings.Builder
	b.WriteString("| Crate | Path | Kind | Version | Depends on |\n|---|---|---|---|---|\n")
	rows := 0
	for _, p := range ws.Packages {
		kinds := packageKinds(p)
		if h.kind != "" && !slices.Contains(kinds, h.kind) {
			continue
		}
		if h.match != "" {
			if ok, _ := path.Match(h.match, p.Name); !ok {
				continue
			}
		}
		rel, err := filepath.Rel(j.mod.Root, p.Dir)
		if err != nil || !filepath.IsLocal(rel) && rel != "." {
			return "", nil, fmt.Errorf("crate %s is outside the repository", p.Name)
		}
		fmt.Fprintf(&b, "| %s | %s | %s | %s | %s |\n", markdown.CodeSpan(p.Name), markdown.CodeSpan(filepath.ToSlash(rel)), strings.Join(kinds, ", "), markdown.Cell(p.Version), h.dependencies(p))
		rows++
	}
	if rows == 0 {
		return "", nil, fmt.Errorf("no crate of the Cargo workspace matches the arguments of the marker")
	}
	return b.String(), nil, nil
}

// dependencies returns the cell of the workspace dependencies of p of the kinds
// of deps=, in name order. A dependency of kind dev or build has the kind after
// it with deps=all.
func (h *cratesHandler) dependencies(p *cargo.Package) string {
	type dep struct{ name, kind string }
	var deps []dep
	for _, d := range p.LocalDeps {
		if h.deps != depAll && d.Kind != h.deps {
			continue
		}
		if i := slices.IndexFunc(deps, func(e dep) bool { return e.name == d.Name }); i >= 0 {
			if deps[i].kind != depNormal && d.Kind == depNormal {
				deps[i].kind = "normal"
			}
			continue
		}
		deps = append(deps, dep{d.Name, d.Kind})
	}
	slices.SortFunc(deps, func(a, b dep) int { return strings.Compare(a.name, b.name) })
	cells := make([]string, len(deps))
	for i, d := range deps {
		cells[i] = markdown.CodeSpan(d.name)
		if h.deps == depAll && d.kind != depNormal {
			cells[i] += " (" + d.kind + ")"
		}
	}
	return strings.Join(cells, ", ")
}

// packageKinds returns the kinds of the library and binary targets of p, in the
// order of crateKinds.
func packageKinds(p *cargo.Package) []string {
	var kinds []string
	for _, k := range crateKinds() {
		if slices.ContainsFunc(p.Targets, func(t cargo.Target) bool { return t.Kind == k }) {
			kinds = append(kinds, k)
		}
	}
	return kinds
}

// rustUsageOf builds the binary bin of the crate pkg, runs it with args in a
// slot and returns its usage as a code block.
func (g *Generator) rustUsageOf(ctx context.Context, pkg, bin string, args []string, feats cargo.Features) (string, error) {
	ws, p, err := g.cargoPackage(ctx, pkg)
	if err != nil {
		return "", err
	}
	if bin, err = targetName(p, kindBin, bin); err != nil {
		return "", err
	}
	exe, err := g.cargoBuild(ctx, ws, pkg, kindBin, bin, feats)
	if err != nil {
		return "", err
	}
	run := pkg + " " + bin + " " + strings.Join(args, " ")
	fmt.Fprintf(g.log, "docgen: running %s\n", run)
	release, err := g.slots.acquire(ctx)
	if err != nil {
		return "", err
	}
	defer release()
	ctx, cancel := context.WithTimeout(ctx, gocmd.ToolTimeout)
	defer cancel()
	usage, err := g.runHelp(ctx, ws.Root, exe, bin, args)
	if err != nil {
		return "", fmt.Errorf("%s: %w", run, err)
	}
	return markdown.Fence("text", usage), nil
}

// targetName returns name, or the one target of kind of p when name is "".
// kind is bin or example. A name that is not a target of p is an error that
// lists them.
func targetName(p *cargo.Package, kind, name string) (string, error) {
	var names []string
	for _, t := range p.Targets {
		if t.Kind == kind {
			names = append(names, t.Name)
		}
	}
	slices.Sort(names)
	one, many := kind, kind+"s"
	if kind == kindBin {
		one, many = "binary", "binaries"
	}
	switch {
	case len(names) == 0:
		return "", fmt.Errorf("crate %s has no %s", p.Name, one)
	case name == "" && len(names) == 1:
		return names[0], nil
	case name == "":
		return "", fmt.Errorf("crate %s has several %s, so the marker needs %s=: %s", p.Name, many, kind, strings.Join(names, ", "))
	case !slices.Contains(names, name):
		return "", fmt.Errorf("crate %s has no %s %s; its %s are: %s", p.Name, one, name, many, strings.Join(names, ", "))
	}
	return name, nil
}
