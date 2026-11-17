package cli

import (
	"testing"
)

const (
	cargoTag   = "Signature: 8a477f597d28d172789f06886806bc55\n# This file is a cache directory tag created by cargo.\n"
	otherTag   = "Signature: 8a477f597d28d172789f06886806bc55\n# This file is a cache directory tag created by another tool.\n"
	badMarker  = "<!-- docgen:const pkg=nope name=X --><!-- /docgen:const -->\n"
	lonelyArea = "# docgen:region lonely\ntext\n# docgen:end\n"
)

func TestScanSkipsCargoTargets(t *testing.T) {
	t.Parallel()
	root := unusedRepo(t, map[string]string{
		"Cargo.toml":                "[workspace]\n",
		"target/doc.md":             badMarker,
		"target/debug/a.sh":         lonelyArea,
		"crates/a/Cargo.toml":       "[package]\n",
		"crates/a/target/doc.md":    badMarker,
		"build-out/CACHEDIR.TAG":    cargoTag,
		"build-out/doc.md":          badMarker,
		"build-out/sub/a.sh":        lonelyArea,
		"docs/target/Cargo.toml":    "[package]\n",
		"docs/target/target/doc.md": badMarker,
	})
	rc, log := docgen(t, root, "check")
	if rc != 0 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
	wantNotIn(t, log, "target", "build-out")
}

func TestScanKeepsOtherTargets(t *testing.T) {
	t.Parallel()
	for name, files := range map[string]map[string]string{
		"a target with no Cargo.toml":                   {"target/doc.md": badMarker},
		"a target next to a directory named Cargo.toml": {"Cargo.toml/x": "x\n", "target/doc.md": badMarker},
		"a name other than target":                      {"Cargo.toml": "[package]\n", "targets/doc.md": badMarker},
		"a tag of another tool":                         {"out/CACHEDIR.TAG": otherTag, "out/doc.md": badMarker},
		"an empty tag":                                  {"out/CACHEDIR.TAG": "", "out/doc.md": badMarker},
	} {
		root := unusedRepo(t, files)
		rc, log := docgen(t, root, "check")
		if rc != 1 {
			t.Errorf("%s: rc %d, log:\n%s", name, rc, log)
		}
		wantIn(t, log, "doc.md")
	}
}

func TestRegionAuditSkipsCargoTargets(t *testing.T) {
	t.Parallel()
	root := unusedRepo(t, map[string]string{
		"Cargo.toml":         "[workspace]\n",
		"target/debug/a.sh":  lonelyArea,
		"out/CACHEDIR.TAG":   cargoTag,
		"out/a.sh":           lonelyArea,
		"target2/Cargo.toml": "x\n",
		"target2/kept/a.sh":  lonelyArea,
	})
	rc, log := docgen(t, root, "check")
	if rc != 1 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
	wantIn(t, log, "unused: target2/kept/a.sh:1: docgen:region lonely\n", "docgen: 1 unused; ")
	wantNotIn(t, log, "target/debug", "out/a.sh")
}
