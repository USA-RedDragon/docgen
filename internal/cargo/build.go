package cargo

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"strings"
)

// BuildBin builds the binary bin of the package pkg of the workspace at dir,
// with --locked, and returns the path of the executable. The build goes into
// the target directory of the session.
func (r *Runner) BuildBin(ctx context.Context, dir, pkg, bin string) (string, error) {
	out, errOut, err := r.Run(ctx, dir, "build", "--locked", "--package", pkg, "--bin", bin, "--message-format", "json-render-diagnostics")
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return "", err
		}
		return "", LockedError("cargo build "+pkg+" --bin "+bin, err, errOut)
	}
	return executableOf(out, pkg, bin)
}

type buildMessage struct {
	Reason     string `json:"reason"`
	PackageID  string `json:"package_id"`
	Executable string `json:"executable"`
	Target     struct {
		Name string   `json:"name"`
		Kind []string `json:"kind"`
	} `json:"target"`
}

// executableOf returns the executable of the binary bin of the package pkg in
// the JSON lines out of a cargo build.
func executableOf(out, pkg, bin string) (string, error) {
	for line := range strings.SplitSeq(out, "\n") {
		if !strings.HasPrefix(line, "{") {
			continue
		}
		var m buildMessage
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			continue
		}
		if m.Reason == "compiler-artifact" && m.Executable != "" && m.Target.Name == bin && packageName(m.PackageID) == pkg {
			return m.Executable, nil
		}
	}
	return "", fmt.Errorf("cargo build made no executable for %s in package %s", bin, pkg)
}

// packageName returns the name of a package from its package ID, in the
// forms name 1.0.0 (source), path+file:///dir/name#1.0.0 and
// path+file:///dir#name@1.0.0.
func packageName(id string) string {
	if i := strings.LastIndex(id, "#"); i >= 0 {
		rest := id[i+1:]
		if name, _, ok := strings.Cut(rest, "@"); ok {
			return name
		}
		path := id[:i]
		return path[strings.LastIndex(path, "/")+1:]
	}
	name, _, _ := strings.Cut(id, " ")
	return name
}
