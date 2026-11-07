package gen

import (
	"context"
	"errors"
	"fmt"
	"go/constant"
	gotypes "go/types"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/USA-RedDragon/docgen/internal/gocmd"
	"github.com/USA-RedDragon/docgen/internal/goprint"
	"github.com/USA-RedDragon/docgen/internal/load"
	"github.com/USA-RedDragon/docgen/internal/types"
)

const (
	valueFile = "zz_docgen_values_test.go"
	valueTest = "TestDocgenValues__"
)

// A valueRef is a Go expression in the source of a package and the print= value
// of its text, where "" is fmt.Sprint.
type valueRef struct {
	expr    string
	printAs string
}

// packageValues holds the values of one package that print in a run of the code.
type packageValues struct {
	pi      *load.Package
	refs    map[valueRef]bool
	imports map[string]string
	out     map[valueRef]string
	err     error
}

// A valueSet holds, by import path, the values that print in a run of the code.
type valueSet struct {
	pkgs map[string]*packageValues
}

// A valueReader is a handler with values to print. It adds them to vs after the
// package load.
type valueReader interface {
	addValues(vs *valueSet, loaded *load.Set)
}

func (vs *valueSet) of(pi *load.Package) *packageValues {
	pv := vs.pkgs[pi.Path]
	if pv == nil {
		pv = &packageValues{pi: pi, refs: map[valueRef]bool{}, imports: map[string]string{}}
		vs.pkgs[pi.Path] = pv
	}
	return pv
}

// add records ref for the run of the code, when a value of type t needs one.
func (vs *valueSet) add(pi *load.Package, ref valueRef, t gotypes.Type) {
	if goprint.NeedsRun(t, ref.printAs) {
		vs.of(pi).refs[ref] = true
	}
}

// text returns the text of the value v of type t with the source ref in pi.
func (vs *valueSet) text(pi *load.Package, ref valueRef, t gotypes.Type, v constant.Value) (string, error) {
	if !goprint.NeedsRun(t, ref.printAs) {
		return goprint.Print(t, v, ref.printAs)
	}
	pv := vs.pkgs[pi.Path]
	if pv == nil {
		return "", errors.New("the value did not print")
	}
	if pv.err != nil {
		return "", pv.err
	}
	out, ok := pv.out[ref]
	if !ok {
		return "", errors.New("the value did not print")
	}
	return out, goprint.CheckOutput(ref.printAs, out)
}

// typeExpr returns the source of t in pi, with an import for the package of t
// when that is another package. It reports false for an unexported type of
// another package, which has no qualified name in pi.
func (pv *packageValues) typeExpr(t gotypes.Type) (string, bool) {
	if !types.HasNameIn(t, pv.pi.Pkg.Types) {
		return "", false
	}
	s := gotypes.TypeString(t, func(q *gotypes.Package) string {
		if q == pv.pi.Pkg.Types {
			return ""
		}
		alias := pv.imports[q.Path()]
		if alias == "" {
			alias = "__dg" + strconv.Itoa(len(pv.imports))
			pv.imports[q.Path()] = alias
		}
		return alias
	})
	return s, true
}

// constExpr returns the source in pi of the constant v of type t. It reports
// false when t has no name in pi, or v is complex.
func (vs *valueSet) constExpr(pi *load.Package, t gotypes.Type, v constant.Value) (string, bool) {
	if v.Kind() == constant.Complex {
		return "", false
	}
	name, ok := vs.of(pi).typeExpr(t)
	if !ok {
		return "", false
	}
	var lit string
	switch v.Kind() {
	case constant.String:
		lit = strconv.Quote(constant.StringVal(v))
	case constant.Float:
		lit = constant.Num(v).ExactString() + ".0/" + constant.Denom(v).ExactString() + ".0"
	default:
		lit = v.ExactString()
	}
	return name + "(" + lit + ")", true
}

// sortedRefs returns the values of pv in a fixed order.
func (pv *packageValues) sortedRefs() []valueRef {
	refs := make([]valueRef, 0, len(pv.refs))
	for r := range pv.refs {
		refs = append(refs, r)
	}
	sort.Slice(refs, func(i, j int) bool {
		if refs[i].expr != refs[j].expr {
			return refs[i].expr < refs[j].expr
		}
		return refs[i].printAs < refs[j].printAs
	})
	return refs
}

// source returns the test file that writes the text of each of refs to out, one
// quoted line each.
func (pv *packageValues) source(refs []valueRef, out string) []byte {
	var b strings.Builder
	fmt.Fprintf(&b, "package %s\n\nimport (\n\t__dgfmt \"fmt\"\n\t__dgos \"os\"\n\t__dgtesting \"testing\"\n", pv.pi.Pkg.Name)
	for _, path := range sortedKeys(pv.imports) {
		fmt.Fprintf(&b, "\t%s %q\n", pv.imports[path], path)
	}
	fmt.Fprintf(&b, ")\n\nfunc %s(__dgt *__dgtesting.T) {\n\tvar __dgb []byte\n", valueTest)
	for _, r := range refs {
		call := "__dgfmt.Sprint(" + r.expr + ")"
		if r.printAs != "" {
			call = "__dgfmt.Sprintf(" + strconv.Quote(r.printAs) + ", " + r.expr + ")"
		}
		fmt.Fprintf(&b, "\t__dgb = __dgfmt.Appendf(__dgb, \"%%q\\n\", %s)\n", call)
	}
	fmt.Fprintf(&b, "\tif __dge := __dgos.WriteFile(%q, __dgb, 0o600); __dge != nil {\n\t\t__dgt.Fatal(__dge)\n\t}\n}\n", out)
	return []byte(b.String())
}

// jobs returns one function for each package with values. Each runs one go test
// and can run at the same time as the others.
func (vs *valueSet) jobs(g *Generator) []func(context.Context) {
	jobs := make([]func(context.Context), 0, len(vs.pkgs))
	for _, ip := range sortedKeys(vs.pkgs) {
		pv := vs.pkgs[ip]
		if len(pv.refs) == 0 {
			continue
		}
		jobs = append(jobs, func(ctx context.Context) { pv.out, pv.err = g.runValues(ctx, pv) })
	}
	return jobs
}

// runValues prints the values of pv with a test file in an overlay, in the
// temporary directory of the session.
func (g *Generator) runValues(ctx context.Context, pv *packageValues) (map[valueRef]string, error) {
	work, err := g.session.WorkDir("values-")
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.RemoveAll(work) }()
	refs := pv.sortedRefs()
	out := filepath.Join(work, "values.txt")
	test := filepath.Join(work, valueFile)
	if err := os.WriteFile(test, pv.source(refs, out), 0o600); err != nil {
		return nil, err
	}
	overlay := filepath.Join(work, "overlay.json")
	file := filepath.Join(filepath.Dir(pv.pi.Pkg.GoFiles[0]), valueFile)
	text := fmt.Sprintf(`{"Replace":{%s:%s}}`, strconv.Quote(file), strconv.Quote(test))
	if err := os.WriteFile(overlay, []byte(text), 0o600); err != nil {
		return nil, err
	}
	short := g.mod.ShortPath(pv.pi.Path)
	fmt.Fprintf(g.log, "docgen: printing %d values of %s\n", len(refs), short)
	ctx, cancel := context.WithTimeout(ctx, gocmd.ExampleTimeout)
	defer cancel()
	stdout, stderr, err := g.session.Run(ctx, g.mod.Dir, "test", "-count=1", "-vet=off", "-overlay", overlay, "-run", "^"+valueTest+"$", pv.pi.Path)
	if err != nil {
		return nil, fmt.Errorf("printing the values of %s: go test: %w\n%s%s", short, err, stdout, stderr)
	}
	values, err := readValues(out, refs)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("printing the values of %s: go test ran no test; a TestMain of the package must call m.Run", short)
	}
	return values, err
}

// readValues returns the text of each of refs from the file at path.
func readValues(path string, refs []valueRef) (map[valueRef]string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimSuffix(string(b), "\n"), "\n")
	if len(lines) != len(refs) {
		return nil, fmt.Errorf("the test wrote %d values, want %d", len(lines), len(refs))
	}
	out := make(map[valueRef]string, len(refs))
	for i, r := range refs {
		s, err := strconv.Unquote(lines[i])
		if err != nil {
			return nil, err
		}
		out[r] = s
	}
	return out, nil
}
