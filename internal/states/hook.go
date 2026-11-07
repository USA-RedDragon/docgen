package states

import (
	"bytes"
	"embed"
	"fmt"
	"sort"
	"text/template"

	"github.com/USA-RedDragon/docgen/internal/load"
)

const (
	hookFile     = "zz_docgen_states.go"
	hookTemplate = "hook.go.tmpl"
)

//go:embed hook.go.tmpl
var hookFS embed.FS

// hookData holds the values of the hook template for one package.
type hookData struct {
	Package  string
	Imports  []hookImport
	Machines []hookMachine
}

type hookImport struct {
	Alias string
	Path  string
}

// A hookMachine holds the hook values of one machine type. Each entry of Comps
// is one state component.
type hookMachine struct {
	Type  string
	Comps []hookComp
}

// A hookComp is one state component. Value is a Go expression of type uint64 on
// the receiver r, and Format the Go expression that appends a value v to b.
type hookComp struct {
	View   string
	Value  string
	Format string
}

func renderHook(d hookData) ([]byte, error) {
	t, err := template.ParseFS(hookFS, hookTemplate)
	if err != nil {
		return nil, err
	}
	var b bytes.Buffer
	if err := t.Execute(&b, d); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// hookSource adds the edits for the machines of r and returns the source of the
// hook file of pi.
func (r *recording) hookSource(pi *load.Package) ([]byte, error) {
	d := hookData{Package: pi.Pkg.Name}
	for _, typ := range r.order {
		d.Machines = append(d.Machines, r.instrument(pi, typ))
	}
	paths := make([]string, 0, len(r.im.alias))
	for p := range r.im.alias {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		d.Imports = append(d.Imports, hookImport{Alias: r.im.alias[p], Path: p})
	}
	src, err := renderHook(d)
	if err != nil {
		return nil, fmt.Errorf("state hook: %w", err)
	}
	return src, nil
}
