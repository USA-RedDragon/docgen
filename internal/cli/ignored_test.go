package cli

import (
	"os/exec"
	"testing"
)

func ignoredRepo(t *testing.T, files map[string]string) string {
	t.Helper()
	all := map[string]string{
		".gitignore":     "gen/\n/out\n",
		"go.mod":         pageModule,
		"p.go":           "package p\n\n// N is a number.\nconst N = 1\n",
		"gen/a.md":       goodPair + "\n",
		"gen/sub/b.md":   goodPair + "\n",
		"gen/skip.md":    badPair + "\n",
		"gen/drop/c.md":  badPair + "\n",
		"gen/.gitignore": "skip.md\ndrop/\n",
		"out/in/d.md":    goodPair + "\n",
	}
	for name, text := range files {
		all[name] = text
	}
	return gitRepo(t, all)
}

func TestExplicitDirectoryThatGitIgnores(t *testing.T) {
	t.Parallel()
	for name, c := range map[string]struct {
		config string
		args   []string
		want   []string
	}{
		"argument":                   {"", []string{"gen"}, []string{"gen/a.md", "gen/sub/b.md"}},
		"setting":                    {"docs: gen\n", nil, []string{"gen/a.md", "gen/sub/b.md"}},
		"below an ignored directory": {"", []string{"out/in"}, []string{"out/in/d.md"}},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := ignoredRepo(t, map[string]string{".docgen.yaml": c.config})
			if rc, log := docgen(t, root, append([]string{"update"}, c.args...)...); rc != 0 {
				t.Fatalf("rc %d, log:\n%s", rc, log)
			}
			for _, name := range c.want {
				wantIn(t, readPage(t, root, name), "`1`")
			}
		})
	}
}

func TestWholeRepositoryScanSkipsIgnoredPaths(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{{"update"}, {"update", "."}} {
		root := ignoredRepo(t, map[string]string{"gen/a.md": badPair + "\n", "gen/sub/b.md": badPair + "\n", "out/in/d.md": badPair + "\n"})
		if rc, log := docgen(t, root, args...); rc != 0 {
			t.Fatalf("%q: rc %d, log:\n%s", args, rc, log)
		}
	}
}

func TestExplicitDirectoryWithATrackedFile(t *testing.T) {
	t.Parallel()
	root := ignoredRepo(t, map[string]string{"skip.md": "Text.\n"})
	out, err := exec.CommandContext(t.Context(), "git", "-C", root, "add", "-f", "gen/sub/b.md", "skip.md").CombinedOutput()
	if err != nil {
		t.Fatalf("git add: %v\n%s", err, out)
	}
	if rc, log := docgen(t, root, "update", "gen"); rc != 0 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
	wantIn(t, readPage(t, root, "gen/a.md"), "`1`")
	wantIn(t, readPage(t, root, "gen/sub/b.md"), "`1`")
}

func TestExplicitDirectoryThroughASymbolicLink(t *testing.T) {
	t.Parallel()
	root := ignoredRepo(t, nil)
	link(t, "gen", root+"/alias")
	if rc, log := docgen(t, root, "update", "alias"); rc != 0 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
	wantIn(t, readPage(t, root, "gen/a.md"), "`1`")
}

func TestAuditOfAnExplicitIgnoredDirectory(t *testing.T) {
	t.Parallel()
	root := ignoredRepo(t, map[string]string{
		"gen/a.md":           regionMarker("gen/r.md"),
		"gen/sub/b.md":       "<!-- docgen:region idle -->\nText.\n<!-- docgen:end -->\n[x](missing.md)\n",
		"gen/r.md":           "<!-- docgen:region r -->\ntext\n<!-- docgen:end -->\n",
		"gen/states/old.svg": "<svg/>\n",
		"gen/code.sh":        regionText,
		"built.sh":           "x\n",
		"out/x.sh":           regionText,
	})
	if rc, log := docgen(t, root, "update", "gen"); rc != 0 {
		t.Fatalf("update: rc %d, log:\n%s", rc, log)
	}
	rc, log := docgen(t, root, "check", "gen")
	if rc != 1 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
	wantIn(t, log,
		"unused: gen/sub/b.md:1: docgen:region idle\n",
		"unused: gen/states/old.svg: docgen:states diagram\n",
		"broken: gen/sub/b.md:4: missing.md\n",
	)
	wantNotIn(t, log, "gen/r.md", "gen/code.sh", "out/x.sh")
}

func TestUnpairedMarkOfAnExplicitIgnoredDirectory(t *testing.T) {
	t.Parallel()
	root := ignoredRepo(t, map[string]string{"gen/a.md": "<!-- docgen:end -->\n", "gen/sub/b.md": "Text.\n"})
	rc, log := docgen(t, root, "check", "gen")
	if rc != 1 {
		t.Fatalf("rc %d, log:\n%s", rc, log)
	}
	wantIn(t, log, "gen/a.md:1: docgen:end has no docgen:region")
}
