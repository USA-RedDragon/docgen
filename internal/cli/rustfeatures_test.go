package cli

import (
	"strings"
	"testing"
)

const featuresPage = `- <!-- docgen:const crate=feat-gate name=BIG --><!-- /docgen:const -->
- <!-- docgen:const crate=feat-gate name=extra::LEVEL --><!-- /docgen:const -->
- <!-- docgen:const crate=feat-gate name=BASE --><!-- /docgen:const -->

<!-- docgen:output crate=feat-gate example=sizes -->
<!-- /docgen:output -->

<!-- docgen:output crate=feat-gate doctest=BIG -->
<!-- /docgen:output -->

<!-- docgen:cargo field=features crate=feat-gate -->
<!-- /docgen:cargo -->
`

func TestRustFeaturesSetting(t *testing.T) {
	t.Parallel()
	needCargo(t)
	for name, c := range map[string]struct {
		args []string
		yaml string
	}{
		"flag":     {args: []string{"-features", "big,feat-gate/extra"}},
		"all":      {args: []string{"-all-features"}},
		"yaml":     {yaml: "features: big extra\n"},
		"yaml all": {yaml: "all-features: true\n"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			files := map[string]string{"docs/features.md": featuresPage}
			if c.yaml != "" {
				files[".docgen.yaml"] = c.yaml
			}
			root := rustRepo(t, files)
			if rc, log := docgen(t, root, append([]string{"update"}, c.args...)...); rc != 0 {
				t.Fatalf("rc %d, log:\n%s", rc, log)
			}
			got := page(t, root, "features.md")
			wantIn(t, got,
				"name=BIG -->`1024`<",
				"name=extra::LEVEL -->`3`<",
				"name=BASE -->`8`<",
				"base 8\nbig 1024",
				"| `big` |  | yes | `small` |",
				"| `extra` |  | yes |  |",
				"| `small` | yes | yes |  |",
			)
		})
	}
}

func TestRustFeaturesMarker(t *testing.T) {
	t.Parallel()
	needCargo(t)
	root := rustRepo(t, map[string]string{"docs/features.md": `- <!-- docgen:const crate=feat-gate name=BIG features=big --><!-- /docgen:const -->
- <!-- docgen:const crate=feat-gate name=extra::LEVEL features=extra --><!-- /docgen:const -->
- <!-- docgen:const crate=feat-gate name=BASE nodefaultfeatures=true --><!-- /docgen:const -->
- <!-- docgen:const crate=feat-gate name=BASE allfeatures=true --><!-- /docgen:const -->

<!-- docgen:cargo field=features crate=feat-gate nodefaultfeatures=true features=extra -->
<!-- /docgen:cargo -->
`})
	if rc, log := docgen(t, root, "update"); rc != 0 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
	got := page(t, root, "features.md")
	wantIn(t, got, "name=BIG features=big -->`1024`<", "features=extra -->`3`<", "| `extra` |  | yes |  |", "| `small` | yes |  |  |", "| `big` |  |  | `small` |")
}

func TestRustFeaturesOff(t *testing.T) {
	t.Parallel()
	needCargo(t)
	root := rustRepo(t, map[string]string{"docs/features.md": featuresPage})
	rc, log := docgen(t, root, "check")
	if rc != 1 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
	wantIn(t, log,
		"docs/features.md:1: docgen:const crate=feat-gate name=BIG: the value test of the library of crate feat-gate does not compile for BIG:",
		"the item is gated behind the `big` feature",
		"\nenable the feature with the features setting or features= of the marker",
		`docs/features.md:2: docgen:const crate=feat-gate name=extra::LEVEL: the value test of the library of crate feat-gate printed no value for extra::LEVEL; it is in #[cfg(feature = "extra")], which is false in the test build; enable the feature with the features setting or features= of the marker`,
		"cargo build --package feat-gate --example sizes",
		"requires the features: `big`",
	)
}

func TestRustFeaturesUnknown(t *testing.T) {
	t.Parallel()
	needCargo(t)
	for name, c := range map[string]struct {
		args []string
		want string
	}{
		"feature":       {[]string{"-features", "nope"}, `no crate of the Cargo workspace at `},
		"feature list":  {[]string{"-features", "nope"}, "feat-gate: big, default, extra, small"},
		"qualified":     {[]string{"-features", "feat-gate/nope"}, `crate feat-gate has no feature "nope"; its features are: big, default, extra, small`},
		"unknown crate": {[]string{"-features", "nope/big"}, `crate "nope" is not in the Cargo workspace`},
		"no features":   {[]string{"-features", "limits/big"}, `crate limits has no feature "big"; it has no features`},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := rustRepo(t, map[string]string{"docs/features.md": "<!-- docgen:const crate=feat-gate name=BASE --><!-- /docgen:const -->\n"})
			rc, log := docgen(t, root, append([]string{"check"}, c.args...)...)
			if rc != 1 {
				t.Fatalf("rc %d, log:\n%s", rc, log)
			}
			wantIn(t, log, c.want)
		})
	}
}

func TestRustFeaturesInvalid(t *testing.T) {
	t.Parallel()
	root := rustRepo(t, map[string]string{"docs/features.md": "Text.\n"})
	for name, c := range map[string]struct {
		args []string
		want string
	}{
		"slash":   {[]string{"-features", "a/b/c"}, `feature "a/b/c" is not valid`},
		"empty":   {[]string{"-features", "a/"}, `feature "a/" is not valid`},
		"letters": {[]string{"-features", "a$b"}, `feature "a$b" is not valid`},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			rc, log := docgen(t, root, append([]string{"check"}, c.args...)...)
			if rc != 1 || !strings.Contains(log, c.want) {
				t.Errorf("rc %d, log:\n%s", rc, log)
			}
		})
	}
}

func TestRustFeaturesMarkerErrors(t *testing.T) {
	t.Parallel()
	needCargo(t)
	root := rustRepo(t, map[string]string{
		"docs/a.md": "<!-- docgen:const crate=feat-gate name=BASE features=a/b/c --><!-- /docgen:const -->\n",
		"docs/b.md": "<!-- docgen:const crate=feat-gate name=BASE allfeatures=yes --><!-- /docgen:const -->\n",
		"docs/c.md": "<!-- docgen:doc crate=feat-gate features=big -->\n<!-- /docgen:doc -->\n",
	})
	rc, log := docgen(t, root, "check")
	if rc != 1 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
	wantIn(t, log, `features= feature "a/b/c" is not valid`, "allfeatures= is not true or false", "features")
}
