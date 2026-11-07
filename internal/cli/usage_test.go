package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	flagsMod  = "module example.com/acme/widget\n\ngo 1.24.0\n"
	flagsPage = "<!-- docgen:usage cmd=cmd/x -->\n```text\ngood body\n```\n<!-- /docgen:usage -->\n"
)

func TestFlagsErrors(t *testing.T) {
	t.Parallel()
	for name, c := range map[string]struct{ src, want string }{
		"build failure":  {"package main\n\nfunc main() { undefinedThing() }\n", "go build cmd/x"},
		"exit 1, no use": {"package main\n\nimport (\n\t\"fmt\"\n\t\"os\"\n)\n\nfunc main() {\n\tfmt.Fprintln(os.Stderr, \"fatal: non-existent config\")\n\tos.Exit(1)\n}\n", "printed no usage"},
		"exit 0, no use": {"package main\n\nfunc main() { println(\"no-flags\") }\n", "printed no usage"},
		"panic":          {"package main\n\nfunc main() { var m map[string]int; m[\"a-b\"] = 1 }\n", "crashed"},
		"crash, usage":   {"package main\n\nfunc main() { panic(\"usage: x -v\") }\n", "crashed"},
		"signal":         {"package main\n\nimport (\n\t\"os\"\n\t\"syscall\"\n)\n\nfunc main() {\n\tprintln(\"usage: x\")\n\t_ = syscall.Kill(os.Getpid(), syscall.SIGKILL)\n\tselect {}\n}\n", "signal"},
		"usage in text":  {"package main\n\nimport \"log\"\n\nfunc main() { log.Fatal(\"backend usage quota exceeded\") }\n", "printed no usage"},
		"exit 3":         {"package main\n\nimport \"os\"\n\nfunc main() {\n\tprintln(\"usage: x\")\n\tos.Exit(3)\n}\n", "exit status 3"},
		"library":        {"package x\n", "go build cmd/x"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			writeTree(t, root, map[string]string{"go.mod": flagsMod, "docs/a.md": flagsPage, "cmd/x/main.go": c.src})
			rc, log := docgen(t, root, "update")
			if rc != 1 || !strings.Contains(log, c.want) {
				t.Errorf("update: rc %d, want 1 and %q, log:\n%s", rc, c.want, log)
			}
			if got, _ := os.ReadFile(filepath.Join(root, "docs/a.md")); string(got) != flagsPage {
				t.Errorf("update changed the page:\n%s", got)
			}
			if rc, log := docgen(t, root, "check"); rc != 1 {
				t.Errorf("check: rc %d, want 1, log:\n%s", rc, log)
			}
		})
	}
}

func TestFlagsOutput(t *testing.T) {
	t.Parallel()
	const flagPkg = "package main\n\nimport \"flag\"\n\nfunc main() {\n\tflag.Bool(\"v\", false, \"verbose\")\n\tflag.Parse()\n}\n"
	for name, c := range map[string]struct {
		mod, cmd, dir, src, want string
	}{
		"flag.CommandLine": {flagsMod, "cmd/x", "cmd/x/", flagPkg, "Usage of x:\n  -v\tverbose\n"},
		"root package":     {flagsMod, ".", "", flagPkg, "Usage of widget:\n  -v\tverbose\n"},
		"major version":    {"module example.com/acme/widget/v3\n\ngo 1.24.0\n", ".", "", flagPkg, "Usage of widget:\n  -v\tverbose\n"},
		"os.Args[0]": {
			flagsMod, "cmd/x", "cmd/x/",
			"package main\n\nimport (\n\t\"flag\"\n\t\"fmt\"\n\t\"os\"\n)\n\nfunc main() {\n\tflag.Usage = func() { fmt.Fprintf(os.Stderr, \"usage: %s [flags]\\n\", os.Args[0]) }\n\tflag.Parse()\n}\n",
			"usage: x [flags]\n",
		},
		"os.Executable": {
			flagsMod, "cmd/x", "cmd/x/",
			"package main\n\nimport (\n\t\"fmt\"\n\t\"os\"\n)\n\nfunc main() {\n\tp, _ := os.Executable()\n\tfmt.Printf(\"usage: %s\\n\", p)\n}\n",
			"usage: x\n",
		},
		"flag line, exit 1": {flagsMod, "cmd/x", "cmd/x/", "package main\n\nimport \"os\"\n\nfunc main() {\n\tprintln(\"  --name string  the name\")\n\tos.Exit(1)\n}\n", "  --name string  the name\n"},
		"exit status text":  {flagsMod, "cmd/x", "cmd/x/", "package main\n\nfunc main() { println(\"Usage: x\\nexit status 3 is a failure\") }\n", "Usage: x\nexit status 3 is a failure\n"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			page := "<!-- docgen:usage cmd=" + c.cmd + " --><!-- /docgen:usage -->\n"
			writeTree(t, root, map[string]string{"go.mod": c.mod, "docs/a.md": page, c.dir + "main.go": c.src})
			if rc, log := docgen(t, root, "update"); rc != 0 {
				t.Fatalf("update: rc %d, log:\n%s", rc, log)
			}
			got, err := os.ReadFile(filepath.Join(root, "docs/a.md"))
			if err != nil {
				t.Fatal(err)
			}
			if want := "```text\n" + c.want + "```\n"; !strings.Contains(string(got), want) {
				t.Errorf("page:\n%s\nwant it to hold:\n%s", got, want)
			}
			if rc, log := docgen(t, root, "check"); rc != 0 {
				t.Errorf("check after update: rc %d, log:\n%s", rc, log)
			}
		})
	}
}

func TestUsageArgs(t *testing.T) {
	t.Parallel()
	const src = "package main\n\nimport (\n\t\"fmt\"\n\t\"os\"\n\t\"strings\"\n)\n\nfunc main() {\n\tfmt.Printf(\"usage: x %q\\n\", strings.Join(os.Args[1:], \"|\"))\n}\n"
	for args, want := range map[string]string{
		"":                    `usage: x "-h"`,
		` args="help config"`: `usage: x "help|config"`,
		` args=--help`:        `usage: x "--help"`,
		` args="a \"b c\" d"`: `usage: x "a|b c|d"`,
	} {
		t.Run(args, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			page := "<!-- docgen:usage cmd=cmd/x" + args + " --><!-- /docgen:usage -->\n"
			writeTree(t, root, map[string]string{"go.mod": flagsMod, "docs/a.md": page, "cmd/x/main.go": src})
			if rc, log := docgen(t, root, "update"); rc != 0 {
				t.Fatalf("update: rc %d, log:\n%s", rc, log)
			}
			wantIn(t, readPage(t, root, "docs/a.md"), "```text\n"+want+"\n```\n")
			if rc, log := docgen(t, root, "check"); rc != 0 {
				t.Errorf("check: rc %d, log:\n%s", rc, log)
			}
		})
	}
}

func TestUsageArgsErrors(t *testing.T) {
	t.Parallel()
	const src = "package main\n\nfunc main() { println(\"version 1\") }\n"
	for args, want := range map[string]string{
		`args=""`:      "args= is empty",
		`args="  "`:    "args= has no argument",
		`args="a \"b"`: "args= has a \" with no end",
		"args=version": "cmd/x version: the command printed no usage",
	} {
		t.Run(args, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			page := "<!-- docgen:usage cmd=cmd/x " + args + " --><!-- /docgen:usage -->\n"
			writeTree(t, root, map[string]string{"go.mod": flagsMod, "docs/a.md": page, "cmd/x/main.go": src})
			if rc, log := docgen(t, root, "check"); rc != 1 || !strings.Contains(log, want) {
				t.Errorf("rc %d, want 1 and %q, log:\n%s", rc, want, log)
			}
		})
	}
}

func TestUsageOfTwoArgsOfOneCommand(t *testing.T) {
	t.Parallel()
	const src = "package main\n\nimport (\n\t\"fmt\"\n\t\"os\"\n)\n\nfunc main() { fmt.Println(\"usage:\", os.Args[1]) }\n"
	root := t.TempDir()
	page := "<!-- docgen:usage cmd=cmd/x --><!-- /docgen:usage -->\n<!-- docgen:usage cmd=cmd/x args=a --><!-- /docgen:usage -->\n"
	writeTree(t, root, map[string]string{"go.mod": flagsMod, "docs/a.md": page, "cmd/x/main.go": src})
	if rc, log := docgen(t, root, "update"); rc != 0 {
		t.Fatalf("update: rc %d, log:\n%s", rc, log)
	}
	wantIn(t, readPage(t, root, "docs/a.md"), "usage: -h\n", "usage: a\n")
}
