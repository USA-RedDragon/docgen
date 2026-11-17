package gen

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/USA-RedDragon/docgen/internal/excerpt"
	"github.com/USA-RedDragon/docgen/internal/gomod"
	"github.com/USA-RedDragon/docgen/internal/load"
	"github.com/USA-RedDragon/docgen/internal/marker"
	"github.com/USA-RedDragon/docgen/internal/types"
)

const (
	argCmd   = "cmd"
	argPkg   = "pkg"
	argType  = "type"
	argFile  = "file"
	argName  = "name"
	argPrint = "print"
	argCrate = "crate"
)

// typeArgs holds the pkg= and type= arguments of a marker.
type typeArgs struct {
	pkg string
	typ string
}

func (t *typeArgs) bindType(a *marker.Args) {
	t.pkg = a.Required(argPkg)
	t.typ = a.Required(argType)
}

// A handler is one marker of a page. Each kind of marker has a handler type.
// Bind reads and checks its arguments, plan adds its work to a plan, and fill
// returns its body and its output files.
type handler interface {
	marker.Binder
	plan(p *plan)
	fill(ctx context.Context, j job) (string, []outFile, error)
}

// A job is what a handler reads in fill: the generator, the results of the plan,
// the place of the marker, and the render pass.
type job struct {
	*Generator
	*prepared
	pg       *page
	m        marker.Marker
	pass     *pass
	readPage *bool
}

// repoPath returns the path of the file= value rel of a marker. A value that
// leaves the repository is an error.
func (j job) repoPath(rel string) (string, error) {
	return repoFile(j.mod, rel)
}

// repoFile returns the path of the file= value rel of a marker in the module
// mod. A value that leaves the repository is an error.
func repoFile(mod *gomod.Module, rel string) (string, error) {
	p := filepath.FromSlash(rel)
	if !filepath.IsLocal(p) {
		return "", fmt.Errorf("file=%s is outside the repository", rel)
	}
	full := filepath.Join(mod.Root, p)
	if !gomod.Inside(mod.Root, full) {
		return "", fmt.Errorf("file=%s is outside the repository", rel)
	}
	return full, nil
}

// source returns the text of the file at path with LF line endings. The text of
// a scanned page is the text of the last render pass, so that an excerpt holds
// what update writes to that page. The page of the marker itself has no such
// text, and is an error.
func (j job) source(path string) ([]byte, error) {
	key := canonical(path)
	if key == j.pg.key {
		return nil, fmt.Errorf("the excerpt is of the page of the marker, %s", j.pg.rel)
	}
	if text, ok := j.pass.texts[key]; ok {
		*j.readPage = true
		return excerpt.Source([]byte(text)), nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return excerpt.Source(b), nil
}

// A langHandler is a marker kind with a Go form and a Rust form, and for some
// kinds a C form. Bind picks the Rust form for a marker with crate=, the C form
// for one with file= when the kind has a C form, and the Go form for any other.
type langHandler struct {
	handler
	goH   handler
	rustH handler
	cH    handler
}

func (h *langHandler) Bind(a *marker.Args) {
	h.handler = h.goH
	if _, ok := a.Optional(argCrate); ok {
		h.handler = h.rustH
	}
	if _, ok := a.Optional(argFile); ok && h.cH != nil {
		h.handler = h.cH
	}
	h.handler.Bind(a)
}

// newHandler returns a new handler of a marker kind, and nil for an unknown
// kind.
func newHandler(kind string) handler {
	switch kind {
	case "output":
		return &outputHandler{}
	case "code":
		return &codeHandler{}
	case "states":
		return &langHandler{goH: &statesHandler{}, rustH: &rustStatesHandler{}}
	case "config":
		return &langHandler{goH: &configHandler{}, rustH: &rustConfigHandler{}}
	case "const":
		return &langHandler{goH: &constHandler{}, rustH: &rustConstHandler{}, cH: &cConstHandler{}}
	case "errors":
		return &langHandler{goH: &errorsHandler{}, rustH: &rustErrorsHandler{}}
	case "doc":
		return &langHandler{goH: &docHandler{}, rustH: &rustDocHandler{}}
	case "gomod":
		return &goModHandler{}
	case "cargo":
		return &cargoHandler{}
	case "crates":
		return &cratesHandler{}
	case "usage":
		return &usageHandler{}
	case "configulator":
		return &langHandler{goH: &configulatorHandler{}, rustH: &rustConfigulatorHandler{}}
	case "docgen":
		return &docgenHandler{}
	case "var":
		return &varHandler{}
	case "text":
		return &textHandler{}
	}
	return nil
}

// noPlan is the plan of a handler with no work before fill.
type noPlan struct{}

func (noPlan) plan(*plan) {}

// lookupType returns the type name of pi. A name that is not a type is an error.
func lookupType(pi *load.Package, name string) (types.Type, error) {
	t, ok := types.Lookup(pi, name)
	if !ok {
		return t, fmt.Errorf("%s is not a type of %s", name, pi.Path)
	}
	return t, nil
}
