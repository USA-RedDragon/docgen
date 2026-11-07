package gen

import (
	"context"
	"fmt"
	"go/doc"
	"strings"

	"github.com/USA-RedDragon/docgen/internal/markdown"
	"github.com/USA-RedDragon/docgen/internal/marker"
)

func splitTest(s string) (pkg, name string) {
	dot := strings.LastIndex(s, ".")
	if dot < 0 {
		return "", s
	}
	return s[:dot], s[dot+1:]
}

// An outputHandler is a docgen:output marker: the standard output of the Go
// program in the directory cmd, or of the shell command exec, or the Output
// comment of the Example test, in the form <pkg>.<ExampleName>.
type outputHandler struct {
	cmd     string
	command string
	test    string
}

func (h *outputHandler) Bind(a *marker.Args) {
	var hasCmd, hasExec, hasTest bool
	h.cmd, hasCmd = a.Optional(argCmd)
	h.command, hasExec = a.Optional("exec")
	h.test, hasTest = a.Optional("test")
	n := 0
	for _, has := range []bool{hasCmd, hasExec, hasTest} {
		if has {
			n++
		}
	}
	if n != 1 || a.Len() != 1 {
		a.Fail("must have exactly one of cmd=, exec= or test=")
	}
	a.NonEmpty(argCmd, "exec", "test")
}

// program returns the key of the run of the marker, and false for a test.
func (h *outputHandler) program() (programRun, bool) {
	return programRun{cmd: h.cmd, command: h.command}, h.cmd != "" || h.command != ""
}

func (h *outputHandler) plan(p *plan) {
	if run, ok := h.program(); ok {
		if h.cmd == "" || p.mod.LocalDir(argCmd, h.cmd) == nil {
			p.programs[run] = true
		}
		return
	}
	pkg, name := splitTest(h.test)
	if p.outside(pkg) {
		return
	}
	if ip, err := p.mod.PkgPath(pkg); err == nil {
		pkg = p.mod.ShortPath(ip)
		h.test = pkg + "." + name
	}
	ip := p.load(pkg)
	if p.tests[ip] == nil {
		p.tests[ip] = map[string]bool{}
	}
	p.tests[ip][strings.TrimPrefix(name, "Example")] = true
}

func (h *outputHandler) fill(_ context.Context, j job) (string, []outFile, error) {
	key, ok := h.program()
	if !ok {
		return h.fillTest(j)
	}
	if h.cmd != "" {
		if err := j.mod.LocalDir(argCmd, h.cmd); err != nil {
			return "", nil, err
		}
	}
	run, ok := j.programs[key]
	if !ok {
		return "", nil, fmt.Errorf("no result for %s", key)
	}
	if run.err != nil {
		return "", nil, run.err
	}
	return markdown.Fence("text", run.out), nil, nil
}

func (h *outputHandler) fillTest(j job) (string, []outFile, error) {
	pkg, name := splitTest(h.test)
	pi, err := j.loaded.Package(pkg)
	if err != nil {
		return "", nil, err
	}
	if err := j.loaded.TestErr(pi.Path); err != nil {
		return "", nil, err
	}
	name = strings.TrimPrefix(name, "Example")
	e := dedupeExamples(pi.Examples)[name]
	if e == nil {
		return "", nil, fmt.Errorf("no func Example%s in %s", name, pi.Path)
	}
	if e.Output == "" && !e.EmptyOutput {
		return "", nil, fmt.Errorf("func Example%s has no // Output: comment", name)
	}
	if e.EmptyOutput {
		return "", nil, fmt.Errorf("func Example%s has an empty // Output: comment", name)
	}
	if err := j.testErrs[pi.Path]; err != nil {
		return "", nil, err
	}
	return markdown.Fence("text", e.Output), nil, nil
}

func dedupeExamples(in []*doc.Example) map[string]*doc.Example {
	out := map[string]*doc.Example{}
	for _, e := range in {
		out[e.Name] = e
	}
	return out
}
