package goprint

import (
	"go/constant"
	"go/token"
	"go/types"
	"strings"
	"testing"
)

func namedType(pkg, name string, under types.Type) *types.Named {
	return types.NewNamed(types.NewTypeName(token.NoPos, types.NewPackage(pkg, pkg), name, nil), under, nil)
}

func TestPrint(t *testing.T) {
	t.Parallel()
	duration := namedType("time", "Duration", types.Typ[types.Int64])
	other := namedType("x", "Duration", types.Typ[types.Int64])
	big := constant.Shift(constant.MakeInt64(1), token.SHL, 100)
	huge := constant.MakeFromLiteral("1e400", token.FLOAT, 0)
	for _, c := range []struct {
		typ   types.Type
		val   constant.Value
		print string
		want  string
	}{
		{duration, constant.MakeInt64(60_000_000), PrintRaw, "60000000"},
		{namedType("x", "S", types.Typ[types.String]), constant.MakeString("a"), PrintRaw, "a"},
		{types.Typ[types.UntypedRune], constant.MakeInt64(97), PrintRaw, "97"},
		{duration, constant.MakeInt64(60_000_000), "", "60ms"},
		{duration, constant.MakeInt64(120_000_000_000), "", "2m0s"},
		{duration, constant.MakeInt64(60_000_000), "%d", "60000000"},
		{duration, constant.MakeInt64(1), "%T", "time.Duration"},
		{other, constant.MakeInt64(5), "", "5"},
		{types.Typ[types.Float64], constant.MakeFloat64(1.5), "", "1.5"},
		{types.Typ[types.Float64], constant.MakeInt64(1000000), "", "1e+06"},
		{types.Typ[types.Float64], constant.MakeFloat64(0.5), "", "0.5"},
		{types.Typ[types.Float32], constant.MakeFloat64(0.1), "", "0.1"},
		{types.Typ[types.Float64], constant.MakeFloat64(1e30), "", "1e+30"},
		{types.Typ[types.Float64], constant.MakeFloat64(1e-5), "", "1e-05"},
		{types.Typ[types.Float64], constant.MakeFloat64(2.5), "%.3f", "2.500"},
		{types.Typ[types.Complex128], constant.ToComplex(constant.MakeFloat64(1.5)), "", "(1.5+0i)"},
		{types.Typ[types.Complex64], constant.ToComplex(constant.MakeFloat64(0.1)), "", "(0.1+0i)"},
		{types.Typ[types.String], constant.MakeString("a"), "", "a"},
		{types.Typ[types.String], constant.MakeString("a"), "%q", `"a"`},
		{types.Typ[types.Bool], constant.MakeBool(true), "", "true"},
		{types.Typ[types.Uint8], constant.MakeInt64(255), "%#x", "0xff"},
		{types.Typ[types.Int8], constant.MakeInt64(-1), "%d", "-1"},
		{types.Typ[types.Uint64], constant.MakeUint64(1 << 63), "", "9223372036854775808"},
		{types.Typ[types.Uintptr], constant.MakeInt64(7), "", "7"},
		{types.Typ[types.UntypedInt], constant.MakeInt64(7), "%T", "int"},
		{types.Typ[types.UntypedInt], big, "", "1267650600228229401496703205376"},
		{types.Typ[types.UntypedInt], big, "%x", "10000000000000000000000000"},
		{types.Typ[types.UntypedFloat], huge, "", "1e+400"},
		{types.Typ[types.UntypedFloat], constant.MakeInt64(1), "", "1"},
		{types.Typ[types.UntypedRune], constant.MakeInt64('a'), "", "97"},
		{types.Typ[types.UntypedRune], constant.MakeInt64('a'), "%c", "a"},
		{types.Typ[types.UntypedString], constant.MakeString("s"), "", "s"},
		{types.Typ[types.UntypedBool], constant.MakeBool(false), "", "false"},
	} {
		got, err := Print(c.typ, c.val, c.print)
		if err != nil || got != c.want {
			t.Errorf("Print(%v, %v, %q) = %q, %v, want %q", c.typ, c.val, c.print, got, err, c.want)
		}
	}
}

func TestPrintBadVerb(t *testing.T) {
	t.Parallel()
	_, err := Print(types.Typ[types.Int], constant.MakeInt64(1), "%z")
	if err == nil || !strings.Contains(err.Error(), "%!z(int=1)") {
		t.Errorf("err = %v", err)
	}
}

func TestCheckPrint(t *testing.T) {
	t.Parallel()
	for value, ok := range map[string]bool{
		"": true, "go": true, "raw": true, "name": true, "%d": true, "%#x": true, "%q": true, "%6.2f": true, "%-5s": true, "%+v": true, "%.3e": true,
		"Go": false, "d": false, "%": false, "%%": false, "%d items": false, "%*d": false, "x%d": false, "%d%d": false,
	} {
		if err := CheckPrint(value); (err == nil) != ok {
			t.Errorf("CheckPrint(%q) = %v", value, err)
		}
	}
}

func TestNeedsRun(t *testing.T) {
	t.Parallel()
	sig := func(res types.Type) *types.Signature {
		return types.NewSignatureType(nil, nil, nil, nil, types.NewTuple(types.NewVar(token.NoPos, nil, "", res)), false)
	}
	withMethod := func(name string) *types.Named {
		n := namedType("x", "T", types.Typ[types.Int])
		n.AddMethod(types.NewFunc(token.NoPos, n.Obj().Pkg(), name, sig(types.Typ[types.String])))
		return n
	}
	plain := namedType("x", "P", types.Typ[types.Int])
	for _, c := range []struct {
		typ   types.Type
		print string
		want  bool
	}{
		{withMethod("String"), PrintRaw, false},
		{withMethod("Format"), PrintName, false},
		{namedType("time", "Duration", types.Typ[types.Int64]), PrintRaw, false},
		{types.Typ[types.Int], "", false},
		{types.Typ[types.Int], "%T", false},
		{plain, "", false},
		{plain, "%d", false},
		{plain, "%T", true},
		{namedType("time", "Duration", types.Typ[types.Int64]), "%d", false},
		{withMethod("String"), "", true},
		{withMethod("Error"), "", true},
		{withMethod("Format"), "%d", true},
		{withMethod("GoString"), "", true},
		{withMethod("Other"), "", false},
	} {
		if got := NeedsRun(c.typ, c.print); got != c.want {
			t.Errorf("NeedsRun(%v, %q) = %v", c.typ, c.print, got)
		}
	}
}

func TestType(t *testing.T) {
	t.Parallel()
	here := types.NewPackage("example.com/here", "here")
	there := types.NewPackage("example.com/there", "there")
	local := types.NewNamed(types.NewTypeName(token.NoPos, here, "T", nil), types.Typ[types.Int], nil)
	remote := types.NewNamed(types.NewTypeName(token.NoPos, there, "U", nil), types.Typ[types.Int], nil)
	for typ, want := range map[types.Type]string{
		local:                 "T",
		remote:                "there.U",
		types.NewSlice(local): "[]T",
	} {
		if got := Type(typ, here); got != want {
			t.Errorf("Type(%v) = %q, want %q", typ, got, want)
		}
	}
}

func TestPrintOutOfRange(t *testing.T) {
	t.Parallel()
	for _, s := range []string{"1e400", "1e-400"} {
		v := constant.MakeFromLiteral(s, token.FLOAT, 0)
		for _, verb := range []string{"%g", "%e", "%f", "%v"} {
			if out, err := Print(types.Typ[types.UntypedFloat], v, verb); err == nil {
				t.Errorf("Print(%s, %s) = %q, want an error", s, verb, out)
			}
		}
		if _, err := Print(types.Typ[types.UntypedFloat], v, ""); err != nil {
			t.Errorf("Print(%s) = %v", s, err)
		}
	}
}

func TestPrintHugeInteger(t *testing.T) {
	t.Parallel()
	v := constant.Shift(constant.MakeInt64(1), token.SHL, 100)
	for verb, want := range map[string]string{"": "1267650600228229401496703205376", "%d": "1267650600228229401496703205376", "%x": "10000000000000000000000000"} {
		if got, err := Print(types.Typ[types.UntypedInt], v, verb); err != nil || got != want {
			t.Errorf("Print(%q) = %q, %v, want %q", verb, got, err, want)
		}
	}
	for _, verb := range []string{"%T", "%s", "%q", "%c", "%e", "%U"} {
		if out, err := Print(types.Typ[types.UntypedInt], v, verb); err == nil {
			t.Errorf("Print(%s) = %q, want an error", verb, out)
		}
	}
}

func TestPrintTextLikeABadVerb(t *testing.T) {
	t.Parallel()
	const text = "%!v(x)"
	v := constant.MakeString(text)
	for _, value := range []string{"", PrintGo, PrintRaw, "%v"} {
		if got, err := Print(types.Typ[types.String], v, value); err != nil || got != text {
			t.Errorf("Print(%q) = %q, %v", value, got, err)
		}
	}
	s := constant.MakeString("%!s(x)")
	if got, err := Print(types.Typ[types.String], s, "%s"); err != nil || got != "%!s(x)" {
		t.Errorf("Print(%%s) = %q, %v", got, err)
	}
	if out, err := Print(types.Typ[types.String], v, "%d"); err == nil {
		t.Errorf("Print(%%d) = %q, want an error", out)
	}
}

func TestCheckOutput(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		verb, out string
		bad       bool
	}{
		{"", "%!v(PANIC=String method: boom)", true},
		{"", "%!v(text)", false},
		{"", "plain", false},
		{"%d", "%!d(p.T=1)", true},
		{"%d", "1", false},
	} {
		if err := CheckOutput(c.verb, c.out); (err != nil) != c.bad {
			t.Errorf("CheckOutput(%q, %q) = %v", c.verb, c.out, err)
		}
	}
}

func TestTypeOfCgo(t *testing.T) {
	t.Parallel()
	pkg := types.NewPackage("example.com/p", "p")
	named := func(name string) types.Type {
		return types.NewNamed(types.NewTypeName(token.NoPos, pkg, name, nil), types.Typ[types.Int32], nil)
	}
	other := types.NewPackage("example.com/q", "q")
	for _, c := range []struct {
		t    types.Type
		here *types.Package
		want string
	}{
		{named("_Ctype_int"), pkg, "C.int"},
		{types.NewPointer(named("_Ctype_char")), pkg, "*C.char"},
		{types.NewSlice(named("_Ctype_struct_point")), pkg, "[]C.struct_point"},
		{named("_Ctype_int"), other, "C.int"},
		{named("Type_Ctype_int"), pkg, "Type_Ctype_int"},
	} {
		if got := Type(c.t, c.here); got != c.want {
			t.Errorf("Type = %q, want %q", got, c.want)
		}
	}
}
