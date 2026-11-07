// Package goprint writes Go types and constant values as the pages show them. A
// value prints as fmt prints it.
package goprint

import (
	"errors"
	"fmt"
	"go/constant"
	gotypes "go/types"
	"math"
	"math/big"
	"regexp"
	"strings"
	"time"

	"github.com/USA-RedDragon/docgen/internal/types"
)

var cgoName = regexp.MustCompile(`(^|[^\w.])(?:\w+\.)*_Ctype_(\w+)`)

// Type returns t as the source of the package pkg writes it. A type of cgo has the
// name of the source, such as C.int.
func Type(t gotypes.Type, pkg *gotypes.Package) string {
	return cgoName.ReplaceAllString(gotypes.TypeString(t, func(q *gotypes.Package) string {
		if q == pkg {
			return ""
		}
		return q.Name()
	}), "${1}C.$2")
}

// The named values of the print= argument. Any other value but "" is a fmt verb,
// and "" is the same as PrintGo.
const (
	// PrintGo prints a value as fmt.Sprint does.
	PrintGo = "go"
	// PrintRaw prints the underlying value, with no method of the type.
	PrintRaw = "raw"
	// PrintName prints the name of the constant with the value.
	PrintName = "name"
)

var verbRe = regexp.MustCompile(`^%[-+# 0]*[0-9]*(\.[0-9]*)?[a-zA-Z]$`)

// CheckPrint returns an error when value is not "", a named value or one fmt
// verb.
func CheckPrint(value string) error {
	switch value {
	case "", PrintGo, PrintRaw, PrintName:
		return nil
	}
	if !verbRe.MatchString(value) {
		return fmt.Errorf("print=%s is not go, raw, name or a fmt verb", value)
	}
	return nil
}

// CheckOutput returns an error when out, the text of a value with the fmt verb
// verb, is the text of fmt for a verb that is not valid for the type or for a
// panic. "" means fmt.Sprint, which has only the text of a panic.
func CheckOutput(verb, out string) error {
	if verb == "" {
		if strings.HasPrefix(out, "%!v(PANIC=") {
			return fmt.Errorf("the value has no text: %s", out)
		}
		return nil
	}
	if strings.HasPrefix(out, "%!"+verb[len(verb)-1:]+"(") {
		return fmt.Errorf("print=%s is not valid for the value: %s", verb, out)
	}
	return nil
}

// NeedsRun reports whether the text of a constant of type t with the print= value
// in value comes only from a run of the code. A method of the type can print
// anything, and %T of a defined type prints its name. Print handles a
// time.Duration, and PrintRaw and PrintName read no method.
func NeedsRun(t gotypes.Type, value string) bool {
	if value == PrintRaw || value == PrintName || types.IsDuration(t) {
		return false
	}
	if types.HasPrintMethod(t) {
		return true
	}
	return types.IsNamed(t) && strings.HasSuffix(value, "T")
}

// Print returns the text of the constant value v of type t with the print= value
// value, which is not PrintName: fmt.Sprint for "" and PrintGo, the same for the
// underlying type for PrintRaw, or else the fmt verb. An untyped constant has its
// default type. NeedsRun must be false. An integer too large for its type prints
// its exact value, and so does a float that float64 does not hold, with no verb. A
// fmt verb on such a float, or one that a Go integer does not have on such an
// integer, is an error.
func Print(t gotypes.Type, v constant.Value, value string) (string, error) {
	t = gotypes.Default(t)
	verb := value
	switch value {
	case PrintGo:
		verb = ""
	case PrintRaw:
		t, verb = types.Raw(t), ""
	}
	if outOfRange(t, v) {
		if verb == "" {
			return v.String(), nil
		}
		return "", fmt.Errorf("print=%s is not valid for the value %s, which is out of the range of float64", verb, v)
	}
	val, err := goValue(t, v)
	if err != nil {
		return "", err
	}
	if verb == "" {
		return fmt.Sprint(val), nil
	}
	letter := verb[len(verb)-1:]
	if _, huge := val.(*big.Int); huge && !strings.Contains(hugeVerbs, letter) {
		return "", fmt.Errorf("print=%s is not valid for the value %s, which no Go integer type holds", verb, v)
	}
	out := fmt.Sprintf(verb, val)
	if _, text := val.(string); text && strings.Contains(textVerbs, letter) {
		return out, nil
	}
	return out, CheckOutput(verb, out)
}

const (
	// hugeVerbs holds the verbs of a Go integer that big.Int prints the same way.
	hugeVerbs = "vdboOxX"
	// textVerbs holds the verbs of a string, which print its text and no error.
	textVerbs = "vsqxX"
)

// outOfRange reports whether a float or complex constant v of type t is a value
// that its float64 parts do not hold.
func outOfRange(t gotypes.Type, v constant.Value) bool {
	b, ok := types.Basic(t)
	if !ok {
		return false
	}
	switch b.Kind() {
	case gotypes.Float64:
		return float64Lost(constant.ToFloat(v))
	case gotypes.Complex128:
		c := constant.ToComplex(v)
		return float64Lost(constant.Real(c)) || float64Lost(constant.Imag(c))
	default:
		return false
	}
}

// float64Lost reports whether the float64 of v is infinite, or zero for a value
// that is not.
func float64Lost(v constant.Value) bool {
	f, _ := constant.Float64Val(v)
	return math.IsInf(f, 0) || f == 0 && constant.Sign(v) != 0
}

// goValue returns v as a value of the Go type of the kind of t. fmt prints a
// defined type with no method as its underlying type.
func goValue(t gotypes.Type, v constant.Value) (any, error) {
	if types.IsDuration(t) {
		n, _ := constant.Int64Val(constant.ToInt(v))
		return time.Duration(n), nil
	}
	b, ok := types.Basic(t)
	if !ok {
		return nil, fmt.Errorf("type %s has no constant values", t)
	}
	switch info := b.Info(); {
	case info&gotypes.IsBoolean != 0:
		return constant.BoolVal(v), nil
	case info&gotypes.IsString != 0:
		return constant.StringVal(v), nil
	case info&gotypes.IsInteger != 0:
		return intValue(b.Kind(), constant.ToInt(v)), nil
	case info&gotypes.IsFloat != 0:
		return floatValue(b.Kind(), constant.ToFloat(v)), nil
	case info&gotypes.IsComplex != 0:
		return complexValue(b.Kind(), constant.ToComplex(v)), nil
	}
	return nil, errors.New("the constant has no value")
}

func intValue(k gotypes.BasicKind, v constant.Value) any {
	n, exact := constant.Int64Val(v)
	u, _ := constant.Uint64Val(v)
	switch k {
	case gotypes.Int8:
		return int8(n) //nolint:gosec // the type checker holds the value in range
	case gotypes.Int16:
		return int16(n) //nolint:gosec // the type checker holds the value in range
	case gotypes.Int32:
		return int32(n) //nolint:gosec // the type checker holds the value in range
	case gotypes.Int64:
		return n
	case gotypes.Uint:
		return uint(u)
	case gotypes.Uint8:
		return uint8(u) //nolint:gosec // the type checker holds the value in range
	case gotypes.Uint16:
		return uint16(u) //nolint:gosec // the type checker holds the value in range
	case gotypes.Uint32:
		return uint32(u) //nolint:gosec // the type checker holds the value in range
	case gotypes.Uint64:
		return u
	case gotypes.Uintptr:
		return uintptr(u)
	default:
	}
	if !exact {
		i, _ := new(big.Int).SetString(v.ExactString(), 10)
		return i
	}
	return int(n)
}

func floatValue(k gotypes.BasicKind, v constant.Value) any {
	if k == gotypes.Float32 {
		f, _ := constant.Float32Val(v)
		return f
	}
	f, _ := constant.Float64Val(v)
	return f
}

func complexValue(k gotypes.BasicKind, v constant.Value) any {
	re, im := constant.Real(v), constant.Imag(v)
	if k == gotypes.Complex64 {
		r, _ := constant.Float32Val(re)
		i, _ := constant.Float32Val(im)
		return complex(r, i)
	}
	r, _ := constant.Float64Val(re)
	i, _ := constant.Float64Val(im)
	return complex(r, i)
}
