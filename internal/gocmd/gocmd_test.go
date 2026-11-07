package gocmd

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"
)

func envValues(env []string, key string) []string {
	var out []string
	for _, e := range env {
		if k, v, ok := strings.Cut(e, "="); ok && k == key {
			out = append(out, v)
		}
	}
	return out
}

// TestGoEnv sets variables, so it does not run in parallel.
//
//nolint:paralleltest // t.Setenv
func TestGoEnv(t *testing.T) {
	t.Setenv("GOPROXY", "https://proxy.example.com")
	t.Setenv("GOFLAGS", "-mod=mod")
	t.Setenv("GOWORK", "/somewhere/go.work")
	t.Setenv("GOTMPDIR", "/old")
	env := goEnv(2, []string{"a", "b"}, "DOCGEN_STATES_OUT=/x")
	for k, want := range map[string][]string{
		"GOPROXY":           {"https://proxy.example.com"},
		"GOWORK":            {"/somewhere/go.work"},
		"GOFLAGS":           {"-mod=mod -p=2 -tags=a,b"},
		"GOTMPDIR":          nil,
		"DOCGEN_STATES_OUT": {"/x"},
	} {
		if got := envValues(env, k); !slices.Equal(got, want) {
			t.Errorf("%s = %q, want %q", k, got, want)
		}
	}
}

//nolint:paralleltest // t.Setenv
func TestGoEnvLeavesParallelismToGo(t *testing.T) {
	t.Setenv("GOFLAGS", "-p=3")
	if got, want := envValues(goEnv(0, nil), "GOFLAGS"), []string{"-p=3"}; !slices.Equal(got, want) {
		t.Errorf("GOFLAGS = %q, want %q", got, want)
	}
}

func TestGoEnvKeepsOthers(t *testing.T) {
	t.Parallel()
	env := goEnv(2, nil)
	for _, e := range os.Environ() {
		k, _, _ := strings.Cut(e, "=")
		switch k {
		case "GOFLAGS", "GOTMPDIR":
		default:
			if !slices.Contains(env, e) {
				t.Errorf("%q is not in the offline environment", e)
			}
		}
	}
}

func TestRunOutput(t *testing.T) {
	t.Parallel()
	out, errOut, err := Run(t.Context(), t.TempDir(), goEnv(2, nil), "env", "GOFLAGS", "GOMAXPROCS")
	if err != nil {
		t.Fatalf("go env: %v\n%s", err, errOut)
	}
	if want := "-p=2\n"; !strings.HasPrefix(out, want) {
		t.Errorf("output %q does not start with %q", out, want)
	}
}

func TestRunFailure(t *testing.T) {
	t.Parallel()
	out, errOut, err := Run(t.Context(), t.TempDir(), goEnv(2, nil), "no-such-subcommand")
	var ee *exec.ExitError
	if !errors.As(err, &ee) {
		t.Fatalf("err = %v, want an exit error", err)
	}
	if out != "" || !strings.Contains(errOut, "no-such-subcommand") {
		t.Errorf("stdout %q, stderr %q", out, errOut)
	}
}

func TestRunCanceled(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, _, err := Run(ctx, t.TempDir(), goEnv(2, nil), "version"); !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
}

//nolint:paralleltest // t.Setenv
func TestGoEnvMergesTags(t *testing.T) {
	for goflags, want := range map[string]string{
		"":                             "-tags=c",
		"-tags=a":                      "-tags=a -tags=a,c",
		"-tags=a,b -p=2":               "-tags=a,b -p=2 -tags=a,b,c",
		"--tags=a":                     "--tags=a -tags=a,c",
		"-tags=x -tags=a":              "-tags=x -tags=a -tags=a,c",
		`"-tags=a b"`:                  `"-tags=a b" -tags=a,b,c`,
		`-ldflags=-tags=z`:             "-ldflags=-tags=z -tags=c",
		`'-gcflags=all=-N -l' -tags=a`: `'-gcflags=all=-N -l' -tags=a -tags=a,c`,
	} {
		t.Setenv("GOFLAGS", goflags)
		if got := oneValue(t, goEnv(0, []string{"c"}), "GOFLAGS"); got != want {
			t.Errorf("GOFLAGS %q: %q, want %q", goflags, got, want)
		}
	}
	t.Setenv("GOFLAGS", "-tags=a")
	if got := oneValue(t, goEnv(0, nil), "GOFLAGS"); got != "-tags=a" {
		t.Errorf("no tags: %q", got)
	}
	if got := (&Session{Tags: []string{"c"}}).BuildTags(t.Context()); !slices.Equal(got, []string{"a", "c"}) {
		t.Errorf("BuildTags = %q", got)
	}
}
