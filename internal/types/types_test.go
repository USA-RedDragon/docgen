package types

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/USA-RedDragon/docgen/internal/gocmd"
	"github.com/USA-RedDragon/docgen/internal/gomod"
	"github.com/USA-RedDragon/docgen/internal/load"
)

// loadSource writes the source files of package p of the module example.com/m to
// a temporary directory and loads it. Files with a name that holds a slash are
// other packages of the module.
func loadSource(t *testing.T, files map[string]string) *load.Package {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/m\n\ngo 1.24.0\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for name, text := range files {
		path := filepath.Join(root, "p", filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	mod, err := gomod.Open(root, root)
	if err != nil {
		t.Fatal(err)
	}
	session := &gocmd.Session{}
	defer session.Close()
	set, err := load.Packages(t.Context(), mod, session, map[string]bool{"example.com/m/p": true})
	if err != nil {
		t.Fatal(err)
	}
	pi, err := set.Package("p")
	if err != nil {
		t.Fatal(err)
	}
	return pi
}

func lookup(t *testing.T, pi *load.Package, name string) Type {
	t.Helper()
	typ, ok := Lookup(pi, name)
	if !ok {
		t.Fatalf("Lookup(%s) reports no type", name)
	}
	return typ
}

const typeSource = `package p

// Doc is a type with a doc.
type Doc struct{}

type Bare int

// Alias is an alias.
type Alias = Doc

// Gen is generic.
type Gen[T any] struct{ V T }

// GenAlias is an alias of an instance.
type GenAlias = Gen[int]

type hidden int

const NotType = 1
`

func TestLookup(t *testing.T) {
	t.Parallel()
	pi := loadSource(t, map[string]string{"p.go": typeSource})
	for _, name := range []string{"Nope", "NotType"} {
		if _, ok := Lookup(pi, name); ok {
			t.Errorf("Lookup(%s) reports a type", name)
		}
	}
	doc := lookup(t, pi, "Doc")
	if doc.Name != "Doc" || doc.Doc == nil || doc.Doc.Doc != "Doc is a type with a doc.\n" {
		t.Errorf("Lookup(Doc) = %+v", doc)
	}
	if alias := lookup(t, pi, "Alias"); alias.Doc == nil || alias.Go == doc.Go {
		t.Errorf("Lookup(Alias) = %+v, want the alias with its doc", alias)
	}
	if bare := lookup(t, pi, "hidden"); bare.Name != "hidden" {
		t.Errorf("Lookup(hidden) = %+v", bare)
	}
}

func TestOf(t *testing.T) {
	t.Parallel()
	pi := loadSource(t, map[string]string{"p.go": typeSource})
	n := named(lookup(t, pi, "Doc").Go)
	if got := Of(pi, n); got.Name != "Doc" || got.Go != n || got.Doc == nil {
		t.Errorf("Of(pi, Doc) = %+v", got)
	}
	if got := Of(nil, n); got.Name != "Doc" || got.Doc != nil {
		t.Errorf("Of(nil, Doc) = %+v, want no doc", got)
	}
}

func TestExportedTypes(t *testing.T) {
	t.Parallel()
	pi := loadSource(t, map[string]string{"p.go": typeSource})
	types := ExportedTypes(pi)
	got := make([]string, 0, len(types))
	for _, typ := range types {
		got = append(got, typ.Name)
	}
	if want := []string{"Alias", "Bare", "Doc", "Gen", "GenAlias"}; !slices.Equal(got, want) {
		t.Errorf("ExportedTypes = %v, want %v", got, want)
	}
}

func TestGeneric(t *testing.T) {
	t.Parallel()
	pi := loadSource(t, map[string]string{"p.go": typeSource})
	for name, want := range map[string]bool{"Gen": true, "GenAlias": false, "Doc": false, "Alias": false} {
		if got := Generic(lookup(t, pi, name)); got != want {
			t.Errorf("Generic(%s) = %v, want %v", name, got, want)
		}
	}
}
