package cargo

import (
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// A Manifest is a parsed TOML file: Cargo.toml, rust-toolchain.toml or any
// other. Name is how errors call the file.
type Manifest struct {
	Name string
	Doc  map[string]any
}

// ReadManifest parses the TOML file at path. A missing file and invalid TOML
// are errors that name the file as name.
func ReadManifest(path, name string) (*Manifest, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("no %s", name)
	}
	if err != nil {
		return nil, err
	}
	doc := map[string]any{}
	if err := toml.Unmarshal(b, &doc); err != nil {
		var de *toml.DecodeError
		if errors.As(err, &de) {
			row, col := de.Position()
			return nil, fmt.Errorf("%s line %d column %d: %s", name, row, col, de.Error())
		}
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	return &Manifest{Name: name, Doc: doc}, nil
}

// Field returns the value at the dotted key path field of m as text. Cargo
// workspace inheritance applies: a package field, a dependency or the lints
// table of m that is set to workspace = true takes its value from the
// workspace manifest, which ws loads when a value inherits. A nil ws means
// there is no workspace manifest.
//
// A string is itself, a number or a boolean is its TOML text, and an array of
// such values is its items joined with a comma and a space. A table, an array
// that holds a table and a date are errors.
func (m *Manifest) Field(field string, ws func() (*Manifest, error)) (string, error) {
	var cur any = m.Doc
	parts := strings.Split(field, ".")
	for i, key := range parts {
		t, ok := cur.(map[string]any)
		if !ok {
			return "", fmt.Errorf("%s: %s is %s, so it has no key %s", m.Name, strings.Join(parts[:i], "."), describe(cur), key)
		}
		v, ok := t[key]
		if !ok {
			return "", fmt.Errorf("%s has no %s%s", m.Name, strings.Join(parts[:i+1], "."), keysHint(parts[:i], t))
		}
		if tt, ok := v.(map[string]any); ok && tt[keyWorkspace] == true {
			var err error
			if v, err = m.inherit(parts[:i+1], tt, ws); err != nil {
				return "", err
			}
		}
		cur = v
	}
	text, err := scalarText(cur)
	if err != nil {
		return "", fmt.Errorf("%s: %s %w", m.Name, field, err)
	}
	return text, nil
}

func keysHint(parent []string, t map[string]any) string {
	if len(t) == 0 {
		return ""
	}
	at := "the keys are: "
	if len(parent) > 0 {
		at = "the keys of " + strings.Join(parent, ".") + " are: "
	}
	return "; " + at + strings.Join(slices.Sorted(maps.Keys(t)), ", ")
}

func describe(v any) string {
	switch v.(type) {
	case string:
		return "a string"
	case bool:
		return "a boolean"
	case int64, float64:
		return "a number"
	case []any:
		return "an array"
	}
	return "a value"
}

const (
	keyWorkspace    = "workspace"
	keyDependencies = "dependencies"
)

func isDependencyTable(key string) bool {
	switch key {
	case keyDependencies, "dev-dependencies", "build-dependencies":
		return true
	}
	return false
}

// inherit returns the value of the table t at path, which is set to workspace
// = true, from ws.
func (m *Manifest) inherit(path []string, t map[string]any, ws func() (*Manifest, error)) (any, error) {
	var from []string
	switch {
	case path[0] == "package" && len(path) == 2:
		from = []string{keyWorkspace, "package", path[1]}
	case isDependencyTable(path[0]) && len(path) == 2:
		from = []string{keyWorkspace, keyDependencies, path[1]}
	case path[0] == "lints" && len(path) == 1:
		from = []string{keyWorkspace, "lints"}
	default:
		return t, nil
	}
	here := strings.Join(path, ".")
	if ws == nil {
		return nil, fmt.Errorf("%s: %s is set to workspace = true, but there is no workspace manifest", m.Name, here)
	}
	wm, err := ws()
	if err != nil {
		return nil, err
	}
	var v any = wm.Doc
	for _, k := range from {
		tt, _ := v.(map[string]any)
		next, ok := tt[k]
		if !ok {
			return nil, fmt.Errorf("%s: %s is set to workspace = true, but %s has no %s", m.Name, here, wm.Name, strings.Join(from, "."))
		}
		v = next
	}
	if from[1] != keyDependencies {
		return v, nil
	}
	dep, ok := v.(map[string]any)
	if !ok {
		dep = map[string]any{"version": v}
	}
	return mergeDependency(dep, t), nil
}

// mergeDependency returns the workspace dependency base with the keys of the
// dependency local that sets workspace = true: a local key replaces the key of
// base, except that features add to it.
func mergeDependency(base, local map[string]any) map[string]any {
	out := maps.Clone(base)
	for k, v := range local {
		switch k {
		case keyWorkspace:
		case "features":
			a, _ := out[k].([]any)
			b, _ := v.([]any)
			out[k] = append(slices.Clone(a), b...)
		default:
			out[k] = v
		}
	}
	return out
}

func scalarText(v any) (string, error) {
	switch v := v.(type) {
	case string:
		return v, nil
	case bool:
		return strconv.FormatBool(v), nil
	case int64:
		return strconv.FormatInt(v, 10), nil
	case float64:
		return strconv.FormatFloat(v, 'g', -1, 64), nil
	case []any:
		items := make([]string, len(v))
		for i, item := range v {
			var err error
			if _, nested := item.([]any); nested {
				err = errors.New("nested")
			} else {
				items[i], err = scalarText(item)
			}
			if err != nil {
				return "", errors.New("is an array that holds a table, an array or a date, which a marker cannot show")
			}
		}
		return strings.Join(items, ", "), nil
	case map[string]any:
		return "", fmt.Errorf("is a table, not a value; select a key in it%s", keysHint(nil, v))
	}
	return "", errors.New("is a date or time, which a marker cannot show")
}
