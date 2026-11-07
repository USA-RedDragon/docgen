package cli

import (
	"path/filepath"
	"strings"
	"testing"
)

const (
	varConfig = "vars:\n  version: v1.2.3\n"
	varMarker = "Get <!-- docgen:var name=version --><!-- /docgen:var -->.\n"
)

func varRepo(t *testing.T, config, text string) string {
	t.Helper()
	return unusedRepo(t, map[string]string{".docgen.yaml": config, "docs/p.md": text})
}

func TestVarMarker(t *testing.T) {
	t.Parallel()
	root := varRepo(t, varConfig, varMarker)
	if rc, log := docgen(t, root, "update"); rc != 0 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
	wantIn(t, readPage(t, root, "docs/p.md"), "<!-- docgen:var name=version -->`v1.2.3`<!-- /docgen:var -->.\n")
	if rc, log := docgen(t, root, "check"); rc != 0 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
}

func TestTextMarker(t *testing.T) {
	t.Parallel()
	text := `<!-- docgen:text text="go run x@{version} check\n# {{version}" lang=sh -->` + "\n<!-- /docgen:text -->\n"
	root := varRepo(t, varConfig, text)
	if rc, log := docgen(t, root, "update"); rc != 0 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
	wantIn(t, readPage(t, root, "docs/p.md"), "```sh\ngo run x@v1.2.3 check\n# {version}\n```\n<!-- /docgen:text -->\n")
	if rc, log := docgen(t, root, "check"); rc != 0 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
}

func TestTextMarkerDefaultLanguage(t *testing.T) {
	t.Parallel()
	root := varRepo(t, "", "<!-- docgen:text text=plain -->\n<!-- /docgen:text -->\n")
	if rc, log := docgen(t, root, "update"); rc != 0 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
	wantIn(t, readPage(t, root, "docs/p.md"), "```text\nplain\n```\n")
}

func TestUnknownVar(t *testing.T) {
	t.Parallel()
	for _, text := range []string{
		"Get <!-- docgen:var name=nope --><!-- /docgen:var -->.\n",
		"<!-- docgen:text text={nope} -->\n<!-- /docgen:text -->\n",
	} {
		root := varRepo(t, "", text)
		for _, cmd := range []string{"update", "check"} {
			rc, log := docgen(t, root, cmd)
			if rc != 1 {
				t.Fatalf("%s %q: rc %d, log:\n%s", cmd, text, rc, log)
			}
			wantIn(t, log, "docs/p.md:1: ", "var nope is not in the vars setting")
		}
	}
}

func TestUnusedVar(t *testing.T) {
	t.Parallel()
	root := varRepo(t, "vars:\n  zed: 1\n  version: v1\n  beta: 2\n", varMarker+"<!-- docgen:text text={beta} -->\n<!-- /docgen:text -->\n")
	rc, log := docgen(t, root, "update")
	if rc != 0 {
		t.Fatalf("update: rc %d, log:\n%s", rc, log)
	}
	wantNotIn(t, log, "unused")
	rc, log = docgen(t, root, "check")
	if rc != 1 {
		t.Fatalf("check: rc %d, log:\n%s", rc, log)
	}
	wantIn(t, log, "unused: .docgen.yaml: var zed\n", "docgen: 1 unused; ")
	wantNotIn(t, log, "var version", "var beta")
}

func TestUnusedVarOfOtherConfigFile(t *testing.T) {
	t.Parallel()
	root := unusedRepo(t, map[string]string{"conf/other.yaml": "vars:\n  zed: 1\n"})
	rc, log := docgen(t, root, "check", "-config", filepath.Join(root, "conf/other.yaml"))
	if rc != 1 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
	wantIn(t, log, "unused: conf/other.yaml: var zed\n")
}

func TestUnusedVarWithAPageError(t *testing.T) {
	t.Parallel()
	root := unusedRepo(t, map[string]string{
		".docgen.yaml": "vars:\n  zed: 1\n",
		"docs/bad.md":  "<!-- docgen:nope -->\n",
	})
	rc, log := docgen(t, root, "check")
	if rc != 1 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
	wantNotIn(t, log, "var zed")
}

func TestVarKeys(t *testing.T) {
	t.Parallel()
	for config, bad := range map[string]bool{
		"vars:\n  a-b_1: x\n":   false,
		"vars:\n  \"a b\": x\n": true,
		"vars:\n  \"a.b\": x\n": true,
		"vars:\n  \"a}b\": x\n": true,
		"vars:\n  \"\": x\n":    true,
	} {
		root := varRepo(t, config, "Text.\n")
		rc, log := docgen(t, root, "update")
		if bad && (rc != 1 || !strings.Contains(log, "is not valid; a key holds only letters, digits, _ and -")) {
			t.Errorf("%q: rc %d, log:\n%s", config, rc, log)
		}
		if !bad && rc != 0 {
			t.Errorf("%q: rc %d, log:\n%s", config, rc, log)
		}
	}
}

//nolint:paralleltest // sets the environment
func TestVarsHaveNoFlagOrVariable(t *testing.T) {
	t.Setenv("DOCGEN_VARS", "version=v9")
	root := varRepo(t, varConfig, varMarker)
	if rc, log := docgen(t, root, "update"); rc != 0 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
	wantIn(t, readPage(t, root, "docs/p.md"), "`v1.2.3`")
	if rc, _ := docgen(t, root, "check", "-vars", "a"); rc != 2 {
		t.Errorf("rc %d, want 2", rc)
	}
}

func TestSampleVars(t *testing.T) {
	t.Parallel()
	files := sampleCode()
	files[".docgen.yaml"] = "vars:\n  version: inner\n  spare: 1\n"
	files["docs/a.md"] = varMarker
	root := outerRepo(t, nestedMarker("root=testdata/sample page=docs/a.md"), files)
	writeTree(t, root, map[string]string{".docgen.yaml": "vars:\n  version: outer\n"})
	rc, log := docgen(t, root, "update")
	if rc != 0 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
	wantIn(t, readPage(t, root, "docs/p.md"), "`inner`")
	rc, log = docgen(t, root, "check")
	if rc != 1 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
	wantIn(t, log, "unused: .docgen.yaml: var version\n")
	wantNotIn(t, log, "spare")
}

func TestOuterVarsStayOutsideTheSample(t *testing.T) {
	t.Parallel()
	files := sampleCode()
	files["docs/a.md"] = varMarker
	root := outerRepo(t, nestedMarker("root=testdata/sample page=docs/a.md"), files)
	writeTree(t, root, map[string]string{".docgen.yaml": varConfig})
	rc, log := docgen(t, root, "update")
	if rc != 1 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
	wantIn(t, log, "var version is not in the vars setting")
}

func TestSampleVarKeys(t *testing.T) {
	t.Parallel()
	files := sampleCode()
	files[".docgen.yaml"] = "vars:\n  \"a b\": x\n"
	root := outerRepo(t, nestedMarker("root=testdata/sample page=docs/a.md"), files)
	rc, log := docgen(t, root, "update")
	if rc != 1 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
	wantIn(t, log, "is not valid; a key holds only letters, digits, _ and -")
}
