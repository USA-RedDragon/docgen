package excerpt

import (
	"bytes"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"strings"
)

// jsonExcerpt returns the member at the dotted path sym, as "key": value with
// the text of the file. A last segment key[A:B] or key[A] selects items of the
// array at key by the "name" member of each item, or by index.
func jsonExcerpt(src []byte, sym string) (string, error) {
	p, err := parseObjPath(sym)
	if err != nil {
		return "", err
	}
	src = objSource(src)
	check := jsontext.NewDecoder(bytes.NewReader(src))
	if err := check.SkipValue(); err != nil {
		return "", fmt.Errorf("invalid JSON: %w", err)
	}
	if _, err := check.ReadToken(); !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("invalid JSON: %w", err)
	}
	dec := jsontext.NewDecoder(bytes.NewReader(src))
	start := 0
	for i, key := range p.keys {
		path := strings.Join(p.keys[:i], ".")
		if dec.PeekKind() != '{' {
			if path == "" {
				return "", errors.New("the top level of the JSON file is not an object")
			}
			return "", fmt.Errorf("%s in the JSON file is not an object", path)
		}
		if _, err := dec.ReadToken(); err != nil {
			return "", err
		}
		if start, err = jsonFindKey(dec, key); err != nil {
			return "", err
		}
		if start < 0 {
			return "", fmt.Errorf("no key %s in the JSON file", strings.Join(p.keys[:i+1], "."))
		}
	}
	if p.hasItems {
		return jsonItems(dec, src, sym, p)
	}
	if _, err := dec.ReadValue(); err != nil {
		return "", err
	}
	return cutDedent(src, start, int(dec.InputOffset())), nil
}

// jsonFindKey reads the members of an object up to the name key and returns
// the offset of that name, or -1 at the end of the object.
func jsonFindKey(dec *jsontext.Decoder, key string) (int, error) {
	for dec.PeekKind() != '}' {
		name, err := dec.ReadValue()
		if err != nil {
			return 0, err
		}
		s, err := jsontext.AppendUnquote(nil, name)
		if err != nil {
			return 0, err
		}
		if string(s) == key {
			return int(dec.InputOffset()) - len(name), nil
		}
		if err := dec.SkipValue(); err != nil {
			return 0, err
		}
	}
	return -1, nil
}

func jsonItems(dec *jsontext.Decoder, src []byte, sym string, p objPath) (string, error) {
	if dec.PeekKind() != '[' {
		return "", fmt.Errorf("%s in the JSON file is not an array", strings.Join(p.keys, "."))
	}
	if _, err := dec.ReadToken(); err != nil {
		return "", err
	}
	var starts, ends []int
	var names []string
	for dec.PeekKind() != ']' {
		v, err := dec.ReadValue()
		if err != nil {
			return "", err
		}
		end := int(dec.InputOffset())
		starts, ends = append(starts, end-len(v)), append(ends, end)
		names = append(names, jsonItemName(v))
	}
	a, b, err := itemRange(names, sym, p.items, "JSON")
	if err != nil {
		return "", err
	}
	return cutDedent(src, starts[a], ends[b]), nil
}

// jsonItemName returns the "name" member of an object when it is a string, and
// otherwise "".
func jsonItemName(v jsontext.Value) string {
	if v.Kind() != '{' {
		return ""
	}
	var item struct {
		Name jsontext.Value `json:"name"`
	}
	if err := json.Unmarshal(v, &item); err != nil || item.Name.Kind() != '"' {
		return ""
	}
	var s string
	if err := json.Unmarshal(item.Name, &s); err != nil {
		return ""
	}
	return s
}
