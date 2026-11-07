package gen

import (
	"context"
	"fmt"
	"strings"

	"github.com/USA-RedDragon/docgen/internal/excerpt"
	"github.com/USA-RedDragon/docgen/internal/markdown"
	"github.com/USA-RedDragon/docgen/internal/marker"
)

// A codeHandler is a docgen:code marker: the file at the repository path file,
// or the part of it at region, symbol or lines. With raw true, the excerpt keeps
// doc comments and region marks.
type codeHandler struct {
	noPlan
	file   string
	lines  string
	symbol string
	region string
	raw    bool
	warned bool
}

func (h *codeHandler) Bind(a *marker.Args) {
	h.file = a.Required(argFile)
	given := 0
	for _, o := range []struct {
		key string
		arg *string
	}{{"lines", &h.lines}, {"symbol", &h.symbol}, {"region", &h.region}} {
		v, ok := a.Optional(o.key)
		if ok {
			*o.arg = v
			given++
		}
	}
	if given > 1 {
		a.Fail("can have only one of lines=, symbol= and region=")
	}
	a.NonEmpty("lines", "symbol", "region")
	if v, ok := a.Optional("raw"); ok {
		if v != "true" {
			a.Fail("raw= can only be true")
		}
		h.raw = true
	}
	if h.raw && h.region != "" {
		a.Fail("raw=true does not apply to region=")
	}
}

func (h *codeHandler) fill(_ context.Context, j job) (string, []outFile, error) {
	rel := h.file
	path, err := j.repoPath(rel)
	if err != nil {
		return "", nil, err
	}
	src, err := j.source(path)
	if err != nil {
		return "", nil, err
	}
	if h.region != "" {
		ex, err := excerpt.Region(path, src, h.region)
		if err != nil {
			return "", nil, fmt.Errorf("%s: %w", rel, err)
		}
		return markdown.Fence(excerpt.Lang(path), ex), nil, nil
	}
	if h.symbol != "" {
		if h.raw {
			ex, err := excerpt.RawSymbol(path, src, h.symbol)
			if err != nil {
				return "", nil, err
			}
			return markdown.Fence(excerpt.Lang(path), ex), nil, nil
		}
		ex, err := excerpt.Symbol(path, src, h.symbol)
		if err != nil {
			return "", nil, err
		}
		return markdown.Fence(excerpt.Lang(path), excerpt.DropRegionLines(ex)), nil, nil
	}
	if h.lines == "" {
		text := strings.TrimRight(string(src), "\n")
		if !h.raw {
			text = excerpt.DropRegionLines(text)
		}
		if strings.TrimSpace(text) == "" {
			return "", nil, fmt.Errorf("%s holds no text", rel)
		}
		return markdown.Fence(excerpt.Lang(path), text), nil, nil
	}
	if !h.warned {
		h.warned = true
		fmt.Fprintf(j.log, "docgen: %s:%d: docgen:code uses lines=; symbol= does not break when the file changes\n", j.pg.rel, j.m.Line)
	}
	cut := excerpt.Lines
	if h.raw {
		cut = excerpt.LinesRaw
	}
	want, err := cut(path, rel, src, h.lines, strings.Trim(j.m.Body, "\n"))
	if err != nil {
		return "", nil, err
	}
	return want, nil, nil
}
