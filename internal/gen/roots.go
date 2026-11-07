package gen

import (
	"path/filepath"
	"slices"
)

// A rootUser is a page and the canonical directories of the roots of its
// docgen:docgen markers.
type rootUser struct {
	pg   *page
	dirs []string
}

// holds reports whether a page with the canonical path key is in one of the
// canonical directories dirs.
func holds(dirs []string, key string) bool {
	return slices.ContainsFunc(dirs, func(dir string) bool {
		rel, err := filepath.Rel(dir, key)
		return err == nil && filepath.IsLocal(rel)
	})
}

// rootUsers returns the pages with the root of a docgen:docgen marker. A root
// that is not valid is an error of the marker and is not in the result, and so
// is a root that holds the page of its marker.
func (g *Generator) rootUsers(pages []*page) []rootUser {
	var users []rootUser
	for _, pg := range pages {
		var dirs []string
		for _, m := range pg.markers {
			h, ok := m.handler.(*docgenHandler)
			if !ok || h.check(g.mod) != nil {
				continue
			}
			if dir := canonical(g.mod.RootDir(h.root)); !holds([]string{dir}, pg.key) {
				dirs = append(dirs, dir)
			}
		}
		if len(dirs) > 0 {
			users = append(users, rootUser{pg, dirs})
		}
	}
	return users
}

// nestedRoots returns the canonical directories of the docgen:docgen roots of
// pages. A root of a page under another root is relative to that root, so it is
// not in the result. The pages of named stay out of each root. The roots of a
// page that no other root holds count first. When each remaining page is under
// a root of another remaining page, the first page in pages counts.
func (g *Generator) nestedRoots(pages []*page, named map[string]bool) []string {
	users := g.rootUsers(pages)
	var roots []string
	for {
		users = slices.DeleteFunc(users, func(u rootUser) bool { return leftOut(roots, named, u.pg) })
		if len(users) == 0 {
			return roots
		}
		i := max(slices.IndexFunc(users, func(u rootUser) bool {
			return named[u.pg.key] || !slices.ContainsFunc(users, func(o rootUser) bool { return holds(o.dirs, u.pg.key) })
		}), 0)
		roots = append(roots, users[i].dirs...)
		users = slices.Delete(users, i, i+1)
	}
}

// leftOut reports whether pg is under one of roots and not in named.
func leftOut(roots []string, named map[string]bool, pg *page) bool {
	return !named[pg.key] && holds(roots, pg.key)
}
