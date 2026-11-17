package cargo

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"slices"
	"strings"
)

// Build builds the target of kind bin or example with the name of the package
// pkg of the workspace at dir, with --locked, and returns the path of the
// executable. The build goes into the target directory of the session. flags
// are more cargo flags, such as the Args of Features.
func (r *Runner) Build(ctx context.Context, dir, pkg, kind, name string, flags ...string) (string, error) {
	args := append([]string{keyBuild, "--locked", "--package", pkg, "--" + kind, name, "--message-format", "json-render-diagnostics"}, flags...)
	out, errOut, err := r.Run(ctx, dir, args...)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return "", err
		}
		return "", LockedError("cargo build --package "+pkg+" --"+kind+" "+name, err, errOut)
	}
	return executableOf(out, pkg, kind, name)
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

// executableOf returns the executable of the target of kind with the name of
// the package pkg in the JSON lines out of a cargo build.
func executableOf(out, pkg, kind, name string) (string, error) {
	for line := range strings.SplitSeq(out, "\n") {
		if !strings.HasPrefix(line, "{") {
			continue
		}
		var m buildMessage
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			continue
		}
		if m.Reason == "compiler-artifact" && m.Executable != "" && m.Target.Name == name && slices.Contains(m.Target.Kind, kind) && packageName(m.PackageID) == pkg {
			return m.Executable, nil
		}
	}
	return "", fmt.Errorf("cargo build made no executable for the %s %s of package %s", kind, name, pkg)
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
