package gocmd

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func oneValue(t *testing.T, env []string, key string) string {
	t.Helper()
	got := envValues(env, key)
	if len(got) != 1 {
		t.Fatalf("%s = %q, want one value", key, got)
	}
	return got[0]
}

// TestSessionTempDir sets variables, so it does not run in parallel.
//
//nolint:paralleltest // t.Setenv
func TestSessionTempDir(t *testing.T) {
	cache := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cache)
	t.Setenv("GOTMPDIR", "/somewhere/else")
	base := filepath.Join(cache, "docgen")
	s := &Session{}
	if err := s.Close(); err != nil {
		t.Errorf("close of an unused session: %v", err)
	}
	if _, err := os.Stat(base); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("an unused session touched the cache: %v", err)
	}
	env, err := s.Env(t.Context(), "DOCGEN_STATES_OUT=/x")
	if err != nil {
		t.Fatal(err)
	}
	dir := oneValue(t, env, "GOTMPDIR")
	if filepath.Dir(dir) != base || !strings.HasPrefix(filepath.Base(dir), "run-") {
		t.Errorf("GOTMPDIR = %s, want a run directory in %s", dir, base)
	}
	if got := oneValue(t, env, "DOCGEN_STATES_OUT"); got != "/x" {
		t.Errorf("DOCGEN_STATES_OUT = %s", got)
	}
	again, err := s.Env(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if got := oneValue(t, again, "GOTMPDIR"); got != dir {
		t.Errorf("second GOTMPDIR = %s, want %s", got, dir)
	}
	work, err := s.WorkDir("overlay-")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(work) != dir || !strings.HasPrefix(filepath.Base(work), "overlay-") {
		t.Errorf("WorkDir = %s, want a directory in %s", work, dir)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if es, err := os.ReadDir(base); err != nil || len(es) != 0 {
		t.Errorf("after Close: %v, %v", es, err)
	}
	env, err = s.Env(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if got := oneValue(t, env, "GOTMPDIR"); got == dir {
		t.Errorf("GOTMPDIR after Close = %s, want a new directory", got)
	}
}

// TestSessionRelativeCache sets variables, so it does not run in parallel. With
// a relative XDG_CACHE_HOME there is no user cache, and a relative TMPDIR must
// not reach a go command that runs in another directory.
//
//nolint:paralleltest // t.Setenv, t.Chdir
func TestSessionRelativeCache(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("XDG_CACHE_HOME", "relative")
	t.Setenv("TMPDIR", ".")
	s := &Session{}
	defer s.Close()
	env, err := s.Env(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if dir := oneValue(t, env, "GOTMPDIR"); !filepath.IsAbs(dir) {
		t.Errorf("GOTMPDIR = %s, want an absolute path", dir)
	}
}

// TestSessionNoCache sets variables, so it does not run in parallel.
//
//nolint:paralleltest // t.Setenv
func TestSessionNoCache(t *testing.T) {
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CACHE_HOME", file)
	s := &Session{}
	if _, err := s.Env(t.Context()); err == nil {
		t.Error("Env with a cache that is a file passed")
	}
	if _, err := s.WorkDir("x-"); err == nil {
		t.Error("WorkDir with a cache that is a file passed")
	}
	if _, _, err := s.Run(t.Context(), ".", "version"); err == nil {
		t.Error("Run with a cache that is a file passed")
	}
}

// TestSessionRun sets variables, so it does not run in parallel.
//
//nolint:paralleltest // t.Setenv
func TestSessionRun(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	s := &Session{Jobs: 2}
	defer s.Close()
	env, err := s.Env(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	out, errOut, err := s.Run(t.Context(), t.TempDir(), "env", "GOTMPDIR", "GOFLAGS")
	if err != nil {
		t.Fatalf("go env: %v\n%s", err, errOut)
	}
	want := []string{oneValue(t, env, "GOTMPDIR"), "-p=2"}
	if got := strings.Fields(out); !slices.Equal(got, want) {
		t.Errorf("go env = %q, want %q", got, want)
	}
}

// savedFlags points GOENV at a file that holds the GOFLAGS value text, and
// removes GOFLAGS from the environment. It returns the path of the file.
func savedFlags(t *testing.T, text string) string {
	t.Helper()
	file := filepath.Join(t.TempDir(), "env")
	if err := os.WriteFile(file, []byte("GOFLAGS="+text+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOENV", file)
	t.Setenv("GOFLAGS", "")
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	return file
}

// TestSessionSavedGoflags sets variables, so it does not run in parallel.
//
//nolint:paralleltest // t.Setenv
func TestSessionSavedGoflags(t *testing.T) {
	for name, c := range map[string]struct {
		s    *Session
		want string
	}{
		"jobs":          {&Session{Jobs: 2}, "-mod=mod -tags=a -p=2"},
		"tags":          {&Session{Tags: []string{"b"}}, "-mod=mod -tags=a -tags=a,b"},
		"jobs and tags": {&Session{Jobs: 2, Tags: []string{"b"}}, "-mod=mod -tags=a -p=2 -tags=a,b"},
		"no settings":   {&Session{}, ""},
	} {
		t.Run(name, func(t *testing.T) {
			savedFlags(t, "-mod=mod -tags=a")
			defer c.s.Close()
			env, err := c.s.Env(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			if got := oneValue(t, env, "GOFLAGS"); got != c.want {
				t.Errorf("GOFLAGS = %q, want %q", got, c.want)
			}
		})
	}
}

// TestSessionSavedGoflagsReachGo sets variables, so it does not run in parallel.
//
//nolint:paralleltest // t.Setenv
func TestSessionSavedGoflagsReachGo(t *testing.T) {
	savedFlags(t, "-mod=mod")
	s := &Session{Jobs: 2}
	defer s.Close()
	out, errOut, err := s.Run(t.Context(), t.TempDir(), "env", "GOFLAGS")
	if err != nil {
		t.Fatalf("go env: %v\n%s", err, errOut)
	}
	if got := strings.TrimSpace(out); got != "-mod=mod -p=2" {
		t.Errorf("go env GOFLAGS = %q", got)
	}
}

// TestSessionVariableReplacesSavedGoflags sets variables, so it does not run in
// parallel.
//
//nolint:paralleltest // t.Setenv
func TestSessionVariableReplacesSavedGoflags(t *testing.T) {
	savedFlags(t, "-mod=mod")
	t.Setenv("GOFLAGS", "-mod=readonly")
	s := &Session{Jobs: 2}
	defer s.Close()
	env, err := s.Env(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if got := oneValue(t, env, "GOFLAGS"); got != "-mod=readonly -p=2" {
		t.Errorf("GOFLAGS = %q", got)
	}
}

// TestSessionReadsSavedGoflagsOnce sets variables, so it does not run in
// parallel. A change of the file after the first read has no effect, also in a
// nested session.
//
//nolint:paralleltest // t.Setenv
func TestSessionReadsSavedGoflagsOnce(t *testing.T) {
	file := savedFlags(t, "-mod=mod")
	s := &Session{Jobs: 2}
	defer s.Close()
	if _, err := s.Env(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte("GOFLAGS=-mod=vendor\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	inner := s.Nested()
	defer inner.Close()
	for _, session := range []*Session{s, inner} {
		env, err := session.Env(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		if got := oneValue(t, env, "GOFLAGS"); got != "-mod=mod -p=2" {
			t.Errorf("GOFLAGS = %q", got)
		}
	}
}

// TestSessionBuildTagsOfSavedGoflags sets variables, so it does not run in
// parallel.
//
//nolint:paralleltest // t.Setenv
func TestSessionBuildTagsOfSavedGoflags(t *testing.T) {
	savedFlags(t, "-tags=a")
	s := &Session{Tags: []string{"b"}}
	defer s.Close()
	if got := s.BuildTags(t.Context()); !slices.Equal(got, []string{"a", "b"}) {
		t.Errorf("BuildTags = %q", got)
	}
}

// TestNestedSession sets variables, so it does not run in parallel.
//
//nolint:paralleltest // t.Setenv
func TestNestedSession(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	s := &Session{Jobs: 3, Tags: []string{"a"}}
	defer s.Close()
	inner := s.Nested()
	if inner.Jobs != 3 || !slices.Equal(inner.Tags, []string{"a"}) {
		t.Errorf("nested settings: %d, %q", inner.Jobs, inner.Tags)
	}
	outer, err := s.TempDir()
	if err != nil {
		t.Fatal(err)
	}
	dir, err := inner.TempDir()
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(dir) != outer {
		t.Errorf("nested directory %s, want a directory in %s", dir, outer)
	}
	if err := inner.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("after the close of the nested session: %v", err)
	}
	if _, err := os.Stat(outer); err != nil {
		t.Errorf("the close of the nested session removed the outer directory: %v", err)
	}
}
