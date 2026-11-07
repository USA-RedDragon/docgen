package marker

// A testKind holds the rules of the marker kinds of the tests of this package.
// The rules of the real kinds are in package gen, with the tests of their errors.
type testKind struct{ kind string }

func (k testKind) Bind(a *Args) {
	switch k.kind {
	case "const":
		a.Required("pkg")
		a.Required("name")
		a.Inline()
	case "gomod":
		a.Required("field")
		a.Inline()
	case "code":
		a.Required("file")
		a.Optional("lines")
	case "config":
		a.Required("pkg")
		a.Required("type")
	case "doc":
		a.Required("pkg")
		a.Optional("name")
		a.Optional("section")
	case "output":
		_, ex := a.Optional("example")
		_, te := a.Optional("test")
		if ex == te || a.Len() != 1 {
			a.Fail("must have exactly one of cmd=, exec= or test=")
		}
	}
}

func testLookup(kind string) Binder {
	switch kind {
	case "const", "gomod", "code", "config", "doc", "output":
		return testKind{kind}
	}
	return nil
}

func parse(text string) ([]Marker, error) {
	return Parse(text, testLookup)
}

const (
	testOpen  = "<!-- docgen:const pkg=a name=B -->"
	testClose = "<!-- /docgen:const -->"
	testPair  = testOpen + testClose
)
