package gen

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/USA-RedDragon/docgen/internal/cargo"
	"github.com/USA-RedDragon/docgen/internal/goprint"
	"github.com/USA-RedDragon/docgen/internal/marker"
	"github.com/USA-RedDragon/docgen/internal/rustsrc"
)

// A rustSet holds what the Rust markers of a Generator share: the runner of
// the cargo commands, the Cargo workspace in the module directory, which opens
// on first use, and the syn helper, which nested runs share too. A nested run
// has its own rustSet, with its own session and module directory.
type rustSet struct {
	runner *cargo.Runner
	dir    string
	helper *rustsrc.Helper
	once   sync.Once
	ws     *cargo.Workspace
	err    error
}

// newRustSet returns the rustSet of g, with the helper h. Each cargo command
// takes one slot of g, and the Rust work of a plan holds none of its own.
func (g *Generator) newRustSet(h *rustsrc.Helper) *rustSet {
	return &rustSet{
		runner: &cargo.Runner{Session: g.session, Jobs: g.session.Jobs, Acquire: g.slots.acquire},
		dir:    g.mod.Dir,
		helper: h,
	}
}

// cargoRunner returns the runner of the cargo commands of g.
func (g *Generator) cargoRunner() *cargo.Runner {
	return g.rust.runner
}

// cargoWorkspace returns the Cargo workspace in the module directory of g. A
// Generator reads it once.
func (g *Generator) cargoWorkspace(ctx context.Context) (*cargo.Workspace, error) {
	g.rust.once.Do(func() {
		g.rust.ws, g.rust.err = g.rust.runner.Open(ctx, g.rust.dir)
	})
	return g.rust.ws, g.rust.err
}

// rustPrint returns the print= value of a Rust marker with the argument arg:
// the print setting of g when arg is "", where raw is raw and go and config are
// the default.
func (g *Generator) rustPrint(arg string) string {
	if arg == "" && g.printAs == goprint.PrintRaw {
		return rustsrc.PrintRaw
	}
	return arg
}

// bindCrate reads crate= of a Rust marker. pkg= with it is an error.
func bindCrate(a *marker.Args) string {
	crate := a.Required(argCrate)
	if _, ok := a.Optional(argPkg); ok {
		a.Fail("has both crate= and pkg=; crate= is for Rust and pkg= is for Go")
	}
	return crate
}

// A rustValue is a value that a test on a copy of a crate prints: the value of
// a const or static, or with table the configulator table of a type.
type rustValue struct {
	crate   string
	name    string
	printAs string
	table   *rustsrc.Table
	text    string
	err     error
}

// A rustPlan holds the Rust work of a plan: the crates to read and the values
// to print. Its results are ready when prepare returns.
type rustPlan struct {
	crates map[string]bool
	values []*rustValue
	read   map[string]*rustCrate
	err    error
}

// A rustCrate is the result of the read of one crate.
type rustCrate struct {
	pkg   *cargo.Package
	bin   string
	crate *rustsrc.Crate
	err   error
}

// rust returns the Rust plan of p, and makes it on first use.
func (p *plan) rust() *rustPlan {
	if p.rustWork == nil {
		p.rustWork = &rustPlan{crates: map[string]bool{}}
	}
	return p.rustWork
}

// readCrate adds the crate to the crates to read.
func (rp *rustPlan) readCrate(crate string) {
	rp.crates[crate] = true
}

// value adds v to the values to print.
func (rp *rustPlan) value(v *rustValue) *rustValue {
	rp.crates[v.crate] = true
	rp.values = append(rp.values, v)
	return v
}

// crate returns the read of the crate name, or the error of the read.
func (rp *rustPlan) crate(name string) (*rustCrate, error) {
	if rp.err != nil {
		return nil, rp.err
	}
	c := rp.read[name]
	if c == nil {
		return nil, fmt.Errorf("crate %s was not read", name)
	}
	if c.err != nil {
		return nil, c.err
	}
	return c, nil
}

// lookup returns the item or module name of the crate.
func (rp *rustPlan) lookup(crate, name string) (*rustCrate, rustsrc.Found, error) {
	c, err := rp.crate(crate)
	if err != nil {
		return nil, rustsrc.Found{}, err
	}
	f, err := c.crate.Lookup(crate, name)
	return c, f, err
}

// targetOf returns the root file of the target of pkg that a marker reads: the
// library, or else the binary with the name of the package, or else its one
// binary. bin is "" for the library.
func targetOf(pkg *cargo.Package) (root, bin string, err error) {
	if lib := pkg.Lib(); lib != nil {
		return lib.Path, "", nil
	}
	var bins []cargo.Target
	for _, t := range pkg.Targets {
		if t.Kind == "bin" {
			bins = append(bins, t)
		}
	}
	for _, t := range bins {
		if t.Name == pkg.Name {
			return t.Path, t.Name, nil
		}
	}
	if len(bins) == 1 {
		return bins[0].Path, bins[0].Name, nil
	}
	return "", "", fmt.Errorf("crate %s has no library and no binary named %s", pkg.Name, pkg.Name)
}

// rustPrepare reads the crates of rp and prints its values. Each cargo command
// takes a slot of g.
func (g *Generator) rustPrepare(ctx context.Context, rp *rustPlan) {
	rp.read = map[string]*rustCrate{}
	ws, err := g.cargoWorkspace(ctx)
	if err != nil {
		rp.err = err
		return
	}
	r := g.cargoRunner()
	var roots []string
	var names []string
	for _, name := range sortedKeys(rp.crates) {
		c := &rustCrate{}
		rp.read[name] = c
		if c.pkg, c.err = ws.Package(name); c.err != nil {
			continue
		}
		var root string
		if root, c.bin, c.err = targetOf(c.pkg); c.err != nil {
			continue
		}
		roots = append(roots, root)
		names = append(names, name)
	}
	if len(roots) == 0 {
		return
	}
	bin, err := g.rust.helper.Path(ctx, r)
	if err != nil {
		rp.err = err
		return
	}
	crates, err := rustsrc.Read(ctx, r, bin, roots)
	if err != nil {
		rp.err = err
		return
	}
	for i, name := range names {
		c := rp.read[name]
		c.crate = crates[i]
		if len(c.crate.Errors) > 0 {
			c.err = fmt.Errorf("crate %s: %s", name, strings.Join(c.crate.Errors, "; "))
		}
		for _, w := range c.crate.Warnings {
			fmt.Fprintf(g.log, "docgen: crate %s: %s\n", name, g.repoText(w))
		}
	}
	g.rustValues(ctx, r, ws, rp)
}

// repoText returns s with each path in the repository relative to its root.
func (g *Generator) repoText(s string) string {
	for _, root := range []string{canonical(g.mod.Root), g.mod.Root} {
		s = strings.ReplaceAll(s, root+string(filepath.Separator), "")
	}
	return s
}

// rustValues prints the values of rp with one cargo test for each crate.
func (g *Generator) rustValues(ctx context.Context, r *cargo.Runner, ws *cargo.Workspace, rp *rustPlan) {
	targets := map[string]*rustsrc.Target{}
	links := map[*rustsrc.Request]*rustValue{}
	for _, v := range rp.values {
		req, err := rp.request(v)
		if err != nil {
			v.err = err
			continue
		}
		c := rp.read[v.crate]
		t := targets[v.crate]
		if t == nil {
			t = &rustsrc.Target{Package: v.crate, Bin: c.bin}
			targets[v.crate] = t
		}
		t.Requests = append(t.Requests, req)
		links[req] = v
	}
	if len(targets) == 0 {
		return
	}
	list := make([]*rustsrc.Target, 0, len(targets))
	for _, name := range sortedKeys(targets) {
		list = append(list, targets[name])
	}
	work, err := g.session.WorkDir("rust-values-")
	if err == nil {
		err = rustsrc.RunValues(ctx, r, ws.Root, work, list)
	}
	for req, v := range links {
		if err != nil {
			v.err = err
			continue
		}
		v.text, v.err = req.Text, req.Err
	}
}

// request returns the test request of v.
func (rp *rustPlan) request(v *rustValue) (*rustsrc.Request, error) {
	c, f, err := rp.lookup(v.crate, v.name)
	if err != nil {
		return nil, err
	}
	if v.table != nil {
		if err := configStruct(f, v.name, v.table); err != nil {
			return nil, err
		}
		return &rustsrc.Request{Module: f.Module, Name: f.Path, Expr: "super::" + f.Item.Name, Table: v.table, Cfg: f.Cfg()}, nil
	}
	if err := checkValueItem(f, v.name); err != nil {
		return nil, err
	}
	req := &rustsrc.Request{Module: f.Module, Name: f.Path, Expr: "super::" + f.Item.Name, Print: v.printAs, Cfg: f.Cfg()}
	if v.printAs == rustsrc.PrintName {
		req.Names = nameCandidates(c.crate, f)
	}
	return req, nil
}

// checkValueItem returns an error when f is not a const or a static that is
// not mut.
func checkValueItem(f rustsrc.Found, name string) error {
	if f.Item == nil || f.Item.Kind != rustsrc.KindConst && f.Item.Kind != rustsrc.KindStatic {
		return fmt.Errorf("%s is a %s, not a const or static", name, f.Kind())
	}
	if f.Item.Mut {
		return fmt.Errorf("%s is a static mut, which has no value to print", name)
	}
	return nil
}

// nameCandidates returns the consts of the crate other than f with the type of
// f, as written, that the module of f can name.
func nameCandidates(c *rustsrc.Crate, f rustsrc.Found) []rustsrc.Candidate {
	want := strings.Join(strings.Fields(f.Item.Type), "")
	var out []rustsrc.Candidate
	for _, m := range c.Modules {
		for _, it := range m.Items {
			if it.Kind != rustsrc.KindConst || it == f.Item || strings.Join(strings.Fields(it.Type), "") != want {
				continue
			}
			if !visibleFrom(m, it, f.Module) {
				continue
			}
			p := joinRust(m.Path, it.Name)
			out = append(out, rustsrc.Candidate{Expr: "crate::" + p, Name: p})
		}
	}
	slices.SortFunc(out, func(a, b rustsrc.Candidate) int { return strings.Compare(a.Name, b.Name) })
	return out
}

func joinRust(module, name string) string {
	if module == "" {
		return name
	}
	return module + "::" + name
}

// within reports whether the module from is m or a module inside it.
func within(m, from *rustsrc.Module) bool {
	return m.Path == "" || from.Path == m.Path || strings.HasPrefix(from.Path, m.Path+"::")
}

// visibleFrom reports whether code in the module from can name the item it of
// the module m: each module on the way and the item are public, or private
// in a module that holds from.
func visibleFrom(m *rustsrc.Module, it *rustsrc.Item, from *rustsrc.Module) bool {
	if it.Vis == "" && !within(m, from) {
		return false
	}
	for cur := m; cur != nil && cur.Path != ""; cur = cur.Parent() {
		if cur.Vis == "" && !within(cur.Parent(), from) {
			return false
		}
	}
	return true
}
