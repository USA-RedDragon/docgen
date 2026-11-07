package types

import gotypes "go/types"

func named(t gotypes.Type) *gotypes.Named {
	n, _ := gotypes.Unalias(t).(*gotypes.Named)
	return n
}

// IsNamed reports whether t is a defined type, also through an alias.
func IsNamed(t gotypes.Type) bool {
	return named(t) != nil
}

// IsDuration reports whether t is time.Duration.
func IsDuration(t gotypes.Type) bool {
	n := named(t)
	return n != nil && n.Obj().Pkg() != nil && n.Obj().Pkg().Path() == "time" && n.Obj().Name() == "Duration"
}

// HasNameIn reports whether t has a name in the source of pkg. An unexported type
// of another package has no qualified name there.
func HasNameIn(t gotypes.Type, pkg *gotypes.Package) bool {
	n := named(t)
	return n == nil || n.Obj().Pkg() == pkg || n.Obj().Exported()
}

// Raw returns the underlying type of t, which has none of the methods of t.
func Raw(t gotypes.Type) gotypes.Type {
	return t.Underlying()
}

// Basic returns the underlying basic type of t, and false when it has none.
func Basic(t gotypes.Type) (*gotypes.Basic, bool) {
	b, ok := t.Underlying().(*gotypes.Basic)
	return b, ok
}

func isKind(t gotypes.Type, info gotypes.BasicInfo) bool {
	b, ok := Basic(t)
	return ok && b.Info()&info != 0
}

// IsBool reports whether the underlying type of t is a boolean.
func IsBool(t gotypes.Type) bool {
	return isKind(t, gotypes.IsBoolean)
}

// IsInteger reports whether the underlying type of t is an integer.
func IsInteger(t gotypes.Type) bool {
	return isKind(t, gotypes.IsInteger)
}

// IsUnsigned reports whether the underlying type of t is an unsigned integer.
func IsUnsigned(t gotypes.Type) bool {
	return isKind(t, gotypes.IsUnsigned)
}

// NamedInteger returns t as a defined integer type, also through an alias, and
// nil for any other type.
func NamedInteger(t gotypes.Type) *gotypes.Named {
	if n := named(t); n != nil && IsInteger(n) {
		return n
	}
	return nil
}

// Integer returns t with no alias when it is an integer type, named or not, and
// nil for any other type.
func Integer(t gotypes.Type) gotypes.Type {
	if b, ok := gotypes.Unalias(t).(*gotypes.Basic); ok && b.Info()&gotypes.IsInteger != 0 {
		return b
	}
	if n := NamedInteger(t); n != nil {
		return n
	}
	return nil
}
