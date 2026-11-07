package states

import (
	"bufio"
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
	"go/ast"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/USA-RedDragon/docgen/internal/gocmd"
	"github.com/USA-RedDragon/docgen/internal/load"
	"github.com/USA-RedDragon/docgen/internal/types"
)

type edit struct {
	off  int
	text string
}

type recording struct {
	run    *stateRun
	im     *imports
	byType map[string][]*machine
	order  []string
	runRe  []string
	edits  map[string][]edit
	hook   []byte
}

func (rc *Recorder) record(ctx context.Context, ip string, want map[string][]string) (*stateRun, error) {
	pi := rc.loaded.ByPath(ip)
	r, err := resolveAll(pi, want)
	if err != nil {
		return nil, err
	}
	if r.hook, err = r.hookSource(pi); err != nil {
		return nil, err
	}
	work, err := rc.session.WorkDir("overlay-")
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.RemoveAll(work) }()
	ovPath, err := r.writeOverlay(work, filepath.Dir(pi.Pkg.GoFiles[0]))
	if err != nil {
		return nil, err
	}
	rec := filepath.Join(work, "states.txt")
	if err := rc.runRecorded(ctx, ip, ovPath, r.runRe, rec); err != nil {
		return nil, err
	}
	return r.run, readRecord(rec, r.run)
}

func resolveAll(pi *load.Package, want map[string][]string) (*recording, error) {
	r := &recording{
		run:    &stateRun{machines: map[string]*machine{}},
		im:     &imports{here: pi.Pkg.Types, alias: map[string]string{}},
		byType: map[string][]*machine{},
		edits:  map[string][]edit{},
	}
	names := make([]string, 0, len(want))
	seen := map[string]bool{}
	for name, runs := range want {
		names = append(names, name)
		for _, re := range runs {
			if !seen[re] {
				seen[re] = true
				r.runRe = append(r.runRe, re)
			}
		}
	}
	sort.Strings(names)
	sort.Strings(r.runRe)
	if seen[""] {
		r.runRe = nil
	}
	for _, name := range names {
		m, err := resolveView(pi, name, r.im)
		if err != nil {
			return nil, err
		}
		if t, ok := types.Lookup(pi, m.typ); ok && types.Generic(t) {
			return nil, fmt.Errorf("%s is generic", m.typ)
		}
		if r.byType[m.typ] == nil {
			r.order = append(r.order, m.typ)
		}
		r.byType[m.typ] = append(r.byType[m.typ], m)
		r.run.machines[name] = m
	}
	return r, nil
}

// instrument adds the edits for the exported methods of typ and returns
// the hook values of typ.
func (r *recording) instrument(pi *load.Package, typ string) hookMachine {
	views := r.byType[typ]
	observed := map[string]bool{"String": true}
	hm := hookMachine{Type: typ}
	for _, m := range views {
		if m.accessor != "" {
			observed[m.accessor] = true
		}
		for _, o := range m.observers {
			observed[o.name] = true
		}
		vals, fs := m.stateComps()
		for i := range vals {
			hm.Comps = append(hm.Comps, hookComp{View: m.name, Value: vals[i], Format: fs[i]})
		}
	}
	var methods, skipped, valueRecv []string
	for _, f := range pi.Source {
		file := pi.Pkg.Fset.File(f.Pos()).Name()
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || observed[fd.Name.Name] {
				continue
			}
			name, pointer := recvName(fd)
			if resolvedName(pi, name) != typ {
				continue
			}
			if !pointer {
				valueRecv = append(valueRecv, fd.Name.Name)
				continue
			}
			if len(fd.Recv.List[0].Names) == 0 || fd.Recv.List[0].Names[0].Name == "_" {
				skipped = append(skipped, fd.Name.Name)
				continue
			}
			rv := fd.Recv.List[0].Names[0].Name
			off := pi.Pkg.Fset.Position(fd.Body.Lbrace).Offset + 1
			r.edits[file] = append(r.edits[file], edit{off, fmt.Sprintf(" if __dgR, __dgS, __dgOK := __dgIn%s(%s); __dgOK { defer __dgOut%s(__dgR, __dgS, %q) } else if __dgR != nil { defer __dgDown%s(__dgR) };", typ, rv, typ, fd.Name.Name, typ)})
			methods = append(methods, fd.Name.Name)
		}
	}
	sort.Strings(methods)
	sort.Strings(valueRecv)
	promoted := promotedMethods(pi, typ, observed)
	for _, m := range views {
		m.methods, m.skipped, m.valueRecv, m.promoted = methods, skipped, valueRecv, promoted
	}
	return hm
}

// promotedMethods returns the exported methods of the type typ of pi that come from
// an embedded field, other than those in observed. The hook adds its statement to
// the methods of typ, and these have their source in another type.
func promotedMethods(pi *load.Package, typ string, observed map[string]bool) []string {
	t, ok := types.Lookup(pi, typ)
	if !ok {
		return nil
	}
	var out []string
	for _, f := range types.Methods(t) {
		if f.Promoted && !observed[f.Name] {
			out = append(out, f.Name)
		}
	}
	return out
}

// recvName returns the receiver type name of an exported method with a body, and
// whether the receiver is a pointer. The name is "" for any other func.
func recvName(fd *ast.FuncDecl) (string, bool) {
	if fd.Recv == nil || fd.Body == nil || !fd.Name.IsExported() {
		return "", false
	}
	recv := fd.Recv.List[0].Type
	star, pointer := recv.(*ast.StarExpr)
	if pointer {
		recv = star.X
	}
	id, ok := recv.(*ast.Ident)
	if !ok {
		return "", false
	}
	return id.Name, pointer
}

// resolvedName returns the name of the type of pi that the receiver type name
// names: an alias has the name of its type.
func resolvedName(pi *load.Package, name string) string {
	if t, ok := types.Lookup(pi, name); ok {
		if target, ok := aliasTarget(pi, t); ok {
			return target
		}
	}
	return name
}

func (r *recording) writeOverlay(work, pkgDir string) (string, error) {
	root, err := os.OpenRoot(work)
	if err != nil {
		return "", err
	}
	defer root.Close()
	write := func(name string, data []byte) error {
		f, err := root.OpenFile(name, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
		if err != nil {
			return err
		}
		if _, err := f.Write(data); err != nil {
			_ = f.Close()
			return err
		}
		return f.Close()
	}
	ov := map[string]string{}
	files := make([]string, 0, len(r.edits))
	for file := range r.edits {
		files = append(files, file)
	}
	sort.Strings(files)
	for n, file := range files {
		es := r.edits[file]
		src, err := os.ReadFile(file)
		if err != nil {
			return "", err
		}
		sort.Slice(es, func(i, j int) bool { return es[i].off > es[j].off })
		for _, e := range es {
			src = append(src[:e.off:e.off], append([]byte(e.text), src[e.off:]...)...)
		}
		name := fmt.Sprintf("%d.go", n+1)
		if err := write(name, src); err != nil {
			return "", err
		}
		ov[file] = filepath.Join(work, name)
	}
	if err := write(hookFile, r.hook); err != nil {
		return "", err
	}
	ov[filepath.Join(pkgDir, hookFile)] = filepath.Join(work, hookFile)
	ovJSON, err := json.Marshal(map[string]map[string]string{"Replace": ov}, json.Deterministic(true), jsontext.EscapeForHTML(true))
	if err != nil {
		return "", err
	}
	return filepath.Join(work, "overlay.json"), write("overlay.json", ovJSON)
}

func (rc *Recorder) runRecorded(ctx context.Context, ip, ovPath string, runRe []string, rec string) error {
	args := []string{"test", "-count=1", "-overlay", ovPath}
	if len(runRe) > 0 {
		args = append(args, "-run", strings.Join(runRe, "|"))
	}
	args = append(args, ip)
	fmt.Fprintf(rc.log, "docgen: recording the states of %s\n", rc.mod.ShortPath(ip))
	ctx, cancel := context.WithTimeout(ctx, gocmd.RecordTimeout)
	defer cancel()
	env, err := rc.session.Env(ctx, "DOCGEN_STATES_OUT="+rec)
	if err != nil {
		return err
	}
	out, errOut, err := gocmd.Run(ctx, rc.mod.Dir, env, args...)
	if err != nil {
		return fmt.Errorf("go test with the state hook: %w\n%s%s", err, out, errOut)
	}
	return nil
}

func readRecord(rec string, run *stateRun) error {
	f, err := os.Open(rec)
	if err != nil {
		return fmt.Errorf("the tests recorded no state: %w", err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fs := strings.Split(sc.Text(), "\t")
		if len(fs) < 3 {
			continue
		}
		m := run.machines[fs[1]]
		if m == nil {
			continue
		}
		switch {
		case fs[0] == "init" && len(fs) == 3:
			m.initial[fs[2]] = true
		case fs[0] == "edge" && len(fs) == 5:
			k := [2]string{fs[2], fs[4]}
			if m.edges[k] == nil {
				m.edges[k] = map[string]bool{}
			}
			m.edges[k][fs[3]] = true
		}
	}
	return sc.Err()
}
