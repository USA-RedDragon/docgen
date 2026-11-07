package gen

import (
	"context"
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/USA-RedDragon/docgen/internal/excerpt"
	"github.com/USA-RedDragon/docgen/internal/gomod"
	"github.com/USA-RedDragon/docgen/internal/markdown"
	"github.com/USA-RedDragon/docgen/internal/marker"
)

// A docgenHandler is a docgen:docgen marker: the page at the path page of the
// directory root, before and after an update of a copy of root. module is the
// directory of the Go module of that update, relative to root.
type docgenHandler struct {
	root   string
	page   string
	module string
	err    error
	run    *nestedRun
}

func (h *docgenHandler) Bind(a *marker.Args) {
	h.root = a.Required("root")
	h.page = a.Required("page")
	h.module = "."
	if v, _ := a.Optional("module"); v != "" {
		h.module = v
	}
	a.NonEmpty("module")
}

func (h *docgenHandler) plan(p *plan) {
	if h.err = p.gen.checkDepth(); h.err != nil {
		return
	}
	if h.err = h.check(p.mod); h.err != nil {
		return
	}
	h.run = p.nestedRun(h.root, h.module)
	h.run.pages[h.pageKey()] = nil
}

// pageKey returns the page as a slash path in root.
func (h *docgenHandler) pageKey() string {
	return path.Clean(h.page)
}

// check returns an error when root is not a directory of the repository, or
// when page or module leave root.
func (h *docgenHandler) check(mod *gomod.Module) error {
	dir := mod.RootDir(h.root)
	if err := mod.LocalDir("root", h.root); err != nil {
		if name := gomod.GoLinkOutside(mod.Root, dir); name != "" {
			return fmt.Errorf("%s is a symbolic link outside root=%s", name, h.root)
		}
		return err
	}
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return fmt.Errorf("root=%s is not a directory", h.root)
	}
	for _, arg := range []struct{ key, value string }{{"page", h.page}, {"module", h.module}} {
		if !localTo(dir, arg.value) {
			return fmt.Errorf("%s=%s is outside root=%s", arg.key, arg.value, h.root)
		}
	}
	return nil
}

func (h *docgenHandler) fill(context.Context, job) (string, []outFile, error) {
	if h.err != nil {
		return "", nil, h.err
	}
	if h.run.err != nil {
		return "", nil, h.run.err
	}
	pg := h.run.pages[h.pageKey()]
	if pg == nil {
		return "", nil, fmt.Errorf("no page %s in root=%s", h.page, h.root)
	}
	if strings.TrimSpace(pg.after) == "" {
		return "", nil, fmt.Errorf("page %s of root=%s holds no text", h.page, h.root)
	}
	lang := excerpt.Lang(h.page)
	return "#### Before\n\n" + markdown.Fence(lang, pg.before) + "\n\n#### After\n\n" + markdown.Fence(lang, pg.after), nil, nil
}
