package excerpt

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
)

// Symbol returns the source of the func, method (Type.Method), type,
// or const or var block named sym, without its doc comment. An Example func
// has no output comment in the result.
func Symbol(path string, src []byte, sym string) (string, error) {
	switch filepath.Ext(path) {
	case goExt:
	case ".yaml", ".yml":
		return yamlExcerpt(src, sym)
	case ".c", ".h":
		return cExcerpt(src, sym)
	default:
		return "", errors.New("symbol= applies to a Go, YAML or C file")
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, src, parser.ParseComments)
	if err != nil {
		return "", err
	}
	node := findSymbol(f, sym)
	if node == nil {
		return "", noSymbol(path, sym)
	}
	start := fset.Position(node.Pos()).Offset
	_, spec := node.(*ast.TypeSpec)
	if spec {
		start = lineStart(src, start)
	}
	text := string(src[start:fset.Position(node.End()).Offset])
	if fd, ok := node.(*ast.FuncDecl); ok && strings.HasPrefix(fd.Name.Name, "Example") && fd.Body != nil {
		outs := outputComments(f, fd)
		for i := len(outs) - 1; i >= 0; i-- {
			cs := lineStart(src, fset.Position(outs[i].Pos()).Offset) - start
			ce := fset.Position(outs[i].End()).Offset - start
			for ce < len(text) && text[ce] != '\n' {
				ce++
			}
			if ce < len(text) {
				ce++
			}
			text = text[:cs] + text[ce:]
		}
	}
	if spec {
		return dedent(strings.Split(text, "\n")), nil
	}
	return text, nil
}

// RawSymbol returns the text of the symbol sym as it is in the file. In a Go
// file the result starts at the doc comment and an Example keeps its output
// comment, and sym can be package, for the package clause and its doc comment.
func RawSymbol(path string, src []byte, sym string) (string, error) {
	if filepath.Ext(path) != goExt {
		return Symbol(path, src, sym)
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, src, parser.ParseComments)
	if err != nil {
		return "", err
	}
	offset := func(p token.Pos) int { return fset.Position(p).Offset }
	if sym == "package" {
		start := f.Package
		if f.Doc != nil {
			start = f.Doc.Pos()
		}
		return string(src[offset(start):offset(f.Name.End())]), nil
	}
	node := findSymbol(f, sym)
	if node == nil {
		return "", noSymbol(path, sym)
	}
	start := node.Pos()
	if doc := docOf(node); doc != nil {
		start = doc.Pos()
	}
	text := string(src[lineStart(src, offset(start)):offset(node.End())])
	if _, spec := node.(*ast.TypeSpec); spec {
		return dedent(strings.Split(text, "\n")), nil
	}
	return text, nil
}

func noSymbol(path, sym string) error {
	return fmt.Errorf("no func, type, const or var %s in %s", sym, filepath.Base(path))
}

func docOf(node ast.Node) *ast.CommentGroup {
	switch x := node.(type) {
	case *ast.FuncDecl:
		return x.Doc
	case *ast.GenDecl:
		return x.Doc
	case *ast.TypeSpec:
		return x.Doc
	}
	return nil
}

func findSymbol(f *ast.File, sym string) ast.Node {
	recv, name, isMethod := strings.Cut(sym, ".")
	if !isMethod {
		name = sym
	}
	for _, d := range f.Decls {
		switch x := d.(type) {
		case *ast.FuncDecl:
			if x.Name.Name == name && (x.Recv != nil) == isMethod && (!isMethod || recvType(x) == recv) {
				return x
			}
		case *ast.GenDecl:
			if isMethod {
				continue
			}
			if n := findSpec(x, name); n != nil {
				return n
			}
		}
	}
	return nil
}

func recvType(fd *ast.FuncDecl) string {
	t := fd.Recv.List[0].Type
	if st, ok := t.(*ast.StarExpr); ok {
		t = st.X
	}
	switch ix := t.(type) {
	case *ast.IndexExpr:
		t = ix.X
	case *ast.IndexListExpr:
		t = ix.X
	}
	if id, ok := t.(*ast.Ident); ok {
		return id.Name
	}
	return ""
}

func findSpec(x *ast.GenDecl, name string) ast.Node {
	for _, s := range x.Specs {
		switch y := s.(type) {
		case *ast.TypeSpec:
			if y.Name.Name != name {
				continue
			}
			if len(x.Specs) > 1 {
				return y
			}
			return x
		case *ast.ValueSpec:
			for _, n := range y.Names {
				if n.Name == name {
					return x
				}
			}
		}
	}
	return nil
}

func outputComments(f *ast.File, fd *ast.FuncDecl) []*ast.CommentGroup {
	var out []*ast.CommentGroup
	for _, c := range f.Comments {
		if c.Pos() > fd.Body.Lbrace && c.End() < fd.Body.Rbrace {
			t := strings.TrimSpace(c.Text())
			if strings.HasPrefix(t, "Output:") || strings.HasPrefix(t, "Unordered output:") {
				out = append(out, c)
			}
		}
	}
	return out
}

func lineStart(src []byte, off int) int {
	for off > 0 && src[off-1] != '\n' {
		off--
	}
	return off
}
