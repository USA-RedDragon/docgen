package links

import (
	"strings"
	"unicode"
)

// fullFolds holds each rune with a Unicode case folding of several runes, from
// CaseFolding.txt.
//
//nolint:gochecknoglobals // a fixed table
var fullFolds = strings.NewReplacer(
	"\u00df", "\u0073\u0073",
	"\u0130", "\u0069\u0307",
	"\u0149", "\u02bc\u006e",
	"\u01f0", "\u006a\u030c",
	"\u0390", "\u03b9\u0308\u0301",
	"\u03b0", "\u03c5\u0308\u0301",
	"\u0587", "\u0565\u0582",
	"\u1e96", "\u0068\u0331",
	"\u1e97", "\u0074\u0308",
	"\u1e98", "\u0077\u030a",
	"\u1e99", "\u0079\u030a",
	"\u1e9a", "\u0061\u02be",
	"\u1e9e", "\u0073\u0073",
	"\u1f50", "\u03c5\u0313",
	"\u1f52", "\u03c5\u0313\u0300",
	"\u1f54", "\u03c5\u0313\u0301",
	"\u1f56", "\u03c5\u0313\u0342",
	"\u1f80", "\u1f00\u03b9",
	"\u1f81", "\u1f01\u03b9",
	"\u1f82", "\u1f02\u03b9",
	"\u1f83", "\u1f03\u03b9",
	"\u1f84", "\u1f04\u03b9",
	"\u1f85", "\u1f05\u03b9",
	"\u1f86", "\u1f06\u03b9",
	"\u1f87", "\u1f07\u03b9",
	"\u1f88", "\u1f00\u03b9",
	"\u1f89", "\u1f01\u03b9",
	"\u1f8a", "\u1f02\u03b9",
	"\u1f8b", "\u1f03\u03b9",
	"\u1f8c", "\u1f04\u03b9",
	"\u1f8d", "\u1f05\u03b9",
	"\u1f8e", "\u1f06\u03b9",
	"\u1f8f", "\u1f07\u03b9",
	"\u1f90", "\u1f20\u03b9",
	"\u1f91", "\u1f21\u03b9",
	"\u1f92", "\u1f22\u03b9",
	"\u1f93", "\u1f23\u03b9",
	"\u1f94", "\u1f24\u03b9",
	"\u1f95", "\u1f25\u03b9",
	"\u1f96", "\u1f26\u03b9",
	"\u1f97", "\u1f27\u03b9",
	"\u1f98", "\u1f20\u03b9",
	"\u1f99", "\u1f21\u03b9",
	"\u1f9a", "\u1f22\u03b9",
	"\u1f9b", "\u1f23\u03b9",
	"\u1f9c", "\u1f24\u03b9",
	"\u1f9d", "\u1f25\u03b9",
	"\u1f9e", "\u1f26\u03b9",
	"\u1f9f", "\u1f27\u03b9",
	"\u1fa0", "\u1f60\u03b9",
	"\u1fa1", "\u1f61\u03b9",
	"\u1fa2", "\u1f62\u03b9",
	"\u1fa3", "\u1f63\u03b9",
	"\u1fa4", "\u1f64\u03b9",
	"\u1fa5", "\u1f65\u03b9",
	"\u1fa6", "\u1f66\u03b9",
	"\u1fa7", "\u1f67\u03b9",
	"\u1fa8", "\u1f60\u03b9",
	"\u1fa9", "\u1f61\u03b9",
	"\u1faa", "\u1f62\u03b9",
	"\u1fab", "\u1f63\u03b9",
	"\u1fac", "\u1f64\u03b9",
	"\u1fad", "\u1f65\u03b9",
	"\u1fae", "\u1f66\u03b9",
	"\u1faf", "\u1f67\u03b9",
	"\u1fb2", "\u1f70\u03b9",
	"\u1fb3", "\u03b1\u03b9",
	"\u1fb4", "\u03ac\u03b9",
	"\u1fb6", "\u03b1\u0342",
	"\u1fb7", "\u03b1\u0342\u03b9",
	"\u1fbc", "\u03b1\u03b9",
	"\u1fc2", "\u1f74\u03b9",
	"\u1fc3", "\u03b7\u03b9",
	"\u1fc4", "\u03ae\u03b9",
	"\u1fc6", "\u03b7\u0342",
	"\u1fc7", "\u03b7\u0342\u03b9",
	"\u1fcc", "\u03b7\u03b9",
	"\u1fd2", "\u03b9\u0308\u0300",
	"\u1fd3", "\u03b9\u0308\u0301",
	"\u1fd6", "\u03b9\u0342",
	"\u1fd7", "\u03b9\u0308\u0342",
	"\u1fe2", "\u03c5\u0308\u0300",
	"\u1fe3", "\u03c5\u0308\u0301",
	"\u1fe4", "\u03c1\u0313",
	"\u1fe6", "\u03c5\u0342",
	"\u1fe7", "\u03c5\u0308\u0342",
	"\u1ff2", "\u1f7c\u03b9",
	"\u1ff3", "\u03c9\u03b9",
	"\u1ff4", "\u03ce\u03b9",
	"\u1ff6", "\u03c9\u0342",
	"\u1ff7", "\u03c9\u0342\u03b9",
	"\u1ffc", "\u03c9\u03b9",
	"\ufb00", "\u0066\u0066",
	"\ufb01", "\u0066\u0069",
	"\ufb02", "\u0066\u006c",
	"\ufb03", "\u0066\u0066\u0069",
	"\ufb04", "\u0066\u0066\u006c",
	"\ufb05", "\u0073\u0074",
	"\ufb06", "\u0073\u0074",
	"\ufb13", "\u0574\u0576",
	"\ufb14", "\u0574\u0565",
	"\ufb15", "\u0574\u056b",
	"\ufb16", "\u057e\u0576",
	"\ufb17", "\u0574\u056d",
)

// fold returns s in a form that is the same for two strings with one Unicode
// case folding.
func fold(s string) string {
	return strings.Map(simpleFold, fullFolds.Replace(s))
}

// simpleFold returns the lowest rune of the simple case folding orbit of r.
func simpleFold(r rune) rune {
	low := r
	for f := unicode.SimpleFold(r); f != r; f = unicode.SimpleFold(f) {
		low = min(low, f)
	}
	return low
}
