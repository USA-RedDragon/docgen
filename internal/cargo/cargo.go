// Package cargo finds a Cargo workspace and runs cargo for docgen.
package cargo

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/USA-RedDragon/docgen/internal/gocmd"
	"github.com/USA-RedDragon/docgen/internal/proc"
)

// TargetDirName is the name of the directory of the session, in its temporary
// directory, that holds the build output of each cargo command.
const TargetDirName = "cargo-target"

// A Runner runs cargo commands. Each command builds into a target directory
// in the temporary directory of Session, never into a target directory of the
// user, and has the process group, cancellation and wait rules of proc.Run.
type Runner struct {
	// Session holds the temporary directory of the run. It must not be nil.
	Session *gocmd.Session
	// Jobs is the CARGO_BUILD_JOBS value of each command. 0 leaves it to cargo.
	Jobs int
	// Acquire, when it is not nil, is called before each command to take one
	// slot of the limit on concurrent commands. It returns the function that
	// releases the slot, or an error when ctx ends first.
	Acquire func(ctx context.Context) (release func(), err error)
	// Cargo is the cargo command. An empty Cargo is cargo from PATH.
	Cargo string
	// Environ is the environment each command starts from. A nil Environ is
	// the environment of the process.
	Environ []string
}

func (r *Runner) cargo() string {
	if r.Cargo == "" {
		return "cargo"
	}
	return r.Cargo
}

func (r *Runner) environ() []string {
	if r.Environ == nil {
		return os.Environ()
	}
	return r.Environ
}

// ErrNotFound is the error of a missing cargo command.
var ErrNotFound = errors.New("cargo is not on PATH; install a Rust toolchain from https://rustup.rs")

// Env returns the environment of a cargo command of the session, followed by
// extra: the environment of the process with the target directory of the
// session, no terminal color and no progress bar.
func (r *Runner) Env(extra ...string) ([]string, error) {
	tmp, err := r.Session.TempDir()
	if err != nil {
		return nil, err
	}
	base := r.environ()
	env := make([]string, 0, len(base)+4+len(extra))
	for _, e := range base {
		switch k, _, _ := strings.Cut(e, "="); k {
		case "CARGO_TARGET_DIR", "CARGO_BUILD_TARGET_DIR", "CARGO_BUILD_JOBS", "CARGO_TERM_COLOR", "CARGO_TERM_PROGRESS_WHEN":
		default:
			env = append(env, e)
		}
	}
	env = append(env, "CARGO_TARGET_DIR="+filepath.Join(tmp, TargetDirName), "CARGO_TERM_COLOR=never", "CARGO_TERM_PROGRESS_WHEN=never")
	if r.Jobs > 0 {
		env = append(env, "CARGO_BUILD_JOBS="+strconv.Itoa(r.Jobs))
	}
	return append(env, extra...), nil
}

// Run runs cargo with args in dir and returns its standard output and standard
// error. A cargo that is not on PATH gives ErrNotFound. A build command should
// pass --locked, so that it cannot change Cargo.lock; see LockedError.
func (r *Runner) Run(ctx context.Context, dir string, args ...string) (string, string, error) {
	if _, err := exec.LookPath(r.cargo()); err != nil {
		return "", "", ErrNotFound
	}
	env, err := r.Env()
	if err != nil {
		return "", "", err
	}
	if r.Acquire != nil {
		release, err := r.Acquire(ctx)
		if err != nil {
			return "", "", err
		}
		defer release()
	}
	ctx, cancel := context.WithTimeout(ctx, gocmd.ToolTimeout)
	defer cancel()
	return proc.Run(ctx, dir, env, r.cargo(), args...)
}

// LockedError returns err with the standard error of a cargo command, and with
// what to do when the command failed because Cargo.lock is out of date. Use it
// for a command that ran with --locked.
func LockedError(what string, err error, stderr string) error {
	stderr = strings.TrimSpace(stderr)
	if strings.Contains(stderr, "--locked") || strings.Contains(stderr, "lock file") && strings.Contains(stderr, "needs to be updated") {
		return fmt.Errorf("%s: %w\n%s\nCargo.lock is out of date and docgen does not change it; run cargo update --workspace (or cargo generate-lockfile) and commit Cargo.lock", what, err, stderr)
	}
	return fmt.Errorf("%s: %w\n%s", what, err, stderr)
}
