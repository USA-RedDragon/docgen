package sitetest

import (
	"encoding/json/v2"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const pageV9 = "# Page\n\nText of nine. See [go.mod](../go.mod), [the API](https://pkg.go.dev/github.com/acme/widget/sub#X), [the root](https://pkg.go.dev/github.com/acme/widget), [major](https://pkg.go.dev/github.com/acme/widget/v2/sub), [similar](https://pkg.go.dev/github.com/acme/widgets), [pinned](https://pkg.go.dev/github.com/acme/widget@v0.1.0) and [other](https://pkg.go.dev/github.com/other/x).\n"

// git runs git in dir with no user or system configuration.
func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1", "GIT_CEILING_DIRECTORIES="+filepath.Dir(dir),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func commit(t *testing.T, root, message string) {
	t.Helper()
	git(t, root, "add", "-A")
	git(t, root, "commit", "-q", "--allow-empty", "-m", message)
}

func remove(t *testing.T, root, name string) {
	t.Helper()
	if err := os.Remove(filepath.Join(root, filepath.FromSlash(name))); err != nil {
		t.Fatal(err)
	}
}

// gitRepo returns a repository with one commit and no tags.
func gitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := exec.CommandContext(t.Context(), "git", append([]string{"-C", dir}, args...)...).Output()
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

func gitRepo(t *testing.T) string {
	t.Helper()
	root := repo(t)
	git(t, root, "init", "-q", "-b", "main")
	commit(t, root, "first")
	return root
}

// taggedRepo returns a repository with the tags v0.9.0 and v0.10.0, and a
// change in the work tree that is not committed. The tag v0.9.0 has a page that
// v0.10.0 removes, and the work tree has a page that no tag has.
func taggedRepo(t *testing.T) string {
	t.Helper()
	root := repo(t)
	git(t, root, "init", "-q", "-b", "main")
	writeRepoFile(t, root, "docs/nav.md", "- [Home](../README.md)\n- [Page](page.md)\n- [Old](old.md)\n- [Deep](sub/deep.md)\n- [Files](sub)\n- [Reference](https://pkg.go.dev/github.com/acme/widget/sub)\n")
	writeRepoFile(t, root, "docs/page.md", pageV9)
	writeRepoFile(t, root, "docs/old.md", "# Old\n\nOnly in nine.\n")
	writeRepoFile(t, root, "docs/sub/deep.md", "# Deep\n\nOnly in nine.\n")
	commit(t, root, "nine")
	git(t, root, "tag", "-a", "-m", "nine", "v0.9.0")
	writeRepoFile(t, root, "docs/nav.md", "- [Home](../README.md)\n- [Page](page.md)\n- [Ten](ten.md)\n")
	writeRepoFile(t, root, "docs/page.md", "# Page\n\nText of ten.\n")
	writeRepoFile(t, root, "docs/ten.md", "# Ten\n\nOnly in ten.\n")
	remove(t, root, "docs/old.md")
	remove(t, root, "docs/sub/deep.md")
	commit(t, root, "ten")
	git(t, root, "tag", "v0.10.0")
	writeRepoFile(t, root, "docs/nav.md", "- [Home](../README.md)\n- [Page](page.md)\n- [Next](next.md)\n")
	writeRepoFile(t, root, "docs/page.md", "# Page\n\nText of main. A [link](https://pkg.go.dev/github.com/acme/widget/sub#X) and [go.mod](../go.mod).\n")
	writeRepoFile(t, root, "docs/next.md", "# Next\n\nOnly in main.\n")
	remove(t, root, "docs/ten.md")
	return root
}

func buildVersions(t *testing.T, root string) (string, string) {
	t.Helper()
	out, rc := build(t, root, root, nil, "--versions", "site")
	if rc != 0 {
		t.Fatalf("rc %d:\n%s", rc, out)
	}
	return filepath.Join(root, "site"), out
}

func read(t *testing.T, dir, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(name)))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func absent(t *testing.T, dir string, names ...string) {
	t.Helper()
	for _, name := range names {
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(name))); err == nil {
			t.Errorf("%s exists", name)
		}
	}
}

func holds(t *testing.T, name, text string, want ...string) {
	t.Helper()
	for _, w := range want {
		if !strings.Contains(text, w) {
			t.Errorf("%s does not hold %s:\n%s", name, w, text)
		}
	}
}

func lacks(t *testing.T, name, text string, bad ...string) {
	t.Helper()
	for _, b := range bad {
		if strings.Contains(text, b) {
			t.Errorf("%s holds %s", name, b)
		}
	}
}

type versionsFile struct {
	Latest   string   `json:"latest"`
	Versions []string `json:"versions"`
}

func readVersions(t *testing.T, site string) versionsFile {
	t.Helper()
	var v versionsFile
	if err := json.Unmarshal([]byte(read(t, site, "versions.json")), &v); err != nil {
		t.Fatal(err)
	}
	return v
}

func TestVersionsLayout(t *testing.T) {
	t.Parallel()
	needPandoc(t)
	root := taggedRepo(t)
	template := filepath.Join(root, "docs/site/template.html")
	b, err := os.ReadFile(template)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(template, []byte(strings.Replace(string(b), "<footer>", "<footer>Layout fix. ", 1)), 0o600); err != nil {
		t.Fatal(err)
	}
	site, _ := buildVersions(t, root)
	for _, f := range []string{"index.html", "404.html", "versions.json", ".nojekyll", ".docgen-site", "latest/index.html", "latest/ten.html", "main/next.html", "main/search-index.js", "v0.9.0/old.html", "v0.9.0/sub/deep.html", "v0.10.0/ten.html"} {
		exists(t, filepath.Join(site, filepath.FromSlash(f)))
	}
	absent(t, site, "page.html", "search-index.js", ".work", ".pages", "main/.search", "v0.9.0/.search", "v0.9.0/.docgen-site", "main/old.html", "v0.10.0/old.html")
	holds(t, "index.html", read(t, site, "index.html"), `content="0; url=latest/"`, `href="latest/"`)
	if read(t, site, "latest/page.html") != read(t, site, "v0.10.0/page.html") {
		t.Error("latest/page.html differs from the page of the newest tag")
	}
	holds(t, "v0.9.0/page.html", read(t, site, "v0.9.0/page.html"), "Text of nine", "Layout fix.")
	holds(t, "main/page.html", read(t, site, "main/page.html"), "Text of main")
}

func TestVersionsRebuild(t *testing.T) {
	t.Parallel()
	needPandoc(t)
	root := taggedRepo(t)
	buildVersions(t, root)
	writeRepoFile(t, root, "site/stale.txt", "x\n")
	site, _ := buildVersions(t, root)
	absent(t, site, "stale.txt")
}

func TestTagWithNoNav(t *testing.T) {
	t.Parallel()
	needPandoc(t)
	root := gitRepo(t)
	git(t, root, "tag", "v0.1.0")
	remove(t, root, "docs/nav.md")
	commit(t, root, "no nav")
	git(t, root, "tag", "v0.2.0")
	writeRepoFile(t, root, "docs/nav.md", "- [Home](../README.md)\n- [Page](page.md)\n")
	site, out := buildVersions(t, root)
	holds(t, "the output", out, "build.sh: v0.2.0 has no docs/nav.md; left out")
	absent(t, site, "v0.2.0")
	want := versionsFile{Latest: "v0.1.0", Versions: []string{"main", "v0.1.0"}}
	if got := readVersions(t, site); !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestTagThatIsNotAVersion(t *testing.T) {
	t.Parallel()
	needPandoc(t)
	root := gitRepo(t)
	git(t, root, "tag", "vnext")
	site, out := buildVersions(t, root)
	holds(t, "the output", out, "build.sh: vnext is not a semantic version; left out")
	absent(t, site, "vnext")
}

func TestTagThatDoesNotBuild(t *testing.T) {
	t.Parallel()
	needPandoc(t)
	root := gitRepo(t)
	writeRepoFile(t, root, "docs/page.md", "# Page\n\n![x](gone.png)\n")
	commit(t, root, "broken")
	git(t, root, "tag", "v0.1.0")
	writeRepoFile(t, root, "docs/page.md", "# Page\n")
	out, rc := build(t, root, root, nil, "--versions", "site")
	if rc == 0 {
		t.Fatalf("rc 0:\n%s", out)
	}
	holds(t, "the output", out, "image gone.png not found", "build.sh: a version did not build")
	absent(t, root, "site/.work")
}

func versionsScript(t *testing.T, args ...string) string {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "pandoc", append([]string{"lua", "../../docs/site/versions.lua"}, args...)...)
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

func TestVersionsOrder(t *testing.T) {
	t.Parallel()
	needPandoc(t)
	for name, c := range map[string]struct {
		tags []string
		want string
	}{
		"numbers":      {[]string{"v0.9.0", "v0.10.0", "v0.2.0", "v1.0.0"}, "v1.0.0 v0.10.0 v0.9.0 v0.2.0"},
		"prereleases":  {[]string{"v1.0.0-rc.1", "v1.0.0", "v1.0.0-rc.10", "v1.0.0-rc.2", "v1.0.0-1"}, "v1.0.0 v1.0.0-rc.10 v1.0.0-rc.2 v1.0.0-rc.1 v1.0.0-1"},
		"long numbers": {[]string{"v99999999999999999999.0.0", "v100000000000000000000.0.0"}, "v100000000000000000000.0.0 v99999999999999999999.0.0"},
		"not versions": {[]string{"vnext", "v1", "v1.2", "v01.0.0", "v1.0.0-", "v1.0.0-01", "v1.0.0/../x", "v1.0.0"}, "v1.0.0"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := strings.Join(strings.Fields(versionsScript(t, append([]string{"order"}, c.tags...)...)), " ")
			if got != c.want {
				t.Errorf("got %s, want %s", got, c.want)
			}
		})
	}
}

func TestLatestSkipsPrereleases(t *testing.T) {
	t.Parallel()
	needPandoc(t)
	for _, c := range []struct {
		tags []string
		want string
	}{
		{[]string{"v1.0.0-rc.1", "v0.9.0"}, "v0.9.0"},
		{[]string{"v1.0.0-rc.2", "v1.0.0-rc.1"}, "v1.0.0-rc.2"},
		{nil, ""},
	} {
		if got := strings.TrimSpace(versionsScript(t, append([]string{"latest"}, c.tags...)...)); got != c.want {
			t.Errorf("%v: got %q, want %q", c.tags, got, c.want)
		}
	}
}

func TestVersionsFile(t *testing.T) {
	t.Parallel()
	needPandoc(t)
	site, _ := buildVersions(t, taggedRepo(t))
	want := versionsFile{Latest: "v0.10.0", Versions: []string{"main", "v0.10.0", "v0.9.0"}}
	if got := readVersions(t, site); !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestBanner(t *testing.T) {
	t.Parallel()
	needPandoc(t)
	site, _ := buildVersions(t, taggedRepo(t))
	holds(t, "v0.9.0", read(t, site, "v0.9.0/page.html"), `class="banner"`, "v0.9.0 is not the latest version", `href="../latest/page.html"`)
	holds(t, "main", read(t, site, "main/page.html"), `class="banner"`, "Not released", `href="../latest/page.html"`)
	for _, p := range []string{"latest/page.html", "v0.10.0/page.html"} {
		lacks(t, p, read(t, site, p), `class="banner"`)
	}
}

func TestPageInOneVersion(t *testing.T) {
	t.Parallel()
	needPandoc(t)
	site, _ := buildVersions(t, taggedRepo(t))
	holds(t, "v0.9.0/old.html", read(t, site, "v0.9.0/old.html"), `<a href="../latest/">Go to the latest version</a>`)
	holds(t, "v0.9.0/sub/deep.html", read(t, site, "v0.9.0/sub/deep.html"), `<a href="../../latest/">Go to the latest version</a>`)
	holds(t, "main/next.html", read(t, site, "main/next.html"), `<a href="../latest/">Go to the latest version</a>`)
}

func TestPickerIsInThePage(t *testing.T) {
	t.Parallel()
	needPandoc(t)
	site, _ := buildVersions(t, taggedRepo(t))
	holds(t, "old version", read(t, site, "v0.9.0/page.html"), `<select aria-label="Version"><option>v0.9.0</option></select>`)
	lacks(t, "old version", read(t, site, "v0.9.0/page.html"), `data-page="page.html" hidden`)
}

func TestPickerLabelsMainWithItsCommit(t *testing.T) {
	t.Parallel()
	needPandoc(t)
	root := taggedRepo(t)
	site, _ := buildVersions(t, root)
	sha := strings.TrimSpace(gitOut(t, root, "rev-parse", "--short", "HEAD"))
	holds(t, "main", read(t, site, "main/page.html"), `<option>main (`+sha+`)</option>`)
	holds(t, "versions.json", read(t, site, "versions.json"), `"main":"`+sha+`"`)
	lacks(t, "main", read(t, site, "main/page.html"), `main (latest)`)
}

func TestPicker(t *testing.T) {
	t.Parallel()
	needPandoc(t)
	site, _ := buildVersions(t, taggedRepo(t))
	holds(t, "deep page", read(t, site, "v0.9.0/sub/deep.html"), `class="version" data-root="../" data-page="sub/deep.html"`, `"versions.json"`, `method: "HEAD"`)
	holds(t, "top page", read(t, site, "main/page.html"), `class="version" data-root="" data-page="page.html"`)
}

func TestSingleTreeHasNoVersions(t *testing.T) {
	t.Parallel()
	needPandoc(t)
	root := taggedRepo(t)
	lacks(t, "page.html", buildPage(t, root, nil, "page.html"), `class="banner"`, `class="version"`, "versions.json", "@v0.")
	absent(t, root, "artifacts/docs-site/versions.json", "artifacts/docs-site/.nojekyll", "artifacts/docs-site/main")
}

func TestSourceLinksAtTheTag(t *testing.T) {
	t.Parallel()
	needPandoc(t)
	site, _ := buildVersions(t, taggedRepo(t))
	const repoURL = "https://github.com/acme/widget"
	got := read(t, site, "v0.9.0/page.html")
	holds(t, "v0.9.0", got, repoURL+"/blob/v0.9.0/docs/page.md", repoURL+"/blob/v0.9.0/go.mod", repoURL+"/tree/v0.9.0/docs/sub")
	lacks(t, "v0.9.0", got, "/blob/main/", "/tree/main/")
	got = read(t, site, "main/page.html")
	holds(t, "main", got, repoURL+"/blob/main/docs/page.md", repoURL+"/blob/main/go.mod")
	lacks(t, "main", got, "/blob/v0.")
}

func TestAPILinks(t *testing.T) {
	t.Parallel()
	needPandoc(t)
	site, _ := buildVersions(t, taggedRepo(t))
	const api = "https://pkg.go.dev/github.com/acme/widget"
	holds(t, "v0.9.0", read(t, site, "v0.9.0/page.html"),
		`href="`+api+`@v0.9.0/sub#X"`,
		`href="`+api+`@v0.9.0">the root`,
		`href="`+api+`@v0.9.0">Go API`,
		`href="`+api+`@v0.9.0/sub">Reference`,
		`href="`+api+`/v2/sub"`,
		`href="`+api+`s"`,
		`href="`+api+`@v0.1.0"`,
		`href="https://pkg.go.dev/github.com/other/x"`)
	got := read(t, site, "main/page.html")
	holds(t, "main", got, `href="`+api+`/sub#X"`, `href="`+api+`">Go API`)
	lacks(t, "main", got, api+"@")
}

func TestSearchStaysInTheVersion(t *testing.T) {
	t.Parallel()
	needPandoc(t)
	site, _ := buildVersions(t, taggedRepo(t))
	got := read(t, site, "v0.9.0/search-index.js")
	holds(t, "v0.9.0", got, "old.html", "Text of nine")
	lacks(t, "v0.9.0", got, "ten.html", "next.html", "Text of main", "v0.9.0/")
	got = read(t, site, "main/search-index.js")
	holds(t, "main", got, "next.html", "Text of main")
	lacks(t, "main", got, "old.html", "ten.html")
	holds(t, "page", read(t, site, "v0.9.0/sub/deep.html"), `class="search" role="search" data-root="../"`)
}

func TestNoTags(t *testing.T) {
	t.Parallel()
	needPandoc(t)
	site, _ := buildVersions(t, gitRepo(t))
	want := versionsFile{Latest: "main", Versions: []string{"main"}}
	if got := readVersions(t, site); !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
	got := read(t, site, "main/page.html")
	if got != read(t, site, "latest/page.html") {
		t.Error("latest/page.html differs from main/page.html")
	}
	holds(t, "main", got, "Not released", "/blob/main/docs/page.md")
	lacks(t, "main", got, "../latest/")
	holds(t, "index.html", read(t, site, "index.html"), `url=latest/`)
}

func TestVersionsRefusals(t *testing.T) {
	t.Parallel()
	needPandoc(t)
	for name, c := range map[string]struct {
		root func(t *testing.T) string
		args []string
		want string
	}{
		"no repository": {repo, []string{"--versions", "site"}, "--versions needs a git repository"},
		"docs":          {gitRepo, []string{"--versions", "docs"}, "is or holds"},
		"inside docs":   {gitRepo, []string{"--versions", "docs/out"}, "is inside"},
		"root":          {gitRepo, []string{"--versions", "."}, "is or holds"},
		"not a build":   {gitRepo, []string{"--versions", "internal"}, "not an earlier build"},
		"unknown":       {gitRepo, []string{"--version"}, "unknown option --version"},
		"unknown after": {gitRepo, []string{"--versions", "--bad"}, "unknown option --bad"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := c.root(t)
			writeRepoFile(t, root, "internal/keep.go", "package internal\n")
			out, rc := build(t, root, root, nil, c.args...)
			if rc != 2 {
				t.Errorf("rc %d, want 2:\n%s", rc, out)
			}
			holds(t, "the output", out, c.want)
			for _, f := range []string{"README.md", "docs/page.md", "docs/site/build.sh", "internal/keep.go"} {
				exists(t, filepath.Join(root, filepath.FromSlash(f)))
			}
		})
	}
}

func TestShallowCloneKeepsTheOutput(t *testing.T) {
	t.Parallel()
	needPandoc(t)
	origin := gitRepo(t)
	commit(t, origin, "second")
	root := filepath.Join(filepath.Dir(origin), "clone")
	git(t, filepath.Dir(origin), "clone", "-q", "--depth", "1", "file://"+origin, root)
	writeRepoFile(t, root, "site/.docgen-site", "")
	writeRepoFile(t, root, "site/index.html", "old\n")
	out, rc := build(t, root, root, nil, "--versions", "site")
	if rc != 2 {
		t.Errorf("rc %d, want 2:\n%s", rc, out)
	}
	holds(t, "the output", out, "this clone is shallow")
	exists(t, filepath.Join(root, "site/index.html"))
}

func TestEditorBundleInEachVersionWithABlock(t *testing.T) {
	t.Parallel()
	needPandoc(t)
	root := gitRepo(t)
	git(t, root, "tag", "v0.1.0")
	writeRepoFile(t, root, "sample/v2/m.go", "package m\n\nconst Wait = 1\n")
	writeRepoFile(t, root, "docs/page.md", "# Page\n\n<!-- site:editor -->\n"+editorCode+"<!-- /site:editor -->\n")
	commit(t, root, "editor")
	git(t, root, "tag", "v0.2.0")
	writeRepoFile(t, root, "docs/page.md", "# Page\n\nText.\n")
	out, rc := build(t, root, root, nil, "--versions", "site")
	if rc != 0 {
		t.Fatalf("rc %d:\n%s", rc, out)
	}
	for dir, want := range map[string]bool{"v0.1.0": false, "v0.2.0": true, "latest": true, "main": false} {
		for _, f := range []string{"editor.js", "codemirror.js"} {
			_, err := os.Stat(filepath.Join(root, "site", dir, f))
			if (err == nil) != want {
				t.Errorf("%s/%s: err %v, want it to exist: %v", dir, f, err, want)
			}
		}
	}
}
