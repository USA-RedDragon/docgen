package marker

import (
	"fmt"
	"regexp"
	"slices"
	"sort"
)

var (
	argRe = regexp.MustCompile(`([a-z]+)=(?:"(` + quotedText + `)"|([^\s>"]+))`)
	// backslashPair matches a backslash and the character after it.
	backslashPair = regexp.MustCompile(`\\[\s\S]`)
)

// unescape returns the text of a quoted value with each \" as a double quote. Any
// other backslash stays, and \\ stays as two characters.
func unescape(s string) string {
	return backslashPair.ReplaceAllStringFunc(s, func(pair string) string {
		if pair == `\"` {
			return `"`
		}
		return pair
	})
}

// A Binder holds the arguments of one marker of a kind. Bind reads each valid
// argument of the kind from a into the fields of the Binder, and checks the rules
// of the kind.
type Binder interface {
	Bind(a *Args)
}

// A Lookup returns a new Binder for a marker of a kind, and nil for an unknown
// kind.
type Lookup func(kind string) Binder

// Args holds the arguments of one open tag. An argument that no Bind reads is
// an error. Calls after the first error do not change it.
type Args struct {
	kind   string
	line   int
	values map[string]string
	taken  map[string]bool
	inline bool
	err    error
}

func newArgs(m Marker) *Args {
	a := &Args{kind: m.Kind, line: m.Line, values: map[string]string{}, taken: map[string]bool{}}
	for _, kv := range argRe.FindAllStringSubmatch(m.raw, -1) {
		a.values[kv[1]] = unescape(kv[2]) + kv[3]
	}
	return a
}

// Required returns the value of key. A missing or empty value is an error.
func (a *Args) Required(key string) string {
	a.taken[key] = true
	v := a.values[key]
	if v == "" && a.err == nil {
		a.err = fmt.Errorf("line %d: docgen:%s: %s= is required", a.line, a.kind, key)
	}
	return v
}

// Optional returns the value of key and whether the tag holds it.
func (a *Args) Optional(key string) (string, bool) {
	a.taken[key] = true
	v, ok := a.values[key]
	return v, ok
}

// NonEmpty records an error for the first of keys that the tag holds with an
// empty value.
func (a *Args) NonEmpty(keys ...string) {
	for _, key := range keys {
		if v, ok := a.values[key]; ok && v == "" && a.err == nil {
			a.err = fmt.Errorf("line %d: docgen:%s: %s= is empty", a.line, a.kind, key)
		}
	}
}

// Len returns the number of different arguments of the tag.
func (a *Args) Len() int {
	return len(a.values)
}

// Inline makes the body of the marker a part of its line, with no line ending
// of its own.
func (a *Args) Inline() {
	a.inline = true
}

// Fail records the error of a rule of the kind. The error is the line and the
// marker kind, then text.
func (a *Args) Fail(text string) {
	if a.err == nil {
		a.err = fmt.Errorf("line %d: docgen:%s %s", a.line, a.kind, text)
	}
}

// check returns the first error, or else the error of the first argument, in
// name order, that no Bind read.
func (a *Args) check() error {
	keys := make([]string, 0, len(a.values))
	for k := range a.values {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	if i := slices.IndexFunc(keys, func(k string) bool { return !a.taken[k] }); i >= 0 {
		a.Fail("has no argument " + keys[i] + "=")
	}
	return a.err
}
