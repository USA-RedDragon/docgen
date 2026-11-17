package cargo

import (
	"slices"
	"strings"
	"testing"
)

func featureWorkspace() *Workspace {
	a := &Package{Name: "a", Features: map[string][]string{
		"default": {"std"}, "std": {}, "big": {"std", "dep:x", "y/z", "w?/v"}, "w": {}, "y": {},
	}}
	b := &Package{Name: "b", Features: map[string][]string{"big": {}}}
	c := &Package{Name: "c"}
	return &Workspace{Root: "/ws", Packages: []*Package{a, b, c}, byName: map[string]*Package{"a": a, "b": b, "c": c}}
}

func TestSplitFeatures(t *testing.T) {
	t.Parallel()
	got, err := SplitFeatures("a, b/c\n d")
	if err != nil || !slices.Equal(got, []string{"a", "b/c", "d"}) {
		t.Errorf("got %v, %v", got, err)
	}
	for _, bad := range []string{"/a", "a/", "a/b/c", "a$"} {
		if _, err := SplitFeatures(bad); err == nil {
			t.Errorf("%q: no error", bad)
		}
	}
}

func TestFeaturesArgs(t *testing.T) {
	t.Parallel()
	ws := featureWorkspace()
	f := Features{List: []string{"big", "b/big", "a/std", "nope/x"}, All: true, NoDefault: true}
	for pkg, want := range map[string]string{
		"a": "--features big,std --all-features --no-default-features",
		"b": "--features big --all-features --no-default-features",
		"c": "--all-features --no-default-features",
	} {
		if got := strings.Join(f.Args(ws.byName[pkg]), " "); got != want {
			t.Errorf("%s: got %q, want %q", pkg, got, want)
		}
	}
	if got := (Features{}).Args(ws.byName["a"]); len(got) != 0 || !(Features{}).Zero() {
		t.Errorf("zero Features: %v", got)
	}
}

func TestFeaturesValidate(t *testing.T) {
	t.Parallel()
	ws := featureWorkspace()
	if err := (Features{List: []string{"big", "a/std", "b/big"}}).Validate(ws); err != nil {
		t.Error(err)
	}
	for _, c := range []struct{ entry, want string }{
		{"nope", "the features are: a: big, default, std, w, y; b: big"},
		{"a/nope", `crate a has no feature "nope"; its features are: big, default, std, w, y`},
		{"c/big", `crate c has no feature "big"; it has no features`},
		{"d/big", `crate "d" is not in the Cargo workspace`},
	} {
		err := (Features{List: []string{c.entry}}).Validate(ws)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: %v", c.entry, err)
		}
	}
}

func TestFeaturesEnabled(t *testing.T) {
	t.Parallel()
	a := featureWorkspace().byName["a"]
	names := func(f Features) []string {
		var out []string
		for n := range f.Enabled(a) {
			out = append(out, n)
		}
		slices.Sort(out)
		return out
	}
	for name, c := range map[string]struct {
		f    Features
		want []string
	}{
		"default":    {Features{}, []string{"default", "std"}},
		"no default": {Features{NoDefault: true}, nil},
		"big":        {Features{List: []string{"big"}}, []string{"big", "default", "std", "y"}},
		"all":        {Features{All: true, NoDefault: true}, []string{"big", "default", "std", "w", "y"}},
	} {
		if got := names(c.f); !slices.Equal(got, c.want) {
			t.Errorf("%s: got %v, want %v", name, got, c.want)
		}
	}
}

func TestFeaturesKey(t *testing.T) {
	t.Parallel()
	if (Features{List: []string{"b", "a", "a"}}).Key() != (Features{List: []string{"a", "b"}}).Key() {
		t.Error("the order and repeats of the list change the key")
	}
	if (Features{All: true}).Key() == (Features{NoDefault: true}).Key() {
		t.Error("All and NoDefault have one key")
	}
}
