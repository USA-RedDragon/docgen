package cargo

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

const (
	manifestName = "Cargo.toml"
	kindBin      = "bin"
	kindExample  = "example"
	kindTest     = "test"
	kindBench    = "bench"
	keyPackage   = "package"
	keyPath      = "path"
	keyName      = "name"
	keyBuild     = "build"
)

// ReadWorkspace reads the Cargo workspace whose manifest is dir/Cargo.toml
// from its manifests alone, with no cargo command, for a run with no cargo.
// It finds what Open finds: the members, with the globs of
// workspace.members and workspace.exclude, the name, version, edition and
// rust-version of each package with workspace inheritance, the targets that
// a manifest lists and the ones that cargo discovers in src/lib.rs,
// src/main.rs, src/bin, examples, tests, benches and build.rs, the features
// with one for each optional dependency that no feature names as dep:, and
// the path dependencies on other members. It reads no parent workspace above
// dir, no Cargo.lock and no registry.
func ReadWorkspace(dir string) (*Workspace, error) {
	root, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(filepath.Join(root, manifestName)); errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("no %s in %s; set module to the directory of the Cargo workspace", manifestName, root)
	}
	top, err := ReadManifest(filepath.Join(root, manifestName), manifestName)
	if err != nil {
		return nil, err
	}
	dirs, err := memberDirs(root, top)
	if err != nil {
		return nil, err
	}
	r := &manifestReader{root: root, top: top}
	var pkgs []*Package
	deps := map[*Package][]rawDep{}
	for _, d := range dirs {
		p, pd, err := r.readPackage(d)
		if err != nil {
			return nil, err
		}
		pkgs = append(pkgs, p)
		deps[p] = pd
	}
	w := newWorkspace(root, pkgs)
	for _, p := range pkgs {
		for _, d := range deps[p] {
			if q, ok := w.byName[d.pkg]; ok && d.path != "" && filepath.Clean(d.path) == q.Dir {
				p.LocalDeps = append(p.LocalDeps, Dependency{Name: d.pkg, Path: filepath.Clean(d.path), Kind: d.kind})
			}
		}
		slices.SortStableFunc(p.LocalDeps, func(a, b Dependency) int { return strings.Compare(a.Name, b.Name) })
	}
	return w, nil
}

// newWorkspace returns the workspace at root with the packages pkgs, sorted by
// name.
func newWorkspace(root string, pkgs []*Package) *Workspace {
	w := &Workspace{Root: root, Packages: pkgs, byName: map[string]*Package{}}
	slices.SortFunc(w.Packages, func(a, b *Package) int { return strings.Compare(a.Name, b.Name) })
	for _, p := range w.Packages {
		w.byName[p.Name] = p
	}
	return w
}

// memberDirs returns the directories of the members of the workspace with the
// manifest top at root: root itself when it has a package table, and each
// directory that a glob of workspace.members matches and no glob of
// workspace.exclude does.
func memberDirs(root string, top *Manifest) ([]string, error) {
	var dirs []string
	if _, ok := top.Doc[keyPackage]; ok {
		dirs = append(dirs, root)
	}
	wsTable, _ := top.Doc[keyWorkspace].(map[string]any)
	exclude := map[string]bool{}
	for _, e := range stringList(wsTable["exclude"]) {
		matches, err := filepath.Glob(filepath.Join(root, filepath.FromSlash(e)))
		if err != nil {
			return nil, fmt.Errorf("%s: workspace.exclude %q: %w", manifestName, e, err)
		}
		for _, m := range matches {
			exclude[m] = true
		}
	}
	for _, m := range stringList(wsTable["members"]) {
		matches, err := filepath.Glob(filepath.Join(root, filepath.FromSlash(m)))
		if err != nil {
			return nil, fmt.Errorf("%s: workspace.members %q: %w", manifestName, m, err)
		}
		if len(matches) == 0 && !strings.ContainsAny(m, "*?[") {
			return nil, fmt.Errorf("%s: workspace member %s has no %s", manifestName, m, manifestName)
		}
		for _, d := range matches {
			if exclude[d] || slices.Contains(dirs, d) {
				continue
			}
			if info, err := os.Stat(filepath.Join(d, manifestName)); err != nil || !info.Mode().IsRegular() {
				if strings.ContainsAny(m, "*?[") {
					continue
				}
				return nil, fmt.Errorf("%s: workspace member %s has no %s", manifestName, m, manifestName)
			}
			dirs = append(dirs, d)
		}
	}
	if len(dirs) == 0 {
		return nil, fmt.Errorf("%s in %s has no package and no workspace members", manifestName, root)
	}
	return dirs, nil
}

// stringList returns the strings of the TOML array v.
func stringList(v any) []string {
	items, _ := v.([]any)
	var out []string
	for _, it := range items {
		if s, ok := it.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// A manifestReader reads the packages of one workspace.
type manifestReader struct {
	root string
	top  *Manifest
}

// A rawDep is a dependency of a manifest: the name of the package, its
// absolute directory for a path dependency, and its kind.
type rawDep struct {
	pkg  string
	path string
	kind string
}

// workspaceManifest returns the manifest of the workspace, for the fields of a
// package that set workspace = true. It has the type of the ws argument of
// Manifest.Field, whose other callers read the manifest when it is needed.
func (r *manifestReader) workspaceManifest() (*Manifest, error) {
	return r.top, nil
}

// readPackage reads the package in the directory dir.
func (r *manifestReader) readPackage(dir string) (*Package, []rawDep, error) {
	rel, err := filepath.Rel(r.root, filepath.Join(dir, manifestName))
	if err != nil {
		return nil, nil, err
	}
	m := r.top
	if dir != r.root {
		if m, err = ReadManifest(filepath.Join(dir, manifestName), filepath.ToSlash(rel)); err != nil {
			return nil, nil, err
		}
	}
	name, err := m.Field("package.name", nil)
	if err != nil {
		return nil, nil, err
	}
	field := func(key, def string) (string, error) {
		pkg, _ := m.Doc[keyPackage].(map[string]any)
		if _, ok := pkg[key]; !ok {
			return def, nil
		}
		return m.Field("package."+key, r.workspaceManifest)
	}
	p := &Package{Name: name, ManifestPath: filepath.Join(dir, manifestName), Dir: dir}
	if p.Version, err = field("version", "0.0.0"); err != nil {
		return nil, nil, err
	}
	if p.Edition, err = field("edition", "2015"); err != nil {
		return nil, nil, err
	}
	if p.RustVersion, err = field("rust-version", ""); err != nil {
		return nil, nil, err
	}
	if p.Description, err = field("description", ""); err != nil {
		return nil, nil, err
	}
	if p.Targets, err = targets(m, p); err != nil {
		return nil, nil, err
	}
	deps, optional, err := r.dependencies(m, dir)
	if err != nil {
		return nil, nil, err
	}
	p.Features = features(m, optional)
	p.Registries = r.registries(m)
	return p, deps, nil
}

// registries returns the registries of the publish key of the manifest m, as
// cargo metadata reports them: nil when it is not set or true, empty for
// false, and else the names of the array. A publish that sets workspace =
// true takes workspace.package.publish.
func (r *manifestReader) registries(m *Manifest) []string {
	pkg, _ := m.Doc[keyPackage].(map[string]any)
	v := pkg["publish"]
	if t, ok := v.(map[string]any); ok && t[keyWorkspace] == true {
		wsTable, _ := r.top.Doc[keyWorkspace].(map[string]any)
		wsPkg, _ := wsTable[keyPackage].(map[string]any)
		v = wsPkg["publish"]
	}
	switch v := v.(type) {
	case bool:
		if !v {
			return []string{}
		}
	case []any:
		return append([]string{}, stringList(v)...)
	}
	return nil
}

// features returns the features of m, with a feature dep:name for each
// optional dependency name that no feature names as dep:name.
func features(m *Manifest, optional []string) map[string][]string {
	out := map[string][]string{}
	table, _ := m.Doc["features"].(map[string]any)
	named := map[string]bool{}
	for k, v := range table {
		out[k] = stringList(v)
		if out[k] == nil {
			out[k] = []string{}
		}
		for _, item := range out[k] {
			if d, ok := strings.CutPrefix(item, "dep:"); ok {
				named[d] = true
			}
		}
	}
	for _, d := range optional {
		if _, ok := out[d]; !ok && !named[d] {
			out[d] = []string{"dep:" + d}
		}
	}
	return out
}

// dependencies returns the dependencies of the manifest m in dir, from its
// dependency tables and those of each target. optional holds the names, as
// the manifest writes them, of the optional ones.
func (r *manifestReader) dependencies(m *Manifest, dir string) (deps []rawDep, optional []string, err error) {
	tables := []map[string]any{m.Doc}
	if targets, ok := m.Doc["target"].(map[string]any); ok {
		for _, k := range sortedKeys(targets) {
			if t, ok := targets[k].(map[string]any); ok {
				tables = append(tables, t)
			}
		}
	}
	kinds := []struct{ key, kind string }{
		{keyDependencies, "normal"},
		{"dev-dependencies", "dev"},
		{"dev_dependencies", "dev"},
		{"build-dependencies", keyBuild},
		{"build_dependencies", keyBuild},
	}
	for _, t := range tables {
		for _, k := range kinds {
			list, _ := t[k.key].(map[string]any)
			for _, key := range sortedKeys(list) {
				d, opt, err := r.dependency(m, dir, key, list[key])
				if err != nil {
					return nil, nil, err
				}
				d.kind = k.kind
				deps = append(deps, d)
				if opt && !slices.Contains(optional, key) {
					optional = append(optional, key)
				}
			}
		}
	}
	return deps, optional, nil
}

// dependency reads the dependency key with the value v of the manifest m in
// dir. A path is relative to dir, or to the workspace root for a dependency
// that sets workspace = true.
func (r *manifestReader) dependency(m *Manifest, dir, key string, v any) (rawDep, bool, error) {
	t, ok := v.(map[string]any)
	if !ok {
		return rawDep{pkg: key}, false, nil
	}
	base := dir
	if t[keyWorkspace] == true {
		wsTable, _ := r.top.Doc[keyWorkspace].(map[string]any)
		wsDeps, _ := wsTable[keyDependencies].(map[string]any)
		inherited, ok := wsDeps[key]
		if !ok {
			return rawDep{}, false, fmt.Errorf("%s: dependency %s is set to workspace = true, but %s has no workspace.dependencies.%s", m.Name, key, manifestName, key)
		}
		it, ok := inherited.(map[string]any)
		if !ok {
			it = map[string]any{"version": inherited}
		}
		t = mergeDependency(it, t)
		base = r.root
	}
	d := rawDep{pkg: key}
	if name, ok := t[keyPackage].(string); ok {
		d.pkg = name
	}
	if p, ok := t[keyPath].(string); ok {
		d.path = filepath.Join(base, filepath.FromSlash(p))
	}
	opt, _ := t["optional"].(bool)
	return d, opt, nil
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

// isFile reports whether p is a regular file.
func isFile(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.Mode().IsRegular()
}

// auto reports whether the package table of m leaves the discovery of
// targets in key, such as autobins, on.
func auto(m *Manifest, key string) bool {
	pkg, _ := m.Doc[keyPackage].(map[string]any)
	v, ok := pkg[key].(bool)
	return !ok || v
}

// targets returns the targets of the package p with the manifest m: the
// library, the binaries, the examples, the tests, the benches and the build
// script, in that order and each kind by name.
func targets(m *Manifest, p *Package) ([]Target, error) {
	var out []Target
	lib, ok, err := libTarget(m, p)
	if err != nil {
		return nil, err
	}
	if ok {
		out = append(out, lib)
	}
	for _, k := range []struct {
		table, kind, dir, auto string
	}{
		{kindBin, kindBin, "src/bin", "autobins"},
		{kindExample, kindExample, "examples", "autoexamples"},
		{kindTest, kindTest, "tests", "autotests"},
		{kindBench, kindBench, "benches", "autobenches"},
	} {
		list, err := kindTargets(m, p, k.table, k.kind, k.dir, auto(m, k.auto))
		if err != nil {
			return nil, err
		}
		out = append(out, list...)
	}
	if build := buildScript(m, p); build != "" {
		out = append(out, Target{Name: "build-script-build", Kind: "custom-build", Path: build, Edition: p.Edition})
	}
	return out, nil
}

// libTarget returns the library target of p: the lib table of m, or else
// src/lib.rs when autolib is on. ok is false when p has no library.
func libTarget(m *Manifest, p *Package) (lib Target, ok bool, err error) {
	t, has := m.Doc["lib"].(map[string]any)
	path := filepath.Join(p.Dir, "src", "lib.rs")
	if !has && (!auto(m, "autolib") || !isFile(path)) {
		return Target{}, false, nil
	}
	lib = Target{Name: strings.ReplaceAll(p.Name, "-", "_"), Kind: KindLib, Path: path, Edition: p.Edition}
	if v, ok := t[keyName].(string); ok {
		lib.Name = v
	}
	if v, ok := t[keyPath].(string); ok {
		lib.Path = filepath.Join(p.Dir, filepath.FromSlash(v))
	}
	if v, ok := t["edition"].(string); ok {
		lib.Edition = v
	}
	if v, ok := t["proc-macro"].(bool); ok && v {
		lib.Kind = "proc-macro"
	}
	if !isFile(lib.Path) {
		return Target{}, false, fmt.Errorf("%s: the library target %s has no file %s", m.Name, lib.Name, lib.Path)
	}
	return lib, true, nil
}

// kindTargets returns the targets of kind that m lists in the array of
// tables table, and with discover the ones that cargo finds in dir below the
// package: dir/name.rs and dir/name/main.rs, and src/main.rs for a binary.
func kindTargets(m *Manifest, p *Package, table, kind, dir string, discover bool) ([]Target, error) {
	var out []Target
	listed, _ := m.Doc[table].([]any)
	for _, item := range listed {
		t, _ := item.(map[string]any)
		name, _ := t[keyName].(string)
		if name == "" {
			return nil, fmt.Errorf("%s: a [[%s]] target has no name", m.Name, table)
		}
		tg := Target{Name: name, Kind: kind, Edition: p.Edition}
		if v, ok := t["edition"].(string); ok {
			tg.Edition = v
		}
		if v, ok := t[keyPath].(string); ok {
			tg.Path = filepath.Join(p.Dir, filepath.FromSlash(v))
		} else {
			tg.Path = inferPath(p, kind, dir, name)
		}
		if tg.Path == "" {
			return nil, fmt.Errorf("%s: the %s target %s has no path, and no file of the default paths exists", m.Name, kind, name)
		}
		out = append(out, tg)
	}
	if discover {
		for _, tg := range discovered(p, kind, dir) {
			if !slices.ContainsFunc(out, func(o Target) bool { return o.Name == tg.Name || o.Path == tg.Path }) {
				out = append(out, tg)
			}
		}
	}
	slices.SortFunc(out, func(a, b Target) int { return strings.Compare(a.Name, b.Name) })
	return out, nil
}

// inferPath returns the default path of the target name of kind, or "" when
// no such file exists.
func inferPath(p *Package, kind, dir, name string) string {
	var candidates []string
	if kind == kindBin && name == p.Name {
		candidates = append(candidates, filepath.Join(p.Dir, "src", "main.rs"))
	}
	base := filepath.Join(p.Dir, filepath.FromSlash(dir))
	candidates = append(candidates, filepath.Join(base, name+".rs"), filepath.Join(base, name, "main.rs"))
	for _, c := range candidates {
		if isFile(c) {
			return c
		}
	}
	return ""
}

// discovered returns the targets of kind that cargo finds in dir of p, and
// src/main.rs for a binary.
func discovered(p *Package, kind, dir string) []Target {
	var out []Target
	if kind == kindBin {
		if main := filepath.Join(p.Dir, "src", "main.rs"); isFile(main) {
			out = append(out, Target{Name: p.Name, Kind: kind, Path: main, Edition: p.Edition})
		}
	}
	base := filepath.Join(p.Dir, filepath.FromSlash(dir))
	entries, err := os.ReadDir(base)
	if err != nil {
		return out
	}
	for _, e := range entries {
		switch {
		case e.Type().IsRegular() && strings.HasSuffix(e.Name(), ".rs"):
			out = append(out, Target{Name: strings.TrimSuffix(e.Name(), ".rs"), Kind: kind, Path: filepath.Join(base, e.Name()), Edition: p.Edition})
		case e.IsDir():
			if main := filepath.Join(base, e.Name(), "main.rs"); isFile(main) {
				out = append(out, Target{Name: e.Name(), Kind: kind, Path: main, Edition: p.Edition})
			}
		}
	}
	return out
}

// buildScript returns the path of the build script of p, or "".
func buildScript(m *Manifest, p *Package) string {
	pkg, _ := m.Doc[keyPackage].(map[string]any)
	switch v := pkg[keyBuild].(type) {
	case bool:
		if !v {
			return ""
		}
	case string:
		return filepath.Join(p.Dir, filepath.FromSlash(v))
	}
	if path := filepath.Join(p.Dir, "build.rs"); isFile(path) {
		return path
	}
	return ""
}
