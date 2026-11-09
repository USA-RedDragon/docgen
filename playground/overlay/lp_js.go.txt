//go:build js && wasm

package exec

import (
	"os"
	"path/filepath"
	"strings"
)

// jsLookPath finds file in the way of the Unix version, for the process
// bridge of the docgen playground.
func jsLookPath(file string) (string, error) {
	ok := func(p string) bool {
		d, err := os.Stat(p)
		return err == nil && !d.IsDir()
	}
	if strings.Contains(file, "/") {
		if ok(file) {
			return file, nil
		}
		return "", &Error{file, ErrNotFound}
	}
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		if p := filepath.Join(dir, file); filepath.IsAbs(p) && ok(p) {
			return p, nil
		}
	}
	return "", &Error{file, ErrNotFound}
}
