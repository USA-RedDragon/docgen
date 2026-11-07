package gocmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// A Session holds the temporary directory of one docgen run, in the user cache.
// The directory is the GOTMPDIR of each go command of the session, so that the
// build directory of an interrupted go command is where Close removes it, and
// builds are not on a tmpfs. Env and WorkDir make the directory on first use, so
// with no go command there is no change in the cache. Jobs is the -p value of the
// go commands, and 0 adds no -p. Tags holds the build tags of the go commands. The
// zero value is ready to use. The methods are safe for concurrent use, other than
// SetWorkspace, which comes before the first command.
type Session struct {
	Jobs      int
	Tags      []string
	mu        sync.Mutex
	dir       string
	parent    *Session
	workspace *string
	savedOnce sync.Once
	saved     string
	savedErr  error
}

// Nested returns a session with the settings of s. Its temporary directory is
// in the one of s, and it reads the saved GOFLAGS of s.
func (s *Session) Nested() *Session {
	return &Session{Jobs: s.Jobs, Tags: s.Tags, parent: s}
}

// SetWorkspace sets the GOWORK value of the go commands of the session.
func (s *Session) SetWorkspace(value string) {
	s.workspace = &value
}

// Workspace returns the GOWORK value of the go commands of the session: the value
// of SetWorkspace in the session or a parent, or else the value of the process.
func (s *Session) Workspace() string {
	for ; s != nil; s = s.parent {
		if s.workspace != nil {
			return *s.workspace
		}
	}
	return os.Getenv("GOWORK")
}

func (s *Session) tempDir() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.dir != "" {
		return s.dir, nil
	}
	base, err := s.baseDir()
	if err != nil {
		return "", err
	}
	dir, err := os.MkdirTemp(base, "run-")
	if err != nil {
		return "", err
	}
	if s.dir, err = filepath.Abs(dir); err != nil {
		return "", err
	}
	return s.dir, nil
}

// baseDir returns the directory that holds the temporary directory of the
// session: the one of the parent session, or else docgen in the user cache.
func (s *Session) baseDir() (string, error) {
	if s.parent != nil {
		return s.parent.tempDir()
	}
	base, err := os.UserCacheDir()
	if err != nil {
		base = os.TempDir()
	}
	base = filepath.Join(base, "docgen")
	if err := os.MkdirAll(base, 0o750); err != nil {
		return "", err
	}
	return base, nil
}

// TempDir returns the temporary directory of the session.
func (s *Session) TempDir() (string, error) {
	return s.tempDir()
}

// savedGoflags returns the GOFLAGS value of go env -w. The session reads it once.
// A GOFLAGS variable of a go command replaces that value, so the session adds it
// to the variable that it sets.
func (s *Session) savedGoflags(ctx context.Context) (string, error) {
	if s.parent != nil {
		return s.parent.savedGoflags(ctx)
	}
	s.savedOnce.Do(func() {
		dir, err := s.tempDir()
		if err != nil {
			s.savedErr = err
			return
		}
		ctx, cancel := context.WithTimeout(ctx, ToolTimeout)
		defer cancel()
		out, errOut, err := Run(ctx, dir, os.Environ(), "env", "GOFLAGS")
		if err != nil {
			s.savedErr = fmt.Errorf("go env GOFLAGS: %w\n%s", err, errOut)
			return
		}
		s.saved = strings.TrimSpace(out)
	})
	return s.saved, s.savedErr
}

// Env returns the environment of a go command of the session,
// followed by extra.
func (s *Session) Env(ctx context.Context, extra ...string) ([]string, error) {
	dir, err := s.tempDir()
	if err != nil {
		return nil, err
	}
	flags := os.Getenv("GOFLAGS")
	if flags == "" && (s.Jobs > 0 || len(s.Tags) > 0) {
		if flags, err = s.savedGoflags(ctx); err != nil {
			return nil, err
		}
	}
	return flagsEnv(flags, s.Jobs, s.Tags, append([]string{"GOTMPDIR=" + dir, "GOWORK=" + s.Workspace()}, extra...)...), nil
}

// BuildTags returns the build tags of the go commands: those of a -tags in
// GOFLAGS, or else in the saved GOFLAGS, then Tags.
func (s *Session) BuildTags(ctx context.Context) []string {
	flags := os.Getenv("GOFLAGS")
	if flags == "" {
		flags, _ = s.savedGoflags(ctx)
	}
	return append(goflagsTags(flags), s.Tags...)
}

// WorkDir makes a directory in the temporary directory of the session. The
// name of the directory starts with prefix.
func (s *Session) WorkDir(prefix string) (string, error) {
	dir, err := s.tempDir()
	if err != nil {
		return "", err
	}
	return os.MkdirTemp(dir, prefix)
}

// Run runs go with args in dir, as the package function Run does, with the
// environment of Env.
func (s *Session) Run(ctx context.Context, dir string, args ...string) (string, string, error) {
	env, err := s.Env(ctx)
	if err != nil {
		return "", "", err
	}
	return Run(ctx, dir, env, args...)
}

// Close removes the temporary directory of the session and all that it holds.
// The next use makes a new one.
func (s *Session) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	dir := s.dir
	s.dir = ""
	if dir == "" {
		return nil
	}
	return os.RemoveAll(dir)
}
