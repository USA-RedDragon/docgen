package gen

import (
	"context"

	"github.com/USA-RedDragon/docgen/internal/godoc"
	"github.com/USA-RedDragon/docgen/internal/gomod"
	"github.com/USA-RedDragon/docgen/internal/load"
	"github.com/USA-RedDragon/docgen/internal/marker"
)

// A docHandler is a docgen:doc marker: the doc of name in the package pkg, or
// the package doc, or the text below the heading section. A pkg outside the
// module is a directory of the repository.
type docHandler struct {
	pkg     string
	name    string
	section string
}

func (h *docHandler) Bind(a *marker.Args) {
	h.pkg = a.Required(argPkg)
	h.name, _ = a.Optional(argName)
	h.section, _ = a.Optional("section")
	a.NonEmpty(argName, "section")
}

func (h *docHandler) plan(p *plan) {
	h.pkg = p.shortPkg(h.pkg)
	if _, err := p.mod.PkgPath(h.pkg); err != nil && gomod.HasGo(p.mod.RootDir(h.pkg)) {
		return
	}
	p.load(h.pkg)
}

func (h *docHandler) fill(ctx context.Context, j job) (string, []outFile, error) {
	pi, err := j.loaded.Package(h.pkg)
	if err != nil {
		if _, perr := j.mod.PkgPath(h.pkg); perr == nil || !gomod.HasGo(j.mod.RootDir(h.pkg)) {
			return "", nil, err
		}
		if err := j.mod.LocalDir(argPkg, h.pkg); err != nil {
			return "", nil, err
		}
		if pi, err = load.RootPackage(j.mod, j.session.BuildTags(ctx), h.pkg); err != nil {
			return "", nil, err
		}
	}
	body, err := godoc.Markdown(pi, h.name, h.section)
	if err != nil {
		return "", nil, err
	}
	return body, nil, nil
}
