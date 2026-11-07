package types

import gotypes "go/types"

// A Method is an exported method of a type, with the types of its parameters and
// results. Promoted is true for a method of an embedded field.
type Method struct {
	Name     string
	Params   []gotypes.Type
	Results  []gotypes.Type
	Promoted bool
}

// Methods returns the exported methods of a pointer to t in name order, the
// promoted methods included.
func Methods(t Type) []Method {
	ms := gotypes.NewMethodSet(gotypes.NewPointer(t.Go))
	var out []Method
	for i := 0; i < ms.Len(); i++ {
		f, ok := ms.At(i).Obj().(*gotypes.Func)
		if !ok || !f.Exported() {
			continue
		}
		sig, ok := f.Type().(*gotypes.Signature)
		if !ok {
			continue
		}
		out = append(out, Method{Name: f.Name(), Params: tupleTypes(sig.Params()), Results: tupleTypes(sig.Results()), Promoted: len(ms.At(i).Index()) > 1})
	}
	return out
}

func tupleTypes(tuple *gotypes.Tuple) []gotypes.Type {
	out := make([]gotypes.Type, 0, tuple.Len())
	for i := 0; i < tuple.Len(); i++ {
		out = append(out, tuple.At(i).Type())
	}
	return out
}

// HasPrintMethod reports whether a value of type t has a method that fmt calls.
func HasPrintMethod(t gotypes.Type) bool {
	ms := gotypes.NewMethodSet(t)
	for _, name := range []string{"String", "Error", "Format", "GoString"} {
		if ms.Lookup(nil, name) != nil {
			return true
		}
	}
	return false
}
