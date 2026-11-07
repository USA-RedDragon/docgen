package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/goccy/go-yaml"
)

// strictYAML decodes a config file. A key that no field has is an error, and so
// is a value of an integer field that is not an integer.
func strictYAML(b []byte, v any) error {
	return yaml.UnmarshalWithOptions(b, v, yaml.Strict(), yaml.CustomUnmarshaler(yamlInt))
}

func yamlInt(dst *int, b []byte) error {
	s := strings.TrimSpace(string(b))
	n, err := strconv.ParseInt(s, 0, strconv.IntSize)
	if err != nil {
		return fmt.Errorf("%s is not an integer", s)
	}
	*dst = int(n)
	return nil
}
